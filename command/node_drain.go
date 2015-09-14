package command

import (
	"fmt"
	"strings"
)

type NodeDrainCommand struct {
	Meta
}

func (c *NodeDrainCommand) Help() string {
	helpText := `
Usage: nomad node-drain [options] <node>

  Toggles node draining on a specified node. It is required
  that either -enable or -disable is specified, but not both.

General Options:

  ` + generalOptionsUsage() + `

Node Drain Options:

  -disable
    Disable draining for the specified node.

  -enable
    Enable draining for the specified node.
`
	return strings.TrimSpace(helpText)
}

func (c *NodeDrainCommand) Synopsis() string {
	return "Toggle drain mode on a given node"
}

func (c *NodeDrainCommand) Run(args []string) int {
	var enable, disable bool

	flags := c.Meta.FlagSet("node-drain", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&enable, "enable", false, "Enable drain mode")
	flags.BoolVar(&disable, "disable", false, "Disable drain mode")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got either enable or disable, but not both.
	if (enable && disable) || (!enable && !disable) {
		c.Ui.Error(c.Help())
		return 1
	}

	// Check that we got a node ID
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	nodeID := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Toggle node draining
	if _, err := client.Nodes().ToggleDrain(nodeID, enable, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Error toggling drain mode: %s", err))
		return 1
	}
	return 0
}
