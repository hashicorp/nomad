package volumewatcher

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestVolumeWatch_Reap(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	srv := &MockRPCServer{
		state: state.TestStateStore(t),
	}

	plugin := mock.CSIPlugin()
	node := testNode(plugin, srv.State())
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.ClientStatus = structs.AllocClientStatusComplete
	vol := testVolume(plugin, alloc, node.ID)
	vol.PastClaims = vol.ReadClaims

	ctx, exitFn := context.WithCancel(context.Background())
	w := &volumeWatcher{
		v:      vol,
		rpc:    srv,
		state:  srv.State(),
		ctx:    ctx,
		exitFn: exitFn,
		logger: testlog.HCLogger(t),
	}

	err := w.volumeReapImpl(vol)
	require.NoError(err)

	// past claim from a previous pass
	vol.PastClaims = map[string]*structs.CSIVolumeClaim{
		alloc.ID: {
			NodeID: node.ID,
			Mode:   structs.CSIVolumeClaimRead,
			State:  structs.CSIVolumeClaimStateNodeDetached,
		},
	}
	err = w.volumeReapImpl(vol)
	require.NoError(err)
	require.Len(vol.PastClaims, 1)

	// claim emitted by a GC event
	vol.PastClaims = map[string]*structs.CSIVolumeClaim{
		"": {
			NodeID: node.ID,
			Mode:   structs.CSIVolumeClaimGC,
		},
	}
	err = w.volumeReapImpl(vol)
	require.NoError(err)
	require.Len(vol.PastClaims, 2) // alloc claim + GC claim

	// release claims of a previously GC'd allocation
	vol.ReadAllocs[alloc.ID] = nil
	vol.PastClaims = map[string]*structs.CSIVolumeClaim{
		"": {
			NodeID: node.ID,
			Mode:   structs.CSIVolumeClaimRead,
		},
	}
	err = w.volumeReapImpl(vol)
	require.NoError(err)
	require.Len(vol.PastClaims, 2) // alloc claim + GC claim
}
