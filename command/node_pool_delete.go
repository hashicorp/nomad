// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type NodePoolDeleteCommand struct {
	Meta
}

func (c *NodePoolDeleteCommand) Name() string {
	return "node pool delete"
}

func (c *NodePoolDeleteCommand) Synopsis() string {
	return "Delete a node pool"
}

func (c *NodePoolDeleteCommand) Help() string {
	helpText := `
Usage: nomad node pool delete [options] <node-pool>

  Delete is used to remove a node pool.

  If ACLs are enabled, this command requires a token with the 'delete'
  capability in a 'node_pool' policy that matches the node pool being targeted.

  You cannot delete a node pool that has nodes or non-terminal jobs. In
  federated clusters, you cannot delete a node pool that has nodes or
  non-terminal jobs in any of the federated regions.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return strings.TrimSpace(helpText)
}

func (c *NodePoolDeleteCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *NodePoolDeleteCommand) AutocompleteArgs() complete.Predictor {
	return nodePoolPredictor(c.Client, set.From([]string{
		api.NodePoolAll,
		api.NodePoolDefault,
	}))
}

func (c *NodePoolDeleteCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we only have one argument.
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <node-pool>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	pool := args[0]

	// Make API equest.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.NodePools().Delete(pool, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deleting node pool: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deleted node pool %q!", pool))
	return 0
}
