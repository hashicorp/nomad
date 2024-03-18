// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type OperatorAutopilotHealthCommand struct {
	Meta
}

func (c *OperatorAutopilotHealthCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient))
}

func (c *OperatorAutopilotHealthCommand) AutocompleteArgs() complete.Predictor {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json": complete.PredictNothing,
		})
}

func (c *OperatorAutopilotHealthCommand) Name() string { return "operator autopilot health" }
func (c *OperatorAutopilotHealthCommand) Run(args []string) int {
	var fJson bool
	flags := c.Meta.FlagSet("autopilot", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&fJson, "json", false, "")

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
	state, _, err := client.Operator().AutopilotServerHealth(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying Autopilot configuration: %s", err))
		return 1
	}
	if fJson {
		bytes, err := json.Marshal(state)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("failed to serialize client state: %v", err))
			return 1
		}
		c.Ui.Output(string(bytes))
	}

	c.Ui.Output(formatAutopilotState(state))

	return 0
}

func (c *OperatorAutopilotHealthCommand) Synopsis() string {
	return "Display the current Autopilot health"
}

func (c *OperatorAutopilotHealthCommand) Help() string {
	helpText := `
Usage: nomad operator autopilot health [options]

  Displays the current Autopilot state.

  If ACLs are enabled, this command requires a token with the 'operator:read'
  capability.

General Options:

Output Options:

	-json
	Output the autopilot health in JSON format.

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func formatAutopilotState(state *api.OperatorHealthReply) string {
	var out string
	out = fmt.Sprintf("Healthy: %t\n", state.Healthy)
	out = out + fmt.Sprintf("FailureTolerance: %d\n", state.FailureTolerance)
	out = out + fmt.Sprintf("Leader: %s\n", state.Leader)
	out = out + fmt.Sprintf("Voters:  \n\t%s\n", renderServerIDList(state.Voters))
	out = out + fmt.Sprintf("Servers: \n%s\n", formatServerHealth(state.Servers))

	out = formatCommandToEnt(out, state)
	return out
}

func formatVoters(voters []string) string {
	out := make([]string, len(voters))
	for i, p := range voters {
		out[i] = fmt.Sprintf("\t%s", p)
	}
	return formatList(out)
}

func formatServerHealth(servers []api.ServerHealth) string {
	out := make([]string, len(servers)+1)
	out[0] = "ID|Name|Address|SerfStatus|Version|Leader|Voter|Healthy|LastContact|LastTerm|LastIndex|StableSince"
	for i, p := range servers {
		out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s|%t|%t|%t|%s|%d|%d|%s",
			p.ID,
			p.Name,
			p.Address,
			p.SerfStatus,
			p.Version,
			p.Leader,
			p.Voter,
			p.Healthy,
			p.LastContact,
			p.LastTerm,
			p.LastIndex,
			p.StableSince,
		)
	}
	return formatList(out)
}

func renderServerIDList(ids []string) string {
	rows := make([]string, len(ids))
	for i, id := range ids {
		rows[i] = fmt.Sprintf("\t%s", id)
	}
	return formatList(rows)
}

func formatCommandToEnt(out string, state *api.OperatorHealthReply) string {
	if len(state.ReadReplicas) > 0 {
		out = out + "\nReadReplicas:"
		out = out + formatList(state.ReadReplicas)
	}

	if len(state.RedundancyZones) > 0 {
		out = out + "\nRedundancyZones:"
		for _, zone := range state.RedundancyZones {
			out = out + fmt.Sprintf("  %v", zone)
		}
	}

	if state.Upgrade != nil {
		out = out + "Upgrade: \n"
		out = out + fmt.Sprintf(" \tStatus: %v\n", state.Upgrade.Status)
		out = out + fmt.Sprintf(" \tTargetVersion: %v\n", state.Upgrade.TargetVersion)
		if len(state.Upgrade.TargetVersionVoters) > 0 {
			out = out + fmt.Sprintf(" \tTargetVersionVoters: \n\t\t%s\n", renderServerIDList(state.Upgrade.TargetVersionVoters))
		}
		if len(state.Upgrade.TargetVersionNonVoters) > 0 {
			out = out + fmt.Sprintf(" \tTargetVersionNonVoters: \n\t\t%s\n", renderServerIDList(state.Upgrade.TargetVersionNonVoters))
		}
		if len(state.Upgrade.TargetVersionReadReplicas) > 0 {
			out = out + fmt.Sprintf(" \tTargetVersionReadReplicas: \n\t\t%s\n", renderServerIDList(state.Upgrade.TargetVersionReadReplicas))
		}
		if len(state.Upgrade.OtherVersionVoters) > 0 {
			out = out + fmt.Sprintf(" \tOtherVersionVoters: \n\t\t%s\n", renderServerIDList(state.Upgrade.OtherVersionVoters))
		}
		if len(state.Upgrade.OtherVersionNonVoters) > 0 {
			out = out + fmt.Sprintf(" \tOtherVersionNonVoters: \n\t\t%s\n", renderServerIDList(state.Upgrade.OtherVersionNonVoters))
		}
		if len(state.Upgrade.OtherVersionReadReplicas) > 0 {
			out = out + fmt.Sprintf(" \tOtherVersionReadReplicas: \n\t\t%s\n", renderServerIDList(state.Upgrade.OtherVersionReadReplicas))
		}
		if len(state.Upgrade.RedundancyZones) > 0 {

			out = out + " \tRedundancyZones:\n"
			for _, zone := range state.Upgrade.RedundancyZones {
				out = out + fmt.Sprintf("  \t\t%v", zone)
			}
		}
	}
	return out
}
