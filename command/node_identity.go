// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/hashicorp/cli"
)

type NodeIdentityCommand struct {
	Meta
}

func (n *NodeIdentityCommand) Help() string {
	helpText := `
Usage: nomad node identity [subcommand]

  Interact with a node's identity. All commands interact directly with a client
  and require setting the target node via its 36 character ID.

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (n *NodeIdentityCommand) Synopsis() string { return "Force renewal of a nodes identity" }

func (n *NodeIdentityCommand) Name() string { return "node identity" }

func (n *NodeIdentityCommand) Run(_ []string) int {
	return cli.RunResultHelp
}
