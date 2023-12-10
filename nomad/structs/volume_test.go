// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestVolumeRequest_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name           string
		expected       []string
		canariesCount  int
		taskGroupCount int
		req            *VolumeRequest
	}{
		{
			name:     "host volume with empty source",
			expected: []string{"volume has an empty source"},
			req: &VolumeRequest{
				Type: VolumeTypeHost,
			},
		},
		{
			name: "host volume with CSI volume config",
			expected: []string{
				"host volumes cannot have an access mode",
				"host volumes cannot have an attachment mode",
				"host volumes cannot have mount options",
				"volume cannot be per_alloc for system or sysbatch jobs",
				"volume cannot be per_alloc when canaries are in use",
			},
			canariesCount: 1,
			req: &VolumeRequest{
				Type:           VolumeTypeHost,
				ReadOnly:       false,
				AccessMode:     CSIVolumeAccessModeSingleNodeReader,
				AttachmentMode: CSIVolumeAttachmentModeBlockDevice,
				MountOptions: &CSIMountOptions{
					FSType:     "ext4",
					MountFlags: []string{"ro"},
				},
				PerAlloc: true,
			},
		},
		{
			name: "CSI volume multi-reader-single-writer access mode",
			expected: []string{
				"volume with multi-node-single-writer access mode allows only one writer",
			},
			taskGroupCount: 2,
			req: &VolumeRequest{
				Type:       VolumeTypeCSI,
				AccessMode: CSIVolumeAccessModeMultiNodeSingleWriter,
			},
		},
		{
			name: "CSI volume single reader access mode",
			expected: []string{
				"volume with single-node-reader-only access mode allows only one reader",
			},
			taskGroupCount: 2,
			req: &VolumeRequest{
				Type:       VolumeTypeCSI,
				AccessMode: CSIVolumeAccessModeSingleNodeReader,
				ReadOnly:   true,
			},
		},
		{
			name: "CSI volume per-alloc with canaries",
			expected: []string{
				"volume cannot be per_alloc for system or sysbatch jobs",
				"volume cannot be per_alloc when canaries are in use",
			},
			canariesCount: 1,
			req: &VolumeRequest{
				Type:     VolumeTypeCSI,
				PerAlloc: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate(JobTypeSystem, tc.taskGroupCount, tc.canariesCount)
			for _, expected := range tc.expected {
				require.Contains(t, err.Error(), expected)
			}
		})
	}

}

func TestVolumeRequest_Equal(t *testing.T) {
	ci.Parallel(t)

	must.Equal[*VolumeRequest](t, nil, nil)
	must.NotEqual[*VolumeRequest](t, nil, new(VolumeRequest))

	must.StructEqual(t, &VolumeRequest{
		Name:           "name",
		Type:           "type",
		Source:         "source",
		ReadOnly:       true,
		AccessMode:     "access",
		AttachmentMode: "attachment",
		MountOptions: &CSIMountOptions{
			FSType:     "fs1",
			MountFlags: []string{"flag1"},
		},
		PerAlloc: true,
	}, []must.Tweak[*VolumeRequest]{{
		Field: "Name",
		Apply: func(vr *VolumeRequest) { vr.Name = "name2" },
	}, {
		Field: "Type",
		Apply: func(vr *VolumeRequest) { vr.Type = "type2" },
	}, {
		Field: "Source",
		Apply: func(vr *VolumeRequest) { vr.Source = "source2" },
	}, {
		Field: "ReadOnly",
		Apply: func(vr *VolumeRequest) { vr.ReadOnly = false },
	}, {
		Field: "AccessMode",
		Apply: func(vr *VolumeRequest) { vr.AccessMode = "access2" },
	}, {
		Field: "AttachmentMode",
		Apply: func(vr *VolumeRequest) { vr.AttachmentMode = "attachment2" },
	}, {
		Field: "MountOptions",
		Apply: func(vr *VolumeRequest) { vr.MountOptions = nil },
	}, {
		Field: "PerAlloc",
		Apply: func(vr *VolumeRequest) { vr.PerAlloc = false },
	}})
}

func TestVolumeMount_Equal(t *testing.T) {
	ci.Parallel(t)

	must.Equal[*VolumeMount](t, nil, nil)
	must.NotEqual[*VolumeMount](t, nil, new(VolumeMount))

	must.StructEqual(t, &VolumeMount{
		Volume:          "volume",
		Destination:     "destination",
		ReadOnly:        true,
		PropagationMode: "mode",
	}, []must.Tweak[*VolumeMount]{{
		Field: "Volume",
		Apply: func(vm *VolumeMount) { vm.Volume = "vol2" },
	}, {
		Field: "Destination",
		Apply: func(vm *VolumeMount) { vm.Destination = "dest2" },
	}, {
		Field: "ReadOnly",
		Apply: func(vm *VolumeMount) { vm.ReadOnly = false },
	}, {
		Field: "PropogationMode",
		Apply: func(vm *VolumeMount) { vm.PropagationMode = "mode2" },
	}})
}
