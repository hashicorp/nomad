package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestJobDispatchCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobDispatchCommand{}
}

func TestJobDispatchCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobDispatchCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails when specified file does not exist
	if code := cmd.Run([]string{"foo", "/unicorns/leprechauns"}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error reading input data") {
		t.Fatalf("expect error reading input data: %v", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope", "foo"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Failed to dispatch") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestJobDispatchCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobDispatchCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	require.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	// No parameterized jobs, should be 0 results
	res := predictor.Predict(args)
	require.Equal(t, 0, len(res))

	// Create a fake parameterized job
	j1 := mock.Job()
	j1.ParameterizedJob = &structs.ParameterizedJobConfig{}
	require.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 2000, j1))

	prefix = j1.ID[:len(j1.ID)-5]
	args = complete.Args{Last: prefix}
	predictor = cmd.AutocompleteArgs()

	// Should return 1 parameterized job
	res = predictor.Predict(args)
	require.Equal(t, 1, len(res))
	require.Equal(t, j1.ID, res[0])
}
