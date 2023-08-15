// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
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

// IdentitySigner is the interface needed to retrieve signed identities for
// workload identities. At runtime it is implemented by *widmgr.WIDMgr.
type IdentitySigner interface {
	SignIdentities(minIndex uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error)
}

type identityHook struct {
	tr       *TaskRunner
	tokenDir string
	logger   log.Logger
}

func newIdentityHook(tr *TaskRunner, logger log.Logger) *identityHook {
	h := &identityHook{
		tr:       tr,
		tokenDir: tr.taskDir.SecretsDir,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {

	// Handle default workload identity
	if err := h.setDefaultToken(); err != nil {
		return err
	}

	signedWIDs, err := h.getIdentities(req.Alloc, req.Task)
	if err != nil {
		return fmt.Errorf("error fetching alternate identities: %w", err)
	}

	for _, widspec := range req.Task.Identities {
		signedWID := signedWIDs[widspec.Name]
		if signedWID == nil {
			// The only way to hit this should be a bug as it indicates the server
			// did not sign an identity for a task on this alloc.
			return fmt.Errorf("missing workload identity %q", widspec.Name)
		}

		if err := h.setAltToken(widspec, signedWID.JWT); err != nil {
			return err
		}
	}

	return nil
}

// setDefaultToken adds the Nomad token to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *identityHook) setDefaultToken() error {
	token := h.tr.alloc.SignedIdentities[h.tr.taskName]
	if token == "" {
		return nil
	}

	// Handle internal use and env var
	h.tr.setNomadToken(token)

	task := h.tr.Task()

	// Handle file writing
	if id := task.Identity; id != nil && id.File {
		// Write token as owner readable only
		tokenPath := filepath.Join(h.tokenDir, wiTokenFile)
		if err := users.WriteFileFor(tokenPath, []byte(token), task.User); err != nil {
			return fmt.Errorf("failed to write nomad token: %w", err)
		}
	}

	return nil
}

// setAltToken takes an alternate workload identity and sets the env var and/or
// writes the token file as specified by the jobspec.
func (h *identityHook) setAltToken(widspec *structs.WorkloadIdentity, rawJWT string) error {
	if widspec.Env {
		h.tr.envBuilder.SetWorkloadToken(widspec.Name, rawJWT)
	}

	if widspec.File {
		tokenPath := filepath.Join(h.tokenDir, fmt.Sprintf("nomad_%s.jwt", widspec.Name))
		if err := users.WriteFileFor(tokenPath, []byte(rawJWT), h.tr.Task().User); err != nil {
			return fmt.Errorf("failed to write token for identity %q: %w", widspec.Name, err)
		}
	}

	return nil
}

// getIdentities calls Alloc.SignIdentities to get all of the identities for
// this workload signed. If there are no identities to be signed then (nil,
// nil) is returned.
func (h *identityHook) getIdentities(alloc *structs.Allocation, task *structs.Task) (map[string]*structs.SignedWorkloadIdentity, error) {

	if len(task.Identities) == 0 {
		return nil, nil
	}

	req := make([]*structs.WorkloadIdentityRequest, len(task.Identities))
	for i, widspec := range task.Identities {
		req[i] = &structs.WorkloadIdentityRequest{
			AllocID:      alloc.ID,
			TaskName:     task.Name,
			IdentityName: widspec.Name,
		}
	}

	// Get signed workload identities
	signedWIDs, err := h.tr.widmgr.SignIdentities(alloc.CreateIndex, req)
	if err != nil {
		return nil, err
	}

	// Index initial workload identities by name
	widMap := make(map[string]*structs.SignedWorkloadIdentity, len(signedWIDs))
	for _, wid := range signedWIDs {
		widMap[wid.IdentityName] = wid
	}

	return widMap, nil
}
