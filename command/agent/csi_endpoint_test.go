package agent

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestHTTP_CSIEndpointPlugin(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		server := s.Agent.Server()
		cleanup := state.CreateTestCSIPlugin(server.State(), "foo")
		defer cleanup()

		body := bytes.NewBuffer(nil)
		req, err := http.NewRequest("GET", "/v1/plugin/csi/foo", body)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		obj, err := s.Server.CSIPluginSpecificRequest(resp, req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.Code)

		out, ok := obj.(*api.CSIPlugin)
		require.True(t, ok)

		// ControllersExpected is 0 because this plugin was created without a job,
		// which sets expected
		require.Equal(t, 0, out.ControllersExpected)
		require.Equal(t, 1, out.ControllersHealthy)
		require.Len(t, out.Controllers, 1)

		require.Equal(t, 0, out.NodesExpected)
		require.Equal(t, 2, out.NodesHealthy)
		require.Len(t, out.Nodes, 2)
	})
}

func TestHTTP_CSIEndpointUtils(t *testing.T) {
	secrets := structsCSISecretsToApi(structs.CSISecrets{
		"foo": "bar",
	})

	require.Equal(t, "bar", secrets["foo"])

	tops := structsCSITopolgiesToApi([]*structs.CSITopology{{
		Segments: map[string]string{"foo": "bar"},
	}})

	require.Equal(t, "bar", tops[0].Segments["foo"])
}

func TestHTTP_CSIEndpointRegisterVolume(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		server := s.Agent.Server()
		cleanup := state.CreateTestCSIPluginNodeOnly(server.State(), "foo")
		defer cleanup()

		args := structs.CSIVolumeRegisterRequest{
			Volumes: []*structs.CSIVolume{{
				ID:             "bar",
				PluginID:       "foo",
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			}},
		}
		body := encodeReq(args)
		req, err := http.NewRequest("PUT", "/v1/volumes", body)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		_, err = s.Server.CSIVolumesRequest(resp, req)
		require.NoError(t, err, "put error")

		req, err = http.NewRequest("GET", "/v1/volume/csi/bar", nil)
		require.NoError(t, err)
		resp = httptest.NewRecorder()
		raw, err := s.Server.CSIVolumeSpecificRequest(resp, req)
		require.NoError(t, err, "get error")
		out, ok := raw.(*api.CSIVolume)
		require.True(t, ok)
		require.Equal(t, 1, out.ControllersHealthy)
		require.Equal(t, 2, out.NodesHealthy)

		req, err = http.NewRequest("DELETE", "/v1/volume/csi/bar/detach", nil)
		require.NoError(t, err)
		resp = httptest.NewRecorder()
		_, err = s.Server.CSIVolumeSpecificRequest(resp, req)
		require.Equal(t, CodedError(400, "detach requires node ID"), err)
	})
}

func TestHTTP_CSIEndpointCreateVolume(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		server := s.Agent.Server()
		cleanup := state.CreateTestCSIPlugin(server.State(), "foo")
		defer cleanup()

		args := structs.CSIVolumeCreateRequest{
			Volumes: []*structs.CSIVolume{{
				ID:             "baz",
				PluginID:       "foo",
				AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
				AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
			}},
		}
		body := encodeReq(args)
		req, err := http.NewRequest("PUT", "/v1/volumes/create", body)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		_, err = s.Server.CSIVolumesRequest(resp, req)
		require.Error(t, err, "controller validate volume: No path to node")

		req, err = http.NewRequest("DELETE", "/v1/volume/csi/baz", nil)
		require.NoError(t, err)
		resp = httptest.NewRecorder()
		_, err = s.Server.CSIVolumeSpecificRequest(resp, req)
		require.Error(t, err, "volume not found: baz")
	})
}

func TestHTTP_CSIEndpointSnapshot(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		server := s.Agent.Server()
		cleanup := state.CreateTestCSIPlugin(server.State(), "foo")
		defer cleanup()

		args := &api.CSISnapshotCreateRequest{
			Snapshots: []*api.CSISnapshot{{
				Name:           "snap-*",
				PluginID:       "foo",
				SourceVolumeID: "bar",
			}},
		}
		body := encodeReq(args)
		req, err := http.NewRequest("PUT", "/v1/volumes/snapshot", body)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		_, err = s.Server.CSISnapshotsRequest(resp, req)
		require.Error(t, err, "no such volume: bar")
	})
}

// TestHTTP_CSIEndpoint_Cast is a smoke test for converting from structs to
// API structs
func TestHTTP_CSIEndpoint_Cast(t *testing.T) {
	t.Parallel()

	plugin := mock.CSIPlugin()
	plugin.Nodes["node1"] = &structs.CSIInfo{
		PluginID: plugin.ID,
		AllocID:  "alloc1",
		NodeInfo: &structs.CSINodeInfo{ID: "instance-1", MaxVolumes: 3},
	}
	apiPlugin := structsCSIPluginToApi(plugin)
	require.Equal(t,
		plugin.Nodes["node1"].NodeInfo.MaxVolumes,
		apiPlugin.Nodes["node1"].NodeInfo.MaxVolumes)

	vol := mock.CSIVolume(plugin)
	apiVol := structsCSIVolumeToApi(vol)
	require.Equal(t, vol.MountOptions.MountFlags, apiVol.MountOptions.MountFlags)
}
