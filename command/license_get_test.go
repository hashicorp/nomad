package command

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

var _ cli.Command = &LicenseGetCommand{}

func TestCommand_LicenseGet_OSSErr(t *testing.T) {
	t.Parallel()

	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &LicenseGetCommand{Meta: Meta{Ui: ui}}

	if code := cmd.Run([]string{"-address=" + url}); code != 1 {
		require.Equal(t, 1, code)
	}

	require.Contains(t, ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")

}
