package command

import (
	"fmt"

	goversion "github.com/hashicorp/go-version"
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
	c.Ui.Output(c.Version.FullVersionNumber(true))

	// Check the server version
	client, err := c.Meta.Client()
	if err != nil {
		return 0
	}
	members, err := client.Agent().Members()
	if err != nil {
		return 0
	}

	// Return the latest version
	latestServerRevision := ""
	latestServerVersion, err := goversion.NewVersion("v0.0.0")
	if err != nil {
		return 0
	}

	for _, member := range members.Members {
		if version, ok := member.Tags["build"]; ok {
			semver, err := goversion.NewVersion(version)
			if err != nil {
				continue
			}

			if semver.GreaterThanOrEqual(latestServerVersion) {
				latestServerVersion = semver
				if revision, ok := member.Tags["revision"]; ok {
					latestServerRevision = revision
				} else {
					latestServerRevision = "unknown"
				}
			}
		}
	}

	if latestServerVersion != nil {
		c.Ui.Output(fmt.Sprintf("Latest Server Version: %s (%s)", latestServerVersion.String(), latestServerRevision))
	}

	return 0
}

func (c *VersionCommand) Synopsis() string {
	return "Prints the Nomad version"
}
