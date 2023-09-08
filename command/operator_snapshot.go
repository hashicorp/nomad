// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type OperatorSnapshotCommand struct {
	Meta
}

func (f *OperatorSnapshotCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot <subcommand> [options]

  This command has subcommands for saving and inspecting the state
  of the Nomad servers for disaster recovery. These are atomic, point-in-time
  snapshots which include jobs, nodes, allocations, periodic jobs, and ACLs.

  If ACLs are enabled, a management token must be supplied in order to perform
  snapshot operations.

  Create a snapshot:

      $ nomad operator snapshot save backup.snap

  Inspect a snapshot:

      $ nomad operator snapshot inspect backup.snap

  Run a daemon process that locally saves a snapshot every hour (available only in
  Nomad Enterprise) :

      $ nomad operator snapshot agent

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}

func (f *OperatorSnapshotCommand) Synopsis() string {
	return "Saves and inspects snapshots of Nomad server state"
}

func (f *OperatorSnapshotCommand) Name() string { return "operator snapshot" }

func (f *OperatorSnapshotCommand) Run(args []string) int {
	return cli.RunResultHelp
}
