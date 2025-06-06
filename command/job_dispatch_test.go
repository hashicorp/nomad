// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestJobDispatchCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobDispatchCommand{}
}

func TestJobDispatchCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobDispatchCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails when specified file does not exist
	if code := cmd.Run([]string{"foo", "/unicorns/leprechauns"}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error reading input data") {
		t.Fatalf("expect error reading input data: %v", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope", "foo"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying job prefix") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestJobDispatchCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobDispatchCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	// No parameterized jobs, should be 0 results
	res := predictor.Predict(args)
	must.SliceEmpty(t, res)

	// Create a fake parameterized job
	j1 := mock.Job()
	j1.ParameterizedJob = &structs.ParameterizedJobConfig{}
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 2000, nil, j1))

	prefix = j1.ID[:len(j1.ID)-5]
	args = complete.Args{Last: prefix}
	predictor = cmd.AutocompleteArgs()

	// Should return 1 parameterized job
	res = predictor.Predict(args)
	must.SliceLen(t, 1, res)
	must.Eq(t, j1.ID, res[0])
}

func TestJobDispatchCommand_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start server with ACL enabled.
	srv, _, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Create a parameterized job.
	job := mock.MinJob()
	job.Type = "batch"
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	job.Priority = 20 //set priority on parent job
	state := srv.Agent.Server().State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	must.NoError(t, err)

	testCases := []struct {
		name        string
		jobPrefix   bool
		aclPolicy   string
		expectedErr string
	}{
		{
			name:        "no token",
			aclPolicy:   "",
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "missing dispatch-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "dispatch-job allowed but can't monitor eval without read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["dispatch-job"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name: "dispatch-job allowed and can monitor eval with read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["dispatch-job", "read-job"]
}
`,
		},
		{
			name:      "job prefix requires list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["dispatch-job"]
}
`,
			expectedErr: "job not found",
		},
		{
			name:      "job prefix works with list-job but can't monitor eval without read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["dispatch-job", "list-jobs"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name:      "job prefix works with list-job and can monitor eval with read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "dispatch-job", "list-jobs"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobDispatchCommand{Meta: Meta{Ui: ui}}
			args := []string{
				"-address", url,
			}

			if tc.aclPolicy != "" {
				// Create ACL token with test case policy and add it to the
				// command.
				policyName := nonAlphaNum.ReplaceAllString(tc.name, "-")
				token := mock.CreatePolicyAndToken(t, state, uint64(302+i), policyName, tc.aclPolicy)
				args = append(args, "-token", token.SecretID)
			}

			// Add job ID or job ID prefix to the command.
			if tc.jobPrefix {
				args = append(args, job.ID[:3])
			} else {
				args = append(args, job.ID)
			}

			// Run command.
			code := cmd.Run(args)
			if tc.expectedErr == "" {
				must.Zero(t, code)
			} else {
				must.One(t, code)
				must.StrContains(t, ui.ErrorWriter.String(), tc.expectedErr)
			}
		})
	}
}

func TestJobDispatchCommand_Priority(t *testing.T) {
	ci.Parallel(t)
	defaultJobPriority := 50
	// Start server
	srv, client, url := testServer(t, true, nil)
	t.Cleanup(srv.Shutdown)

	waitForNodes(t, client)

	// Create a parameterized job.
	job := mock.MinJob()
	job.Type = "batch"
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	job.Priority = defaultJobPriority // set default priority on parent job
	state := srv.Agent.Server().State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	must.NoError(t, err)

	testCases := []struct {
		name            string
		priority        string
		expectedErr     bool
		additionalFlags []string
		payload         map[string]string
	}{
		{
			name: "no priority",
		},
		{
			name:     "valid priority",
			priority: "80",
		},
		{
			name:        "invalid priority",
			priority:    "-1",
			expectedErr: true,
		},
		{
			name:            "priority + flag",
			priority:        "90",
			additionalFlags: []string{"-verbose"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobDispatchCommand{Meta: Meta{Ui: ui}}
			args := []string{
				"-address", url,
			}
			// Add priority, if present
			if len(tc.priority) >= 1 {
				args = append(args, []string{"-priority", tc.priority}...)
			}

			// Add additional flags, if present
			if len(tc.additionalFlags) >= 1 {
				args = append(args, tc.additionalFlags...)
			}

			// Add job ID to the command.
			args = append(args, job.ID)

			// Run command.
			code := cmd.Run(args)
			if !tc.expectedErr {
				must.Zero(t, code)
			} else {
				// Confirm expected error case
				must.NonZero(t, code)
				out := ui.ErrorWriter.String()
				must.StrContains(t, out, "dispatch job priority must be between [1, 100]")
				return
			}

			// Confirm successful dispatch and parse job ID
			out := ui.OutputWriter.String()
			must.StrContains(t, out, "Dispatched Job ID =")
			parts := strings.Fields(out)
			id := strings.TrimSpace(parts[4])

			// Confirm dispatched job priority set correctly
			job, _, err := client.Jobs().Info(id, nil)
			must.NoError(t, err)
			must.NotNil(t, job)

			if len(tc.priority) >= 1 {
				priority, err := strconv.Atoi(tc.priority)
				must.NoError(t, err)
				must.Eq(t, job.Priority, &priority)
			} else {
				must.Eq(t, defaultJobPriority, *job.Priority)
			}
		})
	}
}
