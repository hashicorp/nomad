// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type JobTagCommand struct {
	Meta
}

func (c *JobTagCommand) Name() string { return "job tag" }

func (c *JobTagCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *JobTagCommand) Synopsis() string {
	return "Manage job version tags"
}

func (c *JobTagCommand) Help() string {
	helpText := `
Usage: nomad job tag <subcommand> [options] [args]

  This command is used to manage tags for job versions. It has subcommands
  for applying and unsetting tags.

For more information on a specific subcommand, run:
  nomad job tag <subcommand> -h
`
	return strings.TrimSpace(helpText)
}
