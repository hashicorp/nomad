package taskrunner

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
)

const (
	// the name of this hook, used in logs
	sidsHookName = "consul_sids"

	// sidsBackoffBaseline is the baseline time for exponential backoff when
	// attempting to retrieve a Consul SI token
	sidsBackoffBaseline = 5 * time.Second

	// sidsBackoffLimit is the limit of the exponential backoff when attempting
	// to retrieve a Consul SI token
	sidsBackoffLimit = 3 * time.Minute

	// sidsTokenFile is the name of the file holding the Consul SI token inside
	// the task's secret directory
	sidsTokenFile = "si_token"

	// sidsTokenFilePerms is the level of file permissions granted on the file
	// in the secrets directory for the task
	sidsTokenFilePerms = 0440
)

type sidsHookConfig struct {
	alloc      *structs.Allocation
	task       *structs.Task
	sidsClient consul.ServiceIdentityAPI
	lifecycle  ti.TaskLifecycle
	logger     hclog.Logger
}

// Service Identities hook for managing SI tokens of connect enabled tasks.
type sidsHook struct {
	// alloc is the allocation
	alloc *structs.Allocation

	// taskName is the name of the task
	taskName string

	// sidsClient is the Consul client [proxy] for requesting SI tokens
	sidsClient consul.ServiceIdentityAPI

	// lifecycle is used to signal, restart, and kill a task
	lifecycle ti.TaskLifecycle

	// logger is used to log
	logger hclog.Logger

	// lock variables that can be manipulated after hook creation
	lock sync.Mutex
	// firstRun keeps track of whether the hook is being called for the first
	// time (for this task) during the lifespan of the Nomad Client process.
	firstRun bool
}

func newSIDSHook(c sidsHookConfig) *sidsHook {
	return &sidsHook{
		alloc:      c.alloc,
		taskName:   c.task.Name,
		sidsClient: c.sidsClient,
		lifecycle:  c.lifecycle,
		logger:     c.logger.Named(sidsHookName),
		firstRun:   true,
	}
}

func (h *sidsHook) Name() string {
	return sidsHookName
}

func (h *sidsHook) Prestart(
	ctx context.Context,
	req *interfaces.TaskPrestartRequest,
	_ *interfaces.TaskPrestartResponse) error {

	h.lock.Lock()
	defer h.lock.Unlock()

	// do nothing if we have already done things
	if h.earlyExit() {
		return nil
	}

	// optimistically try to recover token from disk
	token, err := h.recoverToken(req.TaskDir.SecretsDir)
	if err != nil {
		return err
	}

	// need to ask for a new SI token & persist it to disk
	if token == "" {
		if token, err = h.deriveSIToken(ctx); err != nil {
			return err
		}
		if err := h.writeToken(req.TaskDir.SecretsDir, token); err != nil {
			return err
		}
	}

	return nil
}

// earlyExit returns true if the Prestart hook has already been executed during
// the instantiation of this task runner.
//
// assumes h is locked
func (h *sidsHook) earlyExit() bool {
	if h.firstRun {
		h.firstRun = false
		return false
	}
	return true
}

// writeToken writes token into the secrets directory for the task.
func (h *sidsHook) writeToken(dir string, token string) error {
	tokenPath := filepath.Join(dir, sidsTokenFile)
	if err := ioutil.WriteFile(tokenPath, []byte(token), sidsTokenFilePerms); err != nil {
		return errors.Wrap(err, "failed to write SI token")
	}
	return nil
}

// recoverToken returns the token saved to disk in the secrets directory for the
// task if it exists, or the empty string if the file does not exist. an error
// is returned only for some other (e.g. disk IO) error.
func (h *sidsHook) recoverToken(dir string) (string, error) {
	tokenPath := filepath.Join(dir, sidsTokenFile)
	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", errors.Wrap(err, "failed to recover SI token")
		}
		h.logger.Trace("no pre-existing SI token to recover", "task", h.taskName)
		return "", nil // token file does not exist yet
	}
	h.logger.Trace("recovered pre-existing SI token", "task", h.taskName)
	return string(token), nil
}

// deriveSIToken spawns and waits on a goroutine which will make attempts to
// derive an SI token until a token is successfully created, or ctx is signaled
// done.
func (h *sidsHook) deriveSIToken(ctx context.Context) (string, error) {
	tokenCh := make(chan string)

	// keep trying to get the token in the background
	go h.tryDerive(ctx, tokenCh)

	// wait until we get a token, or we get a signal to quit
	for {
		select {
		case token := <-tokenCh:
			return token, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (h *sidsHook) kill(ctx context.Context, err error) {
	_ = h.lifecycle.Kill(
		ctx,
		structs.NewTaskEvent(structs.TaskKilling).
			SetFailsTask().
			SetDisplayMessage(err.Error()),
	)
}

// tryDerive loops forever until a token is created, or ctx is done.
func (h *sidsHook) tryDerive(ctx context.Context, ch chan<- string) {
	for attempt := 0; backoff(ctx, attempt); attempt++ {

		tokens, err := h.sidsClient.DeriveSITokens(h.alloc, []string{h.taskName})

		switch {

		case err == nil:
			// nothing broke and we can return the token for the task
			ch <- tokens[h.taskName]
			return

		case structs.IsServerSide(err):
			// the error is known to be a server problem, just die
			h.logger.Error("failed to derive SI token", "error", err, "server_side", true)
			h.kill(ctx, errors.Wrap(err, "consul: failed to derive SI token"))

		case !structs.IsRecoverable(err):
			// the error is known not to be recoverable, just die
			h.logger.Error("failed to derive SI token", "error", err, "recoverable", false)
			h.kill(ctx, errors.Wrap(err, "consul: failed to derive SI token"))

		default:
			// the error is marked recoverable, retry after some backoff
			h.logger.Error("failed to derive SI token", "error", err, "recoverable", true)
		}
	}
}

func backoff(ctx context.Context, i int) bool {
	next := computeBackoff(i)
	select {
	case <-ctx.Done():
		return false
	case <-time.After(next):
		return true
	}
}

func computeBackoff(attempt int) time.Duration {
	switch attempt {
	case 0:
		return 0
	case 1:
		// go fast on first retry, because a unit test should be fast
		return 100 * time.Millisecond
	default:
		wait := time.Duration(attempt) * sidsBackoffBaseline
		if wait > sidsBackoffLimit {
			wait = sidsBackoffLimit
		}
		return wait
	}
}
