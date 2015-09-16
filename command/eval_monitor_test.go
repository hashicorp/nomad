package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestEvalMonitorCommand_Implements(t *testing.T) {
	var _ cli.Command = &EvalMonitorCommand{}
}

func TestEvalMonitorCommand_Fails(t *testing.T) {
	srv, _, url := testServer(t, nil)
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &EvalMonitorCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on eval lookup failure
	if code := cmd.Run([]string{"-address=" + url, "3E55C771-76FC-423B-BCED-3E5314F433B1"}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "not found") {
		t.Fatalf("expect not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error reading evaluation") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}
