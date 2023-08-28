package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// jobImplicitIdentityHook adds implicit `identity` blocks for external services,
// like Consul and Vault.
type jobImplicitIdentityHook struct {
	srv *Server
}

func (jobImplicitIdentityHook) Name() string {
	return "implicit-identity"
}

func (h jobImplicitIdentityHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	for _, tg := range job.TaskGroups {
		for _, s := range tg.Services {
			h.handleConsulService(s)
		}

		for _, t := range tg.Tasks {
			h.handleVault(t)

			for _, s := range t.Services {
				h.handleConsulService(s)
			}
		}
	}

	return job, nil, nil
}

func (h jobImplicitIdentityHook) handleVault(task *structs.Task) {
	for _, wid := range task.Identities {
		if wid.Name == "vault" {
			return
		}
	}

	vaultWID := h.srv.config.DefaultVaultIdentity()
	if vaultWID == nil {
		return
	}

	task.Identities = append(task.Identities, vaultWID)
}

func (h jobImplicitIdentityHook) handleConsulService(s *structs.Service) {
	if s.Provider != "consul" {
		return
	}

	serviceWID := s.Identity
	if serviceWID == nil {
		serviceWID = h.srv.config.DefaultConsulServiceIdentity()
		if serviceWID == nil {
			return
		}
	}

	serviceWID.Name = fmt.Sprintf("consul-service/%s", s.Name)
	serviceWID.ServiceName = s.Name

	s.Identity = serviceWID
}
