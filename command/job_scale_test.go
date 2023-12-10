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

func TestJobScaleCommand_SingleGroup(t *testing.T) {
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
	cmd := &JobScaleCommand{Meta: Meta{Ui: ui}}

	// Register a test job and ensure it is running before moving on.
	resp, _, err := client.Jobs().Register(testJob("scale_cmd_single_group"), nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("expected waitForSuccess exit code 0, got: %d", code)
	}

	// Perform the scaling action.
	if code := cmd.Run([]string{"-address=" + url, "-detach", "scale_cmd_single_group", "2"}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "Evaluation ID:") {
		t.Fatalf("Expected Evaluation ID within output: %v", out)
	}
}

func TestJobScaleCommand_MultiGroup(t *testing.T) {
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
	cmd := &JobScaleCommand{Meta: Meta{Ui: ui}}

	// Create a job with two task groups.
	job := testJob("scale_cmd_multi_group")
	task := api.NewTask("task2", "mock_driver").
		SetConfig("kill_after", "1s").
		SetConfig("run_for", "5s").
		SetConfig("exit_code", 0).
		Require(&api.Resources{
			MemoryMB: pointer.Of(256),
			CPU:      pointer.Of(100),
		}).
		SetLogConfig(&api.LogConfig{
			MaxFiles:      pointer.Of(1),
			MaxFileSizeMB: pointer.Of(2),
		})
	group2 := api.NewTaskGroup("group2", 1).
		AddTask(task).
		RequireDisk(&api.EphemeralDisk{
			SizeMB: pointer.Of(20),
		})
	job.AddTaskGroup(group2)

	// Register a test job and ensure it is running before moving on.
	resp, _, err := client.Jobs().Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("expected waitForSuccess exit code 0, got: %d", code)
	}

	// Attempt to scale without specifying the task group which should fail.
	if code := cmd.Run([]string{"-address=" + url, "-detach", "scale_cmd_multi_group", "2"}); code != 1 {
		t.Fatalf("expected cmd run exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Group name required") {
		t.Fatalf("unexpected error message: %v", out)
	}

	// Specify the target group which should be successful.
	if code := cmd.Run([]string{"-address=" + url, "-detach", "scale_cmd_multi_group", "group1", "2"}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "Evaluation ID:") {
		t.Fatalf("Expected Evaluation ID within output: %v", out)
	}
}

func TestJobScaleCommand_ACL(t *testing.T) {
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
			name: "missing scale-job or job-submit",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job-scaling"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "missing read-job-scaling",
			aclPolicy: `
namespace "default" {
	capabilities = ["scale-job"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "read-job-scaling and scale-job allowed but can't monitor eval without read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job-scaling", "scale-job"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name: "read-job-scaling and submit-job allowed but can't monitor eval without read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job-scaling", "submit-job"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name: "read-job-scaling and scale-job allowed and can monitor eval with read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "read-job-scaling", "scale-job"]
}
`,
		},
		{
			name: "read-job-scaling and submit-job allowed and can monitor eval with read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "read-job-scaling", "submit-job"]
}
`,
		},
		{
			name:      "job prefix requires list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job-scaling", "scale-job"]
}
`,
			expectedErr: "job not found",
		},
		{
			name:      "job prefix works with list-job but can't monitor eval without read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job-scaling", "scale-job", "list-jobs"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name:      "job prefix works with list-job and can monitor eval with read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "read-job-scaling", "scale-job", "list-jobs"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobScaleCommand{Meta: Meta{Ui: ui}}
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

			// Run command scaling job to 2.
			args = append(args, "2")
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
