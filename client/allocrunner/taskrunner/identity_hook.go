// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
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
	tokenDir   string
	envBuilder *taskenv.Builder
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
		tokenDir:   tr.taskDir.SecretsDir,
		envBuilder: tr.envBuilder,
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

func (h *identityHook) Prestart(context.Context, *interfaces.TaskPrestartRequest, *interfaces.TaskPrestartResponse) error {

	// Handle default workload identity
	if err := h.setDefaultToken(); err != nil {
		return err
	}

	// Start token watcher loops
	for _, widspec := range h.task.Identities {
		w := widspec
		go h.watchIdentity(w)
	}

	return nil
}

func (h *identityHook) watchIdentity(wid *structs.WorkloadIdentity) {
	id := structs.WIHandle{WorkloadIdentifier: h.task.Name, IdentityName: wid.Name}
	signedIdentitiesChan, stopWatching := h.widmgr.Watch(id)
	defer stopWatching()

	for {
		select {
		case signedWID, ok := <-signedIdentitiesChan:
			h.logger.Trace("receiving renewed identity", "identity_name", wid.Name)
			if !ok {
				// Chan was closed, stop watching
				h.logger.Trace("identity watch closed", "task", h.task.Name, "identity", wid.Name)
				return
			}

			if signedWID == nil {
				// The only way to hit this should be a bug as it indicates the server
				// did not sign an identity for a task on this alloc.
				h.logger.Error("missing workload identity %q", wid.Name)
			}

			if err := h.setAltToken(wid, signedWID.JWT); err != nil {
				h.logger.Error(err.Error())
			}
		case <-h.stopCtx.Done():
			return
		}
	}
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
		tokenPath := filepath.Join(h.tokenDir, wiTokenFile)
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
		tokenPath := filepath.Join(h.tokenDir, fmt.Sprintf("nomad_%s.jwt", widspec.Name))
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
