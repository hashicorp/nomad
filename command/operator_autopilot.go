package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorAutopilotCommand struct {
	Meta
}

func (c *OperatorAutopilotCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *OperatorAutopilotCommand) Synopsis() string {
	return "Provides tools for modifying Autopilot configuration"
}

func (c *OperatorAutopilotCommand) Help() string {
	helpText := `
Usage: nomad operator autopilot <subcommand> [options]

  The Autopilot operator command is used to interact with Nomad's Autopilot
  subsystem. The command can be used to view or modify the current configuration.
`
	return strings.TrimSpace(helpText)
}
