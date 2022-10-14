package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type ContextDefaultCommand struct {
	Meta
}

func (c *ContextDefaultCommand) Help() string {
	helpText := `
Usage: nomad context default <subcommand> [args]

  This command groups subcommands for managing the default context. When
  configured, the default context is loaded on every execution of the Nomad
  CLI.

  Set the default context:

      $ nomad context default set <context-name>

  Unset the default context which results in no default context being
  available:

      $ nomad context default unset

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (c *ContextDefaultCommand) Synopsis() string { return "Manage the default context" }

func (c *ContextDefaultCommand) Name() string { return "context default" }

func (c *ContextDefaultCommand) Run(_ []string) int { return cli.RunResultHelp }
