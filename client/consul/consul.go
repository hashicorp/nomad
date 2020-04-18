package consul

import (
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ConsulServiceAPI is the interface the Nomad Client uses to register and
// remove services and checks from Consul.
//
// ACL requirements
// - service:write
type ConsulServiceAPI interface {
	// RegisterWorkload with Consul. Adds all service entries and checks to Consul.
	RegisterWorkload(*consul.WorkloadServices) error

	// RemoveWorkload from Consul. Removes all service entries and checks.
	RemoveWorkload(*consul.WorkloadServices)

	// UpdateWorkload in Consul. Does not alter the service if only checks have
	// changed.
	UpdateWorkload(old, newTask *consul.WorkloadServices) error

	// AllocRegistrations returns the registrations for the given allocation.
	AllocRegistrations(allocID string) (*consul.AllocRegistration, error)

	// UpdateTTL is used to update the TTL of a check.
	UpdateTTL(id, output, status string) error
}

// TokenDeriverFunc takes an allocation and a set of tasks and derives a
// service identity token for each. Requests go through nomad server.
type TokenDeriverFunc func(*structs.Allocation, []string) (map[string]string, error)

// ServiceIdentityAPI is the interface the Nomad Client uses to request Consul
// Service Identity tokens through Nomad Server.
//
// ACL requirements
// - acl:write (used by Server only)
type ServiceIdentityAPI interface {
	// DeriveSITokens contacts the nomad server and requests consul service
	// identity tokens be generated for tasks in the allocation.
	DeriveSITokens(alloc *structs.Allocation, tasks []string) (map[string]string, error)
}
