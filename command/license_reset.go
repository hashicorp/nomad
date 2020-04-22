package command

import (
	"fmt"
)

type LicenseResetCommand struct {
	Meta
}

func (c *LicenseResetCommand) Help() string {
	helpText := `
Usage: nomad license reset [options]

Resets the Nomad Server and Clients Enterprise license to the builtin one if
it is still valid. IF the builtin license is invalid, the current one stays 
active.

General Options:

	` + generalOptionsUsage()

	return helpText
}

func (c *LicenseResetCommand) Synopsis() string {
	return "Install a new Nomad Enterprise License"
}

func (c *LicenseResetCommand) Name() string { return "license reset" }

func (c *LicenseResetCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	resp, _, err := client.Operator().LicenseReset(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error resetting license: %v", err))
		return 1
	}

	return OutputLicenseReply(c.Ui, resp)
}
