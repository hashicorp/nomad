package command

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestPluginStatusCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &PluginStatusCommand{}
}

func TestPluginStatusCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &PluginStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)

	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()
}

func TestPluginStatusCommand_AutocompleteArgs(t *testing.T) {
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
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
