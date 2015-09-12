package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestAgentInfoCommand_Implements(t *testing.T) {
	var _ cli.Command = &AgentInfoCommand{}
}

func TestAgentInfoCommand_Run(t *testing.T) {
	srv, _, url := testServer(t)
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &AgentInfoCommand{Ui: ui}

	code := cmd.Run([]string{"-http-addr=" + url})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d %s", code)
	}
}

func TestAgentInfoCommand_Fails(t *testing.T) {
	ui := new(cli.MockUi)
	cmd := &AgentInfoCommand{Ui: ui}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}

	// Fails on connection failure
	if code := cmd.Run([]string{"-http-addr=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Failed querying agent info") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}
