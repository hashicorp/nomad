package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
)

func TestStatusCommand_Run_JobStatus(t *testing.T) {
	t.Parallel()
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Register a job
	job1 := testJob("job1_sfx")
	resp, _, err := client.Jobs().Register(job1, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// Query to check the job status
	if code := cmd.Run([]string{"-address=" + url, "job1_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()

	if !strings.Contains(out, "job1_sfx") {
		t.Fatalf("expected job1_sfx, got: %s", out)
	}
	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_EvalStatus(t *testing.T) {
	t.Parallel()

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// get an eval id
	evalID := ""
	if evals, _, err := client.Jobs().Evaluations(jobID, nil); err == nil {
		if len(evals) > 0 {
			evalID = evals[0].ID
		}
	}
	if evalID == "" {
		t.Fatal("unable to find an evaluation")
	}

	// Query to check the eval status
	if code := cmd.Run([]string{"-address=" + url, evalID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()

	if !strings.Contains(out, evalID) {
		t.Fatalf("expected eval id, got: %s", out)
	}

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_NodeStatus(t *testing.T) {
	t.Parallel()

	// Start in dev mode so we get a node registration
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.NodeName = "mynode"
	})
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Wait for a node to appear
	var nodeID string
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		nodeID = nodes[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Query to check the node status
	if code := cmd.Run([]string{"-address=" + url, nodeID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()

	if !strings.Contains(out, "mynode") {
		t.Fatalf("expected node id (mynode), got: %s", out)
	}

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_AllocStatus(t *testing.T) {
	t.Parallel()

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

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// get an alloc id
	allocId1 := ""
	if allocs, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
		if len(allocs) > 0 {
			allocId1 = allocs[0].ID
		}
	}
	if allocId1 == "" {
		t.Fatal("unable to find an allocation")
	}

	if code := cmd.Run([]string{"-address=" + url, allocId1}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, allocId1) {
		t.Fatal("expected to find alloc id in output")
	}

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_NoPrefix(t *testing.T) {
	t.Parallel()
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Register a job
	job1 := testJob("job1_sfx")
	resp, _, err := client.Jobs().Register(job1, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// Query to check status
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()

	if !strings.Contains(out, "job1_sfx") {
		t.Fatalf("expected job1_sfx, got: %s", out)
	}

	ui.OutputWriter.Reset()
}
