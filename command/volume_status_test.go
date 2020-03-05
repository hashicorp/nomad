package command

import (
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestCSIVolumeStatusCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &VolumeStatusCommand{}
}

func TestCSIVolumeStatusCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &VolumeStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)

	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()
}

func TestCSIVolumeStatusCommand_AutocompleteArgs(t *testing.T) {
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &VolumeStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	state := srv.Agent.Server().State()

	vol := &structs.CSIVolume{
		ID:        uuid.Generate(),
		Namespace: "default",
		PluginID:  "glade",
	}

	require.NoError(t, state.CSIVolumeRegister(1000, []*structs.CSIVolume{vol}))

	prefix := vol.ID[:len(vol.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	require.Equal(t, 1, len(res))
	require.Equal(t, vol.ID, res[0])
}
