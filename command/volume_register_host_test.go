// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestHostVolumeRegisterCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	t.Cleanup(srv.Shutdown)

	waitForNodes(t, client)

	_, err := client.Namespaces().Register(&api.Namespace{Name: "prod"}, nil)
	must.NoError(t, err)

	nodes, _, err := client.Nodes().List(nil)
	must.NoError(t, err)
	must.Len(t, 1, nodes)
	nodeID := nodes[0].ID

	hostPath := t.TempDir()

	ui := cli.NewMockUi()
	cmd := &VolumeRegisterCommand{Meta: Meta{Ui: ui}}

	hclTestFile := fmt.Sprintf(`
namespace = "prod"
name      = "database"
type      = "host"
plugin_id = "plugin_id"
node_id   = "%s"
node_pool = "default"
host_path = "%s"

capacity     = "15GB"
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
`, nodeID, hostPath)

	file, err := os.CreateTemp(t.TempDir(), "volume-test-*.hcl")
	must.NoError(t, err)
	_, err = file.WriteString(hclTestFile)
	must.NoError(t, err)

	args := []string{"-address", url, file.Name()}

	code := cmd.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Registered host volume")
	parts := strings.Split(out, " ")
	id := strings.TrimSpace(parts[len(parts)-1])

	// Verify volume was registered
	got, _, err := client.HostVolumes().Get(id, &api.QueryOptions{Namespace: "prod"})
	must.NoError(t, err)
	must.NotNil(t, got)

	// Verify we can update the volume without changes
	args = []string{"-address", url, "-id", got.ID, file.Name()}
	code = cmd.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))
	list, _, err := client.HostVolumes().List(nil, &api.QueryOptions{Namespace: "prod"})
	must.Len(t, 1, list, must.Sprintf("new volume should not be registered on update"))
}
