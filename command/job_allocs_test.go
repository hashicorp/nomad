package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestJobAllocsCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobAllocsCommand{}
}

func TestJobAllocsCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	outerr := ui.ErrorWriter.String()
	require.Equalf(t, 1, code, "expected exit code 1, got: %d", code)
	require.Containsf(t, outerr, commandErrorText(cmd), "expected help output, got: %s", outerr)

	ui.ErrorWriter.Reset()

	// Bad address
	code = cmd.Run([]string{"-address=nope", "foo"})
	outerr = ui.ErrorWriter.String()
	require.Equalf(t, 1, code, "expected exit code 1, got: %d", code)
	require.Containsf(t, outerr, "Error listing jobs", "expected failed query error, got: %s", outerr)

	ui.ErrorWriter.Reset()

	// Bad job name
	code = cmd.Run([]string{"-address=" + url, "foo"})
	outerr = ui.ErrorWriter.String()
	require.Equalf(t, 1, code, "expected exit 1, got: %d", code)
	require.Containsf(t, outerr, "No job(s) with prefix or id \"foo\" found", "expected no job found, got: %s", outerr)

	ui.ErrorWriter.Reset()
}

func TestJobAllocsCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}

	// Create a job without an allocation
	job := mock.Job()
	state := srv.Agent.Server().State()
	require.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 100, job))

	// Should display no match if the job doesn't have allocations
	code := cmd.Run([]string{"-address=" + url, job.ID})
	out := ui.OutputWriter.String()
	require.Equalf(t, 0, code, "expected exit 0, got: %d", code)
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
	out = ui.OutputWriter.String()
	outerr := ui.ErrorWriter.String()
	require.Equalf(t, 0, code, "expected exit 0, got: %d", code)
	require.Emptyf(t, outerr, "expected no error output, got: \n\n%s", outerr)
	require.Containsf(t, out, a.ID, "expected alloc output, got: %s", out)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func TestJobAllocsCommand_Template(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}

	// Create a job
	job := mock.Job()
	state := srv.Agent.Server().State()
	require.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 100, job))

	// Inject a running allocation
	a := mock.Alloc()
	a.Job = job
	a.JobID = job.ID
	a.TaskGroup = job.TaskGroups[0].Name
	a.Metrics = &structs.AllocMetric{}
	a.DesiredStatus = structs.AllocDesiredStatusRun
	a.ClientStatus = structs.AllocClientStatusRunning
	require.Nil(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{a}))

	// Inject a pending allocation
	b := mock.Alloc()
	b.Job = job
	b.JobID = job.ID
	b.TaskGroup = job.TaskGroups[0].Name
	b.Metrics = &structs.AllocMetric{}
	b.DesiredStatus = structs.AllocDesiredStatusRun
	b.ClientStatus = structs.AllocClientStatusPending
	require.Nil(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 300, []*structs.Allocation{b}))

	// Should display an AllocacitonListStub object
	code := cmd.Run([]string{"-address=" + url, "-t", "'{{printf \"%#+v\" .}}'", job.ID})
	out := ui.OutputWriter.String()
	outerr := ui.ErrorWriter.String()

	require.Equalf(t, 0, code, "expected exit 0, got: %d", code)
	require.Emptyf(t, outerr, "expected no error output, got: \n\n%s", outerr)
	require.Containsf(t, out, "api.AllocationListStub", "expected alloc output, got: %s", out)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Should display only the running allocation ID
	code = cmd.Run([]string{"-address=" + url, "-t", "'{{ range . }}{{ if eq .ClientStatus \"running\" }}{{ println .ID }}{{ end }}{{ end }}'", job.ID})
	out = ui.OutputWriter.String()
	outerr = ui.ErrorWriter.String()

	require.Equalf(t, 0, code, "expected exit 0, got: %d", code)
	require.Emptyf(t, outerr, "expected no error output, got: \n\n%s", outerr)
	require.Containsf(t, out, a.ID, "expected ID of alloc a, got: %s", out)
	require.NotContainsf(t, out, b.ID, "should not contain ID of alloc b, got: %s", out)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func TestJobAllocsCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
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
