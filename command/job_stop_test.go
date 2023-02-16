package command

import (
	"encoding/json"
	"os"
	"path/filepath"
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
)

var _ cli.Command = (*JobStopCommand)(nil)

func TestStopCommand_multi(t *testing.T) {
	ci.Parallel(t)

	srv, _, addr := testServer(t, true, func(c *agent.Config) {
		c.DevMode = true
	})
	defer srv.Shutdown()

	// the number of jobs we want to run
	numJobs := 10

	// create and run a handful of jobs
	jobIDs := make([]string, 0, numJobs)
	for i := 0; i < numJobs; i++ {
		jobID := uuid.Generate()
		jobIDs = append(jobIDs, jobID)
	}

	jobFilePath := func(jobID string) string {
		return filepath.Join(os.TempDir(), jobID+".nomad")
	}

	// cleanup job files we will create
	t.Cleanup(func() {
		for _, jobID := range jobIDs {
			_ = os.Remove(jobFilePath(jobID))
		}
	})

	// record cli output
	ui := cli.NewMockUi()

	for _, jobID := range jobIDs {
		job := testJob(jobID)
		job.TaskGroups[0].Tasks[0].Resources.MemoryMB = pointer.Of(16)
		job.TaskGroups[0].Tasks[0].Resources.DiskMB = pointer.Of(32)
		job.TaskGroups[0].Tasks[0].Resources.CPU = pointer.Of(10)
		job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
			"run_for": "30s",
		}

		jobJSON, err := json.MarshalIndent(job, "", " ")
		must.NoError(t, err)

		jobFile := jobFilePath(jobID)
		err = os.WriteFile(jobFile, []byte(jobJSON), 0o644)
		must.NoError(t, err)

		cmd := &JobRunCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"-address", addr, "-json", jobFile})
		must.Zero(t, code,
			must.Sprintf("job stop stdout: %s", ui.OutputWriter.String()),
			must.Sprintf("job stop stderr: %s", ui.ErrorWriter.String()),
		)
	}

	// helper for stopping a list of jobs
	stop := func(args ...string) (stdout string, stderr string, code int) {
		cmd := &JobStopCommand{Meta: Meta{Ui: ui}}
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
	code := cmd.Run([]string{"-some", "-bad", "-args"})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, "flag provided but not defined: -some")

	ui.ErrorWriter.Reset()

	// Fails on nonexistent job ID
	code = cmd.Run([]string{"-address=" + url, "nope"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "No job(s) with prefix or id")

	ui.ErrorWriter.Reset()

	// Fails on connection failure
	code = cmd.Run([]string{"-address=nope", "nope"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Error finding jobs with prefix: nope")
}

func TestStopCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobStopCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, j.ID, res[0])
}
