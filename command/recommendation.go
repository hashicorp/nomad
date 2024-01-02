// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

// Ensure RecommendationCommand satisfies the cli.Command interface.
var _ cli.Command = &RecommendationCommand{}

// RecommendationCommand implements cli.Command.
type RecommendationCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (r *RecommendationCommand) Help() string {
	helpText := `
Usage: nomad recommendation <subcommand> [options]

  This command groups subcommands for interacting with the recommendation API.

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (r *RecommendationCommand) Synopsis() string {
	return "Interact with the Nomad recommendation endpoint"
}

// Name returns the name of this command.
func (r *RecommendationCommand) Name() string { return "recommendation" }

// Run satisfies the cli.Command Run function.
func (r *RecommendationCommand) Run(_ []string) int { return cli.RunResultHelp }
