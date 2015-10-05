package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/Godeps/_workspace/src/github.com/mitchellh/cli"
	"github.com/hashicorp/nomad/testutil"
)

func TestClientConfigCommand_Implements(t *testing.T) {
	var _ cli.Command = &ClientConfigCommand{}
}

func TestClientConfigCommand_UpdateServers(t *testing.T) {
	srv, _, url := testServer(t, func(c *testutil.TestServerConfig) {
		c.Client.Enabled = true
		c.Server.BootstrapExpect = 0
	})
	defer srv.Stop()

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
	code = cmd.Run([]string{"-address=" + url, "-update-servers", "foo", "bar"})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	// Query the servers list
	code = cmd.Run([]string{"-address=" + url, "-servers"})
	if code != 0 {
		t.Fatalf("expect exit 0, got: %d", code)
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "foo") {
		t.Fatalf("missing foo")
	}
	if !strings.Contains(out, "bar") {
		t.Fatalf("missing bar")
	}
}

func TestClientConfigCommand_Fails(t *testing.T) {
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
