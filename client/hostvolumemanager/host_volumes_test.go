// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// db TODO(1.10.0): improve hostvolumemanager tests.

func TestNewHostVolumeManager_restoreState(t *testing.T) {
	log := testlog.HCLogger(t)
	vol := &cstructs.HostVolumeState{
		ID: "test-vol-id",
		CreateReq: &cstructs.ClientHostVolumeCreateRequest{
			Name:     "test-vol-name",
			ID:       "test-vol-id",
			PluginID: "mkdir",
		},
	}
	fNode := newFakeNode()

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
			UpdateNodeVols: fNode.updateVol,
			PluginDir:      "/wherever",
			SharedMountDir: mountDir,
		})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		vols, err := hvm.restoreFromState(ctx)
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

	t.Run("get error", func(t *testing.T) {
		state := &cstate.ErrDB{}
		hvm := NewHostVolumeManager(log, Config{
			StateMgr:       state,
			UpdateNodeVols: fNode.updateVol,
			PluginDir:      "/wherever",
			SharedMountDir: "/wherever",
		})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		vols, err := hvm.restoreFromState(ctx)
		// error loading state should break the world
		must.ErrorIs(t, err, cstate.ErrDBError)
		must.Nil(t, vols)
	})

	// db TODO: test plugin error
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
