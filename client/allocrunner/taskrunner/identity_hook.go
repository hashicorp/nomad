// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
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
	tr       *TaskRunner
	rpc      RPCer
	allocID  string
	taskName string
	taskUser string
	tokenDir string
	logger   log.Logger

	initialWIDs []structs.SignedWorkloadIdentity

	// Manages lifetime of renewal goroutine
	ctx    context.Context
	cancel context.CancelFunc
}

func newIdentityHook(tr *TaskRunner, logger log.Logger, initialWIDs []structs.SignedWorkloadIdentity) *identityHook {
	h := &identityHook{
		tr:          tr,
		allocID:     tr.allocID,
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

	// Handle alternate workload identities: using the intially provided WIDs if
	// possible and falling back to RPC.
	missingSignedWIDs := map[string]*structs.WorkloadIdentity{}
	for _, widspec := range req.Task.Identities {
		if widspec.Name == structs.WorkloadIdentityDefaultName {
			// Default is handled above
			continue
		}

		if !widspec.Env && !widspec.File {
			// Nothing to do with this identity, skip it
			continue
		}

		signedWID, ok := signedWIDs[widspec.Name]
		if !ok {
			// Missed, pull from server
			missingSignedWIDs[widspec.Name] = widspec
			continue
		}

		if err := h.exposeWID(widspec, signedWID); err != nil {
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
	go h.run()

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

func (h *identityHook) exposeWID(widspec *structs.WorkloadIdentity, signedWID structs.SignedWorkloadIdentity) error {
	if widspec.Env {
		h.tr.envBuilder.SetWorkloadToken(widspec.Name, signedWID.JWT)
	}

	if widspec.File {
		tokenPath := filepath.Join(h.tokenDir, fmt.Sprintf("nomad_%s.jwt", widspec.Name))
		if err := users.WriteFileFor(tokenPath, []byte(signedWID.JWT), h.taskUser); err != nil {
			return fmt.Errorf("failed to write token for identity %q: %w", widspec.Name, err)
		}
	}

	return nil
}

// TODO(schmichael) cleanup args
func (h *identityHook) getSignedIDs(missingIDs map[string]*structs.WorkloadIdentity, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	//TODO(schmichael) handle missingSignedWIDs
	// Request missing signed workload identities from servers
	rpcReq := &structs.AllocIdentitiesRequest{
		Identities: make([]structs.WorkloadIdentityRequest, 0, len(missingIDs)),
		QueryOptions: structs.QueryOptions{
			Region:    req.Alloc.Job.Region,
			Namespace: req.Alloc.Namespace,

			// Any server can sign workload identities as long as their statestore
			// contains the alloc.
			MinQueryIndex: req.Alloc.CreateIndex,
			AllowStale:    true,
		},
	}

	for missingID := range missingIDs {
		rpcReq.Identities = append(rpcReq.Identities, structs.WorkloadIdentityRequest{
			AllocID:      h.allocID,
			TaskName:     h.taskName,
			IdentityName: missingID,
		})
	}

	//TODO(schmichael) plumb in rpc
	rpcResp := &structs.AllocIdentitiesResponse{}
	if err := h.rpc.RPC("Alloc.GetIdentities", rpcReq, &rpcResp); err != nil {
		return err
	}

	//TODO(schmichael) see rambling above about failures
	//TODO(schmichael) if we are going to hard fail here fix the error message
	if len(rpcResp.Rejections) > 0 {
		reasons := make([]string, 0, len(rpcResp.Rejections))
		for _, r := range rpcResp.Rejections {
			h.logger.Error("workload identity request rejected", "reason", r.Reason, "wid", r.IdentityName)
			reasons = append(reasons, r.IdentityName+": "+r.Reason)
			delete(missingIDs, r.IdentityName)
		}

		return fmt.Errorf("error getting workload identities: %s", strings.Join(reasons, ", "))
	}

	for _, signedID := range rpcResp.SignedIdentities {
		widspec, ok := missingIDs[signedID.IdentityName]
		if !ok {
			// Server bug! Every requested WID should either have a signed identity
			// or a rejection (above).
			h.logger.Error("bug: unexpected workload identity received",
				signedID.AllocID, signedID.TaskName, signedID.IdentityName)
			continue
		}

		if err := h.exposeWID(widspec, signedID); err != nil {
			return err
		}

		// Track ones we have seen to help detect bugs
		delete(missingIDs, signedID.IdentityName)
	}

	if n := len(missingIDs); n > 0 {
		h.logger.Error("bug: missing signed identity or rejection", "num_missing", n, "missing", missingIDs)
	}

	return nil
}

func (h *identityHook) run() {
	for err := h.ctx.Err(); err == nil; {
		args := &structs.AllocIdentitiesRequest{}
		reply := &structs.AllocIdentitiesResponse{}
		if err := h.rpc.RPC("Alloc.GetIdentities", args, reply); err != nil {

			// Wait and retry
			//TODO standardize somewhere? base it off something meaningful?!
			const base = 10 * time.Second
			const jitter = 20 * time.Second
			wait := base + helper.RandomStagger(jitter)
			h.logger.Warn("failed to get workload identities", "error", err, "retry_in", wait)
			select {
			case <-h.ctx.Done():
				return
			case <-time.After(wait):
				continue
			}
		}

		if len(reply.Rejections) > 0 {
			reasons := make([]string, 0, len(reply.Rejections))
			for _, r := range reply.Rejections {
				reasons = append(reasons, r.IdentityName+": "+r.Reason)
			}
			h.logger.Error("workload identity request rejected", "num_failures", len(reasons), "reasons", reasons)
		}

		for _, signedID := range reply.SignedIdentities {
			widspec, ok := missingIDs[signedID.IdentityName]
			if !ok {
				// Server bug! Every requested WID should either have a signed identity
				// or a rejection (above).
				h.logger.Error("bug: unexpected workload identity received",
					signedID.AllocID, signedID.TaskName, signedID.IdentityName)
				continue
			}

			if err := h.exposeWID(widspec, signedID); err != nil {
				//TODO(schmichael) is there anything more we can do?
				h.logger.Error("error setting workload identity", "error", err)
				return
			}
		}
	}
}

func (h *identityHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	h.cancel()
	return nil
}

func (h *identityHook) Shutdown() {
	h.cancel()
}
