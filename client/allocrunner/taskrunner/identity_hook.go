// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/command/agent/keymgr"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// wiTokenFile is the name of the file holding the Nomad token inside the
	// task's secret directory
	wiTokenFile = "nomad_token"
)

// identityHook sets the task runner's Nomad workload identity token
// based on the signed identity stored on the Allocation as well as manages the
// other workload identities for the task.
type identityHook struct {
	tr      *TaskRunner
	widMgr  *keymgr.WIDMgr
	allocID string

	// allocIndex is the CreateIndex for the task's allocation for use when
	// retrieving identities. Requests must block until the server has at least
	// caught up to when the alloc was created.
	allocIndex uint64

	taskName string
	taskUser string
	tokenDir string

	initialWIDs []structs.SignedWorkloadIdentity

	// Manages lifetime of renewal goroutine
	ctx    context.Context
	cancel context.CancelFunc

	logger log.Logger
}

func newIdentityHook(tr *TaskRunner, logger log.Logger, initialWIDs []structs.SignedWorkloadIdentity) *identityHook {
	h := &identityHook{
		tr:          tr,
		widMgr:      tr.widMgr,
		allocID:     tr.allocID,
		allocIndex:  tr.Alloc().CreateIndex,
		taskName:    tr.taskName,
		taskUser:    tr.Task().User,
		initialWIDs: initialWIDs,
		tokenDir:    tr.taskDir.SecretsDir,
	}
	h.logger = logger.Named(h.Name())

	ctx, cancel := context.WithCancel(context.Background())
	h.ctx = ctx
	h.cancel = cancel
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {

	// Handle the default Nomad workload identity
	if err := h.setNomadToken(h.tokenDir, req.Alloc, h.taskUser); err != nil {
		return fmt.Errorf("error setting default workload identity: %w", err)
	}

	// Index initial workload identities by name
	signedWIDs := map[string]structs.SignedWorkloadIdentity{}
	for _, wid := range h.initialWIDs {
		signedWIDs[wid.IdentityName] = wid
	}

	var minExp time.Time

	// Handle alternate workload identities: using the intially provided WIDs if
	// possible and falling back to RPC.
	missingSignedWIDs := map[string]*structs.WorkloadIdentity{}
	for _, widspec := range req.Task.Identities {
		if widspec.Name == structs.WorkloadIdentityDefaultName {
			// Default is handled above
			continue
		}

		signedWID, ok := signedWIDs[widspec.Name]
		if !ok {
			// Missed, pull from server
			missingSignedWIDs[widspec.Name] = widspec
			continue
		}

		if minExp.IsZero() || minExp.After(signedWID.Exp) {
			minExp = signedWID.Exp
		}

		if err := h.exposeWID(widspec, signedWID.JWT); err != nil {
			return err
		}
	}

	if len(missingSignedWIDs) > 0 {
		if err := h.getSignedIDs(missingSignedWIDs, req, resp); err != nil {
			//TODO(schmichael) hard failing here means clients will kill running
			//workloads if the client is restarted while disconnected
			//
			// soft failing here will probably let things Just Work for *restarts*
			//
			// soft failing here for *reboots* - where the env vars and files no longer
			// exist - will probably cause harder to debug failures down the line
			//
			// hard failing for now to punt the problem down the road a bit
			return err
		}
	}

	// Run renewal loop
	go h.run(minExp)

	return nil
}

// setNomadToken adds the Nomad token to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *identityHook) setNomadToken(path string, alloc *structs.Allocation, owner string) error {
	token := alloc.SignedIdentities[h.taskName]
	if token == "" {
		return nil
	}

	h.tr.setNomadToken(token)

	if id := h.tr.task.Identity; id != nil && id.File {
		tokenPath := filepath.Join(path, wiTokenFile)
		if err := users.WriteFileFor(tokenPath, []byte(token), owner); err != nil {
			return fmt.Errorf("failed to write token: %w", err)
		}
	}

	return nil
}

func (h *identityHook) exposeWID(widspec *structs.WorkloadIdentity, rawJWT string) error {
	if widspec.Env {
		h.tr.envBuilder.SetWorkloadToken(widspec.Name, rawJWT)
	}

	if widspec.File {
		tokenPath := filepath.Join(h.tokenDir, fmt.Sprintf("nomad_%s.jwt", widspec.Name))
		if err := users.WriteFileFor(tokenPath, []byte(rawJWT), h.taskUser); err != nil {
			return fmt.Errorf("failed to write token for identity %q: %w", widspec.Name, err)
		}
	}

	return nil
}

