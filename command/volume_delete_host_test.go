// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestHostVolumeDeleteCommand(t *testing.T) {
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

	ui := cli.NewMockUi()

	hclTestFile := fmt.Sprintf(`
namespace = "prod"
name      = "example"
type      = "host"
plugin_id = "plugin_id"
node_id   = "%s"
node_pool = "default"
`, nodeID)

	file, err := os.CreateTemp(t.TempDir(), "volume-test-*.hcl")
	must.NoError(t, err)
	_, err = file.WriteString(hclTestFile)
	must.NoError(t, err)

	args := []string{"-address", url, file.Name()}
	regCmd := &VolumeRegisterCommand{Meta: Meta{Ui: ui}}
	code := regCmd.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Registered host volume")
	parts := strings.Split(out, " ")
	id := strings.TrimSpace(parts[len(parts)-1])

	ui.OutputWriter.Reset()

	// missing the namespace
	cmd := &VolumeDeleteCommand{Meta: Meta{Ui: ui}}
	args = []string{"-address", url, "-type", "host", id}
	code = cmd.Run(args)
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "no such volume")
	ui.ErrorWriter.Reset()

	// fix the namespace
	args = []string{"-address", url, "-type", "host", "-namespace", "prod", id}
	code = cmd.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))
	out = ui.OutputWriter.String()
	must.StrContains(t, out, fmt.Sprintf("Successfully deleted volume %q!", id))
}
