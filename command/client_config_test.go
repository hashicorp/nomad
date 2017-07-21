package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
)

func TestClientConfigCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &ClientConfigCommand{}
}

func TestClientConfigCommand_UpdateServers(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, true, func(c *agent.Config) {
		c.Server.BootstrapExpect = 0
	})
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &ClientConfigCommand{Meta: Meta{Ui: ui}}

	// Fails if trying to update with no servers
	code := cmd.Run([]string{"-update-servers"})
	if code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Set the servers list
	code = cmd.Run([]string{"-address=" + url, "-update-servers", "127.0.0.42", "198.18.5.5"})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	// Query the servers list
	code = cmd.Run([]string{"-address=" + url, "-servers"})
	if code != 0 {
		t.Fatalf("expect exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "127.0.0.42") {
		t.Fatalf("missing 127.0.0.42")
	}
	if !strings.Contains(out, "198.18.5.5") {
		t.Fatalf("missing 198.18.5.5")
	}
}

func TestClientConfigCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &ClientConfigCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails if no valid flags given
	if code := cmd.Run([]string{}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "-servers"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying server list") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}
