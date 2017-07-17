package command

import (
	"github.com/mitchellh/cli"
)

// VersionCommand is a Command implementation prints the version.
type VersionCommand struct {
	Version string
	Ui      cli.Ui
}

func (c *VersionCommand) Help() string {
	return ""
}

func (c *VersionCommand) Run(_ []string) int {
	c.Ui.Output(c.Version)
	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Nomad version"
}
