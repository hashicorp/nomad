// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
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

	nodes := []*structs.Node{}
	for range 5 {
		n := mock.Node()
		nodes = append(nodes, n)
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
				MaxCount: 3,
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

	iter, err := store.SandboxesByName(nil, job.Namespace, sandboxName, "")
	must.NoError(t, err)
	must.Nil(t, iter.Next())

	sandboxID0 := uuid.Generate()

	alloc0 := mock.AllocForNode(nodes[0])
	alloc0.JobID = job.ID
	alloc0.AllocatedResources.Shared.Sandboxes = []*structs.AllocatedSandbox{{
		ID:            sandboxID0,
		Namespace:     job.Namespace,
		Name:          sandboxName,
		CapacityBytes: 100_000_000,
		TTL:           time.Hour,
	}}

	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocsUpdated: []*structs.Allocation{alloc0},
			Job:           job,
		}))

	sandbox0, err := store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodes[0].ID, sandbox0.NodeID)
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
	must.Eq(t, nodes[0].ID, sandbox0.NodeID)
	must.Len(t, 0, sandbox0.AllocIDs, must.Sprintf("alloc claims: %s", sandbox0.AllocIDs))
	t.Logf("sandbox %q freed by client-terminal alloc", sandbox0.ID[:8])

	// 4. simulate schedyler: need to find where to replace the allocation. give
	// us the same sandbox. upsert the new plan result

	iter, err = store.SandboxesByName(nil, job.Namespace, sandboxName, "")
	must.NoError(t, err)
	obj := iter.Next()
	must.NotNil(t, obj)
	sandbox0 = obj.(*structs.SandboxVolume)
	must.Eq(t, nodes[0].ID, sandbox0.NodeID)

	alloc1 := mock.AllocForNode(nodes[0])
	alloc1.JobID = job.ID
	alloc1.PreviousAllocation = alloc0.ID
	alloc1.AllocatedResources.Shared.Sandboxes = []*structs.AllocatedSandbox{{
		ID:            sandbox0.ID,
		Namespace:     job.Namespace,
		Name:          sandboxName,
		CapacityBytes: 100_000_000,
		TTL:           time.Hour,
	}}

	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocsUpdated: []*structs.Allocation{alloc1},
			Job:           job,
		}))

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodes[0].ID, sandbox0.NodeID)
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

	iter, err = store.SandboxesByName(nil, job.Namespace, sandboxName, "")
	must.NoError(t, err)
	obj = iter.Next()
	must.NotNil(t, obj)
	sandbox0 = obj.(*structs.SandboxVolume)
	must.Eq(t, nodes[0].ID, sandbox0.NodeID)
	must.Len(t, 1, sandbox0.AllocIDs)
	must.Eq(t, alloc1.ID, sandbox0.AllocIDs[0])

	obj = iter.Next() // ran out!
	must.Nil(t, obj)

	alloc1 = alloc1.Copy()
	alloc2 := mock.AllocForNode(nodes[1])
	alloc2.JobID = job.ID
	sandboxID1 := uuid.Generate()
	alloc2.AllocatedResources.Shared.Sandboxes = []*structs.AllocatedSandbox{{
		ID:            sandboxID1,
		Namespace:     job.Namespace,
		Name:          sandboxName,
		CapacityBytes: 100_000_000,
		TTL:           time.Hour,
	}}

	index++
	must.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, index,
		&structs.ApplyPlanResultsRequest{
			AllocsUpdated: []*structs.Allocation{alloc1, alloc2},
			Job:           job,
		}))

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodes[0].ID, sandbox0.NodeID)
	must.Len(t, 1, sandbox0.AllocIDs)
	must.Eq(t, alloc1.ID, sandbox0.AllocIDs[0])
	must.Eq(t, lastModified, sandbox0.ModifyIndex) // should not be modified
	t.Logf("sandbox %q unchanged for alloc %q", sandbox0.ID[:8], alloc1.ID[:8])

	sandbox1, err := store.SandboxVolumeByID(nil, job.Namespace, sandboxID1)
	must.NoError(t, err)
	must.NotNil(t, sandbox1)
	must.Eq(t, nodes[1].ID, sandbox1.NodeID)
	must.Len(t, 1, sandbox1.AllocIDs)
	must.Eq(t, alloc2.ID, sandbox1.AllocIDs[0])
	t.Logf("sandbox %q claimed by new alloc %q", sandbox1.ID[:8], alloc2.ID[:8])

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
				{
					ID:        alloc1.ID,
					Namespace: alloc1.Namespace,
				},
				{
					ID:        alloc2.ID,
					Namespace: alloc2.Namespace,
				},
			},
			Job: job,
		}))

	sandbox0, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID0)
	must.NoError(t, err)
	must.NotNil(t, sandbox0)
	must.Eq(t, nodes[0].ID, sandbox0.NodeID)
	must.Len(t, 1, sandbox0.AllocIDs)
	must.Eq(t, alloc1.ID, sandbox0.AllocIDs[0])
	t.Logf("sandbox %q not freed by server-terminal alloc %q", sandbox0.ID[:8], alloc1.ID[:8])

	sandbox1, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID1)
	must.NoError(t, err)
	must.NotNil(t, sandbox1)
	must.Eq(t, nodes[1].ID, sandbox1.NodeID)
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
	must.Eq(t, nodes[0].ID, sandbox0.NodeID)
	must.Len(t, 0, sandbox0.AllocIDs)
	t.Logf("sandbox %q freed by client-terminal alloc", sandbox0.ID[:8])

	sandbox1, err = store.SandboxVolumeByID(nil, job.Namespace, sandboxID1)
	must.NoError(t, err)
	must.NotNil(t, sandbox1)
	must.Eq(t, nodes[1].ID, sandbox1.NodeID)
	must.Len(t, 0, sandbox1.AllocIDs)
	t.Logf("sandbox %q freed by client-terminal alloc", sandbox1.ID[:8])

	// 6. a new job comes along and should claim the old
}
