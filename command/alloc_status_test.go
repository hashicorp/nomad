package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestAllocStatusCommand_Implements(t *testing.T) {
	var _ cli.Command = &AllocStatusCommand{}
}

func TestAllocStatusCommand_Fails(t *testing.T) {
	ui := new(cli.MockUi)
	cmd := &AllocStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "foo"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying allocation") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}
