package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestStatusCommand_Run_JobStatus(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	assert.Nil(state.UpsertJob(1000, j))

	// Query to check the job status
	if code := cmd.Run([]string{"-address=" + url, j.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, j.ID)

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_JobStatus_MultiMatch(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create two fake jobs sharing a prefix
	state := srv.Agent.Server().State()
	j := mock.Job()
	j2 := mock.Job()
	j2.ID = fmt.Sprintf("%s-more", j.ID)
	assert.Nil(state.UpsertJob(1000, j))
	assert.Nil(state.UpsertJob(1001, j2))

	// Query to check the job status
	if code := cmd.Run([]string{"-address=" + url, j.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, j.ID)

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_EvalStatus(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake eval
	state := srv.Agent.Server().State()
	eval := mock.Eval()
	assert.Nil(state.UpsertEvals(1000, []*structs.Evaluation{eval}))

	// Query to check the eval status
	if code := cmd.Run([]string{"-address=" + url, eval.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, eval.ID[:shortId])

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

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	alloc := mock.Alloc()
	assert.Nil(state.UpsertAllocs(1000, []*structs.Allocation{alloc}))

	if code := cmd.Run([]string{"-address=" + url, alloc.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, alloc.ID[:shortId])

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_DeploymentStatus(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake deployment
	state := srv.Agent.Server().State()
	deployment := mock.Deployment()
	assert.Nil(state.UpsertDeployment(1000, deployment))

	// Query to check the deployment status
	if code := cmd.Run([]string{"-address=" + url, deployment.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, deployment.ID[:shortId])

	ui.OutputWriter.Reset()
}

func TestStatusCommand_Run_NoPrefix(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	job := mock.Job()
	assert.Nil(state.UpsertJob(1000, job))

	// Query to check status
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	out := ui.OutputWriter.String()
	assert.Contains(out, job.ID)

	ui.OutputWriter.Reset()
}

func TestStatusCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &StatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	job := mock.Job()
	assert.Nil(state.UpsertJob(1000, job))

	prefix := job.ID[:len(job.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Contains(res, job.ID)
}
