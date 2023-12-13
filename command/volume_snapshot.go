// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"

	"github.com/mitchellh/cli"
)

type VolumeSnapshotCommand struct {
	Meta
}

func (f *VolumeSnapshotCommand) Name() string { return "snapshot" }

func (f *VolumeSnapshotCommand) Run(args []string) int {
	return cli.RunResultHelp
}

func (f *VolumeSnapshotCommand) Synopsis() string {
	return "Interact with volume snapshots"
}

func (f *VolumeSnapshotCommand) Help() string {
	helpText := `
Usage: nomad volume snapshot <subcommand> [options] [args]

  This command groups subcommands for interacting with CSI volume snapshots.

  Create a snapshot of an external storage volume:

      $ nomad volume snapshot create <volume id>

  Display a list of CSI volume snapshots along with their
  source volume ID as known to the external storage provider.

      $ nomad volume snapshot list -plugin <plugin id>

  Delete a snapshot of an external storage volume:

      $ nomad volume snapshot delete <snapshot id>

  Please see the individual subcommand help for detailed usage information.
`
	return strings.TrimSpace(helpText)
}
