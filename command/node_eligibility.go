// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type NodeEligibilityCommand struct {
	Meta
}

func (c *NodeEligibilityCommand) Help() string {
	helpText := `
Usage: nomad node eligibility [options] <node>

  Toggles the nodes scheduling eligibility. When a node is marked as ineligible,
  no new allocations will be placed on it but existing allocations will remain.
  To remove existing allocations, use the node drain command.

  It is required that either -enable or -disable is specified, but not both.
  The -self flag is useful to set the scheduling eligibility of the local node.

  If ACLs are enabled, this option requires a token with the 'node:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Node Eligibility Options:

  -disable
    Mark the specified node as ineligible for new allocations.

  -enable
    Mark the specified node as eligible for new allocations.

  -self
    Set the eligibility of the local node.
`
	return strings.TrimSpace(helpText)
}

func (c *NodeEligibilityCommand) Synopsis() string {
	return "Toggle scheduling eligibility for a given node"
}

func (c *NodeEligibilityCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-disable": complete.PredictNothing,
			"-enable":  complete.PredictNothing,
			"-self":    complete.PredictNothing,
		})
}

func (c *NodeEligibilityCommand) AutocompleteArgs() complete.Predictor {
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

func (c *NodeEligibilityCommand) Name() string { return "node eligibility" }

func (c *NodeEligibilityCommand) Run(args []string) int {
	var enable, disable, self bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&enable, "enable", false, "Mark node as eligibile for scheduling")
	flags.BoolVar(&disable, "disable", false, "Mark node as ineligibile for scheduling")
	flags.BoolVar(&self, "self", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got either enable or disable, but not both.
	if (enable && disable) || (!enable && !disable) {
		c.Ui.Error("Either the '-enable' or '-disable' flag must be set")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Check that we got a node ID
	args = flags.Args()
	if l := len(args); self && l != 0 || !self && l != 1 {
		c.Ui.Error("Node ID must be specified if -self isn't being used")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// If -self flag is set then determine the current node.
	var nodeID string
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
		c.Ui.Error("Identifier must contain at least two characters.")
		return 1
	}

	nodeID = sanitizeUUIDPrefix(nodeID)
	nodes, _, err := client.Nodes().PrefixList(nodeID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error updating scheduling eligibility: %s", err))
		return 1
	}
	// Return error if no nodes are found
	if len(nodes) == 0 {
		c.Ui.Error(fmt.Sprintf("No node(s) with prefix or id %q found", nodeID))
		return 1
	}
	if len(nodes) > 1 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple nodes\n\n%s",
			formatNodeStubList(nodes, true)))
		return 1
	}

	// Prefix lookup matched a single node
	node, _, err := client.Nodes().Info(nodes[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error updating scheduling eligibility: %s", err))
		return 1
	}

	// Toggle node eligibility
	if _, err := client.Nodes().ToggleEligibility(node.ID, enable, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Error updating scheduling eligibility: %s", err))
		return 1
	}

	if enable {
		c.Ui.Output(fmt.Sprintf("Node %q scheduling eligibility set: eligible for scheduling", node.ID))
	} else {
		c.Ui.Output(fmt.Sprintf("Node %q scheduling eligibility set: ineligible for scheduling", node.ID))
	}
	return 0
}
