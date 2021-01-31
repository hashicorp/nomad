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

  When ACLs are enabled, this command requires a token with the
  'operator:read' capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return helpText
}

func (c *LicenseGetCommand) Synopsis() string {
	return "Retrieve the current Nomad Enterprise License"
}

func (c *LicenseGetCommand) Name() string { return "license get" }

func (c *LicenseGetCommand) Run(args []string) int {
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

	resp, _, err := client.Operator().LicenseGet(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting license: %v", err))
		return 1
	}

	return OutputLicenseReply(c.Ui, resp)
}
