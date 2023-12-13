// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPoststartHook = (*serviceHook)(nil)
var _ interfaces.TaskExitedHook = (*serviceHook)(nil)
var _ interfaces.TaskPreKillHook = (*serviceHook)(nil)
var _ interfaces.TaskUpdateHook = (*serviceHook)(nil)

func TestUpdate_beforePoststart(t *testing.T) {
	alloc := mock.Alloc()
	alloc.Job.Canonicalize()
	logger := testlog.HCLogger(t)

	c := regMock.NewServiceRegistrationHandler(logger)
	regWrap := wrapper.NewHandlerWrapper(logger, c, nil)

	// Interpolating workload services performs a check on the task env, if it
	// is nil, nil is returned meaning no services. This does not work with the
	// wrapper len protections, so we need a dummy taskenv.
	spoofTaskEnv := taskenv.TaskEnv{NodeAttrs: map[string]string{}}

	hook := newServiceHook(serviceHookConfig{
		alloc:             alloc,
		task:              alloc.LookupTask("web"),
		serviceRegWrapper: regWrap,
		logger:            logger,
	})
	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{
		Alloc:   alloc,
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 0)

	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskPoststartResponse{}))
	require.Len(t, c.GetOps(), 1)

	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{
		Alloc:   alloc,
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 2)

	// When a task exits it could be restarted with new driver info
	// so Update should again wait on Poststart.

	require.NoError(t, hook.Exited(context.Background(), &interfaces.TaskExitedRequest{}, &interfaces.TaskExitedResponse{}))
	require.Len(t, c.GetOps(), 3)

	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{
		Alloc:   alloc,
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 3)

	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskPoststartResponse{}))
	require.Len(t, c.GetOps(), 4)

	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{
		Alloc:   alloc,
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 5)

	require.NoError(t, hook.PreKilling(context.Background(), &interfaces.TaskPreKillRequest{}, &interfaces.TaskPreKillResponse{}))
	require.Len(t, c.GetOps(), 6)

	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{
		Alloc:   alloc,
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 6)
}

func Test_serviceHook_multipleDeRegisterCall(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	logger := testlog.HCLogger(t)

	c := regMock.NewServiceRegistrationHandler(logger)
	regWrap := wrapper.NewHandlerWrapper(logger, c, nil)

	hook := newServiceHook(serviceHookConfig{
		alloc:             alloc,
		task:              alloc.LookupTask("web"),
		serviceRegWrapper: regWrap,
		logger:            logger,
	})

	// Interpolating workload services performs a check on the task env, if it
	// is nil, nil is returned meaning no services. This does not work with the
	// wrapper len protections, so we need a dummy taskenv.
	spoofTaskEnv := taskenv.TaskEnv{NodeAttrs: map[string]string{}}

	// Add a registration, as we would in normal operation.
	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskPoststartResponse{}))
	require.Len(t, c.GetOps(), 1)

	// Call all three deregister backed functions in a row. Ensure the number
	// of operations does not increase and that the second is always a remove.
	require.NoError(t, hook.Exited(context.Background(), &interfaces.TaskExitedRequest{}, &interfaces.TaskExitedResponse{}))
	require.Len(t, c.GetOps(), 2)
	require.Equal(t, c.GetOps()[1].Op, "remove")

	require.NoError(t, hook.PreKilling(context.Background(), &interfaces.TaskPreKillRequest{}, &interfaces.TaskPreKillResponse{}))
	require.Len(t, c.GetOps(), 2)
	require.Equal(t, c.GetOps()[1].Op, "remove")

	require.NoError(t, hook.Stop(context.Background(), &interfaces.TaskStopRequest{}, &interfaces.TaskStopResponse{}))
	require.Len(t, c.GetOps(), 2)
	require.Equal(t, c.GetOps()[1].Op, "remove")

	// Now we act like a restart.
	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{
		TaskEnv: &spoofTaskEnv,
	}, &interfaces.TaskPoststartResponse{}))
	require.Len(t, c.GetOps(), 3)
	require.Equal(t, c.GetOps()[2].Op, "add")

	// Go again through the process or shutting down.
	require.NoError(t, hook.Exited(context.Background(), &interfaces.TaskExitedRequest{}, &interfaces.TaskExitedResponse{}))
	require.Len(t, c.GetOps(), 4)
	require.Equal(t, c.GetOps()[3].Op, "remove")

	require.NoError(t, hook.PreKilling(context.Background(), &interfaces.TaskPreKillRequest{}, &interfaces.TaskPreKillResponse{}))
	require.Len(t, c.GetOps(), 4)
	require.Equal(t, c.GetOps()[3].Op, "remove")

	require.NoError(t, hook.Stop(context.Background(), &interfaces.TaskStopRequest{}, &interfaces.TaskStopResponse{}))
	require.Len(t, c.GetOps(), 4)
	require.Equal(t, c.GetOps()[3].Op, "remove")
}

// Test_serviceHook_Nomad performs a normal operation test of the serviceHook
// when using task services which utilise the Nomad provider.
func Test_serviceHook_Nomad(t *testing.T) {
	ci.Parallel(t)

	// Create a mock alloc, and add a task service using provider Nomad.
	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Tasks[0].Services = []*structs.Service{
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

	h := newServiceHook(serviceHookConfig{
		alloc:             alloc,
		task:              alloc.LookupTask("web"),
		providerNamespace: "default",
		serviceRegWrapper: regWrapper,
		restarter:         agentconsul.NoopRestarter(),
		logger:            logger,
	})

	// Create a taskEnv builder to use in requests, otherwise interpolation of
	// services will always return nil.
	taskEnvBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region)

	// Trigger our initial hook function.
	require.NoError(t, h.Poststart(context.Background(), &interfaces.TaskPoststartRequest{
		TaskEnv: taskEnvBuilder.Build()}, nil))

	// Trigger all the possible stop functions to ensure we only deregister
	// once.
	require.NoError(t, h.PreKilling(context.Background(), nil, nil))
	require.NoError(t, h.Exited(context.Background(), nil, nil))
	require.NoError(t, h.Stop(context.Background(), nil, nil))

	// Ensure the Nomad mock provider has the expected operations.
	nomadOps := nomadMockClient.GetOps()
	require.Len(t, nomadOps, 2)
	require.Equal(t, "add", nomadOps[0].Op)    // Poststart
	require.Equal(t, "remove", nomadOps[1].Op) // PreKilling,Exited,Stop

	// Ensure the Consul mock provider has zero operations.
	require.Len(t, consulMockClient.GetOps(), 0)
}
