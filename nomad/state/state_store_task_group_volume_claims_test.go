// Copyright (c) HashiCorp, Inc.
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
