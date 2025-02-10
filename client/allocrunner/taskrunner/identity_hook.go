// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hashicorp/consul-template/signals"
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
)

// identityHook sets the task runner's Nomad workload identity token
// based on the signed identity stored on the Allocation

const (
	// wiTokenFile is the name of the file holding the Nomad token inside the
	// task's secret directory
	wiTokenFile = "nomad_token"
)

// tokenSetter provides methods for exposing workload identities to other
// internal Nomad components.
type tokenSetter interface {
	setNomadToken(token string)
}

type identityHook struct {
	alloc      *structs.Allocation
	task       *structs.Task
	taskDir    *allocdir.TaskDir
	envBuilder *taskenv.Builder
	lifecycle  ti.TaskLifecycle
	ts         tokenSetter
	widmgr     widmgr.IdentityManager
	logger     log.Logger

	stopCtx context.Context
	stop    context.CancelFunc
}

func newIdentityHook(tr *TaskRunner, logger log.Logger) *identityHook {
	stopCtx, stop := context.WithCancel(context.Background())
	h := &identityHook{
		alloc:      tr.Alloc(),
		task:       tr.Task(),
		taskDir:    tr.taskDir,
		envBuilder: tr.envBuilder,
		lifecycle:  tr,
		ts:         tr,
		widmgr:     tr.widmgr,
		stopCtx:    stopCtx,
		stop:       stop,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prestart(ctx context.Context, _ *interfaces.TaskPrestartRequest, _ *interfaces.TaskPrestartResponse) error {

	// Handle default workload identity
	if err := h.setDefaultToken(); err != nil {
		return err
	}

	// Track first run signals from watchers
	firstRunCh := make(chan struct{}, len(h.task.Identities))

	// Start token watcher loops
	for _, widspec := range h.task.Identities {
		w := widspec
		go h.watchIdentity(w, firstRunCh)
	}

	// Don't block indefinitely for identities
	deadlineTimer := time.NewTimer(time.Minute)
	defer deadlineTimer.Stop()

	// Wait until every watcher ticks the first run chan
	for i := range h.task.Identities {
		select {
		case <-firstRunCh:
			// Identity fetched, loop
		case <-deadlineTimer.C:
			h.logger.Warn("timed out waiting for initial identity tokens to be fetched",
				"num_fetched", i, "num_total", len(h.task.Identities))
			return nil
		case <-ctx.Done():
			h.logger.Debug("task prestart cancelled before initial identity tokens were fetched",
				"num_fetched", i, "num_total", len(h.task.Identities))
			return nil
		case <-h.stopCtx.Done():
			h.logger.Debug("task stopped before initial identity tokens were fetched",
				"num_fetched", i, "num_total", len(h.task.Identities))
			return nil
		}
	}

	return nil
}

func (h *identityHook) watchIdentity(wid *structs.WorkloadIdentity, runCh chan struct{}) {
	id := structs.WIHandle{WorkloadIdentifier: h.task.Name, IdentityName: wid.Name}
	signedIdentitiesChan, stopWatching := h.widmgr.Watch(id)
	defer stopWatching()

	firstRun := true

	for {
		select {
		case signedWID, ok := <-signedIdentitiesChan:
			h.logger.Trace("receiving renewed identity", "identity", wid.Name)
			if !ok {
				// Chan was closed, stop watching
				h.logger.Trace("identity watch closed", "identity", wid.Name)
				return
			}

			if signedWID == nil {
				// The only way to hit this should be a bug as it indicates the server
				// did not sign an identity for a task on this alloc.
				h.logger.Error("missing workload identity %q", wid.Name)
				return
			}

			if err := h.setAltToken(wid, signedWID.JWT); err != nil {
				h.logger.Error(err.Error())
			}

			// Skip ChangeMode on firstRun and notify caller it can proceed
			if firstRun {
				select {
				case runCh <- struct{}{}:
				default:
					// Not great but not necessarily fatal
					h.logger.Warn("task started before identity %q was fetched", wid.Name)
				}

				firstRun = false
				continue
			}

			switch wid.ChangeMode {
			case structs.WIChangeModeRestart:
				const noFailure = false
				err := h.lifecycle.Restart(h.stopCtx, structs.NewTaskEvent(structs.TaskRestartSignal).
					SetDisplayMessage(fmt.Sprintf("Identity[%s]: new token acquired", wid.Name)), noFailure)
				if err != nil {
					// Ignore error from kill because if that fails there's really
					// nothing to be done.
					_ = h.lifecycle.Kill(h.stopCtx, structs.NewTaskEvent(structs.TaskKilling).
						SetFailsTask().
						SetDisplayMessage(fmt.Sprintf("Identity[%s]: failed to restart: %v", wid.Name, err)))
					return
				}

			case structs.WIChangeModeSignal:
				if err := h.signalTask(wid); err != nil {
					h.logger.Error("failed to send signal", "identity", wid.Name, "signal", wid.ChangeSignal)
					// Ignore error from kill because if that fails there's really
					// nothing to be done.
					_ = h.lifecycle.Kill(h.stopCtx, structs.NewTaskEvent(structs.TaskKilling).
						SetFailsTask().
						SetDisplayMessage(fmt.Sprintf("Identity[%s]: failed to send signal: %v", wid.Name, err)))
					return
				}

			}

			// Note: any code added here will not run on first run

		case <-h.stopCtx.Done():
			return
		}
	}
}

// signalTask sends the configured signal to a task or returns an error.
func (h *identityHook) signalTask(wid *structs.WorkloadIdentity) error {
	s, err := signals.Parse(wid.ChangeSignal)
	if err != nil {
		return fmt.Errorf("failed to parse signal: %w", err)
	}

	event := structs.NewTaskEvent(structs.TaskSignaling).
		SetTaskSignal(s).
		SetDisplayMessage(fmt.Sprintf("Identity[%s]: new Identity token acquired", wid.Name))
	return h.lifecycle.Signal(event, wid.ChangeSignal)
}

// setDefaultToken adds the Nomad token to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *identityHook) setDefaultToken() error {
	token := h.alloc.SignedIdentities[h.task.Name]
	if token == "" {
		return nil
	}

	// Handle internal use and env var
	h.ts.setNomadToken(token)

	// Handle file writing
	if id := h.task.Identity; id != nil && id.File {
		// Write token as owner readable only
		tokenPath := filepath.Join(h.taskDir.SecretsDir, wiTokenFile)
		if id.Filepath != "" {
			tokenPath = filepath.Join(h.taskDir.Dir, id.Filepath)
		}
		if err := users.WriteFileFor(tokenPath, []byte(token), h.task.User); err != nil {
			return fmt.Errorf("failed to write nomad token: %w", err)
		}
	}

	return nil
}

// setAltToken takes an alternate workload identity and sets the env var and/or
// writes the token file as specified by the jobspec.
func (h *identityHook) setAltToken(widspec *structs.WorkloadIdentity, rawJWT string) error {
	if widspec.Env {
		h.envBuilder.SetWorkloadToken(widspec.Name, rawJWT)
	}

	if widspec.File {
		tokenPath := filepath.Join(h.taskDir.SecretsDir, fmt.Sprintf("nomad_%s.jwt", widspec.Name))
		if widspec.Filepath != "" {
			tokenPath = filepath.Join(h.taskDir.Dir, widspec.Filepath)
		}
		if err := users.WriteFileFor(tokenPath, []byte(rawJWT), h.task.User); err != nil {
			return fmt.Errorf("failed to write token for identity %q: %w", widspec.Name, err)
		}
	}

	return nil
}

// Stop implements interfaces.TaskStopHook
func (h *identityHook) Stop(context.Context, *interfaces.TaskStopRequest, *interfaces.TaskStopResponse) error {
	h.stop()
	return nil
}

// Shutdown implements interfaces.ShutdownHook
func (h *identityHook) Shutdown() {
	h.stop()
}
