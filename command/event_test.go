package command

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestEventCommand_BaseCommand(t *testing.T) {
	t.Parallel()

	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &EventCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url})

	require.Equal(t, -18511, code)
}
