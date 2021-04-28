package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

var _ cli.Command = &LicenseCommand{}

type LicenseCommand struct {
	Meta
}

func (l *LicenseCommand) Help() string {
	helpText := `
Usage: nomad license <subcommand> [options] [args]

This command has subcommands for managing the Nomad Enterprise license.
For more detailed examples see:
https://www.nomadproject.io/docs/commands/license/

Retrieve the server's license:

	$ nomad license get

	`
	return strings.TrimSpace(helpText)
}

func (l *LicenseCommand) Synopsis() string {
	return "Interact with Nomad Enterprise License"
}

func (l *LicenseCommand) Name() string { return "license" }

func (l *LicenseCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func OutputLicenseReply(ui cli.Ui, resp *api.LicenseReply) int {
	now := time.Now()
	expired := resp.License.ExpirationTime.Before(now)
	outputLicenseInfo(ui, resp.License, expired)
	if expired {
		return 1
	}
	return 0
}

func outputLicenseInfo(ui cli.Ui, lic *api.License, expired bool) {
	expStr := ""
	if expired {
		expStr = fmt.Sprintf("Expired At|%s", lic.ExpirationTime.String())
	} else {
		expStr = fmt.Sprintf("Expires At|%s", lic.ExpirationTime.String())
	}

	validity := "valid"
	if expired {
		validity = "expired!"
	}
	output := []string{
		fmt.Sprintf("Product|%s", lic.Product),
		fmt.Sprintf("License Status|%s", validity),
		fmt.Sprintf("License ID|%s", lic.LicenseID),
		fmt.Sprintf("Customer ID|%s", lic.CustomerID),
		fmt.Sprintf("Issued At|%s", lic.IssueTime),
		expStr,
		fmt.Sprintf("Datacenter|%s", lic.InstallationID),
	}
	ui.Output(formatKV(output))

	if len(lic.Modules) > 0 {
		ui.Output("Modules:")
		for _, mod := range lic.Modules {
			ui.Output(fmt.Sprintf("\t%s", mod))
		}
	}
	if len(lic.Features) > 0 {
		ui.Output("Licensed Features:")
		for _, f := range lic.Features {
			ui.Output(fmt.Sprintf("\t%s", f))
		}
	}
}
