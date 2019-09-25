package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
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
	logger := testlog.HCLogger(t)
	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(logger, alloc, consulClient)
	require.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	require.NoError(t, h.Update(req))

	require.NoError(t, h.Postrun())

	ops := consulClient.GetOps()
	require.Len(t, ops, 3)
	require.Equal(t, "add_group", ops[0].Op)
	require.Equal(t, "update_group", ops[1].Op)
	require.Equal(t, "remove_group", ops[2].Op)
}

// TestGroupServiceHook_GroupServices asserts group service hooks with group
// services does not error.
func TestGroupServiceHook_GroupServices(t *testing.T) {
	t.Parallel()

	alloc := mock.ConnectAlloc()
	logger := testlog.HCLogger(t)
	consulClient := consul.NewMockConsulServiceClient(t, logger)

	h := newGroupServiceHook(logger, alloc, consulClient)
	require.NoError(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	require.NoError(t, h.Update(req))

	require.NoError(t, h.Postrun())

	ops := consulClient.GetOps()
	require.Len(t, ops, 3)
	require.Equal(t, "add_group", ops[0].Op)
	require.Equal(t, "update_group", ops[1].Op)
	require.Equal(t, "remove_group", ops[2].Op)
}

// TestGroupServiceHook_Error asserts group service hooks with group
// services but no group network returns an error.
func TestGroupServiceHook_Error(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
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

	// No need to set Consul client or call Run. This hould fail before
	// attempting to register.
	consulClient := agentconsul.NewServiceClient(nil, logger, false)

	h := newGroupServiceHook(logger, alloc, consulClient)
	require.Error(t, h.Prerun())

	req := &interfaces.RunnerUpdateRequest{Alloc: alloc}
	require.Error(t, h.Update(req))

	require.Error(t, h.Postrun())
}