// TODO(schmichael) cleanup args
func (h *identityHook) getSignedIDs(missingIDs map[string]*structs.WorkloadIdentity, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	ids := make([]structs.WorkloadIdentityRequest, 0, len(missingIDs))

	for missingID := range missingIDs {
		ids = append(ids, structs.WorkloadIdentityRequest{
			AllocID:      h.allocID,
			TaskName:     h.taskName,
			IdentityName: missingID,
		})
	}

	tokens, err := h.widMgr.GetIdentities(h.ctx, h.allocIndex, ids)
	if err != nil {
		return err
	}

	for _, token := range tokens {
		widspec, ok := missingIDs[token.Name]
		if !ok {
			// Bug: Every requested WID should either have a signed identity
			// or a rejection (above).
			h.logger.Error("bug: unexpected workload identity received", "identity", token.Name)
			continue
		}

		if err := h.exposeWID(widspec, token.JWT); err != nil {
			return err
		}

		// Track ones we have seen to help detect bugs
		delete(missingIDs, token.Name)
	}

	if n := len(missingIDs); n > 0 {
		h.logger.Error("bug: missing signed identity or rejection", "num_missing", n, "missing", missingIDs)
	}

	return nil
}

func (h *identityHook) run(nextExp time.Time) {
	origWIDSpecs := h.tr.Task().Identities

	// widspecs maps identity names to their specs
	widspecs := make(map[string]*structs.WorkloadIdentity, len(origWIDSpecs))

	// req is a list of identities to request be signed
	req := make([]structs.WorkloadIdentityRequest, len(origWIDSpecs))
	for i, widspec := range origWIDSpecs {
		widspecs[widspec.Name] = widspec
		req[i] = structs.WorkloadIdentityRequest{
			AllocID:      h.tr.allocID,
			TaskName:     h.taskName,
			IdentityName: widspec.Name,
		}
	}

	// Wait until we need to renew again.
	wait := keymgr.ExpiryToRenewTime(nextExp, time.Now)

	timer, timerStop := helper.NewSafeTimer(wait)
	defer timerStop()

	for err := h.ctx.Err(); err == nil; {
		select {
		case <-timer.C:
			h.logger.Debug("getting new workload identities")
		case <-h.ctx.Done():
			return
		}

		tokens, err := h.widMgr.GetIdentities(h.ctx, h.allocIndex, req)
		if err != nil {
			// Wait and retry
			//TODO standardize somewhere? base it off something meaningful?!
			const base = 10 * time.Second
			const jitter = 20 * time.Second
			timer.Reset(base + helper.RandomStagger(jitter))
			h.logger.Warn("failed to get workload identities", "error", err, "retry_in", wait)
			continue
		}
		if len(tokens) == 0 {
			// Wait and retry
			//TODO standardize somewhere? base it off something meaningful?!
			const base = 10 * time.Second
			const jitter = 20 * time.Second
			timer.Reset(base + helper.RandomStagger(jitter))
			h.logger.Warn("failed to get workload identities", "error", "no tokens", "retry_in", wait)
			continue
		}

		var minExp time.Time

		for _, token := range tokens {
			widspec, ok := widspecs[token.Name]
			if !ok {
				// Bug: Every requested WID should either have a signed identity or a
				// rejection (above).
				h.logger.Error("bug: unexpected workload identity received",
					"identity", token.Name)
				continue
			}

			if err := h.exposeWID(widspec, token.JWT); err != nil {
				//TODO(schmichael) is there anything more we can do?
				h.logger.Error("error setting workload identity", "error", err)
				return
			}

			h.logger.Debug(">>>> minexp", "minexp", minExp, "token", widspec.Name, "exp", token.Exp)
			if minExp.IsZero() || minExp.After(token.Exp) {
				minExp = token.Exp
			}
		}

		wait = keymgr.ExpiryToRenewTime(minExp, time.Now)
		h.logger.Debug("waiting to renew tokens", "wait_until", wait)
		timer.Reset(wait)
	}
}

func (h *identityHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	h.cancel()
	return nil
}

func (h *identityHook) Shutdown() {
	h.cancel()
}
