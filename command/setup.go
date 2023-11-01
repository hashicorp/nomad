// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type SetupCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (c *SetupCommand) Help() string {
	helpText := `
Usage: nomad setup <subcommand> [options] [args]

  This command groups helper subcommands used for setting up external tools.

  Setup Consul for Nomad:

      $ nomad setup consul -y

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (c *SetupCommand) Synopsis() string { return "Interact with setup helpers" }

// Name returns the name of this command.
func (c *SetupCommand) Name() string { return "setup" }

// Run satisfies the cli.Command Run function.
func (c *SetupCommand) Run(_ []string) int { return cli.RunResultHelp }
