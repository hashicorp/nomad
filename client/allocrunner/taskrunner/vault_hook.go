// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/go-hclog"
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
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
	vaultBlock       *structs.Vault
	vaultConfigsFunc func(hclog.Logger) map[string]*sconfig.VaultConfig
	clientFunc       vaultclient.VaultClientFunc
	events           ti.EventEmitter
	lifecycle        ti.TaskLifecycle
	updater          vaultTokenUpdateHandler
	logger           log.Logger
	alloc            *structs.Allocation
	task             *structs.Task
	widmgr           widmgr.IdentityManager
}

type vaultHook struct {
	// vaultBlock is the vault block for the task
	vaultBlock *structs.Vault

	// vaultConfig is the Nomad client configuration for Vault.
	vaultConfig      *sconfig.VaultConfig
	vaultConfigsFunc func(hclog.Logger) map[string]*sconfig.VaultConfig

	// eventEmitter is used to emit events to the task
	eventEmitter ti.EventEmitter

	// lifecycle is used to signal, restart and kill a task
	lifecycle ti.TaskLifecycle

	// updater is used to update the Vault token
	updater vaultTokenUpdateHandler

	// client is the Vault client to retrieve and renew the Vault token, and
	// clientFunc is the injected function that retrieves it
	client     vaultclient.VaultClient
	clientFunc vaultclient.VaultClientFunc

	// logger is used to log
	logger log.Logger

	// ctx and cancel are used to kill the long running token manager
	ctx    context.Context
	cancel context.CancelFunc

	// privateDirTokenPath is the path inside the task's private directory where
	// the Vault token is read and written.
	privateDirTokenPath string

	// secretsDirTokenPath is the path inside the task's secret directory where the
	// Vault token is written unless disabled by the task.
	secretsDirTokenPath string

	// alloc is the allocation
	alloc *structs.Allocation

	// task is the task to run.
	task *structs.Task

	// firstRun stores whether it is the first run for the hook
	firstRun bool

	// widmgr is used to access signed tokens for workload identities.
	widmgr widmgr.IdentityManager

	// widName is the workload identity name to use to retrieve signed JWTs.
	widName string

	// allowTokenExpiration determines if a renew loop should be run
	allowTokenExpiration bool

	// future is used to wait on retrieving a Vault token
	future *tokenFuture
}

