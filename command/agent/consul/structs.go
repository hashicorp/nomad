package consul

import (
	"github.com/hashicorp/nomad/client/driver"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

type TaskServices struct {
	AllocID string
	// Index of the task
	AllocIndex int
	// Name of the task
	Name string

	// Canary indicates whether or not the allocation is a canary
	Canary bool

	// Restarter allows restarting the task depending on the task's
	// check_restart stanzas.
	Restarter TaskRestarter

	// Services and checks to register for the task.
	Services []*structs.Service

	// Networks from the task's resources stanza.
	Networks structs.Networks

	// DriverExec is the script executor for the task's driver.
	DriverExec driver.ScriptExecutor

	// DriverNetwork is the network specified by the driver and may be nil.
	DriverNetwork *cstructs.DriverNetwork
}

func NewTaskServices(alloc *structs.Allocation, task *structs.Task, restarter TaskRestarter, exec driver.ScriptExecutor, net *cstructs.DriverNetwork) *TaskServices {
	ts := TaskServices{
		AllocID:       alloc.ID,
		AllocIndex:    int(alloc.Index()),
		Name:          task.Name,
		Restarter:     restarter,
		Services:      task.Services,
		DriverExec:    exec,
		DriverNetwork: net,
	}

	if task.Resources != nil {
		ts.Networks = task.Resources.Networks
	}

	if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Canary {
		ts.Canary = true
	}

	return &ts
}

// Copy method for easing tests
func (t *TaskServices) Copy() *TaskServices {
	newTS := new(TaskServices)
	*newTS = *t

	// Deep copy Services
	newTS.Services = make([]*structs.Service, len(t.Services))
	for i := range t.Services {
		newTS.Services[i] = t.Services[i].Copy()
	}
	return newTS
}
