// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"testing"

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
			// this should not happen, but test anyway
			name:         "change present",
			vols:         VolumeMap{"changeme": {Path: "before"}},
			volName:      "changeme",
			vol:          &structs.ClientHostVolumeConfig{Path: "after"},
			expectMap:    VolumeMap{"changeme": {Path: "after"}},
			expectChange: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			changed := UpdateVolumeMap(tc.vols, tc.volName, tc.vol)
			must.Eq(t, tc.expectMap, tc.vols)

			if tc.expectChange {
				must.True(t, changed, must.Sprint("expect volume to have been changed"))
			} else {
				must.False(t, changed, must.Sprint("expect volume not to have been changed"))
			}

		})
	}
}
