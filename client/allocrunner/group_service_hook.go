// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	groupServiceHookName = "group_services"
)

// groupServiceHook manages task group Consul service registration and
// deregistration.
type groupServiceHook struct {
	allocID          string
	jobID            string
	group            string
	tg               *structs.TaskGroup
	namespace        string
	restarter        serviceregistration.WorkloadRestarter
	prerun           bool
	deregistered     bool
	networkStatus    structs.NetworkStatus
	shutdownDelayCtx context.Context

	// providerNamespace is the Nomad or Consul namespace in which service
	// registrations will be made. This field may be updated.
	providerNamespace string

	// serviceRegWrapper is the handler wrapper that is used to perform service
	// and check registration and deregistration.
	serviceRegWrapper *wrapper.HandlerWrapper

	hookResources *cstructs.AllocHookResources

	logger hclog.Logger

	// The following fields may be updated
	canary   bool
	services []*structs.Service
	networks structs.Networks
	ports    structs.AllocatedPorts
	delay    time.Duration

	// Since Update() may be called concurrently with any other hook all
	// hook methods must be fully serialized
	mu sync.Mutex
}

type groupServiceHookConfig struct {
	alloc            *structs.Allocation
	restarter        serviceregistration.WorkloadRestarter
	networkStatus    structs.NetworkStatus
	shutdownDelayCtx context.Context
	logger           hclog.Logger

	// providerNamespace is the Nomad or Consul namespace in which service
	// registrations will be made.
	providerNamespace string

	// serviceRegWrapper is the handler wrapper that is used to perform service
	// and check registration and deregistration.
	serviceRegWrapper *wrapper.HandlerWrapper

	hookResources *cstructs.AllocHookResources
}

func newGroupServiceHook(cfg groupServiceHookConfig) *groupServiceHook {
	var shutdownDelay time.Duration
	tg := cfg.alloc.Job.LookupTaskGroup(cfg.alloc.TaskGroup)

	if tg.ShutdownDelay != nil {
		shutdownDelay = *tg.ShutdownDelay
	}

	h := &groupServiceHook{
		allocID:           cfg.alloc.ID,
		jobID:             cfg.alloc.JobID,
		group:             cfg.alloc.TaskGroup,
		namespace:         cfg.alloc.Namespace,
		restarter:         cfg.restarter,
		providerNamespace: cfg.providerNamespace,
		delay:             shutdownDelay,
		networkStatus:     cfg.networkStatus,
		logger:            cfg.logger.Named(groupServiceHookName),
		serviceRegWrapper: cfg.serviceRegWrapper,
		services:          tg.Services,
		tg:                tg,
		hookResources:     cfg.hookResources,
		shutdownDelayCtx:  cfg.shutdownDelayCtx,
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

// statically assert the hook implements the expected interfaces
var (
	_ interfaces.RunnerPrerunHook      = (*groupServiceHook)(nil)
	_ interfaces.RunnerPreKillHook     = (*groupServiceHook)(nil)
	_ interfaces.RunnerPostrunHook     = (*groupServiceHook)(nil)
	_ interfaces.RunnerUpdateHook      = (*groupServiceHook)(nil)
	_ interfaces.RunnerTaskRestartHook = (*groupServiceHook)(nil)
)

func (*groupServiceHook) Name() string {
	return groupServiceHookName
}

func (h *groupServiceHook) Prerun(allocEnv *taskenv.TaskEnv) error {
	h.mu.Lock()
	defer func() {
		// Mark prerun as true to unblock Updates
		h.prerun = true
		// Mark deregistered as false to allow de-registration
		h.deregistered = false
		h.mu.Unlock()
	}()

	return h.preRunLocked(allocEnv)
}

// caller must hold h.mu
func (h *groupServiceHook) preRunLocked(env *taskenv.TaskEnv) error {
	if len(h.services) == 0 {
		return nil
	}

	// TODO(tgross): this will be nil in PreTaskKill method because we have some
	// false sharing of this method
	if env != nil {
		h.services = taskenv.InterpolateServices(env, h.services)
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
	h.services = taskenv.InterpolateServices(req.AllocEnv, tg.Services)

	h.canary = canary
	h.delay = shutdown

	// An update may change the service provider, therefore we need to account
	// for how namespaces work across providers also.
	h.providerNamespace = req.Alloc.ServiceProviderNamespace()

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
		// Mark deregistered as false to allow de-registration
		h.deregistered = false
		h.mu.Unlock()
	}()

	h.preKillLocked()
	return h.preRunLocked(nil)
}

func (h *groupServiceHook) PreKill() {
	helper.WithLock(&h.mu, h.preKillLocked)
}

// implements the PreKill hook
//
// caller must hold h.mu
func (h *groupServiceHook) preKillLocked() {
	// Optimization: If this allocation has already been deregistered,
	// skip waiting for the shutdown_delay again.
	if h.deregistered && h.delay != 0 {
		h.logger.Debug("tasks already deregistered, skipping shutdown_delay")
		return
	}

	// If we have a shutdown delay deregister group services and then wait
	// before continuing to kill tasks.
	h.deregisterLocked()

	if h.delay == 0 {
		return
	}

	h.logger.Debug("delay before killing tasks", "group", h.group, "shutdown_delay", h.delay)

	timer, cancel := helper.NewSafeTimer(h.delay)
	defer cancel()

	select {
	// Wait for specified shutdown_delay unless ignored
	// This will block an agent from shutting down.
	case <-timer.C:
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
	allocTokens := h.hookResources.GetConsulTokens()

	tokens := map[string]string{}
	for _, service := range h.services {
		cluster := service.GetConsulClusterName(h.tg)
		if token, ok := allocTokens[cluster][service.MakeUniqueIdentityName()]; ok {
			tokens[service.Name] = token.SecretID
		}
	}

	var netStatus *structs.AllocNetworkStatus
	if h.networkStatus != nil {
		netStatus = h.networkStatus.NetworkStatus()
	}

	info := structs.AllocInfo{
		AllocID:   h.allocID,
		JobID:     h.jobID,
		Group:     h.group,
		Namespace: h.namespace,
	}

	// Create task services struct with request's driver metadata
	return &serviceregistration.WorkloadServices{
		AllocInfo:         info,
		ProviderNamespace: h.providerNamespace,
		Restarter:         h.restarter,
		Services:          h.services,
		Networks:          h.networks,
		NetworkStatus:     netStatus,
		Ports:             h.ports,
		Canary:            h.canary,
		Tokens:            tokens,
	}
}
