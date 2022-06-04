package command

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStopCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobStopCommand{}
}

func TestStopCommand_JSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	stop := func(args ...string) (stdout string, stderr string, code int) {
		cmd := &JobStopCommand{
			Meta: Meta{Ui: ui},
		}
		t.Logf("run: nomad job stop %s", strings.Join(args, " "))
		code = cmd.Run(args)
		return ui.OutputWriter.String(), ui.ErrorWriter.String(), code
	}

	// Agent startup is slow, do some work while we wait
	agentReady := make(chan string)
	var srv *agent.TestAgent
	var client *api.Client
	go func() {
		var addr string
		srv, client, addr = testServer(t, false, nil)
		agentReady <- addr
	}()
	defer srv.Shutdown()

	// Wait for agent to start and get its address
	select {
	case <-agentReady:
	case <-time.After(20 * time.Second):
		t.Fatalf("timed out waiting for agent to start")
	}

	// create and run 10 jobs
	jobIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		jobID := uuid.Generate()
		jobIDs = append(jobIDs, jobID)

		job := testJob(jobID)
		job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
			"run_for": "30s",
		}

		resp, _, err := client.Jobs().Register(job, nil)
		if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
			t.Fatalf("[DEBUG] waiting for job to register; status code non zero saw %d", code)
		}

		require.NoError(t, err)
	}

	// stop all jobs
	var args []string
	args = append(args, "-detach")
	args = append(args, jobIDs...)
	stdout, stderr, code := stop(args...)
	t.Logf("[DEBUG] run: nomad job stop stdout/stderr: %s/%s", stdout, stderr)
	require.Zero(t, code)
	require.Empty(t, stderr)

}

func TestStopCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobStopCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on nonexistent job ID
	if code := cmd.Run([]string{"-address=" + url, "nope"}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No job(s) with prefix or id") {
		t.Fatalf("expect not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error deregistering job") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}

func TestStopCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobStopCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(j.ID, res[0])
}
