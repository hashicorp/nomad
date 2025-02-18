// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestTaskGroupHostVolumeClaimEndpoint(t *testing.T) {
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
	dhv1 := &structs.HostVolume{
		Namespace:             structs.DefaultNamespace,
		ID:                    uuid.Generate(),
		Name:                  "foo",
		NodeID:                uuid.Generate(),
		RequestedCapabilities: hostVolCapsReadWrite,
		State:                 structs.HostVolumeStateReady,
	}
	stickyRequests := map[string]*structs.VolumeRequest{
		"foo": {
			Type:           "host",
			Source:         "foo",
			Sticky:         true,
			AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		},
	}
	stickyJob := mock.Job()
	stickyJob.TaskGroups[0].Volumes = stickyRequests

	existingClaims := []*structs.TaskGroupHostVolumeClaim{
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         stickyJob.ID,
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhv1.ID,
			VolumeName:    dhv1.Name,
		},
		{
			ID:            uuid.Generate(),
			Namespace:     "foo",
			JobID:         stickyJob.ID,
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhv1.ID,
			VolumeName:    dhv1.Name,
		},
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "fooooo",
			TaskGroupName: stickyJob.TaskGroups[0].Name,
			VolumeID:      dhv1.ID,
			VolumeName:    dhv1.Name,
		},
	}

	httpTest(t, nil, func(s *TestAgent) {

		// Create a volume claim on the test node

		for _, claim := range existingClaims {
			must.NoError(t, s.Agent.Server().State().UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 1000, claim))
		}

		respW := httptest.NewRecorder()

		// List claims

		req, err := http.NewRequest(http.MethodGet, "/v1/volumes/claims", nil)
		must.NoError(t, err)
		claims, err := s.Server.TaskGroupHostVolumeClaimListRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, claims)
		respClaims := claims.([]*structs.TaskGroupHostVolumeClaim)
		must.NotNil(t, respClaims)
		// must contain all the claims except for the one in other ns
		must.SliceLen(t, len(existingClaims)-1, respClaims)

		// list by fooooo
		req, err = http.NewRequest(http.MethodGet, "/v1/volumes/claims?job_id=fooooo", nil)
		must.NoError(t, err)
		claims, err = s.Server.TaskGroupHostVolumeClaimListRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, claims)
		respClaims = claims.([]*structs.TaskGroupHostVolumeClaim)
		must.NotNil(t, respClaims)
		must.Eq(t, existingClaims[2].ID, respClaims[0].ID)

		// Delete claim number 1

		req, err = http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/volumes/claim/%s", existingClaims[0].ID), nil)
		must.NoError(t, err)
		_, err = s.Server.taskGroupHostVolumeClaimDelete(existingClaims[0].ID, respW, req)
		must.NoError(t, err)

		// Verify claim was deleted

		req, err = http.NewRequest(http.MethodGet, "/v1/volumes/claims", nil)
		must.NoError(t, err)
		claims, err = s.Server.TaskGroupHostVolumeClaimListRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, claims)
		respClaims = claims.([]*structs.TaskGroupHostVolumeClaim)
		must.NotNil(t, respClaims)
		must.SliceNotContains(t, respClaims, existingClaims[0])
	})
}
