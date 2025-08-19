// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestNodeIdentityGetCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodeIntroCreateCommand{}
}

func TestNodeIdentityGetCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait until our test node is ready.
	testutil.WaitForClient(
		t,
		srv.Agent.Client().RPC,
		srv.Agent.Client().NodeID(),
		srv.Agent.Client().Region(),
	)

	ui := cli.NewMockUi()

	cmd := &NodeIdentityGetCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	t.Run("with no command argument", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.One(t, cmd.Run([]string{}))
		must.StrContains(t, ui.ErrorWriter.String(), "This command takes one argument")
	})

	t.Run("node not found", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.One(t, cmd.Run([]string{"--address=" + url, "f4b2f0a1-7898-ad4e-de19-d9fc9a773961"}))
		must.StrContains(t, ui.ErrorWriter.String(), "No node(s) with prefix or id")
	})

	t.Run("standard output", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.Zero(t, cmd.Run([]string{"--address=" + url, srv.Agent.Client().NodeID()}))
		must.StrContains(t, ui.OutputWriter.String(), "Claim Key")
		must.StrContains(t, ui.OutputWriter.String(), "Claim Value")
	})

	t.Run("json output", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.Zero(t, cmd.Run([]string{"--address=" + url, "-json", srv.Agent.Client().NodeID()}))

		var resp map[string]any
		must.NoError(t, json.Unmarshal(ui.OutputWriter.Bytes(), &resp))
		must.MapContainsKey(t, resp, "nomad_node_id")
	})

	t.Run("template output", func(t *testing.T) {
		t.Cleanup(func() { resetUI(ui) })

		must.Zero(t, cmd.Run([]string{"--address=" + url, "-t", "{{ .nomad_node_id }}", srv.Agent.Client().NodeID()}))
		must.StrContains(t, ui.OutputWriter.String(), srv.Agent.Client().NodeID())
	})
}
