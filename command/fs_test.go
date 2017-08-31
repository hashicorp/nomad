package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
)

func TestFSCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &FSCommand{}
}

func TestFSCommand_Fails(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &FSCommand{Meta: Meta{Ui: ui}}

	// Fails on lack of job ID
	if code := cmd.Run([]string{"-job"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "job ID is required") {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on lack of allocation ID
	if code := cmd.Run([]string{}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "allocation ID is required") {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "foobar"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying allocation") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on missing alloc
	if code := cmd.Run([]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No allocation(s) with prefix or id") {
		t.Fatalf("expected not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	if code := cmd.Run([]string{"-address=" + url, "2"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "must contain at least two characters.") {
		t.Fatalf("expected too few characters error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Identifiers with uneven length should produce a query result
	if code := cmd.Run([]string{"-address=" + url, "123"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No allocation(s) with prefix or id") {
		t.Fatalf("expected not found error, got: %s", out)
	}
}

func TestFSCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &FSCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	assert.Nil(state.UpsertAllocs(1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(a.ID, res[0])
}
