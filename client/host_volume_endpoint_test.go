// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/ci"
	hvm "github.com/hashicorp/nomad/client/hostvolumemanager"
	"github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestHostVolume(t *testing.T) {
	ci.Parallel(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()

	memdb := state.NewMemDB(testlog.HCLogger(t))
	client.stateDB = memdb

	tmp := t.TempDir()
	manager := hvm.NewHostVolumeManager(testlog.HCLogger(t), hvm.Config{
		StateMgr:       client.stateDB,
		UpdateNodeVols: client.updateNodeFromHostVol,
		PluginDir:      "/no/ext/plugins",
		VolumesDir:     tmp,
	})
	client.hostVolumeManager = manager
	hostPathCreate := filepath.Join(tmp, "test-vol-id-1")
	hostPathRegister := t.TempDir()

	t.Run("happy", func(t *testing.T) {

		/* create */

		req := &cstructs.ClientHostVolumeCreateRequest{
			Name:     "created-volume",
			ID:       "test-vol-id-1",
			PluginID: "mkdir", // real plugin really makes a dir
		}
		var resp cstructs.ClientHostVolumeCreateResponse
		err := client.ClientRPC("HostVolume.Create", req, &resp)
		must.NoError(t, err)
		must.Eq(t, cstructs.ClientHostVolumeCreateResponse{
			HostPath:      hostPathCreate,
			CapacityBytes: 0, // "mkdir" always returns zero
		}, resp)
		// technically this is testing "mkdir" more than the RPC
		must.DirExists(t, hostPathCreate)
		// ensure we saved to client state
		vols, err := memdb.GetDynamicHostVolumes()
		must.NoError(t, err)
		must.Len(t, 1, vols)
		expectState := &cstructs.HostVolumeState{
			ID:        req.ID,
			CreateReq: req,
			HostPath:  hostPathCreate,
		}
		must.Eq(t, expectState, vols[0])
		// and should be fingerprinted
		must.Eq(t, hvm.VolumeMap{
			req.Name: {
				ID:   req.ID,
				Name: req.Name,
				Path: hostPathCreate,
			},
		}, client.Node().HostVolumes)

		/* register */

		regReq := &cstructs.ClientHostVolumeRegisterRequest{
			ID:            "test-vol-id-2",
			Name:          "registered-volume",
			NodeID:        uuid.Generate(),
			HostPath:      hostPathRegister,
			CapacityBytes: 1000,
		}
		var regResp cstructs.ClientHostVolumeRegisterResponse
		err = client.ClientRPC("HostVolume.Register", regReq, &regResp)
		must.NoError(t, err)

		// ensure we saved to client state
		vols, err = memdb.GetDynamicHostVolumes()
		must.NoError(t, err)
		must.Len(t, 2, vols)
		sort.Slice(vols, func(i, j int) bool { return vols[i].ID < vols[j].ID })
		expectState = &cstructs.HostVolumeState{
			ID:       regReq.ID,
			HostPath: hostPathRegister,
			CreateReq: &cstructs.ClientHostVolumeCreateRequest{
				ID:     regReq.ID,
				Name:   regReq.Name,
				NodeID: regReq.NodeID,
			},
		}
		must.Eq(t, expectState, vols[1])
		// and should be fingerprinted
		must.Eq(t, hvm.VolumeMap{
			req.Name: {
				ID:   req.ID,
				Name: req.Name,
				Path: hostPathCreate,
			},
			regReq.Name: {
				ID:   regReq.ID,
				Name: regReq.Name,
				Path: hostPathRegister,
			},
		}, client.Node().HostVolumes)

		/* delete */

		delReq := &cstructs.ClientHostVolumeDeleteRequest{
			Name:     "created-volume",
			ID:       "test-vol-id-1",
			PluginID: "mkdir",
			HostPath: hostPathCreate,
		}
		var delResp cstructs.ClientHostVolumeDeleteResponse
		err = client.ClientRPC("HostVolume.Delete", delReq, &delResp)
		must.NoError(t, err)
		must.NotNil(t, delResp)
		// again, actually testing the "mkdir" plugin
		must.DirNotExists(t, hostPathCreate)
		// client state should be deleted
		vols, err = memdb.GetDynamicHostVolumes()
		must.NoError(t, err)
		must.Len(t, 1, vols)
		// and the fingerprint, too
		must.Eq(t, hvm.VolumeMap{
			regReq.Name: {
				ID:   regReq.ID,
				Name: regReq.Name,
				Path: hostPathRegister,
			},
		}, client.Node().HostVolumes)

		delReq.Name = "registered-volume"
		delReq.ID = "test-vol-id-2"
		err = client.ClientRPC("HostVolume.Delete", delReq, &delResp)
		must.NoError(t, err)
		must.NotNil(t, delResp)

		vols, err = memdb.GetDynamicHostVolumes()
		must.NoError(t, err)
		must.Len(t, 0, vols)
		must.Eq(t, hvm.VolumeMap{}, client.Node().HostVolumes)
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
		client.hostVolumeManager = hvm.NewHostVolumeManager(testlog.HCLogger(t), hvm.Config{
			StateMgr:       client.stateDB,
			UpdateNodeVols: client.updateNodeFromHostVol,
			PluginDir:      "/no/ext/plugins",
			VolumesDir:     "host_volume_endpoint_test.go",
		})

		req := &cstructs.ClientHostVolumeCreateRequest{
			ID:       "test-vol-id-1",
			Name:     "created-volume",
			PluginID: "mkdir",
		}
		var resp cstructs.ClientHostVolumeCreateResponse
		err := client.ClientRPC("HostVolume.Create", req, &resp)
		must.ErrorContains(t, err, "host_volume_endpoint_test.go/test-vol-id-1: not a directory")

		delReq := &cstructs.ClientHostVolumeDeleteRequest{
			ID:       "test-vol-id-1",
			PluginID: "mkdir",
		}
		var delResp cstructs.ClientHostVolumeDeleteResponse
		err = client.ClientRPC("HostVolume.Delete", delReq, &delResp)
		must.ErrorContains(t, err, "host_volume_endpoint_test.go/test-vol-id-1: not a directory")
	})
}
