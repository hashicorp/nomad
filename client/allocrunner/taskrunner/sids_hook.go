// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// the name of this hook, used in logs
	sidsHookName = "consul_si_token"

	// sidsBackoffBaseline is the baseline time for exponential backoff when
	// attempting to retrieve a Consul SI token
	sidsBackoffBaseline = 5 * time.Second

	// sidsBackoffLimit is the limit of the exponential backoff when attempting
	// to retrieve a Consul SI token
	sidsBackoffLimit = 3 * time.Minute

	// sidsDerivationTimeout limits the amount of time we may spend trying to
	// derive a SI token. If the hook does not get a token within this amount of
	// time, the result is a failure.
	sidsDerivationTimeout = 5 * time.Minute

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
	task *structs.Task

	// sidsClient is the Consul client [proxy] for requesting SI tokens
	sidsClient consul.ServiceIdentityAPI

	// lifecycle is used to signal, restart, and kill a task
	lifecycle ti.TaskLifecycle

	// derivationTimeout is the amount of time we may wait for Consul to successfully
	// provide a SI token. Making this configurable for testing, otherwise
	// default to sidsDerivationTimeout
	derivationTimeout time.Duration

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
		alloc:             c.alloc,
		task:              c.task,
		sidsClient:        c.sidsClient,
		lifecycle:         c.lifecycle,
		derivationTimeout: sidsDerivationTimeout,
		logger:            c.logger.Named(sidsHookName),
		firstRun:          true,
	}
}

func (h *sidsHook) Name() string {
	return sidsHookName
}

func (h *sidsHook) Prestart(
	ctx context.Context,
	req *interfaces.TaskPrestartRequest,
	resp *interfaces.TaskPrestartResponse) error {

	h.lock.Lock()
	defer h.lock.Unlock()

	// do nothing if we have already done things
	if h.earlyExit() {
		resp.Done = true
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

	h.logger.Info("derived SI token", "task", h.task.Name, "si_task", h.task.Kind.Value())

	resp.Done = true
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
	if err := os.WriteFile(tokenPath, []byte(token), sidsTokenFilePerms); err != nil {
		return fmt.Errorf("failed to write SI token: %w", err)
	}
	return nil
}

// recoverToken returns the token saved to disk in the secrets directory for the
// task if it exists, or the empty string if the file does not exist. an error
// is returned only for some other (e.g. disk IO) error.
func (h *sidsHook) recoverToken(dir string) (string, error) {
	tokenPath := filepath.Join(dir, sidsTokenFile)
	token, err := os.ReadFile(tokenPath)
	if err != nil {
		if !os.IsNotExist(err) {
			h.logger.Error("failed to recover SI token", "error", err)
			return "", fmt.Errorf("failed to recover SI token: %w", err)
		}
		h.logger.Trace("no pre-existing SI token to recover", "task", h.task.Name)
		return "", nil // token file does not exist yet
	}
	h.logger.Trace("recovered pre-existing SI token", "task", h.task.Name)
	return string(token), nil
}

// siDerivationResult is used to pass along the result of attempting to derive
// an SI token between the goroutine doing the derivation and its caller
type siDerivationResult struct {
	token string
	err   error
}

// deriveSIToken spawns and waits on a goroutine which will make attempts to
// derive an SI token until a token is successfully created, or ctx is signaled
// done.
func (h *sidsHook) deriveSIToken(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, h.derivationTimeout)
	defer cancel()

	resultCh := make(chan siDerivationResult)

	// keep trying to get the token in the background
	go h.tryDerive(ctx, resultCh)

	// wait until we get a token, or we get a signal to quit
	for {
		select {
		case result := <-resultCh:
			if result.err != nil {
				h.logger.Error("failed to derive SI token", "error", result.err)
				h.kill(ctx, fmt.Errorf("failed to derive SI token: %w", result.err))
				return "", result.err
			}
			return result.token, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

func (h *sidsHook) kill(ctx context.Context, reason error) {
	if err := h.lifecycle.Kill(ctx,
		structs.NewTaskEvent(structs.TaskKilling).
			SetFailsTask().
			SetDisplayMessage(reason.Error()),
	); err != nil {
		h.logger.Error("failed to kill task", "kill_reason", reason, "error", err)
	}
}

// tryDerive loops forever until a token is created, or ctx is done.
func (h *sidsHook) tryDerive(ctx context.Context, ch chan<- siDerivationResult) {
	for attempt := 0; backoff(ctx, attempt); attempt++ {

		tokens, err := h.sidsClient.DeriveSITokens(h.alloc, []string{h.task.Name})

		switch {
		case err == nil:
			token, exists := tokens[h.task.Name]
			if !exists {
				err := errors.New("response does not include token for task")
				h.logger.Error("derive SI token is missing token for task", "error", err, "task", h.task.Name)
				ch <- siDerivationResult{token: "", err: err}
				return
			}
			ch <- siDerivationResult{token: token, err: nil}
			return
		case structs.IsServerSide(err):
			// the error is known to be a server problem, just die
			h.logger.Error("failed to derive SI token", "error", err, "task", h.task.Name, "server_side", true)
			ch <- siDerivationResult{token: "", err: err}
			return
		case !structs.IsRecoverable(err):
			// the error is known not to be recoverable, just die
			h.logger.Error("failed to derive SI token", "error", err, "task", h.task.Name, "recoverable", false)
			ch <- siDerivationResult{token: "", err: err}
			return

		default:
			// the error is marked recoverable, retry after some backoff
			h.logger.Error("failed attempt to derive SI token", "error", err, "recoverable", true)
		}
	}
}

func backoff(ctx context.Context, attempt int) bool {
	next := computeBackoff(attempt)
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
