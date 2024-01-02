// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorAutopilotCommand struct {
	Meta
}

func (c *OperatorAutopilotCommand) Name() string { return "operator autopilot" }

func (c *OperatorAutopilotCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *OperatorAutopilotCommand) Synopsis() string {
	return "Provides tools for modifying Autopilot configuration"
}

func (c *OperatorAutopilotCommand) Help() string {
	helpText := `
Usage: nomad operator autopilot <subcommand> [options]

  This command groups subcommands for interacting with Nomad's Autopilot
  subsystem. Autopilot provides automatic, operator-friendly management of Nomad
  servers. The command can be used to view or modify the current Autopilot
  configuration. For a full guide see: https://www.nomadproject.io/guides/autopilot.html

  Get the current Autopilot configuration:

      $ nomad operator autopilot get-config

  Set a new Autopilot configuration, enabling automatic dead server cleanup:

      $ nomad operator autopilot set-config -cleanup-dead-servers=true

  Please see the individual subcommand help for detailed usage information.
  `
	return strings.TrimSpace(helpText)
}
