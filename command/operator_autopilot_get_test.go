package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestOperator_Autopilot_GetConfig_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorRaftListCommand{}
}

func TestOperatorAutopilotGetConfigCommand(t *testing.T) {
	t.Parallel()
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := new(cli.MockUi)
	c := &OperatorAutopilotGetCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.OutputWriter.String())
	if !strings.Contains(output, "CleanupDeadServers = true") {
		t.Fatalf("bad: %s", output)
	}
}
