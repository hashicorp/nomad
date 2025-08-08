// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestVolumeCreateCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	t.Cleanup(srv.Shutdown)
	waitForNodes(t, client)

	_, err := client.Namespaces().Register(&api.Namespace{Name: "prod"}, nil)
	must.NoError(t, err)

	ui := cli.NewMockUi()
	cmd := &VolumeCreateCommand{
		Meta: Meta{Ui: ui},
	}

	volumeHCL := `
type = "csi"
id = "test-volume"
name = "test-volume"
external_id = "external-test-volume"
plugin_id = "test-plugin"
capacity_min = "1GiB"
capacity_max = "10GiB"

capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "block-device"
}
`

	file, err := os.CreateTemp(t.TempDir(), "csi-volume-test-*.hcl")
	must.NoError(t, err)
	_, err = file.WriteString(volumeHCL)
	must.NoError(t, err)

	// Since we can't easily mock the API client to fake a CSI plugin running,
	// we'll expect this to fail with a plugin-related error. The flow and
	// parsing can still be tested.
	args := []string{"-address", url, file.Name()}
	code := cmd.Run(args)
	must.Eq(t, 1, code)

	// Verify error output contains expected message about volume creation
	output := ui.ErrorWriter.String()
	must.StrContains(t, output, "Error creating volume")
	must.StrContains(t, output, "no CSI plugin named: test-plugin could be found")
}
