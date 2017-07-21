package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestEvalStatusCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &EvalStatusCommand{}
}

func TestEvalStatusCommand_Fails(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &EvalStatusCommand{Meta: Meta{Ui: ui}}

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
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No evaluation(s) with prefix or id") {
		t.Fatalf("expect not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying evaluation") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Failed on both -json and -t options are specified
	if code := cmd.Run([]string{"-address=" + url, "-json", "-t", "{{.ID}}"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Both json and template formatting are not allowed") {
		t.Fatalf("expected getting formatter error, got: %s", out)
	}

}
