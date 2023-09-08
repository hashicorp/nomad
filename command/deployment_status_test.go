// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &DeploymentStatusCommand{}
}

func TestDeploymentStatusCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &DeploymentStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)
	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "12"})
	require.Equal(t, 1, code)
	out = ui.ErrorWriter.String()
	require.Contains(t, out, "Error retrieving deployment")
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope"})
	require.Equal(t, 1, code)
	out = ui.ErrorWriter.String()
	// "deployments" indicates that we attempted to list all deployments
	require.Contains(t, out, "Error retrieving deployments")
	ui.ErrorWriter.Reset()

	// Fails if monitor passed with json or tmpl flags
	for _, flag := range []string{"-json", "-t"} {
		code = cmd.Run([]string{"-monitor", flag, "12"})
		require.Equal(t, 1, code)
		out = ui.ErrorWriter.String()
		require.Contains(t, out, "The monitor flag cannot be used with the '-json' or '-t' flags")
		ui.ErrorWriter.Reset()
	}
}

func TestDeploymentStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &DeploymentStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake deployment
	state := srv.Agent.Server().State()
	d := mock.Deployment()
	assert.Nil(state.UpsertDeployment(1000, d))

	prefix := d.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(d.ID, res[0])
}
