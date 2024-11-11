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
	"github.com/shoenig/test/must"
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
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))
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

	must.NoError(t, state.UpsertCSIVolume(1000, []*structs.CSIVolume{vol}))

	prefix := vol.ID[:len(vol.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, vol.ID, res[0])
}
