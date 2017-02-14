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

The Raft operator command is used to interact with Nomad's Raft subsystem. The
command can be used to verify Raft peers or in rare cases to recover quorum by
removing invalid peers.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftCommand) Synopsis() string {
	return "Provides access to the Raft subsystem"
}

func (c *OperatorRaftCommand) Run(args []string) int {
	return cli.RunResultHelp
}
