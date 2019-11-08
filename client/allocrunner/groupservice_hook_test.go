package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/taskenv"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var _ interfaces.RunnerPrerunHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerUpdateHook = (*groupServiceHook)(nil)
var _ interfaces.RunnerPostrunHook = (*groupServiceHook)(nil)

// TestGroupServiceHook_NoGroupServices asserts calling group service hooks
// without group services does not error.
func TestGroupServiceHook_NoGroupServices(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Services = []*structs.Service{{
		Name:      "foo",
		PortLabel: "9999",
	}}
	node := mock.Node()
	logger := testlog.HCLogger(t)
	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:     alloc,
		consul:    consulClient,
		restarter: agentconsul.NoopRestarter(),
		taskEnv:   taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region).Build(),
		logger:    logger,
	})
	require.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc, Node: node}
	require.NoError(t, h.Update(req))

	require.NoError(t, h.Postrun())

	ops := consulClient.GetOps()
	require.Len(t, ops, 4)
	require.Equal(t, "add", ops[0].Op)
	require.Equal(t, "update", ops[1].Op)
	require.Equal(t, "remove", ops[2].Op)
}

// TestGroupServiceHook_GroupServices asserts group service hooks with group
// services does not error.
func TestGroupServiceHook_GroupServices(t *testing.T) {
	t.Parallel()

	alloc := mock.ConnectAlloc()
	node := mock.Node()
	logger := testlog.HCLogger(t)
	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(groupServiceHookConfig{
		alloc:     alloc,
		consul:    consulClient,
		restarter: agentconsul.NoopRestarter(),
		taskEnv:   taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region).Build(),
		logger:    logger,
	})
	require.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc, Node: node}
	require.NoError(t, h.Update(req))

	require.NoError(t, h.Postrun())

	ops := consulClient.GetOps()
	require.Len(t, ops, 4)
	require.Equal(t, "add", ops[0].Op)
	require.Equal(t, "update", ops[1].Op)
	require.Equal(t, "remove", ops[2].Op)
}

// TestGroupServiceHook_Error asserts group service hooks with group
// services but no group network returns an error.
func TestGroupServiceHook_NoNetwork(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Networks = []*structs.NetworkResource{}
	node := mock.Node()
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
		alloc:     alloc,
		consul:    consulClient,
		restarter: agentconsul.NoopRestarter(),
		taskEnv:   taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region).Build(),
		logger:    logger,
	})
	require.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc, Node: node}
	require.NoError(t, h.Update(req))

	require.NoError(t, h.Postrun())

	ops := consulClient.GetOps()
	require.Len(t, ops, 4)
	require.Equal(t, "add", ops[0].Op)
	require.Equal(t, "update", ops[1].Op)
	require.Equal(t, "remove", ops[2].Op)
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
		alloc:     alloc,
		consul:    consulClient,
		restarter: agentconsul.NoopRestarter(),
		taskEnv:   taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region).Build(),
		logger:    logger,
	})

	services := h.getWorkloadServices()
	require.Len(t, services.Services, 1)
}
