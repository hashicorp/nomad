package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestMonitorCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &MonitorCommand{}
}

func TestMonitorCommand_Fails(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &MonitorCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("exepected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}

	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("exepected exit code 1, got: %d", code)
	}

	// Fails on nonexistent node
	if code := cmd.Run([]string{"-address=" + url, "-node-id=12345678-abcd-efab-cdef-123456789abc"}); code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "No node(s) with prefix") {
		t.Fatalf("expected not found error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}
