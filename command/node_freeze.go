package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type NodeFreezeCommand struct {
	Meta
}

func (c *NodeFreezeCommand) Help() string {
	helpText := `
Usage: nomad node-freeze [options] <node>

  Toogles node to not accept new allocations. It is required
  that either -enable or -disable is specified, but not both.
  The -self flag is useful to freeze the local node.


General Options:

  ` + generalOptionsUsage() + `

Node Freeze Options:

  -disable
    Disable freeze for the specified node.

  -enable
    Enable freeze for the specified node.

  -self
    Query the status of the local node.

  -yes
    Automatic yes to prompts.
`
	return strings.TrimSpace(helpText)
}

func (c *NodeFreezeCommand) Synopsis() string {
	return "Toggle freeze mode on a given node"
}

func (c *NodeFreezeCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-disable": complete.PredictNothing,
			"-enable":  complete.PredictNothing,
			"-self":    complete.PredictNothing,
			"-yes":     complete.PredictNothing,
		})
}

func (c *NodeFreezeCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Nodes, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Nodes]
	})
}

func (c *NodeFreezeCommand) Run(args []string) int {
	var enable, disable, self, autoYes bool

	flags := c.Meta.FlagSet("node-freeze", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&enable, "enable", false, "Enable freeze mode")
	flags.BoolVar(&disable, "disable", false, "Disable freeze mode")
	flags.BoolVar(&self, "self", false, "")
	flags.BoolVar(&autoYes, "yes", false, "Automatic yes to prompts.")

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
	if l := len(args); self && l != 0 || !self && l != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// If -self flag is set then determine the current node.
	nodeID := ""
	if !self {
		nodeID = args[0]
	} else {
		var err error
		if nodeID, err = getLocalNodeID(client); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	// Check if node exists
	if len(nodeID) == 1 {
		c.Ui.Error(fmt.Sprintf("Identifier must contain at least two characters."))
		return 1
	}

	nodeID = sanatizeUUIDPrefix(nodeID)
	nodes, _, err := client.Nodes().PrefixList(nodeID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error toggling freeze mode: %s", err))
		return 1
	}
	// Return error if no nodes are found
	if len(nodes) == 0 {
		c.Ui.Error(fmt.Sprintf("No node(s) with prefix or id %q found", nodeID))
		return 1
	}
	if len(nodes) > 1 {
		// Format the nodes list that matches the prefix so that the user
		// can create a more specific request
		out := make([]string, len(nodes)+1)
		out[0] = "ID|Datacenter|Name|Class|Drain|Freeze|Status"
		for i, node := range nodes {
			out[i+1] = fmt.Sprintf("%s|%s|%s|%s|%v|%v|%s",
				node.ID,
				node.Datacenter,
				node.Name,
				node.NodeClass,
				node.Drain,
				node.Freeze,
				node.Status)
		}
		// Dump the output
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple nodes\n\n%s", formatList(out)))
		return 1
	}

	// Prefix lookup matched a single node
	node, _, err := client.Nodes().Info(nodes[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error toggling freeze mode: %s", err))
		return 1
	}

	// Confirm freeze if the node was a prefix match.
	if nodeID != node.ID && !autoYes {
		verb := "enable"
		if disable {
			verb = "disable"
		}
		question := fmt.Sprintf("Are you sure you want to %s freeze mode for node %q? [y/N]", verb, node.ID)
		answer, err := c.Ui.Ask(question)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse answer: %v", err))
			return 1
		}

		if answer == "" || strings.ToLower(answer)[0] == 'n' {
			// No case
			c.Ui.Output("Canceling freeze toggle")
			return 0
		} else if strings.ToLower(answer)[0] == 'y' && len(answer) > 1 {
			// Non exact match yes
			c.Ui.Output("For confirmation, an exact ‘y’ is required.")
			return 0
		} else if answer != "y" {
			c.Ui.Output("No confirmation detected. For confirmation, an exact 'y' is required.")
			return 1
		}
	}

	// Toggle node freeze
	if _, err := client.Nodes().ToggleFreeze(node.ID, enable, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Error toggling freeze mode: %s", err))
		return 1
	}
	return 0
}
