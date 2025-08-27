// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/hashicorp/cli"
)

type WindowsServiceCommand struct {
	Meta
}

func (c *WindowsServiceCommand) Help() string {
	helpText := `
Usage: nomad windows service <subcommand> [options]

  This command groups subcommands for managing Nomad as a system service on Windows.

  Install:

      $ nomad windows service install

  Uninstall:

      $ nomad windows service uninstall

  Refer to the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *WindowsServiceCommand) Name() string { return "windows service" }

func (c *WindowsServiceCommand) Synopsis() string {
	return "Manage nomad as a system service on Windows"
}

func (c *WindowsServiceCommand) Run(_ []string) int { return cli.RunResultHelp }
