// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

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

type identityHook struct {
	tr       *TaskRunner
	allocID  string
	taskName string
	tokenDir string
	logger   log.Logger
	lock     sync.Mutex

	initialWIDs []structs.SignedWorkloadIdentity
}

func newIdentityHook(tr *TaskRunner, logger log.Logger, initialWIDs []structs.SignedWorkloadIdentity) *identityHook {
	h := &identityHook{
		tr:          tr,
		allocID:     tr.allocID,
		taskName:    tr.taskName,
		initialWIDs: initialWIDs,
		tokenDir:    tr.taskDir.SecretsDir,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	// Handle the default Nomad workload identity
	if err := h.setNomadToken(h.tokenDir, req.Alloc, req.Task.User); err != nil {
		return fmt.Errorf("error setting default workload identity: %w", err)
	}

	// Index initial workload identities by name
	signedWIDs := map[string]structs.SignedWorkloadIdentity{}
	for _, wid := range h.initialWIDs {
		signedWIDs[wid.IdentityName] = wid
	}

	// Handle alternate workload identities: using the intially provided WIDs if
	// possible and falling back to RPC.
	var missingSignedWIDs []string
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
			missingSignedWIDs = append(missingSignedWIDs, widspec.Name)
			continue
		}

		// We have the JWT!
		if widspec.Env {
			if resp.Env == nil {
				resp.Env = make(map[string]string)
			}

			resp.Env[fmt.Sprintf("NOMAD_TOKEN_%s", widspec.Name)] = signedWID.JWT
		}
		if widspec.File {
			tokenPath := filepath.Join(h.tokenDir, fmt.Sprintf("nomad_%s.jwt", widspec.Name))
			if err := users.WriteFileFor(tokenPath, []byte(signedWID.JWT), req.Task.User); err != nil {
				return fmt.Errorf("failed to write token for identity %q: %w", widspec.Name, err)
			}
		}
	}

	//TODO(schmichael) handle missingSignedWIDs

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
