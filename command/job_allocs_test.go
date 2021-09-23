package command

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestJobAllocsCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &JobAllocsCommand{}
}

func TestJobAllocsCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equalf(t, 1, code, "expected exit code 1, got: %d", code)

	out := ui.ErrorWriter.String()
	require.Containsf(t, out, commandErrorText(cmd), "expected help output, got: %s", out)

	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "foo"})
	require.Equalf(t, 1, code, "expected exit code 1, got: %d", code)

	out = ui.ErrorWriter.String()
	require.Containsf(t, out, "Error listing jobs", "expected failed query error, got: %s", out)

	ui.ErrorWriter.Reset()
}

func TestJobAllocsCommand_Run(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}

	// Should return an error message for no job match
	code := cmd.Run([]string{"-address=" + url, "foo"})
	require.Equalf(t, 1, code, "expected exit 1, got: %d", code)

	// Create a job without an allocation
	job := mock.Job()
	state := srv.Agent.Server().State()
	require.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 100, job))

	// Should display no match if the job doesn't have allocations
	code = cmd.Run([]string{"-address=" + url, job.ID})
	require.Equalf(t, 0, code, "expected exit 0, got: %d", code)

	out := ui.OutputWriter.String()
	require.Containsf(t, out, "No allocations placed", "expected no allocations placed, got: %s", out)

	ui.OutputWriter.Reset()

	// Inject an allocation
	a := mock.Alloc()
	a.Job = job
	a.JobID = job.ID
	a.TaskGroup = job.TaskGroups[0].Name
	a.Metrics = &structs.AllocMetric{}
	a.DesiredStatus = structs.AllocDesiredStatusRun
	a.ClientStatus = structs.AllocClientStatusRunning
	require.Nil(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{a}))

	// Should now display the alloc
	code = cmd.Run([]string{"-address=" + url, "-verbose", job.ID})
	require.Equalf(t, 0, code, "expected exit 0, got: %d", code)

	out = ui.OutputWriter.String()
	require.Containsf(t, out, a.ID, "expected alloc output, got: %s", out)

	ui.OutputWriter.Reset()
}

func TestJobAllocsCommand_AutocompleteArgs(t *testing.T) {
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	require.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	require.Equal(t, 1, len(res))
	require.Equal(t, j.ID, res[0])
}
