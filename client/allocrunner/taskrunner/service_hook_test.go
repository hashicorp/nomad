package taskrunner

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/stretchr/testify/require"
)

// Statically assert the stats hook implements the expected interfaces
var _ interfaces.TaskPoststartHook = (*serviceHook)(nil)
var _ interfaces.TaskExitedHook = (*serviceHook)(nil)
var _ interfaces.TaskPreKillHook = (*serviceHook)(nil)
var _ interfaces.TaskUpdateHook = (*serviceHook)(nil)

func TestUpdate_beforePoststart(t *testing.T) {
	alloc := mock.Alloc()
	logger := testlog.HCLogger(t)
	c := consul.NewMockConsulServiceClient(t, logger)

	hook := newServiceHook(serviceHookConfig{
		alloc:          alloc,
		task:           alloc.LookupTask("web"),
		consulServices: c,
		logger:         logger,
	})
	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{Alloc: alloc}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 0)
	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{}, &interfaces.TaskPoststartResponse{}))
	require.Len(t, c.GetOps(), 1)
	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{Alloc: alloc}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 2)

	// When a task exits it could be restarted with new driver info
	// so Update should again wait on Poststart.

	require.NoError(t, hook.Exited(context.Background(), &interfaces.TaskExitedRequest{}, &interfaces.TaskExitedResponse{}))
	require.Len(t, c.GetOps(), 3)
	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{Alloc: alloc}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 3)
	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{}, &interfaces.TaskPoststartResponse{}))
	require.Len(t, c.GetOps(), 4)
	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{Alloc: alloc}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 5)
	require.NoError(t, hook.PreKilling(context.Background(), &interfaces.TaskPreKillRequest{}, &interfaces.TaskPreKillResponse{}))
	require.Len(t, c.GetOps(), 6)
	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{Alloc: alloc}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 6)
}

func Test_serviceHook_multipleDeRegisterCall(t *testing.T) {

	alloc := mock.Alloc()
	logger := testlog.HCLogger(t)
	c := consul.NewMockConsulServiceClient(t, logger)

	hook := newServiceHook(serviceHookConfig{
		alloc:          alloc,
		task:           alloc.LookupTask("web"),
		consulServices: c,
		logger:         logger,
	})

	// Add a registration, as we would in normal operation.
	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{}, &interfaces.TaskPoststartResponse{}))
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
	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{}, &interfaces.TaskPoststartResponse{}))
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
