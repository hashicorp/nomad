// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/go-hclog"
	log "github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"

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

	// cancel is used to kill the long running token manager
	cancel context.CancelFunc

	// privateDirTokenPath is the path inside the task's private directory where
	// the Vault token is read and written.
	privateDirTokenPath string

	// secretsDirTokenPath is the path inside the task's secret directory where the
	// Vault token is written unless disabled by the task.
	secretsDirTokenPath string

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
}

func newVaultHook(config *vaultHookConfig) *vaultHook {
	h := &vaultHook{
		vaultBlock:           config.vaultBlock,
		vaultConfigsFunc:     config.vaultConfigsFunc,
		clientFunc:           config.clientFunc,
		eventEmitter:         config.events,
		lifecycle:            config.lifecycle,
		updater:              config.updater,
		task:                 config.task,
		firstRun:             true,
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
	token := ""
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
			token = string(data)
			break
		}
	}

	duration := 30
	if token == "" {
		var err error
		token, duration, err = h.deriveVaultToken(ctx)
		if err != nil {
			return err
		}

		// Write the token to disk
		if err := h.writeToken(token); err != nil {
			errorString := "failed to write Vault token to disk"
			h.logger.Error(errorString, "error", err)
			h.lifecycle.Kill(ctx,
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Vault %v", errorString)))
			return err
		}
	}

	if !h.allowTokenExpiration {
		rCtx, cancel := context.WithCancel(context.Background())
		h.cancel = cancel

		go h.run(rCtx, token, time.Duration(duration*int(time.Second)))
	}

	h.updater.updatedVaultToken(token)
	return nil
}

func (h *vaultHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	// Shutdown any created manager
	if h.cancel != nil {
		h.cancel()
	}
	return nil
}

func (h *vaultHook) Shutdown() {
	if h.cancel != nil {
		h.cancel()
	}
}

func (h *vaultHook) run(ctx context.Context, token string, lease time.Duration) {
	var (
		err        error
		expiration time.Time
	)
	renewCh := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(lease / 2):
			lease, expiration, err = h.renewWithBackoff(ctx, token, expiration)
			if err != nil {
				return
			}

			select {
			case renewCh <- struct{}{}:
			default:
			}
		case <-renewCh:
			h.handleRenewal(ctx, token)
		}
	}
}

func (h *vaultHook) handleRenewal(ctx context.Context, token string) {
	var event *structs.TaskEvent
	switch h.vaultBlock.ChangeMode {
	case structs.VaultChangeModeSignal:
		s, err := signals.Parse(h.vaultBlock.ChangeSignal)
		if err != nil {
			h.logger.Error("failed to parse signal", "error", err)
			event = structs.NewTaskEvent(structs.TaskKilling).
				SetFailsTask().
				SetDisplayMessage(fmt.Sprintf("Vault: failed to parse signal: %v", err))
			h.lifecycle.Kill(ctx, event)
			return
		}

		event := structs.NewTaskEvent(structs.TaskSignaling).SetTaskSignal(s).SetDisplayMessage("Vault: new Vault token acquired")
		if err := h.lifecycle.Signal(event, h.vaultBlock.ChangeSignal); err != nil {
			h.logger.Error("failed to send signal", "error", err)
			event = structs.NewTaskEvent(structs.TaskKilling).
				SetFailsTask().
				SetDisplayMessage(fmt.Sprintf("Vault: failed to send signal: %v", err))

			h.lifecycle.Kill(ctx, event)
			return
		}
	case structs.VaultChangeModeRestart:
		event = structs.NewTaskEvent(structs.TaskRestartSignal).SetDisplayMessage("Vault: new Vault token acquired")
		h.lifecycle.Restart(ctx, event, false)
	case structs.VaultChangeModeNoop:
		// True to its name, this is a noop!
	default:
		h.logger.Error("invalid Vault change mode", "mode", h.vaultBlock.ChangeMode)
	}

	// Call the handler
	h.updater.updatedVaultToken(token)
}

func (h *vaultHook) renewWithBackoff(ctx context.Context, token string, expiration time.Time) (time.Duration, time.Time, error) {
	var attempts uint64

	for {
		// Renewing with a duration of 0 renews with the default TTL configured
		duration, newExp, err := h.client.Renew(ctx, token, 0)
		if err == nil {
			return duration, newExp, nil
		}

		metrics.IncrCounter([]string{"client", "vault", "renew_token_error"}, 1)

		if !structs.IsRecoverable(err) {
			metrics.IncrCounter([]string{"client", "vault", "renew_token_failure"}, 1)
			h.logger.Error("failed to renew Vault token", "error", err, "recoverable", false)
			h.lifecycle.Kill(ctx,
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Vault: failed to renew vault token: %v", err)))
			return 0, time.Time{}, err
		}

		backoff := helper.Backoff(vaultBackoffBaseline, vaultBackoffLimit, attempts)
		attempts++

		if time.Now().After(expiration) {
			return 0, time.Time{}, errors.New("Vault: token expired while trying to renew")
		}

		select {
		case <-ctx.Done():
			return 0, time.Time{}, ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// deriveVaultToken derives the Vault token using exponential backoffs. It
// returns the Vault token and whether the manager should exit.
func (h *vaultHook) deriveVaultToken(ctx context.Context) (string, int, error) {
	var attempts uint64
	var backoff time.Duration

	timer, stopTimer := helper.NewSafeTimer(0)
	defer stopTimer()

	for {
		token, lease, err := h.deriveVaultTokenJWT(ctx)
		if err == nil {
			return token, lease, nil
		}

		// Check if we can't recover from the error
		if !structs.IsRecoverable(err) {
			h.logger.Error("failed to derive Vault token", "error", err, "recoverable", false)
			h.lifecycle.Kill(ctx,
				structs.NewTaskEvent(structs.TaskKilling).
					SetFailsTask().
					SetDisplayMessage(fmt.Sprintf("Vault: failed to derive vault token: %v", err)))
			return "", 0, err
		}

		// Handle the retry case
		backoff = helper.Backoff(vaultBackoffBaseline, vaultBackoffLimit, attempts)
		timer.Reset(backoff)
		attempts++

		h.logger.Error("failed to derive Vault token", "error", err, "recoverable", true, "backoff", backoff)

		// Wait till retrying
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-timer.C:
		}
	}
}

// deriveVaultTokenJWT returns a Vault ACL token using JWT auth login.
func (h *vaultHook) deriveVaultTokenJWT(ctx context.Context) (string, int, error) {
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
	token, renewable, leaseDuration, err := h.client.DeriveTokenWithJWT(ctx, vaultclient.JWTLoginRequest{
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
