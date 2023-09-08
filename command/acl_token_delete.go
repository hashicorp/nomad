// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type ACLTokenDeleteCommand struct {
	Meta
}

func (c *ACLTokenDeleteCommand) Help() string {
	helpText := `
Usage: nomad acl token delete <token_accessor_id>

  Delete is used to delete an existing ACL token. Requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *ACLTokenDeleteCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *ACLTokenDeleteCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLTokenDeleteCommand) Synopsis() string {
	return "Delete an existing ACL token"
}

func (c *ACLTokenDeleteCommand) Name() string { return "acl token delete" }

func (c *ACLTokenDeleteCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that the last argument is the token to delete. Return error if no
	// such token was provided.
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

	// Delete the specified token
	_, err = client.ACLTokens().Delete(tokenAccessorID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting token: %s", err))
		return 1
	}

	// Format the output
	c.Ui.Output(fmt.Sprintf("Token %s successfully deleted", tokenAccessorID))
	return 0
}
