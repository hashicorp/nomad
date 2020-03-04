package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/stretchr/testify/require"
)

func TestCSIVolumeParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		hcl string
		q   *api.CSIVolume
		err string
	}{{
		hcl: `
id = "foo"
namespace = "n"
access_mode = "single-node-writer"
attachment_mode = "file-system"
plugin_id = "p"
`,
		q: &api.CSIVolume{
			ID:             "foo",
			Namespace:      "n",
			AccessMode:     "single-node-writer",
			AttachmentMode: "file-system",
			PluginID:       "p",
		},
		err: "",
	}, {
		hcl: `
{"id": "foo", "namespace": "n", "access_mode": "single-node-writer", "attachment_mode": "file-system",
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
			q, err := parseCSIVolume(c.hcl)
			require.Equal(t, c.q, q)
			if c.err != "" {
				require.Contains(t, err.Error(), c.err)
			}
		})
	}
}
