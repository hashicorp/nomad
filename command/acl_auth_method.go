package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

// Ensure ACLAuthMethodCommand satisfies the cli.Command interface.
var _ cli.Command = &ACLAuthMethodCommand{}

// ACLAuthMethodCommand implements cli.Command.
type ACLAuthMethodCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (a *ACLAuthMethodCommand) Help() string {
	helpText := `
Usage: nomad acl auth-method <subcommand> [options] [args]

  This command groups subcommands for interacting with ACL auth methods. 
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (a *ACLAuthMethodCommand) Synopsis() string { return "Interact with ACL auth methods" }

// Name returns the name of this command.
func (a *ACLAuthMethodCommand) Name() string { return "acl auth-method" }

// Run satisfies the cli.Command Run function.
func (a *ACLAuthMethodCommand) Run(_ []string) int { return cli.RunResultHelp }
