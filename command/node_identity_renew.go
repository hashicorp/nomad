// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NodeIdentityRenewCommand struct {
	Meta
}

func (n *NodeIdentityRenewCommand) Help() string {
	helpText := `
Usage: nomad node identity renew [options] <node_id>

  Instruct a node to renew its identity at the next heartbeat. This command only
  applies to client agents.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)

	return strings.TrimSpace(helpText)
}

func (n *NodeIdentityRenewCommand) Synopsis() string { return "Force a node to renew its identity" }

func (n *NodeIdentityRenewCommand) Name() string { return "node identity renew" }

func (n *NodeIdentityRenewCommand) Run(args []string) int {

	flags := n.Meta.FlagSet(n.Name(), FlagSetClient)
	flags.Usage = func() { n.Ui.Output(n.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if len(args) != 1 {
		n.Ui.Error("This command takes one argument: <node_id>")
		n.Ui.Error(commandErrorText(n))
		return 1
	}

	// Get the HTTP client
	client, err := n.Meta.Client()
	if err != nil {
		n.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	nodeID := args[0]

	// Lookup nodeID
	if nodeID != "" {
		nodeID, err = lookupNodeID(client.Nodes(), nodeID)
		if err != nil {
			n.Ui.Error(err.Error())
			return 1
		}
	}

	req := api.NodeIdentityRenewRequest{
		NodeID: nodeID,
	}

	if _, err := client.Nodes().Identity().Renew(&req, nil); err != nil {
		n.Ui.Error(fmt.Sprintf("Error requesting node identity renewal: %s", err))
		return 1
	}

	return 0
}

func (n *NodeIdentityRenewCommand) AutocompleteFlags() complete.Flags {
	return n.Meta.AutocompleteFlags(FlagSetClient)
}

func (n *NodeIdentityRenewCommand) AutocompleteArgs() complete.Predictor {
	return nodePredictor(n.Client, nil)
}
