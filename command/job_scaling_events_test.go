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
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestJobScalingEventsCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	ui := cli.NewMockUi()
	cmd := &JobScalingEventsCommand{Meta: Meta{Ui: ui}}

	// Register a test job and ensure it is running before moving on.
	resp, _, err := client.Jobs().Register(testJob("scale_events_test_job"), nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("expected waitForSuccess exit code 0, got: %d", code)
	}

	// List events without passing the jobID which should result in an error.
	if code := cmd.Run([]string{"-address=" + url}); code != 1 {
		t.Fatalf("expected cmd run exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "This command takes one argument: <job_id>") {
		t.Fatalf("Expected argument error: %v", out)
	}

	// List events for the job, which should present zero.
	if code := cmd.Run([]string{"-address=" + url, "scale_events_test_job"}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "No events found") {
		t.Fatalf("Expected no events output but got: %v", out)
	}

	// Perform a scaling action to generate an event.
	_, _, err = client.Jobs().Scale(
		"scale_events_test_job",
		"group1", pointer.Of(2),
		"searchable custom test message", false, nil, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// List the scaling events which should include an entry.
	if code := cmd.Run([]string{"-address=" + url, "scale_events_test_job"}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "Task Group  Count  PrevCount  Date") {
		t.Fatalf("Expected table headers but got: %v", out)
	}

	// List the scaling events with verbose flag to search for our message as
	// well as the verbose table headers.
	if code := cmd.Run([]string{"-address=" + url, "-verbose", "scale_events_test_job"}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "searchable custom test message") {
		t.Fatalf("Expected to find scaling message but got: %v", out)
	}
	if !strings.Contains(out, "Eval ID") {
		t.Fatalf("Expected to verbose table headers: %v", out)
	}
}

func TestJobScalingEventsCommand_ACL(t *testing.T) {
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
			name: "missing read-job or read-job-scaling",
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "read-job-scaling allowed",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job-scaling"]
}
`,
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
	capabilities = ["read-job-scaling"]
}
`,
			expectedErr: "job not found",
		},
		{
			name:      "job prefix works with list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job-scaling","list-jobs"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobScalingEventsCommand{Meta: Meta{Ui: ui}}
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
