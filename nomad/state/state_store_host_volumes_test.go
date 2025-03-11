// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
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
	must.NoError(t, store.UpsertHostVolume(index, vols[0]))
	must.NoError(t, store.UpsertHostVolume(index, vols[1]))
	must.NoError(t, store.UpsertHostVolume(index, vols[2]))
	must.NoError(t, store.UpsertHostVolume(index, vols[3]))

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
	index++
	for i, vol := range vols {
		vol = vol.Copy()
		vol.RequestedCapacityMaxBytes = 300000
		vols[i] = vol
		must.NoError(t, store.UpsertHostVolume(index, vol))
	}

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
	err = store.DeleteHostVolume(index, vol2.Namespace, vols[2].ID)
	must.EqError(t, err, fmt.Sprintf(
		"could not delete volume %s in use by alloc %s", vols[2].ID, alloc.ID))

	err = store.DeleteHostVolume(index, vol2.Namespace, vols[1].ID)
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

	prefix := vol.ID[:30] // sufficiently long prefix to avoid flakes
	iter, err = store.HostVolumesByIDPrefix(nil, "*", prefix, SortDefault)
	must.NoError(t, err)
	got = consumeIter(iter)
	must.MapLen(t, 1, got, must.Sprint(`expected only one volume to match prefix`))

	iter, err = store.HostVolumesByIDPrefix(nil, vol.Namespace, prefix, SortDefault)
	must.NoError(t, err)
	got = consumeIter(iter)
	must.MapLen(t, 1, got, must.Sprint(`expected only one volume to match prefix`))

	alloc = alloc.Copy()
	alloc.ClientStatus = structs.AllocClientStatusComplete
	index++
	must.NoError(t, store.UpdateAllocsFromClient(structs.MsgTypeTestSetup,
		index, []*structs.Allocation{alloc}))
	for _, v := range vols {
		index++
		must.NoError(t, store.DeleteHostVolume(index, v.Namespace, v.ID))
	}
	iter, err = store.HostVolumes(nil, SortDefault)
	got = consumeIter(iter)
	must.MapLen(t, 0, got, must.Sprint(`expected no volumes to remain`))
}

