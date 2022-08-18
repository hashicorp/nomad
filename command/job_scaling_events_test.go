package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
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
