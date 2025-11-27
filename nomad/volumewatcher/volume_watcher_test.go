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
	"github.com/shoenig/test/must"
)

func TestVolumeWatch_Reap(t *testing.T) {
	ci.Parallel(t)

	// note: this test doesn't put the volume in the state store so that we
	// don't have to have the mock write updates back to it
	store := state.TestStateStore(t)
	srv := &MockRPCServer{
		state: store,
	}

	plugin := mock.CSIPlugin()
	node := testNode(plugin, store)
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.ClientStatus = structs.AllocClientStatusRunning

	index, _ := store.LatestIndex()
	index++
	must.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc}))

	vol := testVolume(plugin, alloc, node.ID)

	ctx, exitFn := context.WithCancel(context.Background())
	w := &volumeWatcher{
		v:      vol,
		rpc:    srv,
		state:  store,
		ctx:    ctx,
		exitFn: exitFn,
		logger: testlog.HCLogger(t),
	}

	vol, _ = store.CSIVolumeDenormalize(nil, vol.Copy())
	err := w.volumeReapImpl(vol)
	must.NoError(t, err)

	// verify no change has been made
	must.MapLen(t, 1, vol.ReadClaims)
	must.MapLen(t, 0, vol.PastClaims)
	must.Eq(t, 0, srv.countCSIUnpublish.Load())

	alloc = alloc.Copy()
	alloc.ClientStatus = structs.AllocClientStatusComplete

	index++
	must.NoError(t, store.UpdateAllocsFromClient(
		structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc}))

	vol, _ = store.CSIVolumeDenormalize(nil, vol.Copy())
	must.MapLen(t, 1, vol.ReadClaims)
	must.MapLen(t, 1, vol.PastClaims)

	err = w.volumeReapImpl(vol)
	must.NoError(t, err)
	must.Eq(t, 1, srv.countCSIUnpublish.Load())

	// simulate updated past claim from a previous pass
	vol.PastClaims = map[string]*structs.CSIVolumeClaim{
		alloc.ID: {
			NodeID: node.ID,
			Mode:   structs.CSIVolumeClaimRead,
			State:  structs.CSIVolumeClaimStateNodeDetached,
		},
	}
	vol, _ = store.CSIVolumeDenormalize(nil, vol.Copy())
	err = w.volumeReapImpl(vol)
	must.NoError(t, err)
	must.MapLen(t, 1, vol.PastClaims)
	must.Eq(t, 2, srv.countCSIUnpublish.Load())

	// claim emitted by a GC event
	vol.PastClaims = map[string]*structs.CSIVolumeClaim{
		"": {
			NodeID: node.ID,
			Mode:   structs.CSIVolumeClaimGC,
		},
	}
	vol, _ = store.CSIVolumeDenormalize(nil, vol.Copy())
	err = w.volumeReapImpl(vol)
	must.NoError(t, err)
	must.MapLen(t, 2, vol.PastClaims) // alloc claim + GC claim
	must.Eq(t, 4, srv.countCSIUnpublish.Load())

	// release claims of a previously GC'd allocation
	vol.ReadAllocs[alloc.ID] = nil
	vol.PastClaims = map[string]*structs.CSIVolumeClaim{
		"": {
			NodeID: node.ID,
			Mode:   structs.CSIVolumeClaimRead,
		},
	}
	vol, _ = store.CSIVolumeDenormalize(nil, vol.Copy())
	err = w.volumeReapImpl(vol)
	must.NoError(t, err)
	must.MapLen(t, 2, vol.PastClaims) // alloc claim + GC claim
	must.Eq(t, 6, srv.countCSIUnpublish.Load())
}

func TestVolumeReapBadState(t *testing.T) {
	ci.Parallel(t)

	store := state.TestStateStore(t)
	err := state.TestBadCSIState(t, store)
	must.NoError(t, err)
	srv := &MockRPCServer{
		state: store,
	}

	vol, err := srv.state.CSIVolumeByID(nil,
		structs.DefaultNamespace, "csi-volume-nfs0")
	must.NoError(t, err)
	srv.state.CSIVolumeDenormalize(nil, vol)

	ctx, exitFn := context.WithCancel(context.Background())
	w := &volumeWatcher{
		v:      vol,
		rpc:    srv,
		state:  store,
		ctx:    ctx,
		exitFn: exitFn,
		logger: testlog.HCLogger(t),
	}

	err = w.volumeReapImpl(vol)
	must.NoError(t, err)
	must.Eq(t, 2, srv.countCSIUnpublish.Load())
}
