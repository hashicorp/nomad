// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/hashicorp/cli"
)

type WindowsCommand struct {
	Meta
}

func (c *WindowsCommand) Help() string {
	helpText := `
Usage: nomad windows <subcommand> [options]

  This command groups subcommands for managing nomad as a system service on Windows.

  Service::

      $ nomad windows service

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (c *WindowsCommand) Name() string { return "windows" }

func (c *WindowsCommand) Synopsis() string { return "Manage nomad as a system service on Windows" }

func (c *WindowsCommand) Run(_ []string) int { return cli.RunResultHelp }
