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

Install a new license from a file:
	$ nomad license put @nomad.license

Install a new license from stdin:
	$ nomad license put -

Install a new license from a string:
	$ nomad license put "<license blob>"

Retrieve the current license:

	$ nomad license get

Reset the current license:
	$ nomad license reset
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
	if resp.Valid {
		ui.Output("License is valid")
		outputLicenseInfo(ui, resp.License, false)
		return 0
	} else if resp.License != nil {
		now := time.Now()
		if resp.License.ExpirationTime.Before(now) {
			ui.Output("License has expired!")
			outputLicenseInfo(ui, resp.License, true)
		} else {
			ui.Output("License is invalid!")
			for _, warn := range resp.Warnings {
				ui.Output(fmt.Sprintf("   %s", warn))
			}
			outputLicenseInfo(ui, resp.License, false)
		}
		return 1
	} else {
		// TODO - remove the expired message here in the future
		//        once the go-licensing library is updated post 1.1
		ui.Output("Nomad is unlicensed or the license has expired")
		return 0
	}
}

func outputLicenseInfo(ui cli.Ui, lic *api.License, expired bool) {
	ui.Output(fmt.Sprintf("License ID: %s", lic.LicenseID))
	ui.Output(fmt.Sprintf("Customer ID: %s", lic.CustomerID))
	if expired {
		ui.Output(fmt.Sprintf("Expired At: %s", lic.ExpirationTime.String()))
	} else {
		ui.Output(fmt.Sprintf("Expires At: %s", lic.ExpirationTime.String()))
	}
	ui.Output(fmt.Sprintf("Terminates At: %s", lic.TerminationTime.String()))
	ui.Output(fmt.Sprintf("Datacenter: %s", lic.InstallationID))
	if len(lic.Modules) > 0 {
		ui.Output("Modules:")
		for _, mod := range lic.Modules {
			ui.Output(fmt.Sprintf("\t%v", mod))
		}
	}
	ui.Output("Licensed Features:")
	for _, f := range lic.Features {
		ui.Output(fmt.Sprintf("\t%s", f))
	}
}
