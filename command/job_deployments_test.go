// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

func TestJobDeploymentsCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobDeploymentsCommand{}
}

func TestJobDeploymentsCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
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

func TestJobDeploymentsCommand_Run(t *testing.T) {
	ci.Parallel(t)

	assert := assert.New(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui}}

	// Should return an error message for no job match
	if code := cmd.Run([]string{"-address=" + url, "foo"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}

	// Create a job without a deployment
	job := mock.Job()
	state := srv.Agent.Server().State()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))

	// Should display no match if the job doesn't have deployments
	if code := cmd.Run([]string{"-address=" + url, job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "No deployments found") {
		t.Fatalf("expected no deployments output, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Inject a deployment
	d := mock.Deployment()
	d.JobID = job.ID
	d.JobCreateIndex = job.CreateIndex
	assert.Nil(state.UpsertDeployment(200, d))

	// Should now display the deployment
	if code := cmd.Run([]string{"-address=" + url, "-verbose", job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, d.ID) {
		t.Fatalf("expected deployment output, got: %s", out)
	}
	ui.OutputWriter.Reset()
}

func TestJobDeploymentsCommand_Run_Latest(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui}}

	// Should return an error message for no job match
	if code := cmd.Run([]string{"-address=" + url, "-latest", "foo"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}

	// Create a job without a deployment
	job := mock.Job()
	state := srv.Agent.Server().State()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))

	// Should display no match if the job doesn't have deployments
	if code := cmd.Run([]string{"-address=" + url, "-latest", job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "No deployment found") {
		t.Fatalf("expected no deployments output, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Inject a deployment
	d := mock.Deployment()
	d.JobID = job.ID
	d.JobCreateIndex = job.CreateIndex
	assert.Nil(state.UpsertDeployment(200, d))

	// Should now display the deployment
	if code := cmd.Run([]string{"-address=" + url, "-verbose", "-latest", job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, d.ID) {
		t.Fatalf("expected deployment output, got: %s", out)
	}
	ui.OutputWriter.Reset()
}

func TestJobDeploymentsCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(j.ID, res[0])
}

func TestJobDeploymentsCommand_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start server with ACL enabled.
	srv, _, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Create a job with a deployment.
	job := mock.Job()
	state := srv.Agent.Server().State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	must.NoError(t, err)

	d := mock.Deployment()
	d.JobID = job.ID
	d.JobCreateIndex = job.CreateIndex
	err = state.UpsertDeployment(101, d)
	must.NoError(t, err)

	testCases := []struct {
		name        string
		jobPrefix   bool
		aclPolicy   string
		expectedErr string
		expectedOut string
	}{
		{
			name:        "no token",
			aclPolicy:   "",
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "missing read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["alloc-lifecycle"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "read-job allowed",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job"]
}
`,
		},
		{
			name:      "job prefix requires list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job"]
}
`,
			expectedOut: "No deployments",
		},
		{
			name:      "job prefix works with list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "list-jobs"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui}}
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
			if tc.expectedOut != "" {
				must.StrContains(t, ui.OutputWriter.String(), tc.expectedOut)
			}
		})
	}
}
