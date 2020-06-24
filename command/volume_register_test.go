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

func TestCSIVolumeParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		hcl string
		q   *api.CSIVolume
		err string
	}{{
		hcl: `
id = "foo"
type = "csi"
namespace = "n"
access_mode = "single-node-writer"
attachment_mode = "file-system"
plugin_id = "p"
secrets {
  mysecret = "secretvalue"
}
`,
		q: &api.CSIVolume{
			ID:             "foo",
			Namespace:      "n",
			AccessMode:     "single-node-writer",
			AttachmentMode: "file-system",
			PluginID:       "p",
			Secrets:        api.CSISecrets{"mysecret": "secretvalue"},
		},
		err: "",
	}, {
		hcl: `
{"id": "foo", "namespace": "n", "type": "csi", "access_mode": "single-node-writer", "attachment_mode": "file-system",
"plugin_id": "p"}
`,
		q: &api.CSIVolume{
			ID:             "foo",
			Namespace:      "n",
			AccessMode:     "single-node-writer",
			AttachmentMode: "file-system",
			PluginID:       "p",
		},
		err: "",
	}}

	for _, c := range cases {
		t.Run(c.hcl, func(t *testing.T) {
			ast, err := hcl.ParseString(c.hcl)
			require.NoError(t, err)
			vol, err := csiDecodeVolume(ast)
			require.Equal(t, c.q, vol)
			if c.err == "" {
				require.NoError(t, err)
			} else {
				require.Contains(t, err.Error(), c.err)
			}
		})
	}
}
