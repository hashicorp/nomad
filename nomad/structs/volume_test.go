package structs

import (
	"strings"
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
				"volume has an empty source",
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
				"volume has an empty source",
				"CSI volumes must have an attachment mode",
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
				"volume has an empty source",
				"CSI volumes must have an attachment mode",
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
				"volume has an empty source",
				"CSI volumes must have an attachment mode",
				"single-node-reader-only volumes must be read-only",
				"volume with single-node-reader-only access mode does not support canaries",
				"volume cannot be per_alloc when canaries are in use"},
			canariesCount: 1,
			req: &VolumeRequest{
				Type:       VolumeTypeCSI,
				AccessMode: CSIVolumeAccessModeSingleNodeReader,
				PerAlloc:   true,
			},
		},
		{
			name: "CSI writer volume per-alloc with canaries",
			expected: []string{
				"volume has an empty source",
				"CSI volumes must have an attachment mode",
				"volume with single-node-writer access mode does not support canaries",
				"volume cannot be per_alloc when canaries are in use"},
			canariesCount: 1,
			req: &VolumeRequest{
				Type:       VolumeTypeCSI,
				AccessMode: CSIVolumeAccessModeSingleNodeWriter,
				PerAlloc:   true,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate(tc.taskGroupCount, tc.canariesCount)
			errs := strings.Split(strings.TrimSpace(err.Error()), "\n\t* ")
			errs = errs[1:]
			require.ElementsMatch(t, errs, tc.expected)
		})
	}
}
