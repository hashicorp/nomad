// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	flaghelper "github.com/hashicorp/nomad/helper/flags"

	"github.com/posener/complete"
)

type NodeStopCommand struct {
	Meta
}

func (c *NodeStopCommand) Help() string {
	helpText := `
`
	return strings.TrimSpace(helpText)
}

func (c *NodeStopCommand) Synopsis() string {
	return "Toggle drain mode on a given node"
}

func (c *NodeStopCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-meta": complete.PredictNothing,
			"-self": complete.PredictNothing,
			"-yes":  complete.PredictNothing,
		})
}

func (c *NodeStopCommand) AutocompleteArgs() complete.Predictor {
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

func (c *NodeStopCommand) Name() string { return "node drain" }

func (c *NodeStopCommand) Run(args []string) int {
	var self, autoYes bool
	var metaVars flaghelper.StringFlag

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&self, "self", false, "")
	flags.BoolVar(&autoYes, "yes", false, "Automatic yes to prompts.")
	flags.Var(&metaVars, "meta", "Drain metadata")

	if err := flags.Parse(args); err != nil {
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
		c.Ui.Error(fmt.Sprintf("Error stopping disconnected node: %s", err))
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
		c.Ui.Error(fmt.Sprintf("Error stopping disconnected node: %s", err))
		return 1
	}

	if node.Status != api.NodeStatusDisconnected {
		c.Ui.Error(fmt.Sprintf("Node %s is not disconnected, only disconnected nodes can be stopped", nodeID))
		return 1
	}

	// Confirm drain if the node was a prefix match.
	if nodeID != node.ID && !autoYes {
		question := fmt.Sprintf("Are you sure you want to stop node %q? [y/N]", node.ID)
		answer, err := c.Ui.Ask(question)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to parse answer: %v", err))
			return 1
		}

		if answer == "" || strings.ToLower(answer)[0] == 'n' {
			// No case
			c.Ui.Output("Canceling drain toggle")
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

	var spec *api.DrainSpec

	spec = &api.DrainSpec{
		Deadline:         -1 * time.Second,
		IgnoreSystemJobs: false,
	}

	// propagate drain metadata if cancelling
	drainMeta := make(map[string]string)

	for _, m := range metaVars {
		if len(m) == 0 {
			continue
		}
		kv := strings.SplitN(m, "=", 2)
		if len(kv) == 2 {
			drainMeta[kv[0]] = kv[1]
		} else {
			drainMeta[kv[0]] = ""
		}
	}

	// Toggle node draining
	drainResponse, err := client.Nodes().UpdateDrainOpts(node.ID,
		&api.DrainOptions{
			DrainSpec:    spec,
			MarkEligible: true,
			Meta:         drainMeta,
		}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error updating stop specification: %s", err))
		return 1
	}

	now := time.Now()
	c.Ui.Info(fmt.Sprintf("%s: Ctrl-C to stop monitoring: will not cancel the node stop", formatTime(now)))
	c.monitorDrain(client, context.Background(), node, drainResponse.LastIndex)

	// Toggle node eligibility
	if _, err := client.Nodes().ToggleEligibility(node.ID, true, nil); err != nil {
		c.Ui.Error(fmt.Sprintf("Error updating scheduling eligibility: %s", err))
		return 1
	}

	return 0
}

func (c *NodeStopCommand) monitorDrain(client *api.Client, ctx context.Context, node *api.Node, index uint64) {
	outCh := client.Nodes().MonitorDrain(ctx, node.ID, index, false)
	for msg := range outCh {
		switch msg.Level {
		case api.MonitorMsgLevelInfo:
			c.Ui.Info(fmt.Sprintf("%s: %s", formatTime(time.Now()), msg))
		case api.MonitorMsgLevelWarn:
			c.Ui.Warn(fmt.Sprintf("%s: %s", formatTime(time.Now()), msg))
		case api.MonitorMsgLevelError:
			c.Ui.Error(fmt.Sprintf("%s: %s", formatTime(time.Now()), msg))
		default:
			c.Ui.Output(fmt.Sprintf("%s: %s", formatTime(time.Now()), msg))
		}
	}
}
