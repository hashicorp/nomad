package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/Godeps/_workspace/src/github.com/mitchellh/cli"
)

func TestNodeDrainCommand_Implements(t *testing.T) {
	var _ cli.Command = &NodeDrainCommand{}
}

func TestNodeDrainCommand_Fails(t *testing.T) {
	srv, _, url := testServer(t, nil)
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &NodeDrainCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "-enable", "nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error toggling") {
		t.Fatalf("expected failed toggle error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on non-existent node
	if code := cmd.Run([]string{"-address=" + url, "-enable", "nope"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "not found") {
		t.Fatalf("expected not exist error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails if both enable and disable specified
	if code := cmd.Run([]string{"-enable", "-disable", "nope"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails if neither enable or disable specified
	if code := cmd.Run([]string{"nope"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
}
