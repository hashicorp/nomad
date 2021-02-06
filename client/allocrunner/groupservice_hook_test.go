package allocrunner

import (
	"io/ioutil"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	ctestutil "github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var _ interfaces.RunnerPrerunHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerUpdateHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerPostrunHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerPreKillHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerTaskRestartHook = (*groupServiceHook)(nil)

// TestGroupServiceHook_NoGroupServices asserts calling group service hooks
// without group services does not error.
func TestGroupServiceHook_NoGroupServices(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Services = []*structs.Service{{
		Name:      "foo",
		PortLabel: "9999",
	}}
	logger := testlog.HCLogger(t)
	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:          alloc,
		consul:         consulClient,
		restarter:      agentconsul.NoopRestarter(),
		taskEnvBuilder: taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:         logger,
	})
	require.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	require.NoError(t, h.Update(req))

	require.NoError(t, h.Postrun())

	require.NoError(t, h.PreTaskRestart())

	ops := consulClient.GetOps()
	require.Len(t, ops, 7)
	require.Equal(t, "add", ops[0].Op)    // Prerun
	require.Equal(t, "update", ops[1].Op) // Update
	require.Equal(t, "remove", ops[2].Op) // Postrun (1st)
	require.Equal(t, "remove", ops[3].Op) // Postrun (2nd)
	require.Equal(t, "remove", ops[4].Op) // Restart -> preKill (1st)
	require.Equal(t, "remove", ops[5].Op) // Restart -> preKill (2nd)
	require.Equal(t, "add", ops[6].Op)    // Restart -> preRun
}

// TestGroupServiceHook_ShutdownDelayUpdate asserts calling group service hooks
// update updates the hooks delay value.
func TestGroupServiceHook_ShutdownDelayUpdate(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].ShutdownDelay = helper.TimeToPtr(10 * time.Second)

	logger := testlog.HCLogger(t)
	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:          alloc,
		consul:         consulClient,
		restarter:      agentconsul.NoopRestarter(),
		taskEnvBuilder: taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:         logger,
	})
	require.NoError(t, h.Prerun())

	// Incease shutdown Delay
	alloc.Job.TaskGroups[0].ShutdownDelay = helper.TimeToPtr(15 * time.Second)
	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	require.NoError(t, h.Update(req))

	// Assert that update updated the delay value
	require.Equal(t, h.delay, 15*time.Second)

	// Remove shutdown delay
	alloc.Job.TaskGroups[0].ShutdownDelay = nil
	req = &interfaces.RunnerUpdateRequest{Alloc: alloc}
	require.NoError(t, h.Update(req))

	// Assert that update updated the delay value
	require.Equal(t, h.delay, 0*time.Second)
}

// TestGroupServiceHook_GroupServices asserts group service hooks with group
// services does not error.
func TestGroupServiceHook_GroupServices(t *testing.T) {
	t.Parallel()

	alloc := mock.ConnectAlloc()
	logger := testlog.HCLogger(t)
	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:          alloc,
		consul:         consulClient,
		restarter:      agentconsul.NoopRestarter(),
		taskEnvBuilder: taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:         logger,
	})
	require.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	require.NoError(t, h.Update(req))

	require.NoError(t, h.Postrun())

	require.NoError(t, h.PreTaskRestart())

	ops := consulClient.GetOps()
	require.Len(t, ops, 7)
	require.Equal(t, "add", ops[0].Op)    // Prerun
	require.Equal(t, "update", ops[1].Op) // Update
	require.Equal(t, "remove", ops[2].Op) // Postrun (1st)
	require.Equal(t, "remove", ops[3].Op) // Postrun (2nd)
	require.Equal(t, "remove", ops[4].Op) // Restart -> preKill (1st)
	require.Equal(t, "remove", ops[5].Op) // Restart -> preKill (2nd)
	require.Equal(t, "add", ops[6].Op)    // Restart -> preRun
}

