// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

var _ interfaces.RunnerPrerunHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerUpdateHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerPostrunHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerPreKillHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerTaskRestartHook = (*groupServiceHook)(nil)

// TestGroupServiceHook_NoGroupServices asserts calling group service hooks
// without group services does not error.
func TestGroupServiceHook_NoGroupServices(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Services = []*structs.Service{{
		Name:      "foo",
		Provider:  "consul",
		PortLabel: "9999",
	}}
	logger := testlog.HCLogger(t)

	consulMockClient := regMock.NewServiceRegistrationHandler(logger)

	regWrapper := wrapper.NewHandlerWrapper(
		logger,
		consulMockClient,
		regMock.NewServiceRegistrationHandler(logger))

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:             alloc,
		serviceRegWrapper: regWrapper,
		restarter:         agentconsul.NoopRestarter(),
		taskEnvBuilder:    taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:            logger,
	})
	must.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	must.NoError(t, h.Update(req))

	must.NoError(t, h.Postrun())

	must.NoError(t, h.PreTaskRestart())

	ops := consulMockClient.GetOps()
	must.Len(t, 4, ops)
	must.Eq(t, "add", ops[0].Op)    // Prerun
	must.Eq(t, "update", ops[1].Op) // Update
	must.Eq(t, "remove", ops[2].Op) // Postrun
	must.Eq(t, "add", ops[3].Op)    // Restart -> preRun
}

// TestGroupServiceHook_ShutdownDelayUpdate asserts calling group service hooks
// update updates the hooks delay value.
func TestGroupServiceHook_ShutdownDelayUpdate(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].ShutdownDelay = pointer.Of(10 * time.Second)

	logger := testlog.HCLogger(t)
	consulMockClient := regMock.NewServiceRegistrationHandler(logger)

	regWrapper := wrapper.NewHandlerWrapper(
		logger,
		consulMockClient,
		regMock.NewServiceRegistrationHandler(logger),
	)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:             alloc,
		serviceRegWrapper: regWrapper,
		restarter:         agentconsul.NoopRestarter(),
		taskEnvBuilder:    taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:            logger,
	})
	must.NoError(t, h.Prerun())

	// Incease shutdown Delay
	alloc.Job.TaskGroups[0].ShutdownDelay = pointer.Of(15 * time.Second)
	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	must.NoError(t, h.Update(req))

	// Assert that update updated the delay value
	must.Eq(t, h.delay, 15*time.Second)

	// Remove shutdown delay
	alloc.Job.TaskGroups[0].ShutdownDelay = nil
	req = &interfaces.RunnerUpdateRequest{Alloc: alloc}
	must.NoError(t, h.Update(req))

	// Assert that update updated the delay value
	must.Eq(t, h.delay, 0*time.Second)
}

// TestGroupServiceHook_GroupServices asserts group service hooks with group
// services does not error.
func TestGroupServiceHook_GroupServices(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.ConnectAlloc()
	alloc.Job.Canonicalize()
	logger := testlog.HCLogger(t)
	consulMockClient := regMock.NewServiceRegistrationHandler(logger)

	regWrapper := wrapper.NewHandlerWrapper(
		logger,
		consulMockClient,
		regMock.NewServiceRegistrationHandler(logger))

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:             alloc,
		serviceRegWrapper: regWrapper,
		restarter:         agentconsul.NoopRestarter(),
		taskEnvBuilder:    taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:            logger,
	})
	must.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	must.NoError(t, h.Update(req))

	must.NoError(t, h.Postrun())

	must.NoError(t, h.PreTaskRestart())

	ops := consulMockClient.GetOps()
	must.Len(t, 4, ops)
	must.Eq(t, "add", ops[0].Op)    // Prerun
	must.Eq(t, "update", ops[1].Op) // Update
	must.Eq(t, "remove", ops[2].Op) // Postrun
	must.Eq(t, "add", ops[3].Op)    // Restart -> preRun
}

