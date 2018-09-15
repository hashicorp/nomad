package taskrunner

import (
	"context"
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/env"
	cstructs "github.com/hashicorp/nomad/client/structs"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

type serviceHookConfig struct {
	alloc  *structs.Allocation
	task   *structs.Task
	consul consul.ConsulServiceAPI

	// Restarter is a subset of the TaskLifecycle interface
	restarter agentconsul.TaskRestarter

	logger log.Logger
}

type serviceHook struct {
	consul    consul.ConsulServiceAPI
	allocID   string
	taskName  string
	restarter agentconsul.TaskRestarter
	logger    log.Logger

	// The following fields may be updated
	driverExec driver.ScriptExecutor
	driverNet  *cstructs.DriverNetwork
	canary     bool
	services   []*structs.Service
	networks   structs.Networks
	taskEnv    *env.TaskEnv

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
	}

	if res := c.alloc.TaskResources[c.task.Name]; res != nil {
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

	// Create task services struct with request's driver metadata
	taskServices := h.getTaskServices()

	return h.consul.RegisterTask(taskServices)
}

func (h *serviceHook) Update(ctx context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create old task services struct with request's driver metadata as it
	// can't change due to Updates
	oldTaskServices := h.getTaskServices()

	// Store new updated values out of request
	canary := false
	if req.Alloc.DeploymentStatus != nil {
		canary = req.Alloc.DeploymentStatus.Canary
	}

	var networks structs.Networks
	if res := req.Alloc.TaskResources[h.taskName]; res != nil {
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

	// Create new task services struct with those new values
	newTaskServices := h.getTaskServices()

	return h.consul.UpdateTask(oldTaskServices, newTaskServices)
}

func (h *serviceHook) Exited(context.Context, *interfaces.TaskExitedRequest, *interfaces.TaskExitedResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	taskServices := h.getTaskServices()
	h.consul.RemoveTask(taskServices)

	// Canary flag may be getting flipped when the alloc is being
	// destroyed, so remove both variations of the service
	taskServices.Canary = !taskServices.Canary
	h.consul.RemoveTask(taskServices)

	return nil
}

func (h *serviceHook) getTaskServices() *agentconsul.TaskServices {
	// Interpolate with the task's environment
	interpolatedServices := interpolateServices(h.taskEnv, h.services)

	// Create task services struct with request's driver metadata
	return &agentconsul.TaskServices{
		AllocID:       h.allocID,
		Name:          h.taskName,
		Restarter:     h.restarter,
		Services:      interpolatedServices,
		DriverExec:    h.driverExec,
		DriverNetwork: h.driverNet,
		Networks:      h.networks,
		Canary:        h.canary,
	}
}

// interpolateServices returns an interpolated copy of services and checks with
// values from the task's environment.
func interpolateServices(taskEnv *env.TaskEnv, services []*structs.Service) []*structs.Service {
	interpolated := make([]*structs.Service, len(services))

	for i, origService := range services {
		// Create a copy as we need to reinterpolate every time the
		// environment changes
		service := origService.Copy()

		for _, check := range service.Checks {
			check.Name = taskEnv.ReplaceEnv(check.Name)
			check.Type = taskEnv.ReplaceEnv(check.Type)
			check.Command = taskEnv.ReplaceEnv(check.Command)
			check.Args = taskEnv.ParseAndReplace(check.Args)
			check.Path = taskEnv.ReplaceEnv(check.Path)
			check.Protocol = taskEnv.ReplaceEnv(check.Protocol)
			check.PortLabel = taskEnv.ReplaceEnv(check.PortLabel)
			check.InitialStatus = taskEnv.ReplaceEnv(check.InitialStatus)
			check.Method = taskEnv.ReplaceEnv(check.Method)
			check.GRPCService = taskEnv.ReplaceEnv(check.GRPCService)
			if len(check.Header) > 0 {
				header := make(map[string][]string, len(check.Header))
				for k, vs := range check.Header {
					newVals := make([]string, len(vs))
					for i, v := range vs {
						newVals[i] = taskEnv.ReplaceEnv(v)
					}
					header[taskEnv.ReplaceEnv(k)] = newVals
				}
				check.Header = header
			}
		}

		service.Name = taskEnv.ReplaceEnv(service.Name)
		service.PortLabel = taskEnv.ReplaceEnv(service.PortLabel)
		service.Tags = taskEnv.ParseAndReplace(service.Tags)
		service.CanaryTags = taskEnv.ParseAndReplace(service.CanaryTags)
		interpolated[i] = service
	}

	return interpolated
}
