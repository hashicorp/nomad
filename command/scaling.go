// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

// Ensure ScalingCommand satisfies the cli.Command interface.
var _ cli.Command = &ScalingCommand{}

// ScalingCommand implements cli.Command.
type ScalingCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (s *ScalingCommand) Help() string {
	helpText := `
Usage: nomad scaling <subcommand> [options]

  This command groups subcommands for interacting with the scaling API.

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (s *ScalingCommand) Synopsis() string {
	return "Interact with the Nomad scaling endpoint"
}

// Name returns the name of this command.
func (s *ScalingCommand) Name() string { return "scaling" }

// Run satisfies the cli.Command Run function.
func (s *ScalingCommand) Run(_ []string) int { return cli.RunResultHelp }
