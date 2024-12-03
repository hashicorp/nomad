// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"path/filepath"
	"testing"
	"time"

	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

// db TODO(1.10.0): improve hostvolumemanager tests.

func TestNewHostVolumeManager_restoreState(t *testing.T) {
	log := testlog.HCLogger(t)
	vol := &cstructs.HostVolumeState{
		ID: "test-vol-id",
		CreateReq: &cstructs.ClientHostVolumeCreateRequest{
			ID:       "test-vol-id",
			PluginID: "mkdir",
		},
	}

	t.Run("happy", func(t *testing.T) {
		// put our volume in state
		state := cstate.NewMemDB(log)
		must.NoError(t, state.PutDynamicHostVolume(vol))

		// new volume manager should load it from state and run Create,
		// resulting in a volume directory in this mountDir.
		mountDir := t.TempDir()

		_, err := NewHostVolumeManager(log, state, time.Second, "/wherever", mountDir)
		must.NoError(t, err)

		volPath := filepath.Join(mountDir, vol.ID)
		must.DirExists(t, volPath)
	})

	t.Run("get error", func(t *testing.T) {
		state := &cstate.ErrDB{}
		_, err := NewHostVolumeManager(log, state, time.Second, "/wherever", "/wherever")
		// error loading state should break the world
		must.ErrorIs(t, err, cstate.ErrDBError)
	})

	// db TODO: test plugin error
}
