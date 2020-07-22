package command

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
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
	if srv.Enterprise {
		require.Equal(t, 0, code)
	} else {
		require.Equal(t, 1, code)
		require.Contains(t, ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
	}
}

func TestOutputLicenseReply(t *testing.T) {
	now := time.Now()
	lic := &api.LicenseReply{
		License: &api.License{
			LicenseID:       "licenseID",
			CustomerID:      "customerID",
			InstallationID:  "*",
			IssueTime:       now,
			StartTime:       now,
			ExpirationTime:  now.Add(1 * time.Hour),
			TerminationTime: now,
			Product:         "nomad",
			Flags: map[string]interface{}{
				"": nil,
			},
		},
	}

	ui := new(cli.MockUi)

	require.Equal(t, 0, OutputLicenseReply(ui, lic))

	out := ui.OutputWriter.String()
	require.Contains(t, out, "Customer ID")
	require.Contains(t, out, "License ID")
}
