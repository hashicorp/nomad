package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
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
				"host volumes do not support per_alloc",
			},
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
		tc = tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate(JobTypeSystem, tc.taskGroupCount, tc.canariesCount)
			for _, expected := range tc.expected {
				require.Contains(t, err.Error(), expected)
			}
		})
	}

}
