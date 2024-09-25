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
)

func TestJobHistoryCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobDispatchCommand{}
}

func TestJobHistoryCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}

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

func TestJobHistoryCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobHistoryCommand{Meta: Meta{Ui: ui, flagAddress: url}}

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

func TestJobHistoryCommand_ACL(t *testing.T) {
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
			name: "missing read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["list-jobs"]
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
			expectedErr: "job versions not found",
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
			cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}
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

func blocksFromOutput(t *testing.T, out string) []string {
	t.Helper()
	rawBlocks := strings.Split(out, "Version")
	// trim empty blocks from whitespace in the output
	var blocks []string
	for _, block := range rawBlocks {
		trimmed := strings.TrimSpace(block)
		if trimmed != "" {
			blocks = append(blocks, trimmed)
		}
	}
	return blocks
}

func TestJobHistoryCommand_Diffs(t *testing.T) {
	ci.Parallel(t)

	// Start test server
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()
	state := srv.Agent.Server().State()

	// Create a job with multiple versions
	v0 := mock.Job()

	v0.ID = "test-job-history"
	v0.TaskGroups[0].Count = 1
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, v0))

	v1 := v0.Copy()
	v1.TaskGroups[0].Count = 2
	v1.VersionTag = &structs.JobVersionTag{
		Name:        "example-tag",
		Description: "example-description",
	}
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, v1))

	v2 := v0.Copy()
	v2.TaskGroups[0].Count = 3
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1002, nil, v2))

	v3 := v0.Copy()
	v3.TaskGroups[0].Count = 4
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1003, nil, v3))

	t.Run("Without diffs", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-address", url, v0.ID})
		must.Zero(t, code)

		out := ui.OutputWriter.String()
		// There should be four outputs
		must.Eq(t, 4, strings.Count(out, "Version"))
		must.Eq(t, 0, strings.Count(out, "Diff"))
	})
	t.Run("With diffs", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-p", "-address", url, v0.ID})
		must.Zero(t, code)

		out := ui.OutputWriter.String()
		blocks := blocksFromOutput(t, out)

		// Check that we have 4 versions
		must.Len(t, 4, blocks)
		must.Eq(t, 4, strings.Count(out, "Version"))
		must.Eq(t, 3, strings.Count(out, "Diff"))

		// Diffs show up for all versions except the first one
		must.StrContains(t, blocks[0], "Diff")
		must.StrContains(t, blocks[1], "Diff")
		must.StrContains(t, blocks[2], "Diff")
		must.StrNotContains(t, blocks[3], "Diff")

		// Check that the diffs are specifically against their predecessor
		must.StrContains(t, blocks[0], "\"3\" => \"4\"")
		must.StrContains(t, blocks[1], "\"2\" => \"3\"")
		must.StrContains(t, blocks[2], "\"1\" => \"2\"")
	})

	t.Run("With diffs against a specific version that doesnt exist", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-p", "-diff-version", "4", "-address", url, v0.ID})
		must.One(t, code)
		// Error that version 4 doesnt exists
		must.StrContains(t, ui.ErrorWriter.String(), "version 4 not found")

	})
	t.Run("With diffs against a specific version", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-p", "-diff-version", "3", "-address", url, v0.ID})
		must.Zero(t, code)

		out := ui.OutputWriter.String()
		blocks := blocksFromOutput(t, out)

		// Check that we have 4 versions
		must.Len(t, 4, blocks)
		must.Eq(t, 4, strings.Count(out, "Version"))
		must.Eq(t, 3, strings.Count(out, "Diff"))

		// Diffs show up for all versions except the specified one
		must.StrNotContains(t, blocks[0], "Diff")
		must.StrContains(t, blocks[1], "Diff")
		must.StrContains(t, blocks[2], "Diff")
		must.StrContains(t, blocks[3], "Diff")

		// Check that the diffs are specifically against the tagged version (which has a count of 4)
		must.StrContains(t, blocks[1], "\"4\" => \"3\"")
		must.StrContains(t, blocks[2], "\"4\" => \"2\"")
		must.StrContains(t, blocks[3], "\"4\" => \"1\"")

	})

	t.Run("With diffs against another specific version", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}

		// Diff against version 1 instead
		code := cmd.Run([]string{"-p", "-diff-version", "2", "-address", url, v0.ID})
		must.Zero(t, code)

		out := ui.OutputWriter.String()
		blocks := blocksFromOutput(t, out)

		// Check that we have 4 versions
		must.Len(t, 4, blocks)
		must.Eq(t, 4, strings.Count(out, "Version"))
		must.Eq(t, 3, strings.Count(out, "Diff"))

		// Diffs show up for all versions except the specified one
		must.StrContains(t, blocks[0], "Diff")
		must.StrNotContains(t, blocks[1], "Diff")
		must.StrContains(t, blocks[2], "Diff")
		must.StrContains(t, blocks[3], "Diff")

		// Check that the diffs are specifically against the tagged version (which has a count of 3)
		must.StrContains(t, blocks[0], "\"3\" => \"4\"")
		must.StrContains(t, blocks[2], "\"3\" => \"2\"")
		must.StrContains(t, blocks[3], "\"3\" => \"1\"")
	})

	t.Run("With diffs against a specific tag that doesnt exist", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{"-p", "-diff-tag", "nonexistent-tag", "-address", url, v0.ID})
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "tag \"nonexistent-tag\" not found")
	})

	t.Run("With diffs against a specific tag", func(t *testing.T) {
		ui := cli.NewMockUi()

		// Run history command with diff against the tag
		cmd := &JobHistoryCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"-p", "-diff-tag", "example-tag", "-address", url, v0.ID})
		must.Zero(t, code)

		out := ui.OutputWriter.String()
		blocks := blocksFromOutput(t, out)

		// Check that we have 4 versions
		must.Len(t, 4, blocks)
		must.Eq(t, 4, strings.Count(out, "Version"))
		must.Eq(t, 3, strings.Count(out, "Diff"))

		// Check that the diff is present for versions other than the tagged version
		must.StrContains(t, blocks[0], "Diff")
		must.StrContains(t, blocks[1], "Diff")
		must.StrNotContains(t, blocks[2], "Diff")
		must.StrContains(t, blocks[3], "Diff")

		// Check that the diffs are specifically against the tagged version (which has a count of 2)
		must.StrContains(t, blocks[0], "\"2\" => \"4\"")
		must.StrContains(t, blocks[1], "\"2\" => \"3\"")
		must.StrContains(t, blocks[3], "\"2\" => \"1\"")
	})
}
