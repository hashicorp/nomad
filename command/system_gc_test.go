package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSystemGCCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &SystemGCCommand{}
}

func TestSystemGCCommand_Good(t *testing.T) {
	t.Parallel()

	// Create a server
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &SystemGCCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
}
