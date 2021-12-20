package allocrunner

import (
	"context"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
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
	group               string
	restarter           agentconsul.WorkloadRestarter
	consulClient        consul.ConsulServiceAPI
	consulNamespace     string
	prerun              bool
	deregistered        bool
	networkStatusGetter networkStatusGetter
	shutdownDelayCtx    context.Context

	logger log.Logger

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
	consul              consul.ConsulServiceAPI
	consulNamespace     string
	restarter           agentconsul.WorkloadRestarter
	taskEnvBuilder      *taskenv.Builder
	networkStatusGetter networkStatusGetter
	shutdownDelayCtx    context.Context
	logger              log.Logger
}

func newGroupServiceHook(cfg groupServiceHookConfig) *groupServiceHook {
	var shutdownDelay time.Duration
	tg := cfg.alloc.Job.LookupTaskGroup(cfg.alloc.TaskGroup)

	if tg.ShutdownDelay != nil {
		shutdownDelay = *tg.ShutdownDelay
	}

	h := &groupServiceHook{
		allocID:             cfg.alloc.ID,
		group:               cfg.alloc.TaskGroup,
		restarter:           cfg.restarter,
		consulClient:        cfg.consul,
		consulNamespace:     cfg.consulNamespace,
		taskEnvBuilder:      cfg.taskEnvBuilder,
		delay:               shutdownDelay,
		networkStatusGetter: cfg.networkStatusGetter,
		logger:              cfg.logger.Named(groupServiceHookName),
		services:            cfg.alloc.Job.LookupTaskGroup(cfg.alloc.TaskGroup).Services,
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
	return h.prerunLocked()
}

func (h *groupServiceHook) prerunLocked() error {
	if len(h.services) == 0 {
		return nil
	}

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

	// Create new task services struct with those new values
	newWorkloadServices := h.getWorkloadServices()

	if !h.prerun {
		// Update called before Prerun. Update alloc and exit to allow
		// Prerun to do initial registration.
		return nil
	}

	return h.consulClient.UpdateWorkload(oldWorkloadServices, newWorkloadServices)
}

func (h *groupServiceHook) PreTaskRestart() error {
	h.mu.Lock()
	defer func() {
		// Mark prerun as true to unblock Updates
		h.prerun = true
		h.mu.Unlock()
	}()

	h.preKillLocked()
	return h.prerunLocked()
}

func (h *groupServiceHook) PreKill() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.preKillLocked()
}

// implements the PreKill hook but requires the caller hold the lock
func (h *groupServiceHook) preKillLocked() {
	// If we have a shutdown delay deregister group services and then wait
	// before continuing to kill tasks.
	h.deregister()
	h.deregistered = true

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
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.deregistered {
		h.deregister()
	}
	return nil
}

// deregister services from Consul.
func (h *groupServiceHook) deregister() {
	if len(h.services) > 0 {
		workloadServices := h.getWorkloadServices()
		h.consulClient.RemoveWorkload(workloadServices)
	}
}

func (h *groupServiceHook) getWorkloadServices() *agentconsul.WorkloadServices {
	// Interpolate with the task's environment
	interpolatedServices := taskenv.InterpolateServices(h.taskEnvBuilder.Build(), h.services)

	var netStatus *structs.AllocNetworkStatus
	if h.networkStatusGetter != nil {
		netStatus = h.networkStatusGetter.NetworkStatus()
	}

	// Create task services struct with request's driver metadata
	return &agentconsul.WorkloadServices{
		AllocID:         h.allocID,
		Group:           h.group,
		ConsulNamespace: h.consulNamespace,
		Restarter:       h.restarter,
		Services:        interpolatedServices,
		Networks:        h.networks,
		NetworkStatus:   netStatus,
		Ports:           h.ports,
		Canary:          h.canary,
	}
}
