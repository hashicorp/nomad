// Copyright IBM Corp. 2015, 2026
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
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *OperatorAutopilotGetCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorAutopilotGetCommand) Name() string { return "operator autopilot get-config" }
func (c *OperatorAutopilotGetCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet("autopilot", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

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

	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, config)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
		c.Ui.Output(out)
		return 0
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

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Get Config Options:

  -json
    Output the Autopilot configuration in JSON format.

  -t
    Format and display the Autopilot configuration using a Go template.
`

	return strings.TrimSpace(helpText)
}
