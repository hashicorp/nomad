package command

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

var _ cli.Command = &LicensePutCommand{}

func TestCommand_LicensePut_Err(t *testing.T) {
	t.Parallel()

	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &LicensePutCommand{Meta: Meta{Ui: ui}, testStdin: strings.NewReader("testlicenseblob")}

	if code := cmd.Run([]string{"-address=" + url, "-"}); code != 1 {
		require.Equal(t, code, 1)
	}

	if srv.Enterprise {
		require.Contains(t, ui.ErrorWriter.String(), "error validating license")
	} else {
		require.Contains(t, ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
	}

}
