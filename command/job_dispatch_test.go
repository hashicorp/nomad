package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestJobDispatchCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &JobDispatchCommand{}
}

func TestJobDispatchCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &JobDispatchCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
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
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &JobDispatchCommand{Meta: Meta{Ui: ui, flagAddress: url}}

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
