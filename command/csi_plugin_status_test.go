package command

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestCSIPluginStatusCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &CSIPluginStatusCommand{}
}

func TestCSIPluginStatusCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &CSIPluginStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)

	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()
}

func TestCSIPluginStatusCommand_AutocompleteArgs(t *testing.T) {
	/*
		t.Parallel()

		srv, _, url := testServer(t, true, nil)
		defer srv.Shutdown()

		ui := new(cli.MockUi)
		cmd := &CSIPluginStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}
		state := srv.Agent.Server().State()
		plug, err := nomad.CreateTestPlugin(state, "glade")
		require.NoError(err)

		prefix := plug.ID[:len(plug.ID)-5]
		args := complete.Args{Last: prefix}
		predictor := cmd.AutocompleteArgs()

		res := predictor.Predict(args)
		require.Equal(t, 0, len(res))
		// require.Equal(t, plug.ID, res[0])
	*/
}
