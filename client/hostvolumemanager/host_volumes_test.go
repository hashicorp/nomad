// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestHostVolumeManager(t *testing.T) {
	log := testlog.HCLogger(t)
	tmp := t.TempDir()
	errDB := &cstate.ErrDB{}
	memDB := cstate.NewMemDB(log)
	node := newFakeNode()

	hvm := NewHostVolumeManager(log, Config{
		PluginDir:      "./test_fixtures",
		SharedMountDir: tmp,
		StateMgr:       errDB,
		UpdateNodeVols: node.updateVol,
	})

	plug := &fakePlugin{mountDir: tmp}
	hvm.builtIns["test-plugin"] = plug

	ctx := timeout(t)

	t.Run("create", func(t *testing.T) {
		// plugin doesn't exist
		req := &cstructs.ClientHostVolumeCreateRequest{
			ID:       "vol-id",
			Name:     "vol-name",
			PluginID: "nope",

			RequestedCapacityMinBytes: 5,
		}
		_, err := hvm.Create(ctx, req)
		must.ErrorIs(t, err, ErrPluginNotExists)

		// error from plugin
		req.PluginID = "test-plugin"
		plug.createErr = errors.New("sad create")
		_, err = hvm.Create(ctx, req)
		must.ErrorIs(t, err, plug.createErr)
		plug.reset()

		// error saving state, then error from cleanup attempt
		plug.deleteErr = errors.New("sad delete")
		_, err = hvm.Create(ctx, req)
		must.ErrorIs(t, err, cstate.ErrDBError)
		must.ErrorIs(t, err, plug.deleteErr)
		plug.reset()

		// error saving state, successful cleanup
		_, err = hvm.Create(ctx, req)
		must.ErrorIs(t, err, cstate.ErrDBError)
		must.Eq(t, "vol-id", plug.deleted)
		plug.reset()

		// happy path
		hvm.stateMgr = memDB
		resp, err := hvm.Create(ctx, req)
		must.NoError(t, err)
		must.Eq(t, &cstructs.ClientHostVolumeCreateResponse{
			VolumeName:    "vol-name",
			VolumeID:      "vol-id",
			HostPath:      tmp,
			CapacityBytes: 5,
		}, resp)
		stateDBs, err := memDB.GetDynamicHostVolumes()
		must.NoError(t, err)
		// should be saved to state
		must.Len(t, 1, stateDBs)
		must.Eq(t, "vol-id", stateDBs[0].ID)
		must.Eq(t, "vol-id", stateDBs[0].CreateReq.ID)
		// should be registered with node
		must.MapContainsKey(t, node.vols, "vol-name", must.Sprintf("no vol-name in %+v", node.vols))
	})

	t.Run("delete", func(t *testing.T) {
		// plugin doesn't exist
		req := &cstructs.ClientHostVolumeDeleteRequest{
			ID:       "vol-id",
			Name:     "vol-name",
			PluginID: "nope",
		}
		_, err := hvm.Delete(ctx, req)
		must.ErrorIs(t, err, ErrPluginNotExists)

		// error from plugin
		req.PluginID = "test-plugin"
		plug.deleteErr = errors.New("sad delete")
		_, err = hvm.Delete(ctx, req)
		must.ErrorIs(t, err, plug.deleteErr)
		plug.reset()

		// error saving state
		hvm.stateMgr = errDB
		_, err = hvm.Delete(ctx, req)
		must.ErrorIs(t, err, cstate.ErrDBError)

		// happy path
		// add stuff that should be deleted
		hvm.stateMgr = memDB
		must.NoError(t, memDB.PutDynamicHostVolume(&cstructs.HostVolumeState{
			ID:        "vol-id",
			CreateReq: &cstructs.ClientHostVolumeCreateRequest{},
		}))
		node.vols = VolumeMap{
			"vol-name": &structs.ClientHostVolumeConfig{},
		}
		// and delete it
		resp, err := hvm.Delete(ctx, req)
		must.NoError(t, err)
		must.Eq(t, &cstructs.ClientHostVolumeDeleteResponse{
			VolumeName: "vol-name",
			VolumeID:   "vol-id",
		}, resp)
		must.Eq(t, VolumeMap{}, node.vols, must.Sprint("vols should be deleted from node"))
		stateVols, err := memDB.GetDynamicHostVolumes()
		must.NoError(t, err)
		must.Nil(t, stateVols, must.Sprint("vols should be deleted from state"))
	})
}

type fakePlugin struct {
	mountDir       string
	created        string
	deleted        string
	fingerprintErr error
	createErr      error
	deleteErr      error
}

func (p *fakePlugin) reset() {
	p.deleted, p.fingerprintErr, p.createErr, p.deleteErr = "", nil, nil, nil
}

func (p *fakePlugin) Fingerprint(_ context.Context) (*PluginFingerprint, error) {
	if p.fingerprintErr != nil {
		return nil, p.fingerprintErr
	}
	v, err := version.NewVersion("0.0.1")
	return &PluginFingerprint{
		Version: v,
	}, err
}

