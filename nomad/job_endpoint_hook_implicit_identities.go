// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// jobImplicitIdentitiesHook adds implicit `identity` blocks for external
// services, like Consul and Vault.
type jobImplicitIdentitiesHook struct {
	srv *Server
}

func (jobImplicitIdentitiesHook) Name() string {
	return "implicit-identities"
}

func (h jobImplicitIdentitiesHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {

	for _, tg := range job.TaskGroups {
		for _, s := range tg.Services {
			h.handleConsulService(s)
		}

		for _, t := range tg.Tasks {
			for _, s := range t.Services {
				h.handleConsulService(s)
			}
			if len(t.Templates) > 0 {
				h.handleConsulTasks(t)
			}
			h.handleVault(t)
		}
	}

	return job, nil, nil
}

// handleConsulService injects a workload identity to the service if:
//  1. The service uses the Consul provider, and
//  2. The server is configured with `consul.service_identity`
//
// If the service already has an identity the server sets the identity name and
// service name values.
func (h jobImplicitIdentitiesHook) handleConsulService(s *structs.Service) {
	if s.Provider != "" && s.Provider != "consul" {
		return
	}

	// Use the identity specified in the service.
	serviceWID := s.Identity
	if serviceWID == nil {
		// If the service doesn't specify an identity, fallback to the service
		// identity defined in the server configuration.
		serviceWID = h.srv.config.ConsulServiceIdentity()
		if serviceWID == nil {
			// If no identity is found, skip injecting the implicit identity
			// and fallback to the legacy flow.
			return
		}
	}

	// Set the expected identity name and service name.
	serviceWID.Name = s.MakeUniqueIdentityName()
	serviceWID.ServiceName = s.Name

	s.Identity = serviceWID
}

func (h jobImplicitIdentitiesHook) handleConsulTasks(t *structs.Task) {
	widName := t.Consul.IdentityName()

	// Use the Consul identity specified in the task if present
	for _, wid := range t.Identities {
		if wid.Name == widName {
			return
		}
	}

	// If task doesn't specify an identity for Consul, fallback to the
	// default identity defined in the server configuration.
	taskWID := h.srv.config.ConsulTaskIdentity()
	if taskWID == nil {
		// If no identity is found skip inject the implicit identity and
		// fallback to the legacy flow.
		return
	}
	taskWID.Name = widName
	t.Identities = append(t.Identities, taskWID)
}

// handleVault injects a workload identity to the task if:
//  1. The task has a Vault block.
//  2. The task does not have an identity for the Vault cluster.
//  3. The server is configured with a `vault.default_identity`.
func (h jobImplicitIdentitiesHook) handleVault(t *structs.Task) {
	if t.Vault == nil {
		return
	}

	// Use the Vault identity specified in the task.
	vaultWIDName := t.Vault.IdentityName()
	vaultWID := t.GetIdentity(vaultWIDName)
	if vaultWID != nil {
		return
	}

	// If the task doesn't specify an identity for Vault, fallback to the
	// default identity defined in the server configuration.
	vaultWID = h.srv.config.VaultIdentityConfig(t.Vault.Cluster)
	if vaultWID == nil {
		// If no identity is found skip inject the implicit identity and
		// fallback to the legacy flow.
		return
	}

	// Set the expected identity name and inject it into the task.
	vaultWID.Name = vaultWIDName
	t.Identities = append(t.Identities, vaultWID)
}