func TestStateStore_UpdateHostVolumesFromFingerprint(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)
	index, err := store.LatestIndex()
	must.NoError(t, err)

	node := mock.Node()
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		"static-vol": {Name: "static-vol", Path: "/srv/static"},
		"dhv-zero":   {Name: "dhv-zero", Path: "/var/nomad/alloc_mounts" + uuid.Generate()},
	}
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup,
		index, node, NodeUpsertWithNodePool))

	otherNode := mock.Node()

	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup,
		index, otherNode, NodeUpsertWithNodePool))

	ns := structs.DefaultNamespace

	vols := []*structs.HostVolume{
		mock.HostVolume(),
		mock.HostVolume(),
		mock.HostVolume(),
		mock.HostVolume(),
	}

	// a volume that's been fingerprinted before we can write it to state
	vols[0].Name = "dhv-zero"
	vols[0].NodeID = node.ID

	// a volume that will match the new fingerprint
	vols[1].Name = "dhv-one"
	vols[1].NodeID = node.ID

	// a volume that matches the new fingerprint but on the wrong node
	vols[2].Name = "dhv-one"
	vols[2].NodeID = otherNode.ID

	// a volume that won't be fingerprinted
	vols[3].Name = "dhv-two"
	vols[3].NodeID = node.ID

	index++
	oldIndex := index
	must.NoError(t, store.UpsertHostVolume(index, vols[0]))
	must.NoError(t, store.UpsertHostVolume(index, vols[1]))
	must.NoError(t, store.UpsertHostVolume(index, vols[2]))
	must.NoError(t, store.UpsertHostVolume(index, vols[3]))

	vol0, err := store.HostVolumeByID(nil, ns, vols[0].ID, false)
	must.NoError(t, err)
	must.Eq(t, structs.HostVolumeStateReady, vol0.State,
		must.Sprint("previously-fingerprinted volume should be in ready state"))

	// update the fingerprint

	node = node.Copy()
	node.HostVolumes["dhv-one"] = &structs.ClientHostVolumeConfig{
		Name: "dhv-one",
		Path: "/var/nomad/alloc_mounts" + uuid.Generate(),
	}

	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, node))

	vol0, err = store.HostVolumeByID(nil, ns, vols[0].ID, false)
	must.NoError(t, err)
	must.Eq(t, oldIndex, vol0.ModifyIndex, must.Sprint("expected no further update"))
	must.Eq(t, structs.HostVolumeStateReady, vol0.State)

	vol1, err := store.HostVolumeByID(nil, ns, vols[1].ID, false)
	must.NoError(t, err)
	must.Eq(t, index, vol1.ModifyIndex,
		must.Sprint("fingerprint should update pending volume"))
	must.Eq(t, structs.HostVolumeStateReady, vol1.State)

	vol2, err := store.HostVolumeByID(nil, ns, vols[2].ID, false)
	must.NoError(t, err)
	must.Eq(t, oldIndex, vol2.ModifyIndex,
		must.Sprint("volume on other node should not change"))
	must.Eq(t, structs.HostVolumeStatePending, vol2.State)

	vol3, err := store.HostVolumeByID(nil, ns, vols[3].ID, false)
	must.NoError(t, err)
	must.Eq(t, oldIndex, vol3.ModifyIndex,
		must.Sprint("volume not fingerprinted should not change"))
	must.Eq(t, structs.HostVolumeStatePending, vol3.State)

	// update the node pool and fingerprint
	otherNode = otherNode.Copy()
	otherNode.NodePool = "new-node-pool"
	otherNode.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		"dhv-one": {Name: "dhv-one", Path: "/var/nomad/alloc_mounts" + uuid.Generate()},
	}
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, otherNode))

	vol2, err = store.HostVolumeByID(nil, ns, vols[2].ID, false)
	must.NoError(t, err)
	must.Eq(t, index, vol2.ModifyIndex,
		must.Sprint("node pool change should update pending volume"))
	must.Eq(t, "new-node-pool", vol2.NodePool)
	must.Eq(t, structs.HostVolumeStateReady, vol2.State)

	// node restarts and fails to restore
	node = node.Copy()
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		"static-vol": {Name: "static-vol", Path: "/srv/static"},
	}
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, node))

	vol0, err = store.HostVolumeByID(nil, ns, vols[0].ID, false)
	must.NoError(t, err)
	must.Eq(t, index, vol0.ModifyIndex,
		must.Sprint("failed restore should update ready volume"))
	must.Eq(t, structs.HostVolumeStateUnavailable, vol0.State)

	vol1, err = store.HostVolumeByID(nil, ns, vols[1].ID, false)
	must.NoError(t, err)
	must.Eq(t, index, vol1.ModifyIndex,
		must.Sprint("failed restore should update ready volume"))
	must.Eq(t, structs.HostVolumeStateUnavailable, vol1.State)

	// make sure we can go from unavailable to available

	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		"static-vol": {Name: "static-vol", Path: "/srv/static"},
		"dhv-zero":   {Name: "dhv-zero", Path: "/var/nomad/alloc_mounts" + uuid.Generate()},
	}
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, node))

	vol0, err = store.HostVolumeByID(nil, ns, vols[0].ID, false)
	must.NoError(t, err)
	must.Eq(t, index, vol0.ModifyIndex,
		must.Sprint("recovered node should update unavailable volume"))
	must.Eq(t, structs.HostVolumeStateReady, vol0.State)

	// down a node
	node = node.Copy()
	node.Status = structs.NodeStatusDown
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, node))
	vol0, err = store.HostVolumeByID(nil, ns, vols[0].ID, false)
	must.NoError(t, err)
	must.Eq(t, index, vol0.ModifyIndex,
		must.Sprint("downed node should mark volume unavailable"))
	must.Eq(t, structs.HostVolumeStateUnavailable, vol0.State)

	// bring the node back up
	node = node.Copy()
	node.Status = structs.NodeStatusReady
	index++
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, node))
	vol0, err = store.HostVolumeByID(nil, ns, vols[0].ID, false)
	must.NoError(t, err)
	must.Eq(t, index, vol0.ModifyIndex,
		must.Sprint("ready node should update unavailable volume"))
	must.Eq(t, structs.HostVolumeStateReady, vol0.State)
}
