package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type SystemdCommand struct {
	Meta
}

func (c *SystemdCommand) Help() string {
	helpText := `
Usage: nomad systemd <subcommand> [options] [args]

  This command groups subcommands for interacting with systemd.

  Users can create, inspect, and delete unit files derived from job
  specifications, without scheduling them with Nomad. Please see the individual
  subcommand help for detailed usage information.

  This is a terrible idea and no one should use it, of course.
`
	return strings.TrimSpace(helpText)
}

func (c *SystemdCommand) Synopsis() string {
	return "Interact with systemd directly"
}

func (c *SystemdCommand) Name() string { return "systemd" }

func (c *SystemdCommand) Run(args []string) int {
	return cli.RunResultHelp
}
