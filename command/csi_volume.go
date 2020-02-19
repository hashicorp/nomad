package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type CSIVolumeCommand struct {
	Meta
}

func (c *CSIVolumeCommand) Help() string {
	helpText := `
Usage: nomad csi volume <subcommand> [options]

  csi volume groups all the CSI (Container Storage Interface) commands that
  operate on volumes. CSI Volumes must be created in the remote provider before
  the volume is available to a nomad task.

  Register a new volume or update an existing volume:

      $ nomad csi volume register <input>

  Examine the status of a volume:

      $ nomad csi volume status <id>

  List existing volumes:

      $ nomad csi volume list

  Deregister a volume, removing it from the system:

      $ nomad csi volume deregister <id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *CSIVolumeCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *CSIVolumeCommand) Synopsis() string {
	return "Interact with CSI volumes"
}
