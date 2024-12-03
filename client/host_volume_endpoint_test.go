// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	hvm "github.com/hashicorp/nomad/client/hostvolumemanager"
	"github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

func TestHostVolume(t *testing.T) {
	ci.Parallel(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()

	memdb := state.NewMemDB(testlog.HCLogger(t))
	client.stateDB = memdb

	tmp := t.TempDir()
	var err error
	expectDir := filepath.Join(tmp, "test-vol-id")
	client.hostVolumeManager, err = hvm.NewHostVolumeManager(testlog.HCLogger(t),
		client.stateDB, time.Second, "/no/ext/plugins", tmp)
	must.NoError(t, err)

	t.Run("happy", func(t *testing.T) {
		req := &cstructs.ClientHostVolumeCreateRequest{
			ID:       "test-vol-id",
			Name:     "test-vol-name",
			PluginID: "mkdir", // real plugin really makes a dir
		}
		var resp cstructs.ClientHostVolumeCreateResponse
		err := client.ClientRPC("HostVolume.Create", req, &resp)
		must.NoError(t, err)
		must.Eq(t, cstructs.ClientHostVolumeCreateResponse{
			HostPath:      expectDir,
			CapacityBytes: 0, // "mkdir" always returns zero
		}, resp)
		// technically this is testing "mkdir" more than the RPC
		must.DirExists(t, expectDir)
		// ensure we saved to client state
		vols, err := memdb.GetDynamicHostVolumes()
		must.NoError(t, err)
		must.Len(t, 1, vols)
		expectState := &cstructs.HostVolumeState{
			ID:        req.ID,
			CreateReq: req,
		}
		must.Eq(t, expectState, vols[0])

		delReq := &cstructs.ClientHostVolumeDeleteRequest{
			ID:       "test-vol-id",
			PluginID: "mkdir",
			HostPath: expectDir,
		}
		var delResp cstructs.ClientHostVolumeDeleteResponse
		err = client.ClientRPC("HostVolume.Delete", delReq, &delResp)
		must.NoError(t, err)
		must.NotNil(t, delResp)
		// again, actually testing the "mkdir" plugin
		must.DirNotExists(t, expectDir)
		// client state should be deleted
		vols, err = memdb.GetDynamicHostVolumes()
		must.NoError(t, err)
		must.Len(t, 0, vols)
	})

	t.Run("missing plugin", func(t *testing.T) {
		req := &cstructs.ClientHostVolumeCreateRequest{
			PluginID: "non-existent",
		}
		var resp cstructs.ClientHostVolumeCreateResponse
		err := client.ClientRPC("HostVolume.Create", req, &resp)
		must.EqError(t, err, `no such plugin: "non-existent"`)

		delReq := &cstructs.ClientHostVolumeDeleteRequest{
			PluginID: "non-existent",
		}
		var delResp cstructs.ClientHostVolumeDeleteResponse
		err = client.ClientRPC("HostVolume.Delete", delReq, &delResp)
		must.EqError(t, err, `no such plugin: "non-existent"`)
	})

	t.Run("error from plugin", func(t *testing.T) {
		// "mkdir" plugin can't create a directory within a file
		client.hostVolumeManager, err = hvm.NewHostVolumeManager(testlog.HCLogger(t),
			client.stateDB, time.Second, "/no/ext/plugins", "host_volume_endpoint_test.go")
		must.NoError(t, err)

		req := &cstructs.ClientHostVolumeCreateRequest{
			ID:       "test-vol-id",
			Name:     "test-vol-name",
			PluginID: "mkdir",
		}
		var resp cstructs.ClientHostVolumeCreateResponse
		err := client.ClientRPC("HostVolume.Create", req, &resp)
		must.ErrorContains(t, err, "host_volume_endpoint_test.go/test-vol-id: not a directory")

		delReq := &cstructs.ClientHostVolumeDeleteRequest{
			ID:       "test-vol-id",
			PluginID: "mkdir",
		}
		var delResp cstructs.ClientHostVolumeDeleteResponse
		err = client.ClientRPC("HostVolume.Delete", delReq, &delResp)
		must.ErrorContains(t, err, "host_volume_endpoint_test.go/test-vol-id: not a directory")
	})
}
