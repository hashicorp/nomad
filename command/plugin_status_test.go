// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestPluginStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &PluginStatusCommand{}
}

func TestPluginStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &PluginStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)

	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Test an unsupported plugin type.
	code = cmd.Run([]string{"-type=not-a-plugin"})
	require.Equal(t, 1, code)

	out = ui.ErrorWriter.String()
	require.Contains(t, out, "Unsupported plugin type: not-a-plugin")
	ui.ErrorWriter.Reset()
}

func TestPluginStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &PluginStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a plugin
	id := "long-plugin-id"
	s := srv.Agent.Server().State()
	cleanup := state.CreateTestCSIPlugin(s, id)
	defer cleanup()
	ws := memdb.NewWatchSet()
	plug, err := s.CSIPluginByID(ws, id)
	require.NoError(t, err)

	prefix := plug.ID[:len(plug.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	require.Equal(t, 1, len(res))
	require.Equal(t, plug.ID, res[0])
}
