// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type SentinelListCommand struct {
	Meta
}

func (c *SentinelListCommand) Help() string {
	helpText := `
Usage: nomad sentinel list [options]

  List is used to display all the installed Sentinel policies.

  Sentinel commands are only available when ACLs are enabled. This command
  requires a management token.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

`
	return strings.TrimSpace(helpText)
}

func (c *SentinelListCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *SentinelListCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *SentinelListCommand) Synopsis() string {
	return "Display all Sentinel policies"
}

func (c *SentinelListCommand) Name() string { return "sentinel list" }

func (c *SentinelListCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	if args = flags.Args(); len(args) > 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
	}
	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Get the list of policies
	policies, _, err := client.SentinelPolicies().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing Sentinel policies: %s", err))
		return 1
	}

	if len(policies) == 0 {
		c.Ui.Output("No policies found")
		return 0
	}

	out := []string{}
	out = append(out, "Name|Scope|Enforcement Level|Description")
	for _, p := range policies {
		line := fmt.Sprintf("%s|%s|%s|%s", p.Name, p.Scope, p.EnforcementLevel, p.Description)
		out = append(out, line)
	}
	c.Ui.Output(formatList(out))
	return 0
}
