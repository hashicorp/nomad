// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

var _ cli.Command = (*JobStartCommand)(nil)

func TestStartCommand(t *testing.T) {
	ci.Parallel(t)

	srv, _, addr := testServer(t, true, func(c *agent.Config) {
		c.DevMode = true
	})
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobStartCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: addr,
		},
	}

	t.Run("succeeds when starting a stopped job", func(t *testing.T) {
		job := testJob(uuid.Generate())

		client, err := cmd.Meta.Client()
		must.NoError(t, err)

		_, _, err = client.Jobs().Register(job, nil)
		must.NoError(t, err)

		waitForJobAllocsStatus(t, client, *job.ID, api.AllocClientStatusRunning, "")

		_, _, err = client.Jobs().Deregister(*job.ID, false, nil)
		must.Nil(t, err)

		waitForJobAllocsStatus(t, client, *job.ID, api.AllocClientStatusComplete, "")

		res := cmd.Run([]string{"-address", addr, *job.ID})
		must.Zero(t, res)
	})

	t.Run("fails to start a job not previously stopped", func(t *testing.T) {
		job := testJob(uuid.Generate())

		client, err := cmd.Meta.Client()
		must.NoError(t, err)

		_, _, err = client.Jobs().Register(job, nil)
		must.NoError(t, err)

		waitForJobAllocsStatus(t, client, *job.ID, api.AllocClientStatusRunning, "")

		res := cmd.Run([]string{"-address", addr, *job.ID})
		must.Eq(t, 1, res)
	})

	t.Run("fails to start a non-existant job", func(t *testing.T) {
		res := cmd.Run([]string{"-address", addr, "non-existant"})
		must.Eq(t, 1, res)
	})
}

func TestStartCommand_Arguments(t *testing.T) {
	ci.Parallel(t)

	t.Run("fails if client request fails", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobStartCommand{
			Meta: Meta{
				Ui: ui,
			},
		}

		if code := cmd.Run([]string{"-address=nope", "foo"}); code != 1 {
			t.Fatalf("expected exit code 1, got: %d", code)
		}
		if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying job prefix") {
			t.Fatalf("expected failed query error, got: %s", out)
		}
	})
	t.Run("fails if given more than 1 argument", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobStartCommand{
			Meta: Meta{
				Ui: ui,
			},
		}

		if code := cmd.Run([]string{"foo1", "foo2"}); code != 1 {
			t.Fatalf("expected exit code 1, got: %d", code)
		}
		if out := ui.ErrorWriter.String(); !strings.Contains(out, "This command takes one argument: <job>") {
			t.Fatalf("expected failed query error, got: %s", out)
		}
	})
	t.Run("fails if given less than 1 argument", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobStartCommand{
			Meta: Meta{
				Ui: ui,
			},
		}

		if code := cmd.Run([]string{}); code != 1 {
			t.Fatalf("expected exit code 1, got: %d", code)
		}
		if out := ui.ErrorWriter.String(); !strings.Contains(out, "This command takes one argument: <job>") {
			t.Fatalf("expected failed query error, got: %s", out)
		}
	})
}

func TestStartCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobStartCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, j.ID, res[0])
}
