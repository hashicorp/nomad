// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobEvalCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobEvalCommand{}
}

func TestJobEvalCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobEvalCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails when job ID is not specified
	if code := cmd.Run([]string{}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "This command takes one argument") {
		t.Fatalf("unexpected error: %v", out)
	}
	ui.ErrorWriter.Reset()

}

func TestJobEvalCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	ui := cli.NewMockUi()
	cmd := &JobEvalCommand{Meta: Meta{Ui: ui}}
	require := require.New(t)

	state := srv.Agent.Server().State()

	// Create a job
	job := mock.Job()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 11, nil, job)
	require.Nil(err)

	job, err = state.JobByID(nil, structs.DefaultNamespace, job.ID)
	require.Nil(err)

	// Create a failed alloc for the job
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.Namespace = job.Namespace
	alloc.ClientStatus = structs.AllocClientStatusFailed
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 12, []*structs.Allocation{alloc})
	require.Nil(err)

	if code := cmd.Run([]string{"-address=" + url, "-force-reschedule", "-detach", job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	// Lookup alloc again
	alloc, err = state.AllocByID(nil, alloc.ID)
	require.NotNil(alloc)
	require.Nil(err)
	require.True(*alloc.DesiredTransition.ForceReschedule)

}

func TestJobEvalCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobEvalCommand{Meta: Meta{Ui: ui, flagAddress: url}}

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

func TestJobEvalCommand_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start server with ACL enabled.
	srv, _, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Create a job.
	job := mock.MinJob()
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
			name: "missing submit-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["list-jobs"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "submit-job allowed but can't monitor eval without read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name: "submit-job allowed and can monitor eval with read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "submit-job"]
}
`,
		},
		{
			name:      "job prefix requires list-jobs",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job"]
}
`,
			expectedErr: "job not found",
		},
		{
			name:      "job prefix works with list-jobs but can't monitor eval without read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["list-jobs", "submit-job"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name:      "job prefix works with list-jobs and can monitor eval with read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "list-jobs", "submit-job"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobEvalCommand{Meta: Meta{Ui: ui}}
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
