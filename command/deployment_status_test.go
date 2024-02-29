// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
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
	must.One(t, code)
	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "12"})
	must.One(t, code)
	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Error retrieving deployment")
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope"})
	must.One(t, code)
	out = ui.ErrorWriter.String()
	// "deployments" indicates that we attempted to list all deployments
	must.StrContains(t, out, "Error retrieving deployments")
	ui.ErrorWriter.Reset()

	// Fails if monitor passed with json or tmpl flags
	for _, flag := range []string{"-json", "-t"} {
		code = cmd.Run([]string{"-monitor", flag, "12"})
		must.One(t, code)
		out = ui.ErrorWriter.String()
		must.StrContains(t, out, "The monitor flag cannot be used with the '-json' or '-t' flags")
		ui.ErrorWriter.Reset()
	}
}

func TestDeploymentStatusCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &DeploymentStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake deployment
	state := srv.Agent.Server().State()
	d := mock.Deployment()
	must.NoError(t, state.UpsertDeployment(1000, d))

	prefix := d.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.SliceLen(t, 1, res)
	must.Eq(t, d.ID, res[0])
}
