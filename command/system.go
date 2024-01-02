// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type SystemCommand struct {
	Meta
}

func (sc *SystemCommand) Help() string {
	helpText := `
Usage: nomad system <subcommand> [options]

  This command groups subcommands for interacting with the system API. Users
  can perform system maintenance tasks such as trigger the garbage collector or
  perform job summary reconciliation.

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (sc *SystemCommand) Synopsis() string {
	return "Interact with the system API"
}

func (sc *SystemCommand) Name() string { return "system" }

func (sc *SystemCommand) Run(args []string) int {
	return cli.RunResultHelp
}
