package allocrunner

import (
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// groupServiceHook manages task group Consul service registration and
// deregistration.
type groupServiceHook struct {
	allocID      string
	group        string
	restarter    agentconsul.WorkloadRestarter
	consulClient consul.ConsulServiceAPI
	prerun       bool

	logger log.Logger

	// The following fields may be updated
	driverNet *drivers.DriverNetwork
	canary    bool
	services  []*structs.Service
	networks  structs.Networks
	taskEnv   *taskenv.TaskEnv

	// Since Update() may be called concurrently with any other hook all
	// hook methods must be fully serialized
	mu sync.Mutex
}

type groupServiceHookConfig struct {
	alloc     *structs.Allocation
	consul    consul.ConsulServiceAPI
	restarter agentconsul.WorkloadRestarter
	taskEnv   *taskenv.TaskEnv
	logger    log.Logger
}

func newGroupServiceHook(cfg groupServiceHookConfig) *groupServiceHook {
	h := &groupServiceHook{
		allocID:      cfg.alloc.ID,
		group:        cfg.alloc.TaskGroup,
		restarter:    cfg.restarter,
		consulClient: cfg.consul,
		taskEnv:      cfg.taskEnv,
	}
	h.logger = cfg.logger.Named(h.Name())
	h.services = cfg.alloc.Job.LookupTaskGroup(h.group).Services
	h.networks = cfg.alloc.AllocatedResources.Shared.Networks

	//TODO(schmichael) only support one network for now
	net := cfg.alloc.AllocatedResources.Shared.Networks[0]
	//TODO(schmichael) there's probably a better way than hacking driver network
	h.driverNet = &drivers.DriverNetwork{
		AutoAdvertise: true,
		IP:            net.IP,
		// Copy PortLabels from group network
		PortMap: net.PortLabels(),
	}

	if cfg.alloc.DeploymentStatus != nil {
		h.canary = cfg.alloc.DeploymentStatus.Canary
	}
	return h
}

func (*groupServiceHook) Name() string {
	return "group_services"
}

func (h *groupServiceHook) Prerun() error {
	h.mu.Lock()
	defer func() {
		// Mark prerun as true to unblock Updates
		h.prerun = true
		h.mu.Unlock()
	}()
	services := h.getWorkloadServices()
	return h.consulClient.RegisterWorkload(services)
}

func (h *groupServiceHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	oldWorkloadServices := h.getWorkloadServices()

	// Store new updated values out of request
	canary := false
	if req.Alloc.DeploymentStatus != nil {
		canary = req.Alloc.DeploymentStatus.Canary
	}

	var networks structs.Networks
	if req.Alloc.AllocatedResources != nil {
		networks = req.Alloc.AllocatedResources.Shared.Networks
	}

	// Update group service hook fields
	h.networks = networks
	h.services = req.Alloc.Job.LookupTaskGroup(h.group).Services
	h.canary = canary
	h.taskEnv = taskenv.NewBuilder(req.Node, req.Alloc, nil, req.Alloc.Job.Region).Build()

	// Create new task services struct with those new values
	newWorkloadServices := h.getWorkloadServices()

	if !h.prerun {
		// Update called before Prerun. Update alloc and exit to allow
		// Prerun to do initial registration.
		return nil
	}

	return h.consulClient.UpdateWorkload(oldWorkloadServices, newWorkloadServices)
}

func (h *groupServiceHook) Postrun() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deregister()
	return nil
}

// deregister services from Consul.
func (h *groupServiceHook) deregister() {
	workloadServices := h.getWorkloadServices()
	h.consulClient.RemoveWorkload(workloadServices)

	// Canary flag may be getting flipped when the alloc is being
	// destroyed, so remove both variations of the service
	workloadServices.Canary = !workloadServices.Canary
	h.consulClient.RemoveWorkload(workloadServices)

}
func (h *groupServiceHook) getWorkloadServices() *agentconsul.WorkloadServices {
	// Interpolate with the task's environment
	interpolatedServices := taskenv.InterpolateServices(h.taskEnv, h.services)

	// Create task services struct with request's driver metadata
	return &agentconsul.WorkloadServices{
		AllocID:       h.allocID,
		Group:         h.group,
		Restarter:     h.restarter,
		Services:      interpolatedServices,
		DriverNetwork: h.driverNet,
		Networks:      h.networks,
		Canary:        h.canary,
	}
}
