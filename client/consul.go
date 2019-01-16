package client

import (
	"github.com/hashicorp/nomad/command/agent/consul"
)

// ConsulServiceAPI is the interface the Nomad Client uses to register and
// remove services and checks from Consul.
type ConsulServiceAPI interface {
	RegisterTask(*consul.TaskServices) error
	RemoveTask(*consul.TaskServices)
	UpdateTask(old, newTask *consul.TaskServices) error
	AllocRegistrations(allocID string) (*consul.AllocRegistration, error)
}
