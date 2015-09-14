package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestNodeStatusCommand_Implements(t *testing.T) {
	var _ cli.Command = &NodeStatusCommand{}
}

func TestNodeStatusCommand_Run(t *testing.T) {
	srv, _, url := testServer(t)
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &NodeStatusCommand{Meta: Meta{Ui: ui}}

	// Query all node statuses
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	// Expect empty output since we have no nodes
	if out := ui.OutputWriter.String(); out != "<nil>" {
		t.Fatalf("expected empty output, got: %s", out)
	}
}

func TestNodeStatusCommand_Fails(t *testing.T) {
	srv, _, url := testServer(t)
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &NodeStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying node status") {
		t.Fatalf("expected failed query error, got: %s", out)
	}

	// Fails on non-existent node
	if code := cmd.Run([]string{"-address=" + url, "nope"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "not found") {
		t.Fatalf("expected not found error, got: %s", out)
	}
}
