// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type LicenseGetCommand struct {
	Meta
}

func (c *LicenseGetCommand) Help() string {
	helpText := `
Usage: nomad license get [options]

  Gets the license loaded by the server. The command is not forwarded to the
  Nomad leader, and will return the license from the specific server being
  contacted.

  When ACLs are enabled, this command requires a token with the
  'operator:read' capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return helpText
}

func (c *LicenseGetCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *LicenseGetCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
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

	resp, _, err := client.Operator().LicenseGet(&api.QueryOptions{})
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting license: %v", err))
		return 1
	}

	return OutputLicenseReply(c.Ui, resp)
}
