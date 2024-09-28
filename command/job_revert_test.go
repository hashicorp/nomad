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
	structs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestJobRevertCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobDispatchCommand{}
}

func TestJobRevertCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobRevertCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope", "foo", "1"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying job prefix") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestJobRevertCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobRevertCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.SliceLen(t, 1, res)
	must.Eq(t, j.ID, res[0])
}

func TestJobRevertCommand_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start server with ACL enabled.
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

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
	capabilities = ["read-job"]
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
			name:      "job prefix requires list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job"]
}
`,
			expectedErr: "not found",
		},
		{
			name:      "job prefix works with list-job but can't monitor eval without read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job", "list-jobs"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name:      "job prefix works with list-job and can monitor eval with read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "submit-job", "list-jobs"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobRevertCommand{Meta: Meta{Ui: ui}}
			args := []string{
				"-address", url,
			}

			// Create a job.
			job := mock.MinJob()
			state := srv.Agent.Server().State()
			err := state.UpsertJob(structs.MsgTypeTestSetup, uint64(300+i), nil, job)
			must.NoError(t, err)
			defer func() {
				client.Jobs().Deregister(job.ID, true, &api.WriteOptions{
					AuthToken: srv.RootToken.SecretID,
				})
			}()

			// Modify job to create new version.
			newJob := job.Copy()
			newJob.Meta = map[string]string{
				"test": tc.name,
			}
			newJob.Version = uint64(i)
			err = state.UpsertJob(structs.MsgTypeTestSetup, uint64(301+i), nil, newJob)
			must.NoError(t, err)

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

			// Run command reverting job to version 0.
			args = append(args, "0")
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
func TestJobRevertCommand_VersionTag(t *testing.T) {
	ci.Parallel(t)

	// Start test server
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()
	state := srv.Agent.Server().State()

	// Create a job with multiple versions
	v0 := mock.Job()
	v0.ID = "test-job-revert"
	v0.TaskGroups[0].Count = 1
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, v0))

	v1 := v0.Copy()
	v1.TaskGroups[0].Count = 2
	v1.VersionTag = &structs.JobVersionTag{
		Name:        "v1-tag",
		Description: "Version 1 tag",
	}
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, v1))

	v2 := v0.Copy()
	v2.TaskGroups[0].Count = 3
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1002, nil, v2))

	t.Run("Revert to version tag", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobRevertCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-address", url, "-detach", "test-job-revert", "v1-tag"})
		must.Zero(t, code)
	})

	t.Run("Revert to non-existent version tag", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobRevertCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-address", url, "-detach", "test-job-revert", "non-existent-tag"})
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "Error retrieving job versions")
		must.StrContains(t, ui.ErrorWriter.String(), "tag non-existent-tag not found")
	})

	t.Run("Revert to version number", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobRevertCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-address", url, "-detach", "test-job-revert", "0"})
		must.Zero(t, code)
	})

	t.Run("Throws errors with incorrect number of args", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobRevertCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-address", url, "test-job-revert", "v1-tag", "0"})
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "This command takes two arguments")

		code2 := cmd.Run([]string{"-address", url, "test-job-revert"})
		must.One(t, code2)
		must.StrContains(t, ui.ErrorWriter.String(), "This command takes two arguments")
	})

	t.Run("Revert to tagged version doesn't duplicate tag", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobRevertCommand{Meta: Meta{Ui: ui}}

		// First, revert to the tagged version
		code := cmd.Run([]string{"-address", url, "-detach", "test-job-revert", "v1-tag"})
		must.Zero(t, code)

		// Now, fetch the job versions
		historyCmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}
		historyCode := historyCmd.Run([]string{"-address", url, "-version=4", v0.ID})
		must.Zero(t, historyCode)

		// Check the output for the expected version and no tag
		output := ui.OutputWriter.String()
		must.StrContains(t, output, "Version     = 4")
		must.StrNotContains(t, output, "Tag Name")
		must.StrNotContains(t, output, "Tag Description")

		ui.OutputWriter.Reset()

		// Make sure the old version of the tag is still tagged
		historyCmd = &JobHistoryCommand{Meta: Meta{Ui: ui}}
		historyCode = historyCmd.Run([]string{"-address", url, "-version=1", v0.ID})
		must.Zero(t, historyCode)
		output = ui.OutputWriter.String()
		must.StrContains(t, output, "Version         = 1")
		must.StrContains(t, output, "Tag Name        = v1-tag")
		must.StrContains(t, output, "Tag Description = Version 1 tag")
	})
}
