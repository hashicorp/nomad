package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorRaftCommand struct {
	Meta
}

func (c *OperatorRaftCommand) Help() string {
	helpText := `
Usage: nomad operator raft <subcommand> [options]

  This command groups subcommands for interacting with Nomad's Raft subsystem.
  The command can be used to verify Raft peers or in rare cases to recover
  quorum by removing invalid peers.

  List Raft peers:

      $ nomad operator raft list-peers

  Remove a Raft peer:

      $ nomad operator raft remove-peer -peer-address "IP:Port"

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftCommand) Synopsis() string {
	return "Provides access to the Raft subsystem"
}

func (c *OperatorRaftCommand) Name() string { return "operator raft" }

func (c *OperatorRaftCommand) Run(args []string) int {
	return cli.RunResultHelp
}
