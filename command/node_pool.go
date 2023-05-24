// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type NodePoolCommand struct {
	Meta
}

func (c *NodePoolCommand) Name() string {
	return "node pool"
}

func (c *NodePoolCommand) Synopsis() string {
	return "Interact with node pools"
}

func (c *NodePoolCommand) Help() string {
	helpText := `
Usage: nomad node pool <subcommand> [options] [args]

  This command groups subcommands for interacting with node pools. Node pools
  are used to partition and control access to a group of nodes. This command
  can be used to create, update, list, and delete node pools.

  Create or update a node pool:

    $ nomad node pool apply <path>

  List all node pools:

    $ nomad node pool list

  Delete a node pool:

    $ nomad node pool delete <name>

  Please refer to individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *NodePoolCommand) Run(args []string) int {
	return cli.RunResultHelp
}
