package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/stretchr/testify/require"
)

// statically assert network hook implements the expected interfaces
var _ interfaces.RunnerPrerunHook = (*networkHook)(nil)
var _ interfaces.RunnerPostrunHook = (*networkHook)(nil)

type mockNetworkIsolationSetter struct {
	t            *testing.T
	expectedSpec *drivers.NetworkIsolationSpec
	called       bool
}

func (m *mockNetworkIsolationSetter) SetNetworkIsolation(spec *drivers.NetworkIsolationSpec) {
	m.called = true
	require.Exactly(m.t, m.expectedSpec, spec)
}

// Test that the prerun and postrun hooks call the setter with the expected spec when
// the network mode is not host
func TestNetworkHook_Prerun_Postrun(t *testing.T) {
	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
		{
			Mode: "bridge",
		},
	}
	spec := &drivers.NetworkIsolationSpec{
		Mode:   drivers.NetIsolationModeGroup,
		Path:   "test",
		Labels: map[string]string{"abc": "123"},
	}

	destroyCalled := false
	nm := &testutils.MockDriver{
		MockNetworkManager: testutils.MockNetworkManager{
			CreateNetworkF: func(allocID string) (*drivers.NetworkIsolationSpec, bool, error) {
				require.Equal(t, alloc.ID, allocID)
				return spec, false, nil
			},

			DestroyNetworkF: func(allocID string, netSpec *drivers.NetworkIsolationSpec) error {
				destroyCalled = true
				require.Equal(t, alloc.ID, allocID)
				require.Exactly(t, spec, netSpec)
				return nil
			},
		},
	}
	setter := &mockNetworkIsolationSetter{
		t:            t,
		expectedSpec: spec,
	}
	require := require.New(t)

	logger := testlog.HCLogger(t)
	hook := newNetworkHook(logger, setter, alloc, nm, &hostNetworkConfigurator{})
	require.NoError(hook.Prerun())
	require.True(setter.called)
	require.False(destroyCalled)
	require.NoError(hook.Postrun())
	require.True(destroyCalled)

	// reset and use host network mode
	setter.called = false
	destroyCalled = false
	alloc.Job.TaskGroups[0].Networks[0].Mode = "host"
	hook = newNetworkHook(logger, setter, alloc, nm, &hostNetworkConfigurator{})
	require.NoError(hook.Prerun())
	require.False(setter.called)
	require.False(destroyCalled)
	require.NoError(hook.Postrun())
	require.False(destroyCalled)

}
