package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mitchellh/cli"
	"github.com/ryanuber/columnize"
)

type NodeStatusCommand struct {
	Ui cli.Ui
}

func (c *NodeStatusCommand) Help() string {
	helpText := `
Usage: nomad node-status [options] [node]

  Displays status information about a given node. The list of nodes
  returned includes only nodes jobs may be scheduled to, and
  includes status and other high-level information.

  If a node ID is passed, information for that specific node will
  be displayed. If no node ID's are passed, then a short-hand
  list of all nodes will be displayed.

Options:

  -help
    Display this message

  -http-addr
    Address of the Nomad API to connect. Can also be specified
    using the environment variable NOMAD_HTTP_ADDR.
    Default = http://127.0.0.1:4646
`
	return strings.TrimSpace(helpText)
}

func (c *NodeStatusCommand) Synopsis() string {
	return "Display information about nodes"
}

func (c *NodeStatusCommand) Run(args []string) int {
	var httpAddr *string

	flags := flag.NewFlagSet("node-status", flag.ContinueOnError)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	httpAddr = httpAddrFlag(flags)

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got either a single node or none
	if len(flags.Args()) > 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := httpClient(*httpAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed initializing Nomad client: %s", err))
		return 1
	}

	// Use list mode if no node name was provided
	if len(flags.Args()) == 0 {
		// Query the node info
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed querying node info: %s", err))
			return 1
		}

		// Return nothing if no nodes found
		if len(nodes) == 0 {
			return 0
		}

		// Format the nodes list
		out := make([]string, len(nodes)+1)
		out[0] = "ID|DC|Name|Class|Drain|Status"
		for i, node := range nodes {
			out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%v|%s",
				node.ID,
				node.Datacenter,
				node.Name,
				node.NodeClass,
				node.Drain,
				node.Status)
		}

		// Dump the output
		c.Ui.Output(columnize.SimpleFormat(out))
		return 0
	}

	// Query the specific node
	nodeID := flags.Args()[0]
	node, _, err := client.Nodes().Info(nodeID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed querying node info: %s", err))
		return 1
	}

	// Format the output
	out := []string{
		fmt.Sprintf("ID | %s", node.ID),
		fmt.Sprintf("Name | %s", node.Name),
		fmt.Sprintf("Class | %s", node.NodeClass),
		fmt.Sprintf("Datacenter | %s", node.Datacenter),
		fmt.Sprintf("Drain | %v", node.Drain),
		fmt.Sprintf("Status | %s", node.Status),
	}

	// Dump the output
	c.Ui.Output(columnize.SimpleFormat(out))
	return 0
}
