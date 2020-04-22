package command

import (
	"fmt"
)

type LicenseGetCommand struct {
	Meta
}

func (c *LicenseGetCommand) Help() string {
	helpText := `
Usage: nomad license get [options]

Gets a new license in Servers and Clients
General Options:

	` + generalOptionsUsage() + `

Get Options:
	
	-signed
	  Determines if the returned license should be a signed blob instead of a
	  parsed license.

	`
	return helpText
}

func (c *LicenseGetCommand) Synopsis() string {
	return "Install a new Nomad Enterprise License"
}

func (c *LicenseGetCommand) Name() string { return "license get" }

func (c *LicenseGetCommand) Run(args []string) int {
	var signed bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&signed, "signed", false, "Gets the signed license blob instead of a parsed license")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if signed {
		resp, _, err := client.Operator().LicenseGetSigned(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error getting signed license: %v", err))
			return 1
		}
		c.Ui.Output(resp)
		return 0
	}

	resp, _, err := client.Operator().LicenseGet(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting license: %v", err))
		return 1
	}

	return OutputLicenseReply(c.Ui, resp)
}
