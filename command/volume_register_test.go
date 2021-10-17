package command

import (
	"testing"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/require"
)

func TestVolumeDispatchParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		hcl string
		t   string
		err string
	}{{
		hcl: `
type = "foo"
rando = "bar"
`,
		t:   "foo",
		err: "",
	}, {
		hcl: `{"id": "foo", "type": "foo", "other": "bar"}`,
		t:   "foo",
		err: "",
	}}

	for _, c := range cases {
		t.Run(c.hcl, func(t *testing.T) {
			_, s, err := parseVolumeType(c.hcl)
			require.Equal(t, c.t, s)
			if c.err == "" {
				require.NoError(t, err)
			} else {
				require.Contains(t, err.Error(), c.err)
			}

		})
	}
}

func TestCSIVolumeDecode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		hcl      string
		expected *api.CSIVolume
		err      string
	}{{
		name: "volume creation",
		hcl: `
id              = "testvolume"
name            = "test"
type            = "csi"
plugin_id       = "myplugin"

capacity_min = "10 MiB"
capacity_max = "1G"
snapshot_id  = "snap-12345"

mount_options {
  fs_type     = "ext4"
  mount_flags = ["ro"]
}

secrets {
  password = "xyzzy"
}

parameters {
  skuname = "Premium_LRS"
}

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "block-device"
}
`,
		expected: &api.CSIVolume{
			ID:                   "testvolume",
			Name:                 "test",
			PluginID:             "myplugin",
			SnapshotID:           "snap-12345",
			RequestedCapacityMin: 10485760,
			RequestedCapacityMax: 1000000000,
			RequestedCapabilities: []*api.CSIVolumeCapability{
				{
					AccessMode:     api.CSIVolumeAccessModeSingleNodeWriter,
					AttachmentMode: api.CSIVolumeAttachmentModeFilesystem,
				},
				{
					AccessMode:     api.CSIVolumeAccessModeSingleNodeReader,
					AttachmentMode: api.CSIVolumeAttachmentModeBlockDevice,
				},
			},
			MountOptions: &api.CSIMountOptions{
				FSType:     "ext4",
				MountFlags: []string{"ro"},
			},
			Parameters: map[string]string{"skuname": "Premium_LRS"},
			Secrets:    map[string]string{"password": "xyzzy"},
		},
		err: "",
	}, {
		name: "volume registration",
		hcl: `
id              = "testvolume"
external_id     = "vol-12345"
name            = "test"
type            = "csi"
plugin_id       = "myplugin"
capacity_min    = "" # meaningless for registration

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}
`,
		expected: &api.CSIVolume{
			ID:         "testvolume",
			ExternalID: "vol-12345",
			Name:       "test",
			PluginID:   "myplugin",
			RequestedCapabilities: []*api.CSIVolumeCapability{
				{
					AccessMode:     api.CSIVolumeAccessModeSingleNodeWriter,
					AttachmentMode: api.CSIVolumeAttachmentModeFilesystem,
				},
			},
		},
		err: "",
	},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ast, err := hcl.ParseString(c.hcl)
			require.NoError(t, err)
			vol, err := csiDecodeVolume(ast)
			if c.err == "" {
				require.NoError(t, err)
			} else {
				require.Contains(t, err.Error(), c.err)
			}
			require.Equal(t, c.expected, vol)

		})

	}
}