func (p *fakePlugin) Create(_ context.Context, req *cstructs.ClientHostVolumeCreateRequest) (*HostVolumePluginCreateResponse, error) {
	if p.createErr != nil {
		return nil, p.createErr
	}
	p.created = req.ID
	return &HostVolumePluginCreateResponse{
		Path:      p.mountDir,
		SizeBytes: req.RequestedCapacityMinBytes,
	}, nil
}

func (p *fakePlugin) Delete(_ context.Context, req *cstructs.ClientHostVolumeDeleteRequest) error {
	if p.deleteErr != nil {
		return p.deleteErr
	}
	p.deleted = req.ID
	return nil
}

func TestHostVolumeManager_restoreState(t *testing.T) {
	log := testlog.HCLogger(t)
	vol := &cstructs.HostVolumeState{
		ID: "test-vol-id",
		CreateReq: &cstructs.ClientHostVolumeCreateRequest{
			Name:     "test-vol-name",
			ID:       "test-vol-id",
			PluginID: "mkdir",
		},
	}
	node := newFakeNode()

	t.Run("no vols", func(t *testing.T) {
		state := cstate.NewMemDB(log)
		hvm := NewHostVolumeManager(log, Config{
			StateMgr: state,
			// no other fields are necessary when there are zero volumes
		})
		vols, err := hvm.restoreFromState(timeout(t))
		must.NoError(t, err)
		must.Eq(t, VolumeMap{}, vols)
	})

	t.Run("happy", func(t *testing.T) {
		// put our volume in state
		state := cstate.NewMemDB(log)
		must.NoError(t, state.PutDynamicHostVolume(vol))

		// new volume manager should load it from state and run Create,
		// resulting in a volume directory in this mountDir.
		mountDir := t.TempDir()
		volPath := filepath.Join(mountDir, vol.ID)

		hvm := NewHostVolumeManager(log, Config{
			StateMgr:       state,
			UpdateNodeVols: node.updateVol,
			PluginDir:      "/wherever",
			SharedMountDir: mountDir,
		})

		vols, err := hvm.restoreFromState(timeout(t))
		must.NoError(t, err)

		expect := map[string]*structs.ClientHostVolumeConfig{
			"test-vol-name": {
				Name:     "test-vol-name",
				ID:       "test-vol-id",
				Path:     volPath,
				ReadOnly: false,
			},
		}
		must.Eq(t, expect, vols)

		must.DirExists(t, volPath)
	})

	t.Run("state error", func(t *testing.T) {
		state := &cstate.ErrDB{}
		hvm := NewHostVolumeManager(log, Config{StateMgr: state})
		vols, err := hvm.restoreFromState(timeout(t))
		must.ErrorIs(t, err, cstate.ErrDBError)
		must.Nil(t, vols)
	})

	t.Run("plugin missing", func(t *testing.T) {
		state := cstate.NewMemDB(log)
		vol := &cstructs.HostVolumeState{
			CreateReq: &cstructs.ClientHostVolumeCreateRequest{
				PluginID: "nonexistent-plugin",
			},
		}
		must.NoError(t, state.PutDynamicHostVolume(vol))

		hvm := NewHostVolumeManager(log, Config{StateMgr: state})
		vols, err := hvm.restoreFromState(timeout(t))
		must.ErrorIs(t, err, ErrPluginNotExists)
		must.MapEmpty(t, vols)
	})

	t.Run("plugin error", func(t *testing.T) {
		state := cstate.NewMemDB(log)
		vol := &cstructs.HostVolumeState{
			ID: "test-volume",
			CreateReq: &cstructs.ClientHostVolumeCreateRequest{
				PluginID: "test-plugin",
			},
		}
		must.NoError(t, state.PutDynamicHostVolume(vol))

		log, getLogs := logRecorder(t)
		hvm := NewHostVolumeManager(log, Config{StateMgr: state})
		plug := &fakePlugin{
			createErr: errors.New("sad create"),
		}
		hvm.builtIns["test-plugin"] = plug

		vols, err := hvm.restoreFromState(timeout(t))
		must.NoError(t, err)
		must.NotNil(t, vols)
		logs := getLogs()
		must.StrContains(t, logs, "[ERROR]")
		must.StrContains(t, logs, `failed to restore: plugin_id=test-plugin volume_id=test-volume error="sad create"`)
	})
}

type fakeNode struct {
	vols VolumeMap
}

func (n *fakeNode) updateVol(name string, volume *structs.ClientHostVolumeConfig) {
	UpdateVolumeMap(n.vols, name, volume)
}

func newFakeNode() *fakeNode {
	return &fakeNode{
		vols: make(VolumeMap),
	}
}

// timeout provides a context that times out in 1 second
func timeout(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return ctx
}

// logRecorder is here so we can assert that stdout/stderr appear in logs
func logRecorder(t *testing.T) (hclog.Logger, func() string) {
	t.Helper()
	buf := &bytes.Buffer{}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:            "log-recorder",
		Output:          buf,
		Level:           hclog.Debug,
		IncludeLocation: true,
		DisableTime:     true,
	})
	return logger, func() string {
		bts, err := io.ReadAll(buf)
		test.NoError(t, err)
		return string(bts)
	}
}
