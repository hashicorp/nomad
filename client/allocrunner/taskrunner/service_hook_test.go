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
		alloc:  alloc,
		task:   alloc.LookupTask("web"),
		consul: c,
		logger: logger,
	})
	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{Alloc: alloc}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 0)
	require.NoError(t, hook.Poststart(context.Background(), &interfaces.TaskPoststartRequest{}, &interfaces.TaskPoststartResponse{}))
	require.Len(t, c.GetOps(), 1)
	require.NoError(t, hook.Update(context.Background(), &interfaces.TaskUpdateRequest{Alloc: alloc}, &interfaces.TaskUpdateResponse{}))
	require.Len(t, c.GetOps(), 2)
}
