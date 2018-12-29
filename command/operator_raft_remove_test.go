package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestOperator_Raft_RemovePeers_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorRaftRemoveCommand{}
}

func TestOperator_Raft_RemovePeer(t *testing.T) {
	t.Parallel()
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := new(cli.MockUi)
	c := &OperatorRaftRemoveCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr, "-peer-address=nope"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// If we get this error, it proves we sent the address all they through.
	output := strings.TrimSpace(ui.ErrorWriter.String())
	if !strings.Contains(output, "address \"nope\" was not found in the Raft configuration") {
		t.Fatalf("bad: %s", output)
	}
}
