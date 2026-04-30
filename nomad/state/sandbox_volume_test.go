// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// TestSandboxClaims helps exercise the workflows we want to use for scheduling sandboxes
func TestSandboxClaims(t *testing.T) {

	ci.Parallel(t)
	store := testStateStore(t)
	index, err := store.LatestIndex()
	must.NoError(t, err)

	for range 5 {
		n := mock.Node()
		index++
		must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, index, n))
	}

	// 1. new job

	sandboxName := "foo"

	job := mock.MinJob()
	job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"foo": {
			Name:       sandboxName,
			Type:       "sandbox",
			Source:     sandboxName,
			ReadOnly:   false,
			Sticky:     false,
			AccessMode: structs.HostVolumeAccessModeSingleNodeSingleWriter,
			Sandbox: &structs.SandboxVolumeRequest{
				MaxCount: 2,
				TTL:      time.Hour,
				MinBytes: 100_000_000,
				MaxBytes: 100_000_000,
			},
		},
	}
	index++
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))

	// 2. simulate scheduler: job wants a sandbox but none exists, so the
	// scheduler gives us a new sandbox (with ID) and a node to place it on

	planned := mockScheduleSandboxVolume(t, store, sandboxName, job)
	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocsUpdated: planned,
			Job:           job,
		}))

	iter, err := store.SandboxesByName(nil, job.Namespace, sandboxName, "")
	must.NoError(t, err)
	obj := iter.Next()
	must.NotNil(t, obj)
	sandbox0 := obj.(*structs.SandboxVolume)
	sandboxID0 := sandbox0.ID

	allocs, err := store.AllocsByJob(nil, job.Namespace, job.ID, true)
	must.NoError(t, err)
	alloc0 := allocs[0]

	nodeID0 := sandbox0.NodeID
	must.Len(t, 1, sandbox0.AllocIDs)
	must.Eq(t, alloc0.ID, sandbox0.AllocIDs[0])
	t.Logf("sandbox %q created and claimed by alloc %q", sandbox0.ID[:8], alloc0.ID[:8])

	// 3. allocation is client terminal, so claim is freed

	alloc0 = alloc0.Copy()
	alloc0.ClientStatus = structs.AllocClientStatusComplete

	index++
	must.NoError(t, store.UpdateAllocsFromClient(structs.MsgTypeTestSetup, index,
		[]*structs.Allocation{alloc0}))

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodeID0, sandbox0.NodeID)
	must.Len(t, 0, sandbox0.AllocIDs, must.Sprintf("alloc claims: %s", sandbox0.AllocIDs))
	t.Logf("sandbox %q freed by client-terminal alloc", sandbox0.ID[:8])

	// 4. simulate scheduler: need to find where to replace the allocation. give
	// us the same sandbox. upsert the new plan result

	planned = mockScheduleSandboxVolume(t, store, sandboxName, job)
	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocsUpdated: planned,
			Job:           job,
		}))

	allocs, err = store.AllocsByJob(nil, job.Namespace, job.ID, true)
	must.NoError(t, err)
	alloc1 := allocs[slices.IndexFunc(allocs, func(a *structs.Allocation) bool {
		return a.ID != alloc0.ID
	})]

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodeID0, sandbox0.NodeID)
	must.Len(t, 1, sandbox0.AllocIDs)
	must.Eq(t, alloc1.ID, sandbox0.AllocIDs[0])
	lastModified := sandbox0.ModifyIndex
	t.Logf("sandbox %q claimed by replacement alloc %q", sandbox0.ID[:8], alloc1.ID[:8])

	// 5. job count increases

	job = job.Copy()
	job.TaskGroups[0].Count = 2
	job.Version++
	index++
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))

	// 6. simulate scheduler: keeps one sandbox and creates one new one
	planned = mockScheduleSandboxVolume(t, store, sandboxName, job)
	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocsUpdated: planned,
			Job:           job,
		}))

	allocs, err = store.AllocsByJob(nil, job.Namespace, job.ID, true)
	must.NoError(t, err)
	alloc2 := allocs[slices.IndexFunc(allocs, func(a *structs.Allocation) bool {
		return a.ID != alloc0.ID && a.ID != alloc1.ID
	})]
	sandboxID1 := alloc2.AllocatedResources.Shared.Sandboxes[0].ID

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodeID0, sandbox0.NodeID)
	must.Len(t, 1, sandbox0.AllocIDs)
	must.Eq(t, alloc1.ID, sandbox0.AllocIDs[0])
	must.Eq(t, lastModified, sandbox0.ModifyIndex) // should not be modified
	t.Logf("sandbox %q unchanged for alloc %q", sandbox0.ID[:8], alloc1.ID[:8])

	sandbox1, err := store.SandboxVolumeByID(nil, job.Namespace, sandboxID1)
	must.NoError(t, err)
	must.NotNil(t, sandbox1)
	nodeID1 := sandbox1.NodeID
	must.Len(t, 1, sandbox1.AllocIDs)
	must.Eq(t, alloc2.ID, sandbox1.AllocIDs[0])
	t.Logf("sandbox %q created and claimed by new alloc %q",
		sandbox1.ID[:8], alloc2.ID[:8])

	// 5. job becomes terminal, but claims aren't freed until allocs are client-terminal
	// sandboxes.

	job = job.Copy()
	job.Stop = true

	index++
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))
	alloc1 = alloc1.Copy()
	alloc1.DesiredStatus = structs.AllocDesiredStatusStop
	alloc2 = alloc2.Copy()
	alloc2.DesiredStatus = structs.AllocDesiredStatusStop

	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocsStopped: []*structs.AllocationDiff{
				{ID: alloc1.ID, Namespace: alloc1.Namespace},
				{ID: alloc2.ID, Namespace: alloc2.Namespace},
			},
			Job: job,
		}))

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodeID0, sandbox0.NodeID)
	must.Len(t, 1, sandbox0.AllocIDs)
	must.Eq(t, alloc1.ID, sandbox0.AllocIDs[0])
	t.Logf("sandbox %q not freed by server-terminal alloc %q", sandbox0.ID[:8], alloc1.ID[:8])

	sandbox1, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID1)
	must.NoError(t, err)
	must.NotNil(t, sandbox1)
	must.Eq(t, nodeID1, sandbox1.NodeID)
	must.Len(t, 1, sandbox1.AllocIDs)
	must.Eq(t, alloc2.ID, sandbox1.AllocIDs[0])
	t.Logf("sandbox %q not freed by server-terminal alloc %q", sandbox1.ID[:8], alloc2.ID[:8])

	alloc1 = alloc1.Copy()
	alloc1.ClientStatus = structs.AllocClientStatusComplete
	alloc2 = alloc2.Copy()
	alloc2.ClientStatus = structs.AllocClientStatusComplete

	index++
	must.NoError(t, store.UpdateAllocsFromClient(structs.MsgTypeTestSetup, index,
		[]*structs.Allocation{alloc1, alloc2}))

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodeID0, sandbox0.NodeID)
	must.Len(t, 0, sandbox0.AllocIDs)
	t.Logf("sandbox %q freed by client-terminal alloc", sandbox0.ID[:8])

	sandbox1, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID1)
	must.NoError(t, err)
	must.NotNil(t, sandbox1)
	must.Eq(t, nodeID1, sandbox1.NodeID)
	must.Len(t, 0, sandbox1.AllocIDs)
	t.Logf("sandbox %q freed by client-terminal alloc", sandbox1.ID[:8])

	// 6. a new job comes along

	job2 := mock.MinJob()
	job2.ID = "job2"
	job2.TaskGroups[0].Count = 2
	job2.TaskGroups[0].Volumes = job.TaskGroups[0].Volumes
	index++
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job2))

	// 7. simulate scheduler: new job should claim the old sandboxes, spreading
	// out among them
	newAllocs := mockScheduleSandboxVolume(t, store, sandboxName, job2)

	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocsUpdated: newAllocs,
			Job:           job2,
		}))

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodeID0, sandbox0.NodeID)
	must.Len(t, 1, sandbox0.AllocIDs)
	must.SliceContainsFunc(t, newAllocs, sandbox0.AllocIDs[0],
		func(a *structs.Allocation, id string) bool { return a.ID == id })

	sandbox1, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID1)
	must.NoError(t, err)
	must.NotNil(t, sandbox1)
	must.Eq(t, nodeID1, sandbox1.NodeID)
	must.Len(t, 1, sandbox1.AllocIDs)
	must.SliceContainsFunc(t, newAllocs, sandbox1.AllocIDs[0],
		func(a *structs.Allocation, id string) bool { return a.ID == id })

}

