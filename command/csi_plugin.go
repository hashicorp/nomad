package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type CSIPluginCommand struct {
	Meta
}

func (c *CSIPluginCommand) Help() string {
	helpText := `
Usage: nomad csi volume <subcommand> [options]

  csi plugin groups all the CSI (Container Storage Interface) commands that
  operate on plugins. CSI Plugins are configured in jobs, these commands only
  report current status.

  Examine the status of a plugin:

      $ nomad csi plugin status <id>

  List existing plugins:

      $ nomad csi plugin list

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *CSIPluginCommand) Run(args []string) int {
	return cli.RunResultHelp
}
