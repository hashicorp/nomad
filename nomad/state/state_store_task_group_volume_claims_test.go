// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_UpsertTaskGroupHostVolumeClaim(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Mock some objects
	stickyJob := mock.Job()
	hostVolCapsReadWrite := []*structs.HostVolumeCapability{
		{
			AttachmentMode: structs.HostVolumeAttachmentModeFilesystem,
			AccessMode:     structs.HostVolumeAccessModeSingleNodeReader,
		},
		{
			AttachmentMode: structs.HostVolumeAttachmentModeFilesystem,
			AccessMode:     structs.HostVolumeAccessModeSingleNodeWriter,
		},
	}
	node := mock.Node()
	dhv := &structs.HostVolume{
		Namespace:             structs.DefaultNamespace,
		ID:                    uuid.Generate(),
		Name:                  "foo",
		NodeID:                node.ID,
		RequestedCapabilities: hostVolCapsReadWrite,
		State:                 structs.HostVolumeStateReady,
	}
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	claim := mock.TaskGroupHostVolumeClaim(stickyJob, alloc, dhv)

	must.NoError(t, testState.UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 10, claim))

	// Check that the index for the table was modified as expected.
	initialIndex, err := testState.Index(TableTaskGroupHostVolumeClaim)
	must.NoError(t, err)
	must.Eq(t, 10, initialIndex)

	// List all the claims in the table and check the count
	ws := memdb.NewWatchSet()
	iter, err := testState.GetTaskGroupHostVolumeClaims(ws)
	must.NoError(t, err)

	var count int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		claim := raw.(*structs.TaskGroupHostVolumeClaim)
		must.Eq(t, 10, claim.CreateIndex)
		must.Eq(t, 10, claim.ModifyIndex)
	}
	must.Eq(t, 1, count)

	// Try writing another claim for the same alloc
	anotherClaim := mock.TaskGroupHostVolumeClaim(stickyJob, alloc, dhv)
	must.NoError(t, testState.UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 10, anotherClaim))

	// List all the claims in the table and check the count
	iter, err = testState.GetTaskGroupHostVolumeClaims(ws)
	must.NoError(t, err)

	count = 0
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++

		// Ensure the create and modify indexes are populated correctly.
		claim := raw.(*structs.TaskGroupHostVolumeClaim)
		must.Eq(t, 10, claim.CreateIndex)
		must.Eq(t, 10, claim.ModifyIndex)
	}
	must.Eq(t, 1, count)
}

func TestStateStore_TaskGroupHostVolumeClaimsByFields(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Mock some objects
	fooVolID := uuid.Generate()
	claims := []*structs.TaskGroupHostVolumeClaim{
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "foo job",
			TaskGroupName: "foo tg",
			VolumeName:    "foo volume",
			VolumeID:      fooVolID,
		},
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "foo2 job",
			TaskGroupName: "foo tg",
			VolumeName:    "foo volume",
			VolumeID:      fooVolID,
		},
		{
			ID:            uuid.Generate(),
			Namespace:     "bar namespace",
			JobID:         "bar job",
			TaskGroupName: "bar tg",
			VolumeName:    "foo volume",
			VolumeID:      uuid.Generate(),
		},
	}

	for _, c := range claims {
		must.NoError(t, testState.UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 10, c))
	}

	// Check that the index for the table was modified as expected.
	initialIndex, err := testState.Index(TableTaskGroupHostVolumeClaim)
	must.NoError(t, err)
	must.Eq(t, 10, initialIndex)

	// List all the claims in the table and check the count
	ws := memdb.NewWatchSet()
	iter, err := testState.GetTaskGroupHostVolumeClaims(ws)
	must.NoError(t, err)

	var count int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++
	}
	must.Eq(t, 3, count)

	// Get all claims for foo tg, must be exactly 2
	iter, err = testState.TaskGroupHostVolumeClaimsByFields(ws, TgvcSearchableFields{TaskGroupName: "foo tg"})
	must.NoError(t, err)

	foundClaims := []*structs.TaskGroupHostVolumeClaim{}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claim := raw.(*structs.TaskGroupHostVolumeClaim)
		foundClaims = append(foundClaims, claim)
	}
	must.SliceLen(t, 2, foundClaims)
	must.Eq(t, claims[0].ID, foundClaims[0].ID)
	must.Eq(t, claims[1].ID, foundClaims[1].ID)

	// Get all claims for foo job and default ns, must be exactly 1
	iter, err = testState.TaskGroupHostVolumeClaimsByFields(ws, TgvcSearchableFields{Namespace: structs.DefaultNamespace, JobID: "foo job"})
	must.NoError(t, err)

	foundClaims = []*structs.TaskGroupHostVolumeClaim{}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claim := raw.(*structs.TaskGroupHostVolumeClaim)
		foundClaims = append(foundClaims, claim)
	}
	must.SliceLen(t, 1, foundClaims)
	must.Eq(t, claims[0].ID, foundClaims[0].ID)

	// Get all claims for bar ns and bar vol, should be none
	iter, err = testState.TaskGroupHostVolumeClaimsByFields(ws, TgvcSearchableFields{Namespace: "bar namespace", VolumeName: "bar volume"})
	must.NoError(t, err)

	foundClaims = []*structs.TaskGroupHostVolumeClaim{}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claim := raw.(*structs.TaskGroupHostVolumeClaim)
		foundClaims = append(foundClaims, claim)
	}
	must.SliceLen(t, 0, foundClaims)

	// Get all claims for foo volume and wildcard ns, must be exactly 3
	iter, err = testState.TaskGroupHostVolumeClaimsByFields(ws, TgvcSearchableFields{Namespace: structs.AllNamespacesSentinel, VolumeName: "foo volume"})
	must.NoError(t, err)

	foundClaims = []*structs.TaskGroupHostVolumeClaim{}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claim := raw.(*structs.TaskGroupHostVolumeClaim)
		foundClaims = append(foundClaims, claim)
	}
	must.SliceLen(t, 3, foundClaims)
}

