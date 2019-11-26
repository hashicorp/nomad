package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

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

type serviceHookConfig struct {
	alloc  *structs.Allocation
	task   *structs.Task
	consul consul.ConsulServiceAPI

	// Restarter is a subset of the TaskLifecycle interface
	restarter agentconsul.WorkloadRestarter

	logger log.Logger
}

type serviceHook struct {
	consul    consul.ConsulServiceAPI
	allocID   string
	taskName  string
	restarter agentconsul.WorkloadRestarter
	logger    log.Logger

	// The following fields may be updated
	delay      time.Duration
	driverExec tinterfaces.ScriptExecutor
	driverNet  *drivers.DriverNetwork
	canary     bool
	services   []*structs.Service
	networks   structs.Networks
	taskEnv    *taskenv.TaskEnv

	// Since Update() may be called concurrently with any other hook all
	// hook methods must be fully serialized
	mu sync.Mutex
}

func newServiceHook(c serviceHookConfig) *serviceHook {
	h := &serviceHook{
		consul:    c.consul,
		allocID:   c.alloc.ID,
		taskName:  c.task.Name,
		services:  c.task.Services,
		restarter: c.restarter,
		delay:     c.task.ShutdownDelay,
	}

	// COMPAT(0.11): AllocatedResources was added in 0.9 so assume its set
	//               in 0.11.
	if c.alloc.AllocatedResources != nil {
		if res := c.alloc.AllocatedResources.Tasks[c.task.Name]; res != nil {
			h.networks = res.Networks
		}
	} else {
		if res := c.alloc.TaskResources[c.task.Name]; res != nil {
			h.networks = res.Networks
		}
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

	// Create task services struct with request's driver metadata
	workloadServices := h.getWorkloadServices()

	return h.consul.RegisterWorkload(workloadServices)
}

func (h *serviceHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create old task services struct with request's driver metadata as it
	// can't change due to Updates
	oldWorkloadServices := h.getWorkloadServices()

	// Store new updated values out of request
	canary := false
	if req.Alloc.DeploymentStatus != nil {
		canary = req.Alloc.DeploymentStatus.Canary
	}

	// COMPAT(0.11): AllocatedResources was added in 0.9 so assume its set
	//               in 0.11.
	var networks structs.Networks
	if req.Alloc.AllocatedResources != nil {
		if res := req.Alloc.AllocatedResources.Tasks[h.taskName]; res != nil {
			networks = res.Networks
		}
	} else {
		if res := req.Alloc.TaskResources[h.taskName]; res != nil {
			networks = res.Networks
		}
	}

	task := req.Alloc.LookupTask(h.taskName)
	if task == nil {
		return fmt.Errorf("task %q not found in updated alloc", h.taskName)
	}

	// Update service hook fields
	h.delay = task.ShutdownDelay
	h.taskEnv = req.TaskEnv
	h.services = task.Services
	h.networks = networks
	h.canary = canary

	// Create new task services struct with those new values
	newWorkloadServices := h.getWorkloadServices()

	return h.consul.UpdateWorkload(oldWorkloadServices, newWorkloadServices)
}

func (h *serviceHook) PreKilling(ctx context.Context, req *interfaces.TaskPreKillRequest, resp *interfaces.TaskPreKillResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Deregister before killing task
	h.deregister()

	// If there's no shutdown delay, exit early
	if h.delay == 0 {
		return nil
	}

	h.logger.Debug("waiting before killing task", "shutdown_delay", h.delay)
	select {
	case <-ctx.Done():
	case <-time.After(h.delay):
	}
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
	workloadServices := h.getWorkloadServices()
	h.consul.RemoveWorkload(workloadServices)

	// Canary flag may be getting flipped when the alloc is being
	// destroyed, so remove both variations of the service
	workloadServices.Canary = !workloadServices.Canary
	h.consul.RemoveWorkload(workloadServices)

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
		AllocID:       h.allocID,
		Task:          h.taskName,
		Restarter:     h.restarter,
		Services:      interpolatedServices,
		DriverExec:    h.driverExec,
		DriverNetwork: h.driverNet,
		Networks:      h.networks,
		Canary:        h.canary,
	}
}
