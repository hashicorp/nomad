package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
)

func TestScalingPolicyListCommand_Run(t *testing.T) {
	t.Parallel()
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
	cmd := &ScalingPolicyListCommand{Meta: Meta{Ui: ui}}

	// Perform an initial list, which should return zero results.
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "No policies found") {
		t.Fatalf("expected no policies found within output: %v", out)
	}

	// Generate two test jobs.
	jobs := []*api.Job{testJob("scaling_policy_list_1"), testJob("scaling_policy_list_2")}

	// Generate an example scaling policy.
	scalingPolicy := api.ScalingPolicy{
		Enabled: helper.BoolToPtr(true),
		Min:     helper.Int64ToPtr(1),
		Max:     helper.Int64ToPtr(1),
	}

	// Iterate the jobs, add the scaling policy and register.
	for _, job := range jobs {
		job.TaskGroups[0].Scaling = &scalingPolicy
		resp, _, err := client.Jobs().Register(job, nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
			t.Fatalf("expected waitForSuccess exit code 0, got: %d", code)
		}
	}

	// Perform a new list which should yield results..
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected cmd run exit code 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "ID") ||
		!strings.Contains(out, "Enabled") ||
		!strings.Contains(out, "Target") {
		t.Fatalf("expected table headers within output: %v", out)
	}
	if !strings.Contains(out, "scaling_policy_list_1") {
		t.Fatalf("expected job scaling_policy_list_1 within output: %v", out)
	}
	if !strings.Contains(out, "scaling_policy_list_2") {
		t.Fatalf("expected job scaling_policy_list_2 within output: %v", out)
	}
}
