// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/posener/complete"
)

type AgentInfoCommand struct {
	Meta
}

func (c *AgentInfoCommand) Help() string {
	helpText := `
Usage: nomad agent-info [options]

  Display status information about the local agent.

  When ACLs are enabled, this command requires a token with the 'agent:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Agent Info Options:

  -json
    Output the node in its JSON format.

  -t
    Format and display node using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *AgentInfoCommand) Synopsis() string {
	return "Display status information about the local agent"
}

func (c *AgentInfoCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
			"-t":    complete.PredictAnything,
		})
}

func (c *AgentInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *AgentInfoCommand) Name() string { return "agent-info" }

func (c *AgentInfoCommand) Run(args []string) int {
	var json bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if len(args) > 0 {
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

	// Query the agent info
	info, err := client.Agent().Self()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying agent info: %s", err))
		return 1
	}

	// If output format is specified, format and output the agent info
	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, info)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error formatting output: %s", err))
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Sort and output agent info
	statsKeys := make([]string, 0, len(info.Stats))
	for key := range info.Stats {
		statsKeys = append(statsKeys, key)
	}
	sort.Strings(statsKeys)

	for _, key := range statsKeys {
		c.Ui.Output(key)
		statsData := info.Stats[key]
		statsDataKeys := make([]string, len(statsData))
		i := 0
		for key := range statsData {
			statsDataKeys[i] = key
			i++
		}
		sort.Strings(statsDataKeys)

		for _, key := range statsDataKeys {
			c.Ui.Output(fmt.Sprintf("  %s = %v", key, statsData[key]))
		}
	}

	return 0
}
