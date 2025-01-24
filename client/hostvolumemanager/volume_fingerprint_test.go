// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestUpdateVolumeMap(t *testing.T) {
	cases := []struct {
		name string

		vols    VolumeMap
		volName string
		vol     *structs.ClientHostVolumeConfig

		expectMap    VolumeMap
		expectChange bool

		expectLog string
	}{
		{
			name:         "delete absent",
			vols:         VolumeMap{},
			volName:      "anything",
			vol:          nil,
			expectMap:    VolumeMap{},
			expectChange: false,
		},
		{
			name:         "delete present",
			vols:         VolumeMap{"deleteme": {}},
			volName:      "deleteme",
			vol:          nil,
			expectMap:    VolumeMap{},
			expectChange: true,
		},
		{
			name:         "add absent",
			vols:         VolumeMap{},
			volName:      "addme",
			vol:          &structs.ClientHostVolumeConfig{},
			expectMap:    VolumeMap{"addme": {}},
			expectChange: true,
		},
		{
			name:         "add present",
			vols:         VolumeMap{"ignoreme": {}},
			volName:      "ignoreme",
			vol:          &structs.ClientHostVolumeConfig{},
			expectMap:    VolumeMap{"ignoreme": {}},
			expectChange: false,
		},
		{
			// this should not happen with dynamic vols, but test anyway
			name:         "change present",
			vols:         VolumeMap{"changeme": {ID: "before"}},
			volName:      "changeme",
			vol:          &structs.ClientHostVolumeConfig{ID: "after"},
			expectMap:    VolumeMap{"changeme": {ID: "after"}},
			expectChange: true,
		},
		{
			// this should only happen during agent start, if a static vol has
			// been added to config after a previous dynamic vol was created
			// with the same name.
			name:         "override static",
			vols:         VolumeMap{"overrideme": {ID: ""}}, // static vols have no ID
			volName:      "overrideme",
			vol:          &structs.ClientHostVolumeConfig{ID: "dynamic-vol-id"},
			expectMap:    VolumeMap{"overrideme": {ID: "dynamic-vol-id"}},
			expectChange: true,
			expectLog:    "overriding static host volume with dynamic: name=overrideme id=dynamic-vol-id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log, getLogs := logRecorder(t)

			changed := UpdateVolumeMap(log, tc.vols, tc.volName, tc.vol)
			must.Eq(t, tc.expectMap, tc.vols)

			if tc.expectChange {
				must.True(t, changed, must.Sprint("expect volume to have been changed"))
			} else {
				must.False(t, changed, must.Sprint("expect volume not to have been changed"))
			}

			must.StrContains(t, getLogs(), tc.expectLog)
		})
	}
}

func TestWaitForFirstFingerprint(t *testing.T) {
	log := testlog.HCLogger(t)
	tmp := t.TempDir()
	memDB := state.NewMemDB(log)
	node := newFakeNode(t)
	hvm := NewHostVolumeManager(log, Config{
		PluginDir:      "",
		VolumesDir:     tmp,
		StateMgr:       memDB,
		UpdateNodeVols: node.updateVol,
	})
	plug := &fakePlugin{volsDir: tmp}
	hvm.builtIns = map[string]HostVolumePlugin{
		"test-plugin": plug,
	}
	must.NoError(t, memDB.PutDynamicHostVolume(&cstructs.HostVolumeState{
		ID: "vol-id",
		CreateReq: &cstructs.ClientHostVolumeCreateRequest{
			ID:       "vol-id",
			Name:     "vol-name",
			PluginID: "test-plugin",
		},
	}))

	ctx := timeout(t)
	done := hvm.WaitForFirstFingerprint(ctx)
	select {
	case <-ctx.Done():
		t.Fatal("fingerprint timed out")
	case <-done:
	}

	must.Eq(t, "vol-id", plug.created)
	must.Eq(t, VolumeMap{
		"vol-name": &structs.ClientHostVolumeConfig{
			Name:     "vol-name",
			ID:       "vol-id",
			Path:     filepath.Join(tmp, "vol-id"),
			ReadOnly: false,
		},
	}, node.vols)
}
