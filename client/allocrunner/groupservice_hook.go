package allocrunner

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	groupServiceHookName = "group_services"
)

type networkStatusGetter interface {
	NetworkStatus() *structs.AllocNetworkStatus
}

// groupServiceHook manages task group Consul service registration and
// deregistration.
type groupServiceHook struct {
	allocID             string
	jobID               string
	group               string
	restarter           agentconsul.WorkloadRestarter
	prerun              bool
	deregistered        bool
	networkStatusGetter networkStatusGetter
	shutdownDelayCtx    context.Context

	// namespace is the Nomad or Consul namespace in which service
	// registrations will be made. This field may be updated.
	namespace string

	// serviceRegWrapper is the handler wrapper that is used to perform service
	// and check registration and deregistration.
	serviceRegWrapper *wrapper.HandlerWrapper

	logger hclog.Logger

	// The following fields may be updated
	canary         bool
	services       []*structs.Service
	networks       structs.Networks
	ports          structs.AllocatedPorts
	taskEnvBuilder *taskenv.Builder
	delay          time.Duration

	// Since Update() may be called concurrently with any other hook all
	// hook methods must be fully serialized
	mu sync.Mutex
}

type groupServiceHookConfig struct {
	alloc               *structs.Allocation
	restarter           serviceregistration.WorkloadRestarter
	taskEnvBuilder      *taskenv.Builder
	networkStatusGetter networkStatusGetter
	shutdownDelayCtx    context.Context
	logger              hclog.Logger

	// namespace is the Nomad or Consul namespace in which service
	// registrations will be made.
	namespace string

	// serviceRegWrapper is the handler wrapper that is used to perform service
	// and check registration and deregistration.
	serviceRegWrapper *wrapper.HandlerWrapper
}

func newGroupServiceHook(cfg groupServiceHookConfig) *groupServiceHook {
	var shutdownDelay time.Duration
	tg := cfg.alloc.Job.LookupTaskGroup(cfg.alloc.TaskGroup)

	if tg.ShutdownDelay != nil {
		shutdownDelay = *tg.ShutdownDelay
	}

	h := &groupServiceHook{
		allocID:             cfg.alloc.ID,
		jobID:               cfg.alloc.JobID,
		group:               cfg.alloc.TaskGroup,
		restarter:           cfg.restarter,
		namespace:           cfg.namespace,
		taskEnvBuilder:      cfg.taskEnvBuilder,
		delay:               shutdownDelay,
		networkStatusGetter: cfg.networkStatusGetter,
		logger:              cfg.logger.Named(groupServiceHookName),
		serviceRegWrapper:   cfg.serviceRegWrapper,
		services:            tg.Services,
		shutdownDelayCtx:    cfg.shutdownDelayCtx,
	}

	if cfg.alloc.AllocatedResources != nil {
		h.networks = cfg.alloc.AllocatedResources.Shared.Networks
		h.ports = cfg.alloc.AllocatedResources.Shared.Ports
	}

	if cfg.alloc.DeploymentStatus != nil {
		h.canary = cfg.alloc.DeploymentStatus.Canary
	}

	return h
}

func (*groupServiceHook) Name() string {
	return groupServiceHookName
}

func (h *groupServiceHook) Prerun() error {
	h.mu.Lock()
	defer func() {
		// Mark prerun as true to unblock Updates
		h.prerun = true
		h.mu.Unlock()
	}()
	return h.preRunLocked()
}

// caller must hold h.lock
func (h *groupServiceHook) preRunLocked() error {
	if len(h.services) == 0 {
		return nil
	}

	services := h.getWorkloadServicesLocked()
	return h.serviceRegWrapper.RegisterWorkload(services)
}

// Update is run when a job submitter modifies service(s) (but not much else -
// otherwise a full alloc replacement would occur).
func (h *groupServiceHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	oldWorkloadServices := h.getWorkloadServicesLocked()

	// Store new updated values out of request
	canary := false
	if req.Alloc.DeploymentStatus != nil {
		canary = req.Alloc.DeploymentStatus.Canary
	}

	var networks structs.Networks
	if req.Alloc.AllocatedResources != nil {
		networks = req.Alloc.AllocatedResources.Shared.Networks
		h.ports = req.Alloc.AllocatedResources.Shared.Ports
	}

	tg := req.Alloc.Job.LookupTaskGroup(h.group)
	var shutdown time.Duration
	if tg.ShutdownDelay != nil {
		shutdown = *tg.ShutdownDelay
	}

	// Update group service hook fields
	h.networks = networks
	h.services = tg.Services
	h.canary = canary
	h.delay = shutdown
	h.taskEnvBuilder.UpdateTask(req.Alloc, nil)

	// An update may change the service provider, therefore we need to account
	// for how namespaces work across providers also.
	h.namespace = req.Alloc.ServiceProviderNamespace()

	// Create new task services struct with those new values
	newWorkloadServices := h.getWorkloadServicesLocked()

	if !h.prerun {
		// Update called before Prerun. Update alloc and exit to allow
		// Prerun to do initial registration.
		return nil
	}

	return h.serviceRegWrapper.UpdateWorkload(oldWorkloadServices, newWorkloadServices)
}

func (h *groupServiceHook) PreTaskRestart() error {
	h.mu.Lock()
	defer func() {
		// Mark prerun as true to unblock Updates
		h.prerun = true
		h.mu.Unlock()
	}()

	h.preKillLocked()
	return h.preRunLocked()
}

func (h *groupServiceHook) PreKill() {
	helper.WithLock(&h.mu, h.preKillLocked)
}

// implements the PreKill hook
//
// caller must hold h.lock
func (h *groupServiceHook) preKillLocked() {
	// If we have a shutdown delay deregister group services and then wait
	// before continuing to kill tasks.
	h.deregisterLocked()

	if h.delay == 0 {
		return
	}

	h.logger.Debug("delay before killing tasks", "group", h.group, "shutdown_delay", h.delay)

	select {
	// Wait for specified shutdown_delay unless ignored
	// This will block an agent from shutting down.
	case <-time.After(h.delay):
	case <-h.shutdownDelayCtx.Done():
	}
}

func (h *groupServiceHook) Postrun() error {
	helper.WithLock(&h.mu, h.deregisterLocked)
	return nil
}

// deregisterLocked will deregister services from Consul/Nomad service provider.
//
// caller must hold h.lock
func (h *groupServiceHook) deregisterLocked() {
	if h.deregistered {
		return
	}

	if len(h.services) > 0 {
		workloadServices := h.getWorkloadServicesLocked()
		h.serviceRegWrapper.RemoveWorkload(workloadServices)
	}

	h.deregistered = true
}

// getWorkloadServicesLocked returns the set of workload services currently
// on the hook.
//
// caller must hold h.lock
func (h *groupServiceHook) getWorkloadServicesLocked() *serviceregistration.WorkloadServices {
	// Interpolate with the task's environment
	interpolatedServices := taskenv.InterpolateServices(h.taskEnvBuilder.Build(), h.services)

	var netStatus *structs.AllocNetworkStatus
	if h.networkStatusGetter != nil {
		netStatus = h.networkStatusGetter.NetworkStatus()
	}

	// Create task services struct with request's driver metadata
	return &serviceregistration.WorkloadServices{
		AllocID:       h.allocID,
		JobID:         h.jobID,
		Group:         h.group,
		Namespace:     h.namespace,
		Restarter:     h.restarter,
		Services:      interpolatedServices,
		Networks:      h.networks,
		NetworkStatus: netStatus,
		Ports:         h.ports,
		Canary:        h.canary,
	}
}
