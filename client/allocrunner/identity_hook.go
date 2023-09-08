// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"

	log "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
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
	ar            *allocRunner
	hookResources *cstructs.AllocHookResources
	logger        log.Logger
}

func newIdentityHook(ar *allocRunner, hookResources *cstructs.AllocHookResources, logger log.Logger) *identityHook {
	h := &identityHook{
		ar:            ar,
		hookResources: hookResources,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prerun() error {
	for _, t := range h.ar.tasks {
		task := t.Task()
		if task == nil {
			// hitting this means a bug, but better safe than sorry
			continue
		}

		signedWIDs, err := h.getIdentities(h.ar.Alloc(), task)
		if err != nil {
			return fmt.Errorf("error fetching alternate identities: %w", err)
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
	signedWIDs, err := h.ar.widmgr.SignIdentities(alloc.CreateIndex, req)
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
