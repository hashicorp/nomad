package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
)

type NodeDrainCommand struct {
	Ui cli.Ui
}

func (c *NodeDrainCommand) Help() string {
	helpText := `
Usage: nomad node-drain [options] <node>

  Toggles node draining on a specified node. It is required
  that either -enable or -disable is specified, but not both.

Options:

  -disable
    Disable draining for the specified node.

  -enable
    Enable draining for the specified node.

  -help
    Display this message

  -http-addr
    Address of the Nomad API to connect. Can also be specified
    using the environment variable NOMAD_HTTP_ADDR.
    Default = http://127.0.0.1:4646
`
	return strings.TrimSpace(helpText)
}

func (c *NodeDrainCommand) Synopsis() string {
	return "Toggle drain mode on a given node"
}

func (c *NodeDrainCommand) Run(args []string) int {
	var httpAddr *string
	var enable, disable bool

	flags := flag.NewFlagSet("node-drain", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&enable, "enable", false, "Enable drain mode")
	flags.BoolVar(&disable, "disable", false, "Disable drain mode")
	httpAddr = httpAddrFlag(flags)

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got either enable or disable, but not both.
	if (enable && disable) || (!enable && !disable) {
		c.Ui.Error(c.Help())
		return 1
	}

	// Check that we got a node ID
	if len(flags.Args()) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	nodeID := flags.Args()[0]

	// Get the HTTP client
	client, err := httpClient(*httpAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed initializing Nomad client: %s", err))
		return 1
	}

	// Toggle node draining
	if _, err := client.Nodes().ToggleDrain(nodeID, enable, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to toggle drain mode: %s", err))
		return 1
	}
	return 0
}
