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
