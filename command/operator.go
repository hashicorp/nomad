// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorCommand struct {
	Meta
}

func (f *OperatorCommand) Help() string {
	helpText := `
Usage: nomad operator <subcommand> [options]

  Provides cluster-level tools for Nomad operators, such as interacting with
  the Raft subsystem. NOTE: Use this command with extreme caution, as improper
  use could lead to a Nomad outage and even loss of data.

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (f *OperatorCommand) Synopsis() string {
	return "Provides cluster-level tools for Nomad operators"
}

func (f *OperatorCommand) Name() string { return "operator" }

func (f *OperatorCommand) Run(args []string) int {
	return cli.RunResultHelp
}
