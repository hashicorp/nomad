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
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestHostVolumeStatusCommand_Args(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &VolumeStatusCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{
		"-type", "host",
		"-node", "6063016a-9d4c-11ef-85fc-9be98efe7e76",
		"-node-pool", "prod",
		"6e3e80f2-9d4c-11ef-97b1-d38cf64416a4",
	})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, "-node or -node-pool options can only be used when no ID is provided")
}

func TestHostVolumeStatusCommand_List(t *testing.T) {
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

	vols := []api.NamespacedID{
		{Namespace: "prod", ID: "database"},
		{Namespace: "prod", ID: "certs"},
		{Namespace: "default", ID: "example"},
	}

	for _, vol := range vols {
		hclTestFile := fmt.Sprintf(`
namespace = "%s"
name      = "%s"
type      = "host"
plugin_id = "mkdir"
node_id   = "%s"
node_pool = "default"
capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "file-system"
}
`, vol.Namespace, vol.ID, nodeID)

		file, err := os.CreateTemp(t.TempDir(), "volume-test-*.hcl")
		must.NoError(t, err)
		_, err = file.WriteString(hclTestFile)
		must.NoError(t, err)

		args := []string{"-address", url, "-detach", file.Name()}
		cmd := &VolumeCreateCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run(args)
		must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))

		out := ui.OutputWriter.String()
		must.StrContains(t, out, "Created host volume")
		ui.OutputWriter.Reset()
	}

	cmd := &VolumeStatusCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address", url, "-type", "host", "-namespace", "prod"}
	code := cmd.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "certs")
	must.StrContains(t, out, "database")
	must.StrNotContains(t, out, "example")
}

func TestHostVolumeStatusCommand_Get(t *testing.T) {
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

	hclTestFile := fmt.Sprintf(`
namespace = "prod"
name      = "example"
type      = "host"
plugin_id = "mkdir"
node_id   = "%s"
node_pool = "default"
host_path = "%s"
capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "file-system"
}
`, nodeID, hostPath)

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

	// autocomplete
	cmd := &VolumeStatusCommand{Meta: Meta{Ui: ui, namespace: "*", flagAddress: url}}
	cmd.Meta.namespace = "*"
	prefix := id[:len(id)-5]
	cargs := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(cargs)
	must.SliceLen(t, 1, res)
	must.Eq(t, id, res[0])

	// missing the namespace
	cmd = &VolumeStatusCommand{Meta: Meta{Ui: ui}}
	args = []string{"-address", url, "-type", "host", id}
	code = cmd.Run(args)
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(),
		"Error listing host volumes: no volumes with prefix or ID")
	ui.ErrorWriter.Reset()

	args = []string{"-address", url, "-type", "host", "-namespace", "prod", id}
	code = cmd.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error: %s", ui.ErrorWriter.String()))
	out = ui.OutputWriter.String()
	must.StrContains(t, out, "example")
}
