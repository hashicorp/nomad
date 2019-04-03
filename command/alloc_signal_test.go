package command

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestAllocSignalCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &AllocSignalCommand{}
}

func TestAllocSignalCommand_Fails(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	require := require.New(t)

	ui := new(cli.MockUi)
	cmd := &AllocSignalCommand{Meta: Meta{Ui: ui}}

	// Fails on lack of alloc ID
	require.Equal(1, cmd.Run([]string{}))
	require.Contains(ui.ErrorWriter.String(), "This command takes up to two arguments")
	ui.ErrorWriter.Reset()

	// Fails on misuse
	require.Equal(1, cmd.Run([]string{"some", "bad", "args"}))
	require.Contains(ui.ErrorWriter.String(), "This command takes up to two arguments")
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	require.Equal(1, cmd.Run([]string{"-address=nope", "foobar"}))
	require.Contains(ui.ErrorWriter.String(), "Error querying allocation")
	ui.ErrorWriter.Reset()

	// Fails on missing alloc
	code := cmd.Run([]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"})
	require.Equal(1, code)
	require.Contains(ui.ErrorWriter.String(), "No allocation(s) with prefix or id")
	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	require.Equal(1, cmd.Run([]string{"-address=" + url, "2"}))
	require.Contains(ui.ErrorWriter.String(), "must contain at least two characters.")
	ui.ErrorWriter.Reset()
}
