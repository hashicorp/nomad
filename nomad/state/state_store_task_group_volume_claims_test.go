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
