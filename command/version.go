package command

import (
	"fmt"

	"github.com/hashicorp/nomad/version"
	"github.com/mitchellh/cli"
)

// VersionCommand is a Command implementation prints the version.
type VersionCommand struct {
	Meta
	Version *version.VersionInfo
	Ui      cli.Ui
}

func (c *VersionCommand) Help() string {
	return ""
}

func (c *VersionCommand) Name() string { return "version" }

func (c *VersionCommand) Run(_ []string) int {
	c.Ui.Output(fmt.Sprintf("Client Version: %s", c.Version.FullVersionNumber(true)))

	// Check the server version
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not get server version: %s", err))
		return 1
	}
	serverVersion, err := client.Status().Version()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not get server version: %s", err))
		return 1
	}
	c.Ui.Output(fmt.Sprintf("Server Version: %s", *serverVersion))
	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Nomad version"
}
