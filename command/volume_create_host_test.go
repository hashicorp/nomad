// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/shoenig/test/must"
)

func TestHostVolumeCreateCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.Client.Meta = map[string]string{"rack": "foo"}
	})
	t.Cleanup(srv.Shutdown)

	waitForNodes(t, client)

	_, err := client.Namespaces().Register(&api.Namespace{Name: "prod"}, nil)
	must.NoError(t, err)

	ui := cli.NewMockUi()
	cmd := &VolumeCreateCommand{Meta: Meta{Ui: ui}}

	hclTestFile := `
namespace = "prod"
name      = "database"
type      = "host"
plugin_id = "mkdir"
node_pool = "default"

capacity_min = "10GiB"
capacity_max = "20G"

constraint {
  attribute = "${meta.rack}"
  value     = "foo"
}

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "block-device"
}

parameters {
  foo = "bar"
}
`

	file, err := os.CreateTemp(t.TempDir(), "volume-test-*.hcl")
	must.NoError(t, err)
	_, err = file.WriteString(hclTestFile)
	must.NoError(t, err)

	args := []string{"-address", url, "-detach", file.Name()}

	code := cmd.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Created host volume")
	parts := strings.Split(out, " ")
	id := strings.TrimSpace(parts[len(parts)-1])

	// Verify volume was created
	got, _, err := client.HostVolumes().Get(id, &api.QueryOptions{Namespace: "prod"})
	must.NoError(t, err)
	must.NotNil(t, got)

	// Verify we can update the volume without changes
	args = []string{"-address", url, "-detach", "-id", got.ID, file.Name()}
	code = cmd.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))
	list, _, err := client.HostVolumes().List(nil, &api.QueryOptions{Namespace: "prod"})
	must.Len(t, 1, list, must.Sprintf("new volume should not be created on update"))
}

func TestHostVolume_HCLDecode(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		hcl      string
		expected *api.HostVolume
		errMsg   string
	}{
		{
			name: "full spec",
			hcl: `
namespace = "prod"
name      = "database"
type      = "host"
plugin_id = "mkdir"
node_pool = "default"

capacity_min = "10GiB"
capacity_max = "20G"

constraint {
  attribute = "${attr.kernel.name}"
  value     = "linux"
}

constraint {
  attribute = "${meta.rack}"
  value     = "foo"
}

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "block-device"
}

parameters {
  foo = "bar"
}
`,
			expected: &api.HostVolume{
				Namespace: "prod",
				Name:      "database",
				PluginID:  "mkdir",
				NodePool:  "default",
				Constraints: []*api.Constraint{{
					LTarget: "${attr.kernel.name}",
					RTarget: "linux",
					Operand: "=",
				}, {
					LTarget: "${meta.rack}",
					RTarget: "foo",
					Operand: "=",
				}},
				RequestedCapacityMinBytes: 10737418240,
				RequestedCapacityMaxBytes: 20000000000,
				RequestedCapabilities: []*api.HostVolumeCapability{
					{
						AttachmentMode: api.HostVolumeAttachmentModeFilesystem,
						AccessMode:     api.HostVolumeAccessModeSingleNodeWriter,
					},
					{
						AttachmentMode: api.HostVolumeAttachmentModeBlockDevice,
						AccessMode:     api.HostVolumeAccessModeSingleNodeReader,
					},
				},
				Parameters: map[string]string{"foo": "bar"},
			},
		},

		{
			name: "mostly empty spec",
			hcl: `
namespace = "prod"
name      = "database"
type      = "host"
plugin_id = "mkdir"
node_pool = "default"
`,
			expected: &api.HostVolume{
				Namespace: "prod",
				Name:      "database",
				PluginID:  "mkdir",
				NodePool:  "default",
			},
		},

		{
			name: "invalid capacity",
			hcl: `
namespace = "prod"
name      = "database"
type      = "host"
plugin_id = "mkdir"
node_pool = "default"

capacity_min = "a"
`,
			expected: nil,
			errMsg:   "invalid capacity_min: could not parse value as bytes: strconv.ParseFloat: parsing \"\": invalid syntax",
		},

		{
			name: "invalid constraint",
			hcl: `
namespace = "prod"
name      = "database"
type      = "host"
plugin_id = "mkdir"
node_pool = "default"

constraint {
  distinct_hosts = "foo"
}

`,
			expected: nil,
			errMsg:   "invalid constraint: distinct_hosts should be set to true or false; strconv.ParseBool: parsing \"foo\": invalid syntax",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ast, err := hcl.ParseString(tc.hcl)
			must.NoError(t, err)
			vol, err := decodeHostVolume(ast)
			if tc.errMsg == "" {
				must.NoError(t, err)
			} else {
				must.EqError(t, err, tc.errMsg)
			}
			must.Eq(t, tc.expected, vol)
		})
	}

}
