package agent

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
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

		require.Equal(t, 1, out.ControllersExpected)
		require.Equal(t, 1, out.ControllersHealthy)
		require.Len(t, out.Controllers, 1)

		require.Equal(t, 2, out.NodesExpected)
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

func TestHTTP_CSIEndpointVolume(t *testing.T) {
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
		require.Equal(t, 200, resp.Code)

		req, err = http.NewRequest("GET", "/v1/volume/csi/bar", nil)
		require.NoError(t, err)

		resp = httptest.NewRecorder()
		raw, err := s.Server.CSIVolumeSpecificRequest(resp, req)
		require.NoError(t, err, "get error")
		require.Equal(t, 200, resp.Code)

		out, ok := raw.(*api.CSIVolume)
		require.True(t, ok)

		pretty.Log(out)

		require.Equal(t, 1, out.ControllersExpected)
		require.Equal(t, 1, out.ControllersHealthy)
		require.Equal(t, 2, out.NodesExpected)
		require.Equal(t, 2, out.NodesHealthy)
	})
}
