package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
)

func TestClientServersCommand_Implements(t *testing.T) {
	var _ cli.Command = &ClientServersCommand{}
}

func TestClientServersCommand_Run(t *testing.T) {
	srv, _, url := testServer(t, func(c *testutil.TestServerConfig) {
		c.Client.Enabled = true
		c.Server.BootstrapExpect = 0
	})
	defer srv.Stop()

	ui := new(cli.MockUi)
	cmd := &ClientServersCommand{Meta: Meta{Ui: ui}}

	// Set the servers list
	code := cmd.Run([]string{"-address=" + url, "-update", "foo", "bar"})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	// Query the servers list
	code = cmd.Run([]string{"-address=" + url})
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

func TestClientServersCommand_Fails(t *testing.T) {
	ui := new(cli.MockUi)
	cmd := &ClientServersCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails if updating with no servers
	if code := cmd.Run([]string{"-update"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying server list") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}
