// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type NodeMetaCommand struct {
	Meta
}

func (c *NodeMetaCommand) Help() string {
	helpText := `
Usage: nomad node meta [subcommand]

	Interact with a node's metadata. The apply subcommand allows for dynamically
	updating node metadata. The read subcommand allows reading all of the
	metadata set on the client. All commands interact directly with a client and
	allow setting a custom target with the -node-id option.

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *NodeMetaCommand) Synopsis() string {
	return "Interact with node metadata"
}

func (c *NodeMetaCommand) Name() string { return "node meta" }

func (c *NodeMetaCommand) Run(args []string) int {
	return cli.RunResultHelp
}
