package consul

import (
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// WorkloadServices describes services defined in either a Task or TaskGroup
// that need to be syncronized with Consul
type WorkloadServices struct {
	AllocID string

	// Name of the task and task group the services are defined for. For
	// group based services, Task will be empty
	Task  string
	Group string

	// Canary indicates whether or not the allocation is a canary
	Canary bool

	// Restarter allows restarting the task or task group depending on the
	// check_restart stanzas.
	Restarter WorkloadRestarter

	// Services and checks to register for the task.
	Services []*structs.Service

	// Networks from the task's resources stanza.
	Networks structs.Networks

	// DriverExec is the script executor for the task's driver.
	// For group services this is nil and script execution is managed by
	// a tasklet in the taskrunner script_check_hook
	DriverExec interfaces.ScriptExecutor

	// DriverNetwork is the network specified by the driver and may be nil.
	DriverNetwork *drivers.DriverNetwork
}

func BuildAllocServices(node *structs.Node, alloc *structs.Allocation, restarter WorkloadRestarter) *WorkloadServices {

	//TODO(schmichael) only support one network for now
	net := alloc.AllocatedResources.Shared.Networks[0]

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)

	ws := &WorkloadServices{
		AllocID:  alloc.ID,
		Group:    alloc.TaskGroup,
		Services: taskenv.InterpolateServices(taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region).Build(), tg.Services),
		Networks: alloc.AllocatedResources.Shared.Networks,

		//TODO(schmichael) there's probably a better way than hacking driver network
		DriverNetwork: &drivers.DriverNetwork{
			AutoAdvertise: true,
			IP:            net.IP,
			// Copy PortLabels from group network
			PortMap: net.PortLabels(),
		},

		Restarter:  restarter,
		DriverExec: nil,
	}

	if alloc.DeploymentStatus != nil {
		ws.Canary = alloc.DeploymentStatus.Canary
	}

	return ws
}

// Copy method for easing tests
func (t *WorkloadServices) Copy() *WorkloadServices {
	newTS := new(WorkloadServices)
	*newTS = *t

	// Deep copy Services
	newTS.Services = make([]*structs.Service, len(t.Services))
	for i := range t.Services {
		newTS.Services[i] = t.Services[i].Copy()
	}
	return newTS
}

func (w *WorkloadServices) Name() string {
	if w.Task != "" {
		return w.Task
	}

	return "group-" + w.Group
}
