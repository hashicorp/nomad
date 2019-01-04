package consul

import (
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

type TaskServices struct {
	AllocID string

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
	DriverExec interfaces.ScriptExecutor

	// DriverNetwork is the network specified by the driver and may be nil.
	DriverNetwork *drivers.DriverNetwork
}

func NewTaskServices(alloc *structs.Allocation, task *structs.Task, restarter TaskRestarter, exec interfaces.ScriptExecutor, net *drivers.DriverNetwork) *TaskServices {
	ts := TaskServices{
		AllocID:       alloc.ID,
		Name:          task.Name,
		Restarter:     restarter,
		Services:      task.Services,
		DriverExec:    exec,
		DriverNetwork: net,
	}

	if alloc.AllocatedResources != nil {
		if tr, ok := alloc.AllocatedResources.Tasks[task.Name]; ok {
			ts.Networks = tr.Networks
		}
	} else if task.Resources != nil {
		// COMPAT(0.11): Remove in 0.11
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
