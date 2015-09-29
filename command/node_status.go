package command

import (
	"fmt"
	"strings"
)

type NodeStatusCommand struct {
	Meta
}

func (c *NodeStatusCommand) Help() string {
	helpText := `
Usage: nomad node-status [options] [node]

  Display status information about a given node. The list of nodes
  returned includes only nodes which jobs may be scheduled to, and
  includes status and other high-level information.

  If a node ID is passed, information for that specific node will
  be displayed. If no node ID's are passed, then a short-hand
  list of all nodes will be displayed.

General Options:

  ` + generalOptionsUsage() + `

Node Status Options:

  -short
    Display short output. Used only when a single node is being
    queried, and drops verbose output about node allocations.
`
	return strings.TrimSpace(helpText)
}

func (c *NodeStatusCommand) Synopsis() string {
	return "Display status information about nodes"
}

func (c *NodeStatusCommand) Run(args []string) int {
	var short bool

	flags := c.Meta.FlagSet("node-status", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got either a single node or none
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// Use list mode if no node name was provided
	if len(args) == 0 {
		// Query the node info
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node status: %v", err))
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
		c.Ui.Output(formatList(out))
		return 0
	}

	// Query the specific node
	nodeID := args[0]
	node, _, err := client.Nodes().Info(nodeID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying node info: %v", err))
		return 1
	}

	// Format the output
	basic := []string{
		fmt.Sprintf("ID|%s", node.ID),
		fmt.Sprintf("Name|%s", node.Name),
		fmt.Sprintf("Class|%s", node.NodeClass),
		fmt.Sprintf("Datacenter|%s", node.Datacenter),
		fmt.Sprintf("Drain|%v", node.Drain),
		fmt.Sprintf("Status|%s", node.Status),
	}

	var allocs []string
	if !short {
		// Query the node allocations
		nodeAllocs, _, err := client.Nodes().Allocations(nodeID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying node allocations: %v", err))
			return 1
		}

		// Format the allocations
		allocs = make([]string, len(nodeAllocs)+1)
		allocs[0] = "ID|EvalID|JobID|TaskGroup|DesiredStatus|ClientStatus"
		for i, alloc := range nodeAllocs {
			allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%s|%s",
				alloc.ID,
				alloc.EvalID,
				alloc.JobID,
				alloc.TaskGroup,
				alloc.DesiredStatus,
				alloc.ClientStatus)
		}
	}

	// Dump the output
	c.Ui.Output(formatKV(basic))
	if !short {
		c.Ui.Output("\n### Allocations")
		c.Ui.Output(formatList(allocs))
	}
	return 0
}