// TestGroupServiceHook_Error asserts group service hooks with group
// services but no group network is handled gracefully.
func TestGroupServiceHook_NoNetwork(t *testing.T) {
	t.Parallel()

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

	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:          alloc,
		consul:         consulClient,
		restarter:      agentconsul.NoopRestarter(),
		taskEnvBuilder: taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:         logger,
	})
	require.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	require.NoError(t, h.Update(req))

	require.NoError(t, h.Postrun())

	require.NoError(t, h.PreTaskRestart())

	ops := consulClient.GetOps()
	require.Len(t, ops, 7)
	require.Equal(t, "add", ops[0].Op)    // Prerun
	require.Equal(t, "update", ops[1].Op) // Update
	require.Equal(t, "remove", ops[2].Op) // Postrun (1st)
	require.Equal(t, "remove", ops[3].Op) // Postrun (2nd)
	require.Equal(t, "remove", ops[4].Op) // Restart -> preKill (1st)
	require.Equal(t, "remove", ops[5].Op) // Restart -> preKill (2nd)
	require.Equal(t, "add", ops[6].Op)    // Restart -> preRun
}

func TestGroupServiceHook_getWorkloadServices(t *testing.T) {
	t.Parallel()

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

	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:          alloc,
		consul:         consulClient,
		restarter:      agentconsul.NoopRestarter(),
		taskEnvBuilder: taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region),
		logger:         logger,
	})

	services := h.getWorkloadServices()
	require.Len(t, services.Services, 1)
}

// TestGroupServiceHook_Update08Alloc asserts that adding group services to a previously
// 0.8 alloc works.
//
// COMPAT(0.11) Only valid for upgrades from 0.8.
func TestGroupServiceHook_Update08Alloc(t *testing.T) {
	// Create an embedded Consul server
	testconsul, err := ctestutil.NewTestServerConfigT(t, func(c *ctestutil.TestServerConfig) {
		// If -v wasn't specified squelch consul logging
		if !testing.Verbose() {
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
		}
	})
	if err != nil {
		t.Fatalf("error starting test consul server: %v", err)
	}
	defer testconsul.Stop()

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = testconsul.HTTPAddr
	consulClient, err := consulapi.NewClient(consulConfig)
	require.NoError(t, err)
	serviceClient := agentconsul.NewServiceClient(consulClient.Agent(), testlog.HCLogger(t), true)

	// Lower periodicInterval to ensure periodic syncing doesn't improperly
	// remove Connect services.
	//const interval = 50 * time.Millisecond
	//serviceClient.periodicInterval = interval

	// Disable deregistration probation to test syncing
	//serviceClient.deregisterProbationExpiry = time.Time{}

	go serviceClient.Run()
	defer serviceClient.Shutdown()

	// Create new 0.10-style alloc
	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{
		{
			Mode: "bridge",
			IP:   "10.0.0.1",
			DynamicPorts: []structs.Port{
				{
					Label: "connect-proxy-testconnect",
					Value: 9999,
					To:    9998,
				},
			},
		},
	}
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	tg.Services = []*structs.Service{
		{
			Name:      "testconnect",
			PortLabel: "9999",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Proxy: &structs.ConsulProxy{
						LocalServicePort: 9000,
					},
				},
			},
		},
	}

	// Create old 0.8-style alloc from new alloc
	oldAlloc := alloc.Copy()
	oldAlloc.AllocatedResources = nil
	oldAlloc.Job.LookupTaskGroup(alloc.TaskGroup).Services = nil

	// Create the group service hook
	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:          oldAlloc,
		consul:         serviceClient,
		restarter:      agentconsul.NoopRestarter(),
		taskEnvBuilder: taskenv.NewBuilder(mock.Node(), oldAlloc, nil, oldAlloc.Job.Region),
		logger:         testlog.HCLogger(t),
	})

	require.NoError(t, h.Prerun())
	require.NoError(t, h.Update(&interfaces.RunnerUpdateRequest{Alloc: alloc}))

	// Assert the group and sidecar services are registered
	require.Eventually(t, func() bool {
		services, err := consulClient.Agent().Services()
		require.NoError(t, err)
		return len(services) == 2
	}, 3*time.Second, 100*time.Millisecond)

}
