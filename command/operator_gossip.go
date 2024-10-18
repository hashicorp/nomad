// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorGossipCommand struct {
	Meta
}

func (*OperatorGossipCommand) Help() string {
	helpText := `
Usage: nomad operator gossip <subcommand> [options] [args]
	
  This command is accessed by using one of the subcommands below.
`
	return strings.TrimSpace(helpText)
}

func (*OperatorGossipCommand) Synopsis() string {
	return "Provides access to the Gossip protocol"
}

func (f *OperatorGossipCommand) Name() string { return "operator gossip" }

func (f *OperatorGossipCommand) Run(_ []string) int {
	return cli.RunResultHelp
}
