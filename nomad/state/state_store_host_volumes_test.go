// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_HostVolumes_CRUD(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)
	index, err := store.LatestIndex()
	must.NoError(t, err)

	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	nodes[2].NodePool = "prod"
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup,
		index, nodes[0], NodeUpsertWithNodePool))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup,
		index, nodes[1], NodeUpsertWithNodePool))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup,
		index, nodes[2], NodeUpsertWithNodePool))

	ns := mock.Namespace()
	must.NoError(t, store.UpsertNamespaces(index, []*structs.Namespace{ns}))

	vols := []*structs.HostVolume{
		mock.HostVolume(),
		mock.HostVolume(),
		mock.HostVolume(),
		mock.HostVolume(),
	}
	vols[0].NodeID = nodes[0].ID
	vols[1].NodeID = nodes[1].ID
	vols[1].Name = "another-example"
	vols[2].NodeID = nodes[2].ID
	vols[2].NodePool = nodes[2].NodePool
	vols[3].Namespace = ns.Name
	vols[3].NodeID = nodes[2].ID
	vols[3].NodePool = nodes[2].NodePool

	index++
	must.NoError(t, store.UpsertHostVolumes(index, vols))

	vol, err := store.HostVolumeByID(nil, vols[0].Namespace, vols[0].ID, true)
	must.NoError(t, err)
	must.NotNil(t, vol)
	must.Eq(t, vols[0].ID, vol.ID)
	must.NotNil(t, vol.Allocations)
	must.Len(t, 0, vol.Allocations)

	vol, err = store.HostVolumeByID(nil, vols[0].Namespace, vols[0].ID, false)
	must.NoError(t, err)
	must.NotNil(t, vol)
	must.Nil(t, vol.Allocations)

	consumeIter := func(iter memdb.ResultIterator) map[string]*structs.HostVolume {
		got := map[string]*structs.HostVolume{}
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			vol := raw.(*structs.HostVolume)
			got[vol.ID] = vol
		}
		return got
	}

	iter, err := store.HostVolumesByName(nil, structs.DefaultNamespace, "example", SortDefault)
	must.NoError(t, err)
	got := consumeIter(iter)
	must.NotNil(t, got[vols[0].ID], must.Sprint("expected vol0"))
	must.NotNil(t, got[vols[2].ID], must.Sprint("expected vol2"))
	must.MapLen(t, 2, got, must.Sprint(`expected 2 volumes named "example" in default namespace`))

	iter, err = store.HostVolumesByNodePool(nil, nodes[2].NodePool, SortDefault)
	must.NoError(t, err)
	got = consumeIter(iter)
	must.NotNil(t, got[vols[2].ID], must.Sprint("expected vol2"))
	must.NotNil(t, got[vols[3].ID], must.Sprint("expected vol3"))
	must.MapLen(t, 2, got, must.Sprint(`expected 2 volumes in prod node pool`))

	iter, err = store.HostVolumesByNodeID(nil, nodes[2].ID, SortDefault)
	must.NoError(t, err)
	got = consumeIter(iter)
	must.NotNil(t, got[vols[2].ID], must.Sprint("expected vol2"))
	must.NotNil(t, got[vols[3].ID], must.Sprint("expected vol3"))
	must.MapLen(t, 2, got, must.Sprint(`expected 2 volumes on node 2`))

	// simulate a node registering one of the volumes
	nodes[2] = nodes[2].Copy()
	nodes[2].HostVolumes = map[string]*structs.ClientHostVolumeConfig{"example": {
		Name: vols[2].Name,
		Path: vols[2].HostPath,
	}}
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, nodes[2]))

	// update all the volumes, which should update the state of vol2 as well
	for i, vol := range vols {
		vol = vol.Copy()
		vol.RequestedCapacityMax = 300000
		vols[i] = vol
	}
	index++
	must.NoError(t, store.UpsertHostVolumes(index, vols))

	iter, err = store.HostVolumesByName(nil, structs.DefaultNamespace, "example", SortDefault)
	must.NoError(t, err)
	got = consumeIter(iter)
	must.MapLen(t, 2, got, must.Sprint(`expected 2 volumes named "example" in default namespace`))

	vol0 := got[vols[0].ID]
	must.NotNil(t, vol0)
	must.Eq(t, index, vol0.ModifyIndex)
	vol2 := got[vols[2].ID]
	must.NotNil(t, vol2)
	must.Eq(t, index, vol2.ModifyIndex)
	must.Eq(t, structs.HostVolumeStateReady, vol2.State, must.Sprint(
		"expected volume state to be updated because its been fingerprinted by a node"))

	alloc := mock.AllocForNode(nodes[2])
	alloc.Job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{"example": {
		Name:   "example",
		Type:   structs.VolumeTypeHost,
		Source: vols[2].Name,
	}}
	index++
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup,
		index, []*structs.Allocation{alloc}))

	index++
	err = store.DeleteHostVolumes(index, vol2.Namespace, []string{vols[1].ID, vols[2].ID})
	must.EqError(t, err, fmt.Sprintf(
		"could not delete volume %s in use by alloc %s", vols[2].ID, alloc.ID))
	vol, err = store.HostVolumeByID(nil, vols[1].Namespace, vols[1].ID, true)
	must.NoError(t, err)
	must.NotNil(t, vol, must.Sprint("volume that didn't error should not be deleted"))

	err = store.DeleteHostVolumes(index, vol2.Namespace, []string{vols[1].ID})
	must.NoError(t, err)
	vol, err = store.HostVolumeByID(nil, vols[1].Namespace, vols[1].ID, true)
	must.NoError(t, err)
	must.Nil(t, vol)

	vol, err = store.HostVolumeByID(nil, vols[2].Namespace, vols[2].ID, true)
	must.NoError(t, err)
	must.NotNil(t, vol)
	must.Len(t, 1, vol.Allocations)

	iter, err = store.HostVolumes(nil, SortReverse)
	must.NoError(t, err)
	got = consumeIter(iter)
	must.MapLen(t, 3, got, must.Sprint(`expected 3 volumes remain`))
}
