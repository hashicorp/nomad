// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestCSIVolumeStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VolumeStatusCommand{}
}

func TestCSIVolumeStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &VolumeStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)

	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()
}

func TestCSIVolumeStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VolumeStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	state := srv.Agent.Server().State()

	vol := &structs.CSIVolume{
		ID:        uuid.Generate(),
		Namespace: "default",
		PluginID:  "glade",
	}

	require.NoError(t, state.UpsertCSIVolume(1000, []*structs.CSIVolume{vol}))

	prefix := vol.ID[:len(vol.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	require.Equal(t, 1, len(res))
	require.Equal(t, vol.ID, res[0])
}
