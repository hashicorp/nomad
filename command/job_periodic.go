// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type JobPeriodicCommand struct {
	Meta
}

func (f *JobPeriodicCommand) Name() string { return "periodic" }

func (f *JobPeriodicCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (f *JobPeriodicCommand) Synopsis() string {
	return "Interact with periodic jobs"
}

func (f *JobPeriodicCommand) Help() string {
	helpText := `
Usage: nomad job periodic <subcommand> [options] [args]

  This command groups subcommands for interacting with periodic jobs.

  Force a periodic job:

      $ nomad job periodic force <job_id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}
