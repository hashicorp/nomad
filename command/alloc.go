// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type AllocCommand struct {
	Meta
}

func (f *AllocCommand) Help() string {
	helpText := `
Usage: nomad alloc <subcommand> [options] [args]

  This command groups subcommands for interacting with allocations. Users can
  inspect the status, examine the filesystem or logs of an allocation.

  Examine an allocations status:

      $ nomad alloc status <alloc-id>

  Stream a task's logs:

      $ nomad alloc logs -f <alloc-id> <task>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *AllocCommand) Synopsis() string {
	return "Interact with allocations"
}

func (f *AllocCommand) Name() string { return "alloc" }

func (f *AllocCommand) Run(args []string) int {
	return cli.RunResultHelp
}
