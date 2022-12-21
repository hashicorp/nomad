package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

var _ cli.Command = (*JobStopCommand)(nil)

func TestStopCommand_multi(t *testing.T) {
	ci.Parallel(t)

	srv, _, addr := testServer(t, true, func(c *agent.Config) {
		c.DevMode = true
	})
	defer srv.Shutdown()

	ui := cli.NewMockUi()

	// the number of jobs we want to run
	numJobs := 10

	// create and run a handful of jobs
	jobIDs := make([]string, 0, numJobs)
	for i := 0; i < numJobs; i++ {
		jobID := uuid.Generate()
		jobIDs = append(jobIDs, jobID)

		job := testJob(jobID)
		job.TaskGroups[0].Tasks[0].Resources.MemoryMB = pointer.Of(16)
		job.TaskGroups[0].Tasks[0].Resources.DiskMB = pointer.Of(32)
		job.TaskGroups[0].Tasks[0].Resources.CPU = pointer.Of(10)
		job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
			"run_for": "30s",
		}

		jobJSON, err := json.MarshalIndent(job, "", " ")
		must.NoError(t, err)

		jobFile := filepath.Join(os.TempDir(), jobID)
		err = os.WriteFile(jobFile, []byte(jobJSON), 0o644)
		must.NoError(t, err)

		cmd := &JobRunCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"-address", addr, "-json", jobFile})
		must.Zero(t, code, must.Sprintf(
			"job run failed, stdout: %s, stderr: %s",
			ui.OutputWriter.String(), ui.ErrorWriter.String(),
		))
	}

	// helper for stopping a list of jobs
	stop := func(args ...string) (stdout string, stderr string, code int) {
		cmd := &JobStopCommand{Meta: Meta{Ui: ui}}
		t.Logf("run nomad job stop %s", strings.Join(args, " "))
		code = cmd.Run(args)
		return ui.OutputWriter.String(), ui.ErrorWriter.String(), code
	}

	// stop all jobs in one command
	args := []string{"-address", addr, "-detach"}
	args = append(args, jobIDs...)
	stdout, stderr, code := stop(args...)
	must.Zero(t, code,
		must.Sprintf("job stop stdout: %s", stdout),
		must.Sprintf("job stop stderr: %s", stderr),
	)
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
