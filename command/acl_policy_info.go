// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type ACLPolicyInfoCommand struct {
	Meta
}

func (c *ACLPolicyInfoCommand) Help() string {
	helpText := `
Usage: nomad acl policy info <name>

  Info is used to fetch information on an existing ACL policy.

  This command requires a management ACL token or a token that has the
  associated policy.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (c *ACLPolicyInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *ACLPolicyInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ACLPolicyInfoCommand) Synopsis() string {
	return "Fetch info on an existing ACL policy"
}

func (c *ACLPolicyInfoCommand) Name() string { return "acl policy info" }

func (c *ACLPolicyInfoCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <name>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the policy name
	policyName := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch info on the policy
	policy, _, err := client.ACLPolicies().Info(policyName, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error fetching info on ACL policy: %s", err))
		return 1
	}

	c.Ui.Output(c.Colorize().Color(formatACLPolicy(policy)))
	return 0
}
