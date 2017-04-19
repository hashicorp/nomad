package client

import (
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ConsulServiceAPI is the interface the Nomad Client uses to register and
// remove services and checks from Consul.
type ConsulServiceAPI interface {
	RegisterTask(allocID string, task *structs.Task, exec driver.ScriptExecutor) error
	RemoveTask(allocID string, task *structs.Task)
	UpdateTask(allocID string, existing, newTask *structs.Task, exec driver.ScriptExecutor) error
}
