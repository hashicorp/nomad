package volumewatcher

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// TestVolumeWatch_OneReap tests one pass through the reaper
func TestVolumeWatch_OneReap(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	cases := []struct {
		Name                          string
		Volume                        *structs.CSIVolume
		Node                          *structs.Node
		ControllerRequired            bool
		ExpectedErr                   string
		ExpectedClaimsCount           int
		ExpectedNodeDetachCount       int
		ExpectedControllerDetachCount int
		ExpectedUpdateClaimsCount     int
		srv                           *MockRPCServer
	}{
		{
			Name:               "No terminal allocs",
			Volume:             mock.CSIVolume(mock.CSIPlugin()),
			ControllerRequired: true,
			srv: &MockRPCServer{
				state:                  state.TestStateStore(t),
				nextCSINodeDetachError: fmt.Errorf("should never see this"),
			},
		},
		{
			Name:                    "NodeDetachVolume fails",
			ControllerRequired:      true,
			ExpectedErr:             "some node plugin error",
			ExpectedNodeDetachCount: 1,
			srv: &MockRPCServer{
				state:                  state.TestStateStore(t),
				nextCSINodeDetachError: fmt.Errorf("some node plugin error"),
			},
		},
		{
			Name:                      "NodeDetachVolume node-only happy path",
			ControllerRequired:        false,
			ExpectedNodeDetachCount:   1,
			ExpectedUpdateClaimsCount: 3,
			srv: &MockRPCServer{
				state: state.TestStateStore(t),
			},
		},
		{
			Name:                      "ControllerDetachVolume no controllers available",
			Node:                      mock.Node(),
			ControllerRequired:        true,
			ExpectedErr:               "Unknown node",
			ExpectedNodeDetachCount:   1,
			ExpectedUpdateClaimsCount: 1,
			srv: &MockRPCServer{
				state: state.TestStateStore(t),
			},
		},
		{
			Name:                          "ControllerDetachVolume controller error",
			ControllerRequired:            true,
			ExpectedErr:                   "some controller error",
			ExpectedNodeDetachCount:       1,
			ExpectedControllerDetachCount: 1,
			ExpectedUpdateClaimsCount:     1,
			srv: &MockRPCServer{
				state:                        state.TestStateStore(t),
				nextCSIControllerDetachError: fmt.Errorf("some controller error"),
			},
		},
		{
			Name:                          "ControllerDetachVolume happy path",
			ControllerRequired:            true,
			ExpectedNodeDetachCount:       1,
			ExpectedControllerDetachCount: 1,
			ExpectedUpdateClaimsCount:     3,
			srv: &MockRPCServer{
				state: state.TestStateStore(t),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			plugin := mock.CSIPlugin()
			plugin.ControllerRequired = tc.ControllerRequired
			node := testNode(tc.Node, plugin, tc.srv.State())
			alloc := mock.Alloc()
			vol := testVolume(tc.Volume, plugin, alloc.ID, node.ID)
			ctx, exitFn := context.WithCancel(context.Background())
			w := &volumeWatcher{
				v:            vol,
				rpc:          tc.srv,
				state:        tc.srv.State(),
				updateClaims: tc.srv.UpdateClaims,
				ctx:          ctx,
				exitFn:       exitFn,
			}

			err := w.volumeReapImpl(vol)
			if tc.ExpectedErr != "" {
				require.Error(err, fmt.Sprintf("expected: %q", tc.ExpectedErr))
				require.Contains(err.Error(), tc.ExpectedErr)
			} else {
				require.NoError(err)
			}
			require.Equal(tc.ExpectedNodeDetachCount,
				tc.srv.countCSINodeDetachVolume, "node detach RPC count")
			require.Equal(tc.ExpectedControllerDetachCount,
				tc.srv.countCSIControllerDetachVolume, "controller detach RPC count")
			require.Equal(tc.ExpectedUpdateClaimsCount,
				tc.srv.countUpdateClaims, "update claims count")
		})
	}
}

// TestVolumeWatch_OneReap tests multiple passes through the reaper,
// updating state after each one
func TestVolumeWatch_ReapStates(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	srv := &MockRPCServer{state: state.TestStateStore(t)}
	plugin := mock.CSIPlugin()
	node := testNode(nil, plugin, srv.State())
	alloc := mock.Alloc()
	vol := testVolume(nil, plugin, alloc.ID, node.ID)

	w := &volumeWatcher{
		v:            vol,
		rpc:          srv,
		state:        srv.State(),
		updateClaims: srv.UpdateClaims,
	}

	srv.nextCSINodeDetachError = fmt.Errorf("some node plugin error")
	err := w.volumeReapImpl(vol)
	require.Error(err)
	require.Equal(structs.CSIVolumeClaimStateTaken, vol.PastClaims[alloc.ID].State)
	require.Equal(1, srv.countCSINodeDetachVolume)
	require.Equal(0, srv.countCSIControllerDetachVolume)
	require.Equal(0, srv.countUpdateClaims)

	srv.nextCSINodeDetachError = nil
	srv.nextCSIControllerDetachError = fmt.Errorf("some controller plugin error")
	err = w.volumeReapImpl(vol)
	require.Error(err)
	require.Equal(structs.CSIVolumeClaimStateNodeDetached, vol.PastClaims[alloc.ID].State)
	require.Equal(1, srv.countUpdateClaims)

	srv.nextCSIControllerDetachError = nil
	err = w.volumeReapImpl(vol)
	require.NoError(err)
	require.Equal(structs.CSIVolumeClaimStateFreed, vol.PastClaims[alloc.ID].State)
	require.Equal(3, srv.countUpdateClaims)
}
