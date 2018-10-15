package taskrunner

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/consul-template/signals"
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// vaultBackoffBaseline is the baseline time for exponential backoff when
	// attempting to retrieve a Vault token
	vaultBackoffBaseline = 5 * time.Second

	// vaultBackoffLimit is the limit of the exponential backoff when attempting
	// to retrieve a Vault token
	vaultBackoffLimit = 3 * time.Minute

	// vaultTokenFile is the name of the file holding the Vault token inside the
	// task's secret directory
	vaultTokenFile = "vault_token"
)

type vaultTokenUpdateHandler interface {
	updatedVaultToken(token string)
}

func (tr *TaskRunner) updatedVaultToken(token string) {
	// Update the task runner and environment
	tr.setVaultToken(token)

	// Trigger update hooks with the new Vault token
	tr.triggerUpdateHooks()
}

type vaultHookConfig struct {
	vaultStanza *structs.Vault
	client      vaultclient.VaultClient
	events      ti.EventEmitter
	lifecycle   ti.TaskLifecycle
	updater     vaultTokenUpdateHandler
	logger      log.Logger
	alloc       *structs.Allocation
	task        string
}

type vaultHook struct {
	// vaultStanza is the vault stanza for the task
	vaultStanza *structs.Vault

	// eventEmitter is used to emit events to the task
	eventEmitter ti.EventEmitter

	// lifecycle is used to signal, restart and kill a task
	lifecycle ti.TaskLifecycle

	// updater is used to update the Vault token
	updater vaultTokenUpdateHandler

	// client is the Vault client to retrieve and renew the Vault token
	client vaultclient.VaultClient

	// logger is used to log
	logger log.Logger

	// ctx and cancel are used to kill the long running token manager
	ctx    context.Context
	cancel context.CancelFunc

	// tokenPath is the path in which to read and write the token
	tokenPath string

	// alloc is the allocation
	alloc *structs.Allocation

	// taskName is the name of the task
	taskName string

	// firstRun stores whether it is the first run for the hook
	firstRun bool

	// future is used to wait on retrieving a Vault token
	future *tokenFuture
}

func newVaultHook(config *vaultHookConfig) *vaultHook {
	ctx, cancel := context.WithCancel(context.Background())
	h := &vaultHook{
		vaultStanza:  config.vaultStanza,
		client:       config.client,
		eventEmitter: config.events,
		lifecycle:    config.lifecycle,
		updater:      config.updater,
		alloc:        config.alloc,
		taskName:     config.task,
		firstRun:     true,
		ctx:          ctx,
		cancel:       cancel,
		future:       newTokenFuture(),
	}
	h.logger = config.logger.Named(h.Name())
	return h
}

func (*vaultHook) Name() string {
	return "vault"
}

func (h *vaultHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	// If we have already run prestart before exit early. We do not use the
	// PrestartDone value because we want to recover the token on restoration.
	first := h.firstRun
	h.firstRun = false
	if !first {
		return nil
	}

	// Try to recover a token if it was previously written in the secrets
	// directory
	recoveredToken := ""
	h.tokenPath = filepath.Join(req.TaskDir.SecretsDir, vaultTokenFile)
	data, err := ioutil.ReadFile(h.tokenPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to recover vault token: %v", err)
		}

		// Token file doesn't exist
	} else {
		// Store the recovered token
		recoveredToken = string(data)
	}

	// Launch the token manager
	go h.run(recoveredToken)

	// Block until we get a token
	select {
	case <-h.future.Wait():
	case <-ctx.Done():
		return nil
	}

	h.updater.updatedVaultToken(h.future.Get())
	return nil
}

func (h *vaultHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	// Shutdown any created manager
	h.cancel()
	return nil
}

