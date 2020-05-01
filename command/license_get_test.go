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

	code := cmd.Run([]string{"-address=" + url})
	require.Equal(t, 1, code)

	if srv.Enterprise {
		require.Contains(t, ui.OutputWriter.String(), "License Status")
	} else {
		require.Contains(t, ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
	}

}
