// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type SentinelReadCommand struct {
	Meta
}

func (c *SentinelReadCommand) Help() string {
	helpText := `
Usage: nomad sentinel read [options] <name>

  Read is used to inspect a Sentinel policy.

  Sentinel commands are only available when ACLs are enabled. This command
  requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Read Options:

  -raw
    Prints only the raw policy

`
	return strings.TrimSpace(helpText)
}

func (c *SentinelReadCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-raw": complete.PredictNothing,
		})
}

func (c *SentinelReadCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *SentinelReadCommand) Synopsis() string {
	return "Inspects an existing Sentinel policies"
}

func (c *SentinelReadCommand) Name() string { return "sentinel read" }

func (c *SentinelReadCommand) Run(args []string) int {
	var raw bool
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&raw, "raw", false, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one arguments
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <name>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the name and file
	policyName := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Query the policy
	policy, _, err := client.SentinelPolicies().Info(policyName, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Sentinel policy: %s", err))
		return 1
	}

	// Check for only the raw policy
	if raw {
		c.Ui.Output(policy.Policy)
		return 0
	}

	// Output the base information
	info := []string{
		fmt.Sprintf("Name|%s", policy.Name),
		fmt.Sprintf("Scope|%s", policy.Scope),
		fmt.Sprintf("Enforcement Level|%s", policy.EnforcementLevel),
		fmt.Sprintf("Description|%s", policy.Description),
	}
	c.Ui.Output(formatKV(info))
	c.Ui.Output("Policy:")
	c.Ui.Output(policy.Policy)
	return 0
}
