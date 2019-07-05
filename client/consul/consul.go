package consul

import (
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ConsulServiceAPI is the interface the Nomad Client uses to register and
// remove services and checks from Consul.
type ConsulServiceAPI interface {
	RegisterAlloc(*structs.Allocation) error
	RemoveAlloc(*structs.Allocation) error
	RegisterTask(*consul.TaskServices) error
	RemoveTask(*consul.TaskServices)
	UpdateTask(old, newTask *consul.TaskServices) error
	AllocRegistrations(allocID string) (*consul.AllocRegistration, error)
}