func newVaultHook(config *vaultHookConfig) *vaultHook {
	ctx, cancel := context.WithCancel(context.Background())
	h := &vaultHook{
		vaultBlock:           config.vaultBlock,
		vaultConfigsFunc:     config.vaultConfigsFunc,
		clientFunc:           config.clientFunc,
		eventEmitter:         config.events,
		lifecycle:            config.lifecycle,
		updater:              config.updater,
		alloc:                config.alloc,
		task:                 config.task,
		firstRun:             true,
		ctx:                  ctx,
		cancel:               cancel,
		future:               newTokenFuture(),
		widmgr:               config.widmgr,
		widName:              config.task.Vault.IdentityName(),
		allowTokenExpiration: config.vaultBlock.AllowTokenExpiration,
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

	cluster := h.task.GetVaultClusterName()
	vclient, err := h.clientFunc(cluster)
	if err != nil {
		return err
	}
	h.client = vclient

	h.vaultConfig = h.vaultConfigsFunc(h.logger)[cluster]
	if h.vaultConfig == nil {
		return fmt.Errorf("No client configuration found for Vault cluster %s", cluster)
	}

	// Try to recover a token if it was previously written in the secrets
	// directory
	recoveredToken := ""
	h.privateDirTokenPath = filepath.Join(req.TaskDir.PrivateDir, vaultTokenFile)
	h.secretsDirTokenPath = filepath.Join(req.TaskDir.SecretsDir, vaultTokenFile)

	// Handle upgrade path by searching for the previous token in all possible
	// paths where the token may be.
	for _, path := range []string{h.privateDirTokenPath, h.secretsDirTokenPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to recover vault token from %s: %v", path, err)
			}

			// Token file doesn't exist in this path.
		} else {
			// Store the recovered token
			recoveredToken = string(data)
			break
		}
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

func (h *vaultHook) Shutdown() {
	h.cancel()
}

// run should be called in a go-routine and manages the derivation, renewal and
// handling of errors with the Vault token. The optional parameter allows
// setting the initial Vault token. This is useful when the Vault token is
// recovered off disk.
func (h *vaultHook) run(token string) {
	// Helper for stopping token renewal
	stopRenewal := func() {
		if h.allowTokenExpiration {
			return
		}
		if err := h.client.StopRenewToken(h.future.Get()); err != nil {
			h.logger.Warn("failed to stop token renewal", "error", err)
		}
	}

	// updatedToken lets us store state between loops. If true, a new token
	// has been retrieved and we need to apply the Vault change mode
	var updatedToken bool
	leaseDuration := 30

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
			token, leaseDuration, exit = h.deriveVaultToken()
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

		if h.allowTokenExpiration {
			h.future.Set(token)
			h.logger.Debug("Vault token will not renew")
			return
		}

		// Start the renewal process.
		//
		// This is the initial renew of the token which we derived from the
		// server. The client does not know how long it took for the token to
		// be generated and derived and also wants to gain control of the
		// process quickly, but not too quickly. We therefore use a hardcoded
		// increment value of 30; this value without a suffix is in seconds.
		//
		// If Vault is having availability issues or is overloaded, a large
		// number of initial token renews can exacerbate the problem.
		if leaseDuration == 0 {
			leaseDuration = 30
		}
		renewCh, err := h.client.RenewToken(token, leaseDuration)

		// An error returned means the token is not being renewed
		if err != nil {
			h.logger.Error("failed to start renewal of Vault token", "error", err)
			token = ""
			goto OUTER
		}

		// The Vault token is valid now, so set it
		h.future.Set(token)

		if updatedToken {
			switch h.vaultBlock.ChangeMode {
			case structs.VaultChangeModeSignal:
				s, err := signals.Parse(h.vaultBlock.ChangeSignal)
				if err != nil {
					h.logger.Error("failed to parse signal", "error", err)
					h.lifecycle.Kill(h.ctx,
						structs.NewTaskEvent(structs.TaskKilling).
							SetFailsTask().
							SetDisplayMessage(fmt.Sprintf("Vault: failed to parse signal: %v", err)))
					return
				}

				event := structs.NewTaskEvent(structs.TaskSignaling).SetTaskSignal(s).SetDisplayMessage("Vault: new Vault token acquired")
				if err := h.lifecycle.Signal(event, h.vaultBlock.ChangeSignal); err != nil {
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
					structs.NewTaskEvent(structs.TaskRestartSignal).
						SetDisplayMessage("Vault: new Vault token acquired"), noFailure)
			case structs.VaultChangeModeNoop:
				// True to its name, this is a noop!
			default:
				h.logger.Error("invalid Vault change mode", "mode", h.vaultBlock.ChangeMode)
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
			updatedToken = true
		case <-h.ctx.Done():
			stopRenewal()
			return
		}
	}
}

// deriveVaultToken derives the Vault token using exponential backoffs. It
// returns the Vault token and whether the manager should exit.
func (h *vaultHook) deriveVaultToken() (string, int, bool) {
	var attempts uint64
	var backoff time.Duration

	timer, stopTimer := helper.NewSafeTimer(0)
	defer stopTimer()

	for {
		token, lease, err := h.deriveVaultTokenJWT()
		if err == nil {
			return token, lease, false
		}

		// Check if we can't recover from the error
		if !structs.IsRecoverable(err) {
			h.logger.Error("failed to derive Vault token", "error", err, "recoverable", false)
			h.lifecycle.Kill(h.ctx,
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Vault: failed to derive vault token: %v", err)))
			return "", 0, true
		}

		// Handle the retry case
		backoff = helper.Backoff(vaultBackoffBaseline, vaultBackoffLimit, attempts)
		timer.Reset(backoff)
		attempts++

		h.logger.Error("failed to derive Vault token", "error", err, "recoverable", true, "backoff", backoff)

		// Wait till retrying
		select {
		case <-h.ctx.Done():
			return "", 0, true
		case <-timer.C:
		}
	}
}

// deriveVaultTokenJWT returns a Vault ACL token using JWT auth login.
func (h *vaultHook) deriveVaultTokenJWT() (string, int, error) {
	// Retrieve signed identity.
	signed, err := h.widmgr.Get(structs.WIHandle{
		IdentityName:       h.widName,
		WorkloadIdentifier: h.task.Name,
		WorkloadType:       structs.WorkloadTypeTask,
	})
	if err != nil {
		return "", 0, structs.NewRecoverableError(
			fmt.Errorf("failed to retrieve signed workload identity: %w", err),
			true,
		)
	}
	if signed == nil {
		return "", 0, structs.NewRecoverableError(
			errors.New("no signed workload identity available"),
			false,
		)
	}

	role := h.vaultConfig.Role
	if h.vaultBlock.Role != "" {
		role = h.vaultBlock.Role
	}

	// Derive Vault token with signed identity.
	token, renewable, leaseDuration, err := h.client.DeriveTokenWithJWT(h.ctx, vaultclient.JWTLoginRequest{
		JWT:       signed.JWT,
		Role:      role,
		Namespace: h.vaultBlock.Namespace,
	})
	if err != nil {
		return "", 0, structs.WrapRecoverable(
			fmt.Sprintf("failed to derive Vault token for identity %s: %v", h.widName, err),
			err,
		)
	}

	// If the token cannot be renewed, it doesn't matter if the user set
	// allow_token_expiration or not, so override the requested behavior
	if !renewable {
		h.allowTokenExpiration = true
	}

	return token, leaseDuration, nil
}

// writeToken writes the given token to disk
func (h *vaultHook) writeToken(token string) error {
	// Handle upgrade path by first checking if the tasks private directory
	// exists. If it doesn't, this allocation probably existed before the
	// private directory was introduced, so keep using the secret directory to
	// prevent unnecessary errors during task recovery.
	if _, err := os.Stat(path.Dir(h.privateDirTokenPath)); os.IsNotExist(err) {
		if err := os.WriteFile(h.secretsDirTokenPath, []byte(token), 0666); err != nil {
			return fmt.Errorf("failed to write vault token to secrets dir: %v", err)
		}
		return nil
	}

	if err := os.WriteFile(h.privateDirTokenPath, []byte(token), 0600); err != nil {
		return fmt.Errorf("failed to write vault token: %v", err)
	}
	if !h.vaultBlock.DisableFile {
		if err := os.WriteFile(h.secretsDirTokenPath, []byte(token), 0666); err != nil {
			return fmt.Errorf("failed to write vault token to secrets dir: %v", err)
		}
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
