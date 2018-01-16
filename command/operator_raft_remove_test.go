package command

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestOperator_Raft_RemovePeers_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &OperatorRaftRemoveCommand{}
}

func TestOperator_Raft_RemovePeer(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := new(cli.MockUi)
	c := &OperatorRaftRemoveCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr, "-peer-address=nope", "-peer-id=nope"}

	// Give both an address and ID
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	assert.Contains(ui.ErrorWriter.String(), "cannot give both an address and id")

	// Neither address nor ID present
	args = args[:1]
	code = c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	assert.Contains(ui.ErrorWriter.String(), "an address or id is required for the peer to remove")
}

func TestOperator_Raft_RemovePeerAddress(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
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
	assert.Contains(ui.ErrorWriter.String(), "address \"nope\" was not found in the Raft configuration")
}

func TestOperator_Raft_RemovePeerID(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := new(cli.MockUi)
	c := &OperatorRaftRemoveCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr, "-peer-id=nope"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// If we get this error, it proves we sent the address all they through.
	assert.Contains(ui.ErrorWriter.String(), "id \"nope\" was not found in the Raft configuration")
}