// TestGroupServiceHook_GroupServices_Nomad asserts group service hooks with
// group services does not error when using the Nomad provider.
func TestGroupServiceHook_GroupServices_Nomad(t *testing.T) {
	ci.Parallel(t)

	// Create a mock alloc, and add a group service using provider Nomad.
	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Services = []*structs.Service{
		{
			Name:     "nomad-provider-service",
			Provider: structs.ServiceProviderNomad,
		},
	}

	// Create our base objects and our subsequent wrapper.
	logger := testlog.HCLogger(t)
	consulMockClient := regMock.NewServiceRegistrationHandler(logger)
	nomadMockClient := regMock.NewServiceRegistrationHandler(logger)

	regWrapper := wrapper.NewHandlerWrapper(logger, consulMockClient, nomadMockClient)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:             alloc,
		serviceRegWrapper: regWrapper,
		restarter:         agentconsul.NoopRestarter(),
		taskEnvBuilder:    taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:            logger,
	})
	must.NoError(t, h.Prerun())

	// Trigger our hook requests.
	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	must.NoError(t, h.Update(req))
	must.NoError(t, h.Postrun())
	must.NoError(t, h.PreTaskRestart())

	// Ensure the Nomad mock provider has the expected operations.
	ops := nomadMockClient.GetOps()
	must.Len(t, 4, ops)
	must.Eq(t, "add", ops[0].Op)    // Prerun
	must.Eq(t, "update", ops[1].Op) // Update
	must.Eq(t, "remove", ops[2].Op) // Postrun
	must.Eq(t, "add", ops[3].Op)    // Restart -> preRun

	// Ensure the Consul mock provider has zero operations.
	must.SliceEmpty(t, consulMockClient.GetOps())
}

// TestGroupServiceHook_Error asserts group service hooks with group
// services but no group network is handled gracefully.
func TestGroupServiceHook_NoNetwork(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Networks = []*structs.NetworkResource{}
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	tg.Services = []*structs.Service{
		{
			Name:      "testconnect",
			Provider:  "consul",
			PortLabel: "9999",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
			},
		},
	}
	logger := testlog.HCLogger(t)

	consulMockClient := regMock.NewServiceRegistrationHandler(logger)

	regWrapper := wrapper.NewHandlerWrapper(
		logger,
		consulMockClient,
		regMock.NewServiceRegistrationHandler(logger))

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:             alloc,
		serviceRegWrapper: regWrapper,
		restarter:         agentconsul.NoopRestarter(),
		taskEnvBuilder:    taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:            logger,
	})
	must.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	must.NoError(t, h.Update(req))

	must.NoError(t, h.Postrun())

	must.NoError(t, h.PreTaskRestart())

	ops := consulMockClient.GetOps()
	must.Len(t, 4, ops)
	must.Eq(t, "add", ops[0].Op)    // Prerun
	must.Eq(t, "update", ops[1].Op) // Update
	must.Eq(t, "remove", ops[2].Op) // Postrun
	must.Eq(t, "add", ops[3].Op)    // Restart -> preRun
}

func TestGroupServiceHook_getWorkloadServices(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Networks = []*structs.NetworkResource{}
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	tg.Services = []*structs.Service{
		{
			Name:      "testconnect",
			PortLabel: "9999",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{},
			},
		},
	}
	logger := testlog.HCLogger(t)

	consulMockClient := regMock.NewServiceRegistrationHandler(logger)

	regWrapper := wrapper.NewHandlerWrapper(
		logger,
		consulMockClient,
		regMock.NewServiceRegistrationHandler(logger))

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:             alloc,
		serviceRegWrapper: regWrapper,
		restarter:         agentconsul.NoopRestarter(),
		taskEnvBuilder:    taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:            logger,
	})

	services := h.getWorkloadServicesLocked()
	must.Len(t, 1, services.Services)
}
