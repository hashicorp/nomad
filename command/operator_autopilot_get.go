// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/posener/complete"
)

type OperatorAutopilotGetCommand struct {
	Meta
}

func (c *OperatorAutopilotGetCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient))
}

func (c *OperatorAutopilotGetCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorAutopilotGetCommand) Name() string { return "operator autopilot get-config" }
func (c *OperatorAutopilotGetCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("autopilot", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch the current configuration.
	config, _, err := client.Operator().AutopilotGetConfiguration(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Autopilot configuration: %s", err))
		return 1
	}
	c.Ui.Output(fmt.Sprintf("CleanupDeadServers = %v", config.CleanupDeadServers))
	c.Ui.Output(fmt.Sprintf("LastContactThreshold = %v", config.LastContactThreshold.String()))
	c.Ui.Output(fmt.Sprintf("MaxTrailingLogs = %v", config.MaxTrailingLogs))
	c.Ui.Output(fmt.Sprintf("MinQuorum = %v", config.MinQuorum))
	c.Ui.Output(fmt.Sprintf("ServerStabilizationTime = %v", config.ServerStabilizationTime.String()))
	c.Ui.Output(fmt.Sprintf("EnableRedundancyZones = %v", config.EnableRedundancyZones))
	c.Ui.Output(fmt.Sprintf("DisableUpgradeMigration = %v", config.DisableUpgradeMigration))
	c.Ui.Output(fmt.Sprintf("EnableCustomUpgrades = %v", config.EnableCustomUpgrades))

	return 0
}

func (c *OperatorAutopilotGetCommand) Synopsis() string {
	return "Display the current Autopilot configuration"
}

func (c *OperatorAutopilotGetCommand) Help() string {
	helpText := `
Usage: nomad operator autopilot get-config [options]

  Displays the current Autopilot configuration.

  If ACLs are enabled, this command requires a token with the 'operator:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}
