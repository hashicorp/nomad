package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type VolumeCommand struct {
	Meta
}

func (c *VolumeCommand) Help() string {
	helpText := `
Usage: nomad volume <subcommand> [options]

  volume groups commands that interact with volumes.

  Register a new volume or update an existing volume:

      $ nomad volume register <input>

  Examine the status of a volume:

      $ nomad volume status <id>

  Deregister an unused volume:

      $ nomad volume deregister <id>

  Detach an unused volume:

      $ nomad volume detach <vol id> <node id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *VolumeCommand) Name() string {
	return "volume"
}

func (c *VolumeCommand) Synopsis() string {
	return "Interact with volumes"
}

func (c *VolumeCommand) Run(args []string) int {
	return cli.RunResultHelp
}
