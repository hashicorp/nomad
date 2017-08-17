package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestStatusCommand_Run_JobStatus(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Register a job
	job1 := testJob("job1_sfx")
	resp, _, err := client.Jobs().Register(job1, nil)
	assert.Nil(err)

	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// Query to check the job status
	if code := cmd.Run([]string{"-address=" + url, "job1_sfx"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, "job1_sfx")

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_EvalStatus(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	assert.Nil(err)

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

	assert.NotEqual("", evalID)

	// Query to check the eval status
	if code := cmd.Run([]string{"-address=" + url, evalID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, evalID)

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_NodeStatus(t *testing.T) {
	assert := assert.New(t)
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
	assert.Contains(out, "mynode")

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_AllocStatus(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	assert.Nil(err)

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
	assert.NotEqual("", allocId1)

	if code := cmd.Run([]string{"-address=" + url, allocId1}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, allocId1)

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_NoPrefix(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Register a job
	job1 := testJob("job1_sfx")
	resp, _, err := client.Jobs().Register(job1, nil)
	assert.Nil(err)

	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// Query to check status
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, "job1_sfx")

	ui.OutputWriter.Reset()
}

func TestStatusCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	assert.Nil(err)

	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	prefix := jobID[:len(jobID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Contains(res, jobID)
}
