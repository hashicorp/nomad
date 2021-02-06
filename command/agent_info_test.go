package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestAgentInfoCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &AgentInfoCommand{}
}

func TestAgentInfoCommand_Run(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AgentInfoCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
}

func TestAgentInfoCommand_Run_JSON(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AgentInfoCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url, "-json"})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "\"config\": {") {
		t.Fatalf("expected config stanza in output json")
	}
}

func TestAgentInfoCommand_Run_Gotemplate(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AgentInfoCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url, "-t", "{{.Stats.raft}}"})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "last_log_index") {
		t.Fatalf("expected raft stats in gotemplate output")
	}
}

func TestAgentInfoCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	cmd := &AgentInfoCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying agent info") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}