func TestStateStore_claimUpsertForAlloc(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)

	job := mock.Job()
	node := mock.Node()
	volumeID := uuid.Generate()
	chv := &structs.ClientHostVolumeConfig{
		Name:     "test-volume",
		Path:     "/data",
		ReadOnly: false,
		ID:       volumeID,
	}

	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 10, nil, job))
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 11, node))

	makeAlloc := func() *structs.Allocation {
		alloc := mock.Alloc()
		alloc.JobID = job.ID
		alloc.Job = job
		alloc.NodeID = node.ID
		alloc.TaskGroup = "web"
		alloc.Namespace = structs.DefaultNamespace
		return alloc
	}

	t.Run("no existing claim creates new claim", func(t *testing.T) {
		txn := store.db.WriteTxn(100)
		t.Cleanup(func() { txn.Abort() })

		alloc := makeAlloc()
		claim, err := store.claimToUpsertForAlloc(txn, alloc, "test-volume", chv)
		must.NoError(t, err)
		must.NotNil(t, claim)
		must.Eq(t, alloc.ID, claim.AllocID)
		must.Eq(t, alloc.JobID, claim.JobID)
		must.Eq(t, alloc.TaskGroup, claim.TaskGroupName)
		must.Eq(t, alloc.Namespace, claim.Namespace)
		must.Eq(t, "test-volume", claim.VolumeName)
		must.Eq(t, volumeID, claim.VolumeID)
	})

	t.Run("existing claim for same alloc is no-op", func(t *testing.T) {
		alloc := makeAlloc()
		existingClaim := &structs.TaskGroupHostVolumeClaim{
			ID:            uuid.Generate(),
			Namespace:     alloc.Namespace,
			JobID:         alloc.JobID,
			TaskGroupName: alloc.TaskGroup,
			AllocID:       alloc.ID,
			VolumeName:    "test-volume",
			VolumeID:      volumeID,
		}

		must.NoError(t, store.UpsertTaskGroupHostVolumeClaim(
			structs.MsgTypeTestSetup, 200, existingClaim))

		txn := store.db.WriteTxn(201)
		t.Cleanup(func() { txn.Abort() })

		claim, err := store.claimToUpsertForAlloc(txn, alloc, "test-volume", chv)
		must.NoError(t, err)
		must.Nil(t, claim) // alloc already owns this claim
	})

	t.Run("existing claim with terminal alloc gets updated for new alloc", func(t *testing.T) {
		oldAlloc := makeAlloc()
		oldAlloc.ClientStatus = structs.AllocClientStatusComplete
		must.NoError(t, store.UpsertAllocs(
			structs.MsgTypeTestSetup, 300, []*structs.Allocation{oldAlloc}))

		existingClaim := &structs.TaskGroupHostVolumeClaim{
			ID:            uuid.Generate(),
			Namespace:     oldAlloc.Namespace,
			JobID:         oldAlloc.JobID,
			TaskGroupName: oldAlloc.TaskGroup,
			AllocID:       oldAlloc.ID,
			VolumeName:    "test-volume",
			VolumeID:      volumeID,
		}

		must.NoError(t, store.UpsertTaskGroupHostVolumeClaim(
			structs.MsgTypeTestSetup, 301, existingClaim))

		newAlloc := makeAlloc()
		newAlloc.ClientStatus = structs.AllocClientStatusRunning

		txn := store.db.WriteTxn(302)
		t.Cleanup(func() { txn.Abort() })

		claim, err := store.claimToUpsertForAlloc(txn, newAlloc, "test-volume", chv)
		must.NoError(t, err)
		must.NotNil(t, claim)
		must.Eq(t, existingClaim.ID, claim.ID) // reuse the existing claim ID
		must.Eq(t, newAlloc.ID, claim.AllocID) // update to new alloc ID
		must.Eq(t, newAlloc.JobID, claim.JobID)
		must.Eq(t, newAlloc.TaskGroup, claim.TaskGroupName)
	})

	t.Run("existing claim with running alloc gets new claim for new alloc", func(t *testing.T) {
		existingAlloc := makeAlloc()
		existingAlloc.ClientStatus = structs.AllocClientStatusRunning
		must.NoError(t, store.UpsertAllocs(
			structs.MsgTypeTestSetup, 400, []*structs.Allocation{existingAlloc}))

		existingClaim := &structs.TaskGroupHostVolumeClaim{
			ID:            uuid.Generate(),
			Namespace:     existingAlloc.Namespace,
			JobID:         existingAlloc.JobID,
			TaskGroupName: existingAlloc.TaskGroup,
			AllocID:       existingAlloc.ID,
			VolumeName:    "test-volume",
			VolumeID:      volumeID,
		}
		must.NoError(t, store.UpsertTaskGroupHostVolumeClaim(
			structs.MsgTypeTestSetup, 401, existingClaim))

		newAlloc := makeAlloc()
		newAlloc.ClientStatus = structs.AllocClientStatusRunning

		txn := store.db.WriteTxn(402)
		t.Cleanup(func() { txn.Abort() })

		claim, err := store.claimToUpsertForAlloc(txn, newAlloc, "test-volume", chv)
		must.NoError(t, err)
		must.NotNil(t, claim)
		must.NotEq(t, existingClaim.ID, claim.ID) // must be new claim ID
		must.Eq(t, newAlloc.ID, claim.AllocID)
		must.Eq(t, newAlloc.JobID, claim.JobID)
		must.Eq(t, newAlloc.TaskGroup, claim.TaskGroupName)
	})
}
