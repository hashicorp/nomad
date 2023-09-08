// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package volumewatcher

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestVolumeWatch_Reap(t *testing.T) {
	ci.Parallel(t)
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

	vol, _ = srv.State().CSIVolumeDenormalize(nil, vol.Copy())
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
	vol, _ = srv.State().CSIVolumeDenormalize(nil, vol.Copy())
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
	vol, _ = srv.State().CSIVolumeDenormalize(nil, vol.Copy())
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
	vol, _ = srv.State().CSIVolumeDenormalize(nil, vol.Copy())
	err = w.volumeReapImpl(vol)
	require.NoError(err)
	require.Len(vol.PastClaims, 2) // alloc claim + GC claim
}

func TestVolumeReapBadState(t *testing.T) {
	ci.Parallel(t)

	store := state.TestStateStore(t)
	err := state.TestBadCSIState(t, store)
	require.NoError(t, err)
	srv := &MockRPCServer{
		state: store,
	}

	vol, err := srv.state.CSIVolumeByID(nil,
		structs.DefaultNamespace, "csi-volume-nfs0")
	require.NoError(t, err)
	srv.state.CSIVolumeDenormalize(nil, vol)

	ctx, exitFn := context.WithCancel(context.Background())
	w := &volumeWatcher{
		v:      vol,
		rpc:    srv,
		state:  srv.State(),
		ctx:    ctx,
		exitFn: exitFn,
		logger: testlog.HCLogger(t),
	}

	err = w.volumeReapImpl(vol)
	require.NoError(t, err)
	require.Equal(t, 2, srv.countCSIUnpublish)
}
