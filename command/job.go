// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type JobCommand struct {
	Meta
}

func (f *JobCommand) Help() string {
	helpText := `
Usage: nomad job <subcommand> [options] [args]

  This command groups subcommands for interacting with jobs.

  Run a new job or update an existing job:

      $ nomad job run <path>

  Plan the run of a job to determine what changes would occur:

      $ nomad job plan <path>

  Stop a running job:

      $ nomad job stop <name>

  Examine the status of a running job:

      $ nomad job status <name>

  Please see the individual subcommand help for detailed usage information.
`

	return strings.TrimSpace(helpText)
}

func (f *JobCommand) Synopsis() string {
	return "Interact with jobs"
}

func (f *JobCommand) Name() string { return "job" }

func (f *JobCommand) Run(args []string) int {
	return cli.RunResultHelp
}
