package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSystemReconcileSummariesCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &SystemReconcileSummariesCommand{}
}

func TestSystemReconcileSummariesCommand_Good(t *testing.T) {
	t.Parallel()

	// Create a server
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &SystemReconcileSummariesCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
}
