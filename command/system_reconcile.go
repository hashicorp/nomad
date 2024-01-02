// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type SystemReconcileCommand struct {
	Meta
}

func (s *SystemReconcileCommand) Help() string {
	helpText := `
Usage: nomad system reconcile <subcommand> [options]

  This command groups subcommands for interacting with the system reconcile API.

  Reconcile the summaries of all registered jobs:

      $ nomad system reconcile summaries

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (s *SystemReconcileCommand) Synopsis() string {
	return "Perform system reconciliation tasks"
}

func (s *SystemReconcileCommand) Name() string { return "system reconcile" }

func (s *SystemReconcileCommand) Run(args []string) int {
	return cli.RunResultHelp
}
