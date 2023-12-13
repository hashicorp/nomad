// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
)

func TestScalingPolicyInfoCommand_Run(t *testing.T) {
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
	cmd := &ScalingPolicyInfoCommand{Meta: Meta{Ui: ui}}

	// Calling without the policyID should result in an error.
	if code := cmd.Run([]string{"-address=" + url}); code != 1 {
		t.Fatalf("expected cmd run exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "This command takes one of the following argument conditions") {
		t.Fatalf("expected argument error within output: %v", out)
	}

	// Calling with more than one argument should result in an error.
	if code := cmd.Run([]string{"-address=" + url, "first", "second"}); code != 1 {
		t.Fatalf("expected cmd run exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "This command takes one of the following argument conditions") {
		t.Fatalf("expected argument error within output: %v", out)
	}

	// Perform an initial info, which should return zero results.
	if code := cmd.Run([]string{"-address=" + url, "scaling_policy_info"}); code != 1 {
		t.Fatalf("expected cmd run exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, `No scaling policies with prefix or id "scaling_policy_inf" found`) {
		t.Fatalf("expected 'no policies found' within output: %v", out)
	}

	// Generate a test job.
	job := testJob("scaling_policy_info")

	// Generate an example scaling policy.
	job.TaskGroups[0].Scaling = &api.ScalingPolicy{
		Enabled: pointer.Of(true),
		Min:     pointer.Of(int64(1)),
		Max:     pointer.Of(int64(1)),
	}

	// Register the job.
	resp, _, err := client.Jobs().Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("expected waitForSuccess exit code 0, got: %d", code)
	}

	// Grab the generated policyID.
	policies, _, err := client.Scaling().ListPolicies(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	numPolicies := len(policies)
	if numPolicies == 0 || numPolicies > 1 {
		t.Fatalf("expected 1 policy return, got %v", numPolicies)
	}

	if code := cmd.Run([]string{"-address=" + url, policies[0].ID}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "Policy:") {
		t.Fatalf("expected policy ID within output: %v", out)
	}

	prefix := policies[0].ID[:2]
	if code := cmd.Run([]string{"-address=" + url, prefix}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "Policy:") {
		t.Fatalf("expected policy ID within output: %v", out)
	}
}
