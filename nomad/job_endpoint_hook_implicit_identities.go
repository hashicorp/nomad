// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	consulServiceIdentityNamePrefix = "consul-service"
	vaultIdentityName               = "vault"
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
			h.handleVault(t)
		}
	}

	return job, nil, nil
}

// handleConsulService injects a workload identity to the service if:
//  1. The service uses the Consul provider.
//  2. The server is configured with `consul.use_identity = true` and a
//     `consul.service_identity` is provided.
//
// If the service already has an identity it sets the identity name and service
// name values.
func (h jobImplicitIdentitiesHook) handleConsulService(s *structs.Service) {
	if !h.srv.config.UseConsulIdentity() {
		return
	}

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
	serviceWID.Name = fmt.Sprintf("%s/%s", consulServiceIdentityNamePrefix, s.Name)
	serviceWID.ServiceName = s.Name

	s.Identity = serviceWID
}

// handleVault injects a workload identity to the task if:
//  1. The task has a Vault block.
//  2. The server is configures with `vault.use_identity = true` and a
//     `vault.default_identity` is provided.
//
// If the task already has an identity named `vault` it sets the identity name
// to the expected value.
func (h jobImplicitIdentitiesHook) handleVault(t *structs.Task) {
	if !h.srv.config.UseVaultIdentity() {
		return
	}

	if t.Vault == nil {
		return
	}

	// Use the Vault identity specified in the task.
	for _, wid := range t.Identities {
		if wid.Name == vaultIdentityName {
			return
		}
	}

	// If the task doesn't specify an identity for Vault, fallback to the
	// default identity defined in the server configuration.
	vaultWID := h.srv.config.VaultDefaultIdentity()
	if vaultWID == nil {
		// If no identity is found skip inject the implicit identity and
		// fallback to the legacy flow.
		return
	}

	// Set the expected identity name and inject it into the task.
	vaultWID.Name = vaultIdentityName
	t.Identities = append(t.Identities, vaultWID)
}
