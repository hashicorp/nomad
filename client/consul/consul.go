package consul

import (
	"github.com/hashicorp/nomad/command/agent/consul"
)

// ConsulServiceAPI is the interface the Nomad Client uses to register and
// remove services and checks from Consul.
type ConsulServiceAPI interface {
	RegisterWorkload(*consul.WorkloadServices) error
	RemoveWorkload(*consul.WorkloadServices)
	UpdateWorkload(old, newTask *consul.WorkloadServices) error
	AllocRegistrations(allocID string) (*consul.AllocRegistration, error)
	UpdateTTL(id, output, status string) error
}
