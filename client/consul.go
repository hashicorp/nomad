package client

import (
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ConsulServiceAPI is the interface the Nomad Client uses to register and
// remove services and checks from Consul.
type ConsulServiceAPI interface {
	RegisterTask(allocID string, task *structs.Task, exec consul.ScriptExecutor) error
	RemoveTask(allocID string, task *structs.Task)
	UpdateTask(allocID string, existing, newTask *structs.Task, exec consul.ScriptExecutor) error
}
