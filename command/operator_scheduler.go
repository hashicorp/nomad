// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

// Ensure OperatorSchedulerCommand satisfies the cli.Command interface.
var _ cli.Command = &OperatorSchedulerCommand{}

type OperatorSchedulerCommand struct {
	Meta
}

func (o *OperatorSchedulerCommand) Help() string {
	helpText := `
Usage: nomad operator scheduler <subcommand> [options]

  This command groups subcommands for interacting with Nomad's scheduler
  subsystem.

  Get the scheduler configuration:

      $ nomad operator scheduler get-config

  Set the scheduler to use the spread algorithm:

      $ nomad operator scheduler set-config -scheduler-algorithm=spread

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (o *OperatorSchedulerCommand) Synopsis() string {
	return "Provides access to the scheduler configuration"
}

func (o *OperatorSchedulerCommand) Name() string { return "operator scheduler" }

func (o *OperatorSchedulerCommand) Run(_ []string) int { return cli.RunResultHelp }