func mockScheduleSandboxVolume(t *testing.T, store *StateStore, sandboxName string, job *structs.Job) []*structs.Allocation {

	plan := []*structs.Allocation{}
	iter, err := store.SandboxesByName(nil, job.Namespace, sandboxName, "")
	must.NoError(t, err)

	nodeIDsSeen := []string{}

	existingAllocs, err := store.AllocsByJob(nil, job.Namespace, job.ID, true)
	must.NoError(t, err)
	existingAllocs = slices.DeleteFunc(existingAllocs,
		func(a *structs.Allocation) bool {
			return a.ClientTerminalStatus()
		})
	count := len(existingAllocs)

	defer func() {
		for _, alloc := range plan {
			fmt.Println("alloc", alloc.ID)
		}
	}()

	req := job.TaskGroups[0].Volumes[sandboxName]
	tgCount := job.TaskGroups[0].Count

	// fast feasibilty path
	// TODO: how do we identify jobs eligible for the fast path?
	// * has a SandboxVolume
	// * has a Sticky Volume
	// * has a constraint on a specific node ID or a "unique" attribute

	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		if count >= req.Sandbox.MaxCount || count >= tgCount {
			return plan
		}

		sandbox := obj.(*structs.SandboxVolume)
		if !sandbox.IsFree(req.AccessMode) {
			continue
		}

		node, _ := store.NodeByID(nil, sandbox.NodeID)
		alloc := mock.AllocForNode(node)
		alloc.JobID = job.ID
		alloc.AllocatedResources.Shared.Sandboxes = []*structs.AllocatedSandbox{{
			ID:            sandbox.ID,
			Namespace:     job.Namespace,
			Name:          sandboxName,
			CapacityBytes: sandbox.CapacityBytes,
			TTL:           sandbox.TTL,
		}}
		plan = append(plan, alloc)
		nodeIDsSeen = append(nodeIDsSeen, node.ID)
		count++
	}

	// normal scheduler path
	for {
		if count >= req.Sandbox.MaxCount || count >= tgCount {
			return plan
		}

		iter, _ := store.Nodes(nil)
		for obj := iter.Next(); obj != nil; obj = iter.Next() {
			node := obj.(*structs.Node)
			if !slices.Contains(nodeIDsSeen, node.ID) {
				alloc := mock.AllocForNode(node)
				alloc.JobID = job.ID
				alloc.AllocatedResources.Shared.Sandboxes = []*structs.AllocatedSandbox{{
					ID:            uuid.Generate(),
					Namespace:     job.Namespace,
					Name:          sandboxName,
					CapacityBytes: req.Sandbox.MaxBytes,
					TTL:           req.Sandbox.TTL,
				}}
				plan = append(plan, alloc)
				break
			}
		}
		count++
	}
}
