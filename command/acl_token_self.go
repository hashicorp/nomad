// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper"
	"github.com/posener/complete"
)

type ACLTokenSelfCommand struct {
	Meta
}

func (c *ACLTokenSelfCommand) Help() string {
	helpText := `
Usage: nomad acl token self

  Self is used to fetch information about the currently set ACL token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *ACLTokenSelfCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *ACLTokenSelfCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLTokenSelfCommand) Synopsis() string {
	return "Lookup self ACL token"
}

func (c *ACLTokenSelfCommand) Name() string { return "acl token self" }

func (c *ACLTokenSelfCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we have no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check what kind of token we have available
	envToken := os.Getenv("NOMAD_TOKEN")
	if envToken == "" {
		c.Ui.Error("No token present in the environment")
		return 1
	}

	// Does this look like a Nomad ACL token?
	if helper.IsUUID(envToken) {
		token, _, err := client.ACLTokens().Self(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error fetching self token: %s", err))
			return 1
		}
		// Format the output
		outputACLToken(c.Ui, token)
		return 0
	}

	policies, _, err := client.ACLPolicies().Self(nil)
	if err == nil && len(policies) > 0 {
		c.Ui.Info("No ACL token found but there are ACL policies attached to this workload identity. You can query them with acl policy self command.")
		return 0
	}
	c.Ui.Error("No ACL tokens, nor ACL policies attached to a workload identity found.")
	return 1
}
