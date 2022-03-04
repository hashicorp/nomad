package taskrunner

import (
	"context"
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	tinterfaces "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

var _ interfaces.TaskPoststartHook = &serviceHook{}
var _ interfaces.TaskPreKillHook = &serviceHook{}
var _ interfaces.TaskExitedHook = &serviceHook{}
var _ interfaces.TaskStopHook = &serviceHook{}
var _ interfaces.TaskUpdateHook = &serviceHook{}

type serviceHookConfig struct {
	alloc           *structs.Allocation
	task            *structs.Task
	consulServices  consul.ConsulServiceAPI
	consulNamespace string

	// Restarter is a subset of the TaskLifecycle interface
	restarter agentconsul.WorkloadRestarter

	logger log.Logger
}

type serviceHook struct {
	allocID         string
	taskName        string
	consulNamespace string
	consulServices  consul.ConsulServiceAPI
	restarter       agentconsul.WorkloadRestarter
	logger          log.Logger

	// The following fields may be updated
	driverExec tinterfaces.ScriptExecutor
	driverNet  *drivers.DriverNetwork
	canary     bool
	services   []*structs.Service
	networks   structs.Networks
	ports      structs.AllocatedPorts
	taskEnv    *taskenv.TaskEnv

	// initialRegistrations tracks if Poststart has completed, initializing
	// fields required in other lifecycle funcs
	initialRegistration bool

	// deregistered tracks whether deregister() has previously been called, so
	// we do not call this multiple times for a single task when not needed.
	deregistered bool

	// Since Update() may be called concurrently with any other hook all
	// hook methods must be fully serialized
	mu sync.Mutex
}

func newServiceHook(c serviceHookConfig) *serviceHook {
	h := &serviceHook{
		allocID:         c.alloc.ID,
		taskName:        c.task.Name,
		consulServices:  c.consulServices,
		consulNamespace: c.consulNamespace,
		services:        c.task.Services,
		restarter:       c.restarter,
		ports:           c.alloc.AllocatedResources.Shared.Ports,
	}

	if res := c.alloc.AllocatedResources.Tasks[c.task.Name]; res != nil {
		h.networks = res.Networks
	}

	if c.alloc.DeploymentStatus != nil && c.alloc.DeploymentStatus.Canary {
		h.canary = true
	}

	h.logger = c.logger.Named(h.Name())
	return h
}

func (h *serviceHook) Name() string {
	return "consul_services"
}

func (h *serviceHook) Poststart(ctx context.Context, req *interfaces.TaskPoststartRequest, _ *interfaces.TaskPoststartResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Store the TaskEnv for interpolating now and when Updating
	h.driverExec = req.DriverExec
	h.driverNet = req.DriverNetwork
	h.taskEnv = req.TaskEnv
	h.initialRegistration = true

	// Ensure deregistered is unset.
	h.deregistered = false

	// Create task services struct with request's driver metadata
	workloadServices := h.getWorkloadServices()

	return h.consulServices.RegisterWorkload(workloadServices)
}

func (h *serviceHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.initialRegistration {
		// no op Consul since initial registration has not finished
		// only update hook fields
		return h.updateHookFields(req)
	}

	// Create old task services struct with request's driver metadata as it
	// can't change due to Updates
	oldWorkloadServices := h.getWorkloadServices()

	if err := h.updateHookFields(req); err != nil {
		return err
	}

	// Create new task services struct with those new values
	newWorkloadServices := h.getWorkloadServices()

	return h.consulServices.UpdateWorkload(oldWorkloadServices, newWorkloadServices)
}

func (h *serviceHook) updateHookFields(req *interfaces.TaskUpdateRequest) error {
	// Store new updated values out of request
	canary := false
	if req.Alloc.DeploymentStatus != nil {
		canary = req.Alloc.DeploymentStatus.Canary
	}

	var networks structs.Networks
	if res := req.Alloc.AllocatedResources.Tasks[h.taskName]; res != nil {
		networks = res.Networks
	}

	task := req.Alloc.LookupTask(h.taskName)
	if task == nil {
		return fmt.Errorf("task %q not found in updated alloc", h.taskName)
	}

	// Update service hook fields
	h.taskEnv = req.TaskEnv
	h.services = task.Services
	h.networks = networks
	h.canary = canary
	h.ports = req.Alloc.AllocatedResources.Shared.Ports

	return nil
}

func (h *serviceHook) PreKilling(ctx context.Context, req *interfaces.TaskPreKillRequest, resp *interfaces.TaskPreKillResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Deregister before killing task
	h.deregister()

	return nil
}

func (h *serviceHook) Exited(context.Context, *interfaces.TaskExitedRequest, *interfaces.TaskExitedResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deregister()
	return nil
}

// deregister services from Consul.
func (h *serviceHook) deregister() {
	if len(h.services) > 0 && !h.deregistered {
		workloadServices := h.getWorkloadServices()
		h.consulServices.RemoveWorkload(workloadServices)
	}
	h.initialRegistration = false
	h.deregistered = true
}

func (h *serviceHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deregister()
	return nil
}

func (h *serviceHook) getWorkloadServices() *agentconsul.WorkloadServices {
	// Interpolate with the task's environment
	interpolatedServices := taskenv.InterpolateServices(h.taskEnv, h.services)

	// Create task services struct with request's driver metadata
	return &agentconsul.WorkloadServices{
		AllocID:         h.allocID,
		Task:            h.taskName,
		ConsulNamespace: h.consulNamespace,
		Restarter:       h.restarter,
		Services:        interpolatedServices,
		DriverExec:      h.driverExec,
		DriverNetwork:   h.driverNet,
		Networks:        h.networks,
		Canary:          h.canary,
		Ports:           h.ports,
	}
}
