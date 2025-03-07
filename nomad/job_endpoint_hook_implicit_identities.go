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
		var hasIdentity bool

		for _, s := range tg.Services {
			h.handleConsulService(s, tg)
			hasIdentity = hasIdentity || s.Identity != nil
		}

		for _, t := range tg.Tasks {
			for _, s := range t.Services {
				h.handleConsulService(s, tg)
				hasIdentity = hasIdentity || s.Identity != nil
			}

			h.handleConsulTask(t, tg)
			hasIdentity = hasIdentity || (len(t.Identities) > 0)

			h.handleVault(t)
			hasIdentity = hasIdentity || (len(t.Identities) > 0)
		}

		if hasIdentity {
			tg.Constraints = append(tg.Constraints, implicitIdentityClientVersionConstraint())
		}
	}

	return job, nil, nil
}

// implicitIdentityClientVersionConstraint is used when the client needs to
// support a workload identity workflow for Consul or Vault, or multiple
// identities in general.
func implicitIdentityClientVersionConstraint() *structs.Constraint {
	// "-a" is used here so that it is "less than" all pre-release versions of
	// Nomad 1.7.0 as well
	return &structs.Constraint{
		LTarget: "${attr.nomad.version}",
		RTarget: ">= 1.7.0-a",
		Operand: structs.ConstraintSemver,
	}
}

// handleConsulService injects a workload identity to the service if:
//  1. The service uses the Consul provider, and
//  2. The server is configured with `consul.service_identity`
//
// If the service already has an identity the server sets the identity name and
// service name values.
func (h jobImplicitIdentitiesHook) handleConsulService(s *structs.Service, tg *structs.TaskGroup) {
	if s.Provider != "" && s.Provider != "consul" {
		return
	}

	// Use the identity specified in the service.
	serviceWID := s.Identity
	if serviceWID == nil {
		// If the service doesn't specify an identity, fallback to the service
		// identity defined in the server configuration.
		serviceWID = h.srv.config.ConsulServiceIdentity(s.GetConsulClusterName(tg))
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

// handleConsulTask injects a workload identity into the task for Consul if the
// task or task group includes a Consul block. The identity is generated in the
// following priority list:
//
//  1. A Consul identity configured in the task by an identity block.
//  2. Generated using the Consul block at the task level.
//  3. Generated using the Consul block at the task group level.
func (h jobImplicitIdentitiesHook) handleConsulTask(t *structs.Task, tg *structs.TaskGroup) {

	// If neither the task nor task group includes a Consul block, exit as we
	// do not need to generate an identity. Operators can still specify
	// identity blocks for Consul tasks which will allow workload access to the
	// Consul API.
	if t.Consul == nil && tg.Consul == nil {
		return
	}

	widName := t.Consul.IdentityName()

	// Use the Consul identity specified in the task if present
	for _, wid := range t.Identities {
		if wid.Name == widName {
			return
		}
	}

	// If task doesn't specify an identity for Consul, fallback to the
	// default identity defined in the server configuration.
	taskWID := h.srv.config.ConsulTaskIdentity(t.GetConsulClusterName(tg))
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
	vaultWID = h.srv.config.VaultIdentityConfig(t.GetVaultClusterName())
	if vaultWID == nil {
		// If no identity is found skip inject the implicit identity and
		// fallback to the legacy flow.
		return
	}

	// Set the expected identity name and inject it into the task.
	vaultWID.Name = vaultWIDName
	t.Identities = append(t.Identities, vaultWID)
}
