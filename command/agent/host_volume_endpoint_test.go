// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestHostVolumeEndpoint_CRUD(t *testing.T) {
	httpTest(t, nil, func(s *TestAgent) {

		// Create a volume on the test node

		vol := mock.HostVolumeRequest(structs.DefaultNamespace)
		vol.NodePool = ""
		vol.Constraints = nil
		reqBody := struct {
			Volume *structs.HostVolume
		}{Volume: vol}
		buf := encodeReq(reqBody)
		req, err := http.NewRequest(http.MethodPut, "/v1/volume/host/create", buf)
		must.NoError(t, err)
		respW := httptest.NewRecorder()

		// Make the request and verify we got a valid volume back

		obj, err := s.Server.HostVolumeSpecificRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, obj)
		resp := obj.(*structs.HostVolumeCreateResponse)
		must.NotNil(t, resp.Volume)
		must.Eq(t, vol.Name, resp.Volume.Name)
		must.Eq(t, s.client.NodeID(), resp.Volume.NodeID)
		must.NotEq(t, "", respW.Result().Header.Get("X-Nomad-Index"))

		volID := resp.Volume.ID

		// Verify volume was created

		path, err := url.JoinPath("/v1/volume/host/", volID)
		must.NoError(t, err)
		req, err = http.NewRequest(http.MethodGet, path, nil)
		must.NoError(t, err)
		obj, err = s.Server.HostVolumeSpecificRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, obj)
		respVol := obj.(*structs.HostVolume)
		must.Eq(t, s.client.NodeID(), respVol.NodeID)

		// Update the volume (note: this doesn't update the volume on the client)

		vol = respVol.Copy()
		vol.Parameters = map[string]string{"bar": "foo"} // swaps key and value
		reqBody = struct {
			Volume *structs.HostVolume
		}{Volume: vol}
		buf = encodeReq(reqBody)
		req, err = http.NewRequest(http.MethodPut, "/v1/volume/host/register", buf)
		must.NoError(t, err)
		obj, err = s.Server.HostVolumeSpecificRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, obj)
		regResp := obj.(*structs.HostVolumeRegisterResponse)
		must.NotNil(t, regResp.Volume)
		must.Eq(t, map[string]string{"bar": "foo"}, regResp.Volume.Parameters)

		// Verify volume was updated

		path = fmt.Sprintf("/v1/volumes?type=host&node_id=%s", s.client.NodeID())
		req, err = http.NewRequest(http.MethodGet, path, nil)
		must.NoError(t, err)
		obj, err = s.Server.HostVolumesListRequest(respW, req)
		must.NoError(t, err)
		vols := obj.([]*structs.HostVolumeStub)
		must.Len(t, 1, vols)

		// Delete the volume

		req, err = http.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/volume/host/%s", volID), nil)
		must.NoError(t, err)
		_, err = s.Server.HostVolumeSpecificRequest(respW, req)
		must.NoError(t, err)

		// Verify volume was deleted

		path, err = url.JoinPath("/v1/volume/host/", volID)
		must.NoError(t, err)
		req, err = http.NewRequest(http.MethodGet, path, nil)
		must.NoError(t, err)
		obj, err = s.Server.HostVolumeSpecificRequest(respW, req)
		must.EqError(t, err, "volume not found")
		must.Nil(t, obj)
	})
}
