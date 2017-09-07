// +build pro ent

package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestNamespaceListCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &NamespaceListCommand{}
}

func TestNamespaceListCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &NamespaceListCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, cmd.Help()) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error retrieving namespaces") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestNamespaceListCommand_List(t *testing.T) {
	t.Parallel()

	// Create a server
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &NamespaceListCommand{Meta: Meta{Ui: ui}}

	// List should contain default deployment
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "default") || !strings.Contains(out, "Default shared namespace") {
		t.Fatalf("expected default namespace, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// List json
	t.Log(url)
	if code := cmd.Run([]string{"-address=" + url, "-json"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
	out = ui.OutputWriter.String()
	if !strings.Contains(out, "CreateIndex") {
		t.Fatalf("expected json output, got: %s", out)
	}
	ui.OutputWriter.Reset()
}