// run should be called in a go-routine and manages the derivation, renewal and
// handling of errors with the Vault token. The optional parameter allows
// setting the initial Vault token. This is useful when the Vault token is
// recovered off disk.
func (h *vaultHook) run(token string) {
	// Helper for stopping token renewal
	stopRenewal := func() {
		if err := h.client.StopRenewToken(h.future.Get()); err != nil {
			h.logger.Warn("failed to stop token renewal", "error", err)
		}
	}

	// updatedToken lets us store state between loops. If true, a new token
	// has been retrieved and we need to apply the Vault change mode
	var updatedToken bool

OUTER:
	for {
		// Check if we should exit
		if h.ctx.Err() != nil {
			stopRenewal()
			return
		}

		// Clear the token
		h.future.Clear()

		// Check if there already is a token which can be the case for
		// restoring the TaskRunner
		if token == "" {
			// Get a token
			var exit bool
			token, exit = h.deriveVaultToken()
			if exit {
				// Exit the manager
				return
			}

			// Write the token to disk
			if err := h.writeToken(token); err != nil {
				errorString := "failed to write Vault token to disk"
				h.logger.Error(errorString, "error", err)
				h.lifecycle.Kill(h.ctx,
					structs.NewTaskEvent(structs.TaskKilling).
						SetFailsTask().
						SetDisplayMessage(fmt.Sprintf("Vault %v", errorString)))
				return
			}
		}

		// Start the renewal process
		renewCh, err := h.client.RenewToken(token, 30)

		// An error returned means the token is not being renewed
		if err != nil {
			h.logger.Error("failed to start renewal of Vault token", "error", err)
			token = ""
			goto OUTER
		}

		// The Vault token is valid now, so set it
		h.future.Set(token)

		if updatedToken {
			switch h.vaultStanza.ChangeMode {
			case structs.VaultChangeModeSignal:
				s, err := signals.Parse(h.vaultStanza.ChangeSignal)
				if err != nil {
					h.logger.Error("failed to parse signal", "error", err)
					h.lifecycle.Kill(h.ctx,
						structs.NewTaskEvent(structs.TaskKilling).
							SetFailsTask().
							SetDisplayMessage(fmt.Sprintf("Vault: failed to parse signal: %v", err)))
					return
				}

				event := structs.NewTaskEvent(structs.TaskSignaling).SetTaskSignal(s).SetDisplayMessage("Vault: new Vault token acquired")
				if err := h.lifecycle.Signal(event, h.vaultStanza.ChangeSignal); err != nil {
					h.logger.Error("failed to send signal", "error", err)
					h.lifecycle.Kill(h.ctx,
						structs.NewTaskEvent(structs.TaskKilling).
							SetFailsTask().
							SetDisplayMessage(fmt.Sprintf("Vault: failed to send signal: %v", err)))
					return
				}
			case structs.VaultChangeModeRestart:
				const noFailure = false
				h.lifecycle.Restart(h.ctx,
					structs.NewTaskEvent(structs.TaskRestarting).
						SetDisplayMessage("Vault: new Vault token acquired"), false)
			case structs.VaultChangeModeNoop:
				fallthrough
			default:
				h.logger.Error("invalid Vault change mode", "mode", h.vaultStanza.ChangeMode)
			}

			// We have handled it
			updatedToken = false

			// Call the handler
			h.updater.updatedVaultToken(token)
		}

		// Start watching for renewal errors
		select {
		case err := <-renewCh:
			// Clear the token
			token = ""
			h.logger.Error("failed to renew Vault token", "error", err)
			stopRenewal()

			// Check if we have to do anything
			if h.vaultStanza.ChangeMode != structs.VaultChangeModeNoop {
				updatedToken = true
			}
		case <-h.ctx.Done():
			stopRenewal()
			return
		}
	}
}

// deriveVaultToken derives the Vault token using exponential backoffs. It
// returns the Vault token and whether the manager should exit.
func (h *vaultHook) deriveVaultToken() (token string, exit bool) {
	attempts := 0
	for {
		tokens, err := h.client.DeriveToken(h.alloc, []string{h.taskName})
		if err == nil {
			return tokens[h.taskName], false
		}

		// Check if this is a server side error
		if structs.IsServerSide(err) {
			h.logger.Error("failed to derive Vault token", "error", err, "server_side", true)
			h.lifecycle.Kill(h.ctx,
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Vault: server failed to derive vault token: %v", err)))
			return "", true
		}

		// Check if we can't recover from the error
		if !structs.IsRecoverable(err) {
			h.logger.Error("failed to derive Vault token", "error", err, "recoverable", false)
			h.lifecycle.Kill(h.ctx,
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Vault: failed to derive vault token: %v", err)))
			return "", true
		}

		// Handle the retry case
		backoff := (1 << (2 * uint64(attempts))) * vaultBackoffBaseline
		if backoff > vaultBackoffLimit {
			backoff = vaultBackoffLimit
		}
		h.logger.Error("failed to derive Vault token", "error", err, "recoverable", true, "backoff", backoff)

		attempts++

		// Wait till retrying
		select {
		case <-h.ctx.Done():
			return "", true
		case <-time.After(backoff):
		}
	}
}

// writeToken writes the given token to disk
func (h *vaultHook) writeToken(token string) error {
	if err := ioutil.WriteFile(h.tokenPath, []byte(token), 0666); err != nil {
		return fmt.Errorf("failed to write vault token: %v", err)
	}

	return nil
}

// tokenFuture stores the Vault token and allows consumers to block till a valid
// token exists
type tokenFuture struct {
	waiting []chan struct{}
	token   string
	set     bool
	m       sync.Mutex
}

// newTokenFuture returns a new token future without any token set
func newTokenFuture() *tokenFuture {
	return &tokenFuture{}
}

// Wait returns a channel that can be waited on. When this channel unblocks, a
// valid token will be available via the Get method
func (f *tokenFuture) Wait() <-chan struct{} {
	f.m.Lock()
	defer f.m.Unlock()

	c := make(chan struct{})
	if f.set {
		close(c)
		return c
	}

	f.waiting = append(f.waiting, c)
	return c
}

// Set sets the token value and unblocks any caller of Wait
func (f *tokenFuture) Set(token string) *tokenFuture {
	f.m.Lock()
	defer f.m.Unlock()

	f.set = true
	f.token = token
	for _, w := range f.waiting {
		close(w)
	}
	f.waiting = nil
	return f
}

// Clear clears the set vault token.
func (f *tokenFuture) Clear() *tokenFuture {
	f.m.Lock()
	defer f.m.Unlock()

	f.token = ""
	f.set = false
	return f
}

// Get returns the set Vault token
func (f *tokenFuture) Get() string {
	f.m.Lock()
	defer f.m.Unlock()
	return f.token
}
