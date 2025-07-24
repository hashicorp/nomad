// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestNodeIntroCreateCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodeIntroCreateCommand{}
}

func TestNodeIntroCreateCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait until our test node is ready.
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		must.NoError(t, err)
	})

	ui := cli.NewMockUi()

	cmd := &NodeIntroCreateCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Run the command with some random arguments to ensure we are performing
	// this check.
	t.Run("with command argument", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.One(t, cmd.Run([]string{"pretty-please"}))
		must.StrContains(t, ui.ErrorWriter.String(), "This command takes no arguments")
		ui.ErrorWriter.Reset()
	})

	// Run the command with an invalid TTL, which should return an error as we
	// parse this within the CLI.
	t.Run("incorrect TTL", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.One(t, cmd.Run([]string{"-ttl=1millennium"}))
		must.StrContains(t, ui.ErrorWriter.String(), "Error parsing TTL")
		ui.ErrorWriter.Reset()
	})

	// Run the command with no flags supplied, which should write the JWT to the
	// console.
	t.Run("standard format output", func(t *testing.T) {
		must.Zero(t, cmd.Run([]string{"--address=" + url}))

		t.Cleanup(func() { resetUI(ui) })
		must.StrContains(t, ui.OutputWriter.String(),
			"Successfully generated client introduction token")
		ui.OutputWriter.Reset()
	})

	// Run the command with all claim specific flags supplied, which should
	// write the JWT to the console.
	t.Run("standard format output all flags", func(t *testing.T) {
		must.Zero(t, cmd.Run([]string{
			"--address=" + url,
			"-ttl=1h",
			"-node-name=test-node",
			"-node-pool=test-pool",
		}))

		t.Cleanup(func() { resetUI(ui) })
		must.StrContains(t, ui.OutputWriter.String(),
			"Successfully generated client introduction token")
		ui.OutputWriter.Reset()
	})

	// Run the command with the JSON flag supplied and ensure the output looks
	// like valid JSON.
	t.Run("json format output", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.Zero(t, cmd.Run([]string{"--address=" + url, "-json"}))

		var jsonObj api.ACLIdentityClientIntroductionTokenResponse
		must.NoError(t, json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonObj))
		must.NotEq(t, "", jsonObj.JWT)
	})

	t.Run("template format output", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.Zero(t, cmd.Run([]string{"--address=" + url, "-t={{.JWT}}"}))
		must.NotEq(t, "", ui.OutputWriter.String())
	})
}

func resetUI(ui *cli.MockUi) {
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()
}
