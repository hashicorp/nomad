// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

var _ cli.Command = (*JobStartCommand)(nil)

// testStartCommandSetup creates a test server and command for job start tests
func testStartCommandSetup(t *testing.T) (*agent.TestAgent, *JobStartCommand, *api.Client, string) {
	srv, _, addr := testServer(t, true, func(c *agent.Config) {
		c.DevMode = true
	})

	ui := cli.NewMockUi()
	cmd := &JobStartCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: addr,
		},
	}

	client, err := cmd.Meta.Client()
	must.NoError(t, err)

	return srv, cmd, client, addr
}

// registerAndStopJob creates, registers, and stops a job for testing
func registerAndStopJob(t *testing.T, client *api.Client, job *api.Job, withSubmission bool) {
	if withSubmission {
		jsonBytes, err := json.Marshal(job)
		must.NoError(t, err)

		_, _, err = client.Jobs().RegisterOpts(job, &api.RegisterOptions{
			Submission: &api.JobSubmission{
				Source: string(jsonBytes),
				Format: "json",
			},
		}, nil)
		must.NoError(t, err)
	} else {
		_, _, err := client.Jobs().RegisterOpts(job, &api.RegisterOptions{}, nil)
		must.NoError(t, err)
	}

	waitForJobAllocsStatus(t, client, *job.ID, api.AllocClientStatusRunning, "")

	_, _, err := client.Jobs().Deregister(*job.ID, false, nil)
	must.Nil(t, err)

	waitForJobAllocsStatus(t, client, *job.ID, api.AllocClientStatusComplete, "")
}

// verifyScalingPolicies checks the scaling policies after job start
func verifyScalingPolicies(t *testing.T, client *api.Client, expectedCount int, expectedEnabled *bool) {
	pol, _, err := client.Scaling().ListPolicies(nil)
	must.NoError(t, err)

	if expectedCount == 0 {
		must.Zero(t, len(pol))
	} else {
		must.Eq(t, expectedCount, len(pol))
		if expectedEnabled != nil {
			must.Eq(t, *expectedEnabled, pol[0].Enabled)
		}
	}
}

func TestStartCommand(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name             string
		setupJob         func(*api.Job)
		withSubmission   bool
		expectedPolicies int
		expectedEnabled  *bool
		shouldSucceed    bool
	}{
		{
			name:             "succeeds when starting a stopped job",
			setupJob:         func(job *api.Job) {}, // default scaling enabled
			withSubmission:   true,
			expectedPolicies: 1,
			expectedEnabled:  pointer.Of(true),
			shouldSucceed:    true,
		},
		{
			name: "succeeds when starting a stopped job with disabled scaling policies and no submissions",
			setupJob: func(job *api.Job) {
				job.TaskGroups[0].Scaling.Enabled = pointer.Of(false)
			},
			withSubmission:   false,
			expectedPolicies: 1,
			expectedEnabled:  pointer.Of(false),
			shouldSucceed:    true,
		},
		{
			name: "succeeds when starting a stopped job with enabled scaling policies",
			setupJob: func(job *api.Job) {
				job.TaskGroups[0].Scaling.Enabled = pointer.Of(true)
			},
			withSubmission:   true,
			expectedPolicies: 1,
			expectedEnabled:  pointer.Of(true),
			shouldSucceed:    true,
		},
		{
			name: "succeeds when starting a stopped job with no scaling policies",
			setupJob: func(job *api.Job) {
				job.TaskGroups[0].Scaling = nil
			},
			withSubmission:   true,
			expectedPolicies: 0,
			expectedEnabled:  nil,
			shouldSucceed:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv, cmd, client, addr := testStartCommandSetup(t)
			defer srv.Shutdown()

			job := testJob(uuid.Generate())
			tc.setupJob(job)

			registerAndStopJob(t, client, job, tc.withSubmission)

			res := cmd.Run([]string{"-address", addr, *job.ID})
			if tc.shouldSucceed {
				must.Zero(t, res)
				verifyScalingPolicies(t, client, tc.expectedPolicies, tc.expectedEnabled)
			} else {
				must.Eq(t, 1, res)
			}
		})
	}

	t.Run("fails to start a job not previously stopped", func(t *testing.T) {
		srv, cmd, client, addr := testStartCommandSetup(t)
		defer srv.Shutdown()

		job := testJob(uuid.Generate())

		_, _, err := client.Jobs().Register(job, nil)
		must.NoError(t, err)

		waitForJobAllocsStatus(t, client, *job.ID, api.AllocClientStatusRunning, "")

		res := cmd.Run([]string{"-address", addr, *job.ID})
		must.Eq(t, 1, res)
	})

	t.Run("fails to start a non-existant job", func(t *testing.T) {
		srv, cmd, _, addr := testStartCommandSetup(t)
		defer srv.Shutdown()

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

		code := cmd.Run([]string{"-address=nope", "foo"})
		must.Eq(t, code, 1)

		out := ui.ErrorWriter.String()
		must.StrContains(t, out, "Error querying job prefix")
	})
	t.Run("fails if given more than 1 argument", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobStartCommand{
			Meta: Meta{
				Ui: ui,
			},
		}

		code := cmd.Run([]string{"foo1", "foo2"})
		must.Eq(t, code, 1)

		out := ui.ErrorWriter.String()
		must.StrContains(t, out, "This command takes one argument: <job>")
	})
	t.Run("fails if given less than 1 argument", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobStartCommand{
			Meta: Meta{
				Ui: ui,
			},
		}

		code := cmd.Run([]string{})
		must.Eq(t, code, 1)

		out := ui.ErrorWriter.String()
		must.StrContains(t, out, "This command takes one argument: <job>")
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
