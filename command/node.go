package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type NodeCommand struct {
	Meta
}

func (f *NodeCommand) Help() string {
	helpText := `
Usage: nomad node <subcommand> [options] [args]

  This command groups subcommands for interacting with nodes. Nodes in Nomad are
  agent's that can run submitted workloads. This command can be used to examine
  nodes and operate on nodes, such as draining workloads off of them.

  Examine the status of a node:

      $ nomad node status <node-id>

  Mark a node as ineligible for running workloads. This is useful when the node
  is expected to be removed or upgraded so new allocations aren't placed on it:

      $ nomad node eligibility -disabled <node-id>

  Mark a node to be drained, allowing batch jobs four hours to finished before
  forcing them off the node:

      $ nomad node drain -enable -deadline 4h <node-id>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *NodeCommand) Synopsis() string {
	return "Interact with nodes"
}

func (f *NodeCommand) Name() string { return "node" }

func (f *NodeCommand) Run(args []string) int {
	return cli.RunResultHelp
}
