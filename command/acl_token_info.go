// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type ACLTokenInfoCommand struct {
	Meta
}

func (c *ACLTokenInfoCommand) Help() string {
	helpText := `
Usage: nomad acl token info <token_accessor_id>

  Info is used to fetch information on an existing ACL tokens. Requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *ACLTokenInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *ACLTokenInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLTokenInfoCommand) Synopsis() string {
	return "Fetch information on an existing ACL token"
}

func (c *ACLTokenInfoCommand) Name() string { return "acl token info" }

func (c *ACLTokenInfoCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we have exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <token_accessor_id>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	tokenAccessorID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Get the specified token information
	token, _, err := client.ACLTokens().Info(tokenAccessorID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error fetching token: %s", err))
		return 1
	}

	// Format the output
	outputACLToken(c.Ui, token)
	return 0
}
