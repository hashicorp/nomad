// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type QuotaDeleteCommand struct {
	Meta
}

func (c *QuotaDeleteCommand) Help() string {
	helpText := `
Usage: nomad quota delete [options] <quota>

  Delete is used to delete an existing quota specification.

  If ACLs are enabled, this command requires a token with the 'quota:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return strings.TrimSpace(helpText)
}

func (c *QuotaDeleteCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *QuotaDeleteCommand) AutocompleteArgs() complete.Predictor {
	return QuotaPredictor(c.Meta.Client)
}

func (c *QuotaDeleteCommand) Synopsis() string {
	return "Delete a quota specification"
}

func (c *QuotaDeleteCommand) Name() string { return "quota delete" }

func (c *QuotaDeleteCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <quota>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	name := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.Quotas().Delete(name, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting quota: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted quota %q!", name))
	return 0
}
