package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestJobDeploymentsCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &JobDeploymentsCommand{}
}

func TestJobDeploymentsCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope", "foo"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error listing jobs") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestJobDeploymentsCommand_Run(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui}}

	// Should return an error message for no job match
	if code := cmd.Run([]string{"-address=" + url, "foo"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}

	// Create a job without a deployment
	job := mock.Job()
	state := srv.Agent.Server().State()
	assert.Nil(state.UpsertJob(100, job))

	// Should display no match if the job doesn't have deployments
	if code := cmd.Run([]string{"-address=" + url, job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "No deployments found") {
		t.Fatalf("expected no deployments output, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Inject a deployment
	d := mock.Deployment()
	d.JobID = job.ID
	assert.Nil(state.UpsertDeployment(200, d))

	// Should now display the deployment
	if code := cmd.Run([]string{"-address=" + url, "-verbose", job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, d.ID) {
		t.Fatalf("expected deployment output, got: %s", out)
	}
	ui.OutputWriter.Reset()
}

func TestJobDeploymentsCommand_Run_Latest(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui}}

	// Should return an error message for no job match
	if code := cmd.Run([]string{"-address=" + url, "-latest", "foo"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}

	// Create a job without a deployment
	job := mock.Job()
	state := srv.Agent.Server().State()
	assert.Nil(state.UpsertJob(100, job))

	// Should display no match if the job doesn't have deployments
	if code := cmd.Run([]string{"-address=" + url, "-latest", job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "No deployment found") {
		t.Fatalf("expected no deployments output, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// Inject a deployment
	d := mock.Deployment()
	d.JobID = job.ID
	assert.Nil(state.UpsertDeployment(200, d))

	// Should now display the deployment
	if code := cmd.Run([]string{"-address=" + url, "-verbose", "-latest", job.ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, d.ID) {
		t.Fatalf("expected deployment output, got: %s", out)
	}
	ui.OutputWriter.Reset()
}

func TestJobDeploymentsCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &JobDeploymentsCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	assert.Nil(state.UpsertJob(1000, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(j.ID, res[0])
}
