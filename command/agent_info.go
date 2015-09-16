package command

import (
	"fmt"
	"strings"
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

func (c *AgentInfoCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("agent-info", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we either got no jobs or exactly one.
	args = flags.Args()
	if len(args) > 0 {
		c.Ui.Error(c.Help())
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

	var stats map[string]interface{}
	stats, _ = info["stats"]

	for section, data := range stats {
		c.Ui.Output(section)
		d, _ := data.(map[string]interface{})
		for k, v := range d {
			c.Ui.Output(fmt.Sprintf("  %s = %v", k, v))
		}
	}

	return 0
}
