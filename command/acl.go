package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type ACLCommand struct {
	Meta
}

func (f *ACLCommand) Help() string {
	helpText := `
Usage: nomad acl <subcommand> [options]

  Interact with ACL policies and tokens.

  Run nomad acl <subcommand> with no arguments for help on that subcommand.
`
	return strings.TrimSpace(helpText)
}

func (f *ACLCommand) Synopsis() string {
	return "Interact with ACL policies and tokens"
}

func (f *ACLCommand) Run(args []string) int {
	return cli.RunResultHelp
}
