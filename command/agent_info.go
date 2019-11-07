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

General Options:

  ` + generalOptionsUsage()
	return strings.TrimSpace(helpText)
}

func (c *AgentInfoCommand) Synopsis() string {
	return "Display status information about the local agent"
}

func (c *AgentInfoCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *AgentInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *AgentInfoCommand) Name() string { return "agent-info" }

func (c *AgentInfoCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
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

	// Sort and output agent info
	statsKeys := make([]string, 0, len(info.Stats))
	for key := range info.Stats {
		statsKeys = append(statsKeys, key)
	}
	sort.Strings(statsKeys)

	for _, key := range statsKeys {
		c.Ui.Output(key)
		statsData, _ := info.Stats[key]
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
