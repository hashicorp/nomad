// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type OperatorSnapshotRestoreCommand struct {
	Meta
}

func (c *OperatorSnapshotRestoreCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot restore [options] <file>

  Restores an atomic, point-in-time snapshot of the state of the Nomad servers
  which includes jobs, nodes, allocations, periodic jobs, and ACLs.

  Restores involve a potentially dangerous low-level Raft operation that is not
  designed to handle server failures during a restore. This command is primarily
  intended to be used when recovering from a disaster, restoring into a fresh
  cluster of Nomad servers.

  If ACLs are enabled, a management token must be supplied in order to perform
  snapshot operations.

  To restore a snapshot from the file "backup.snap":

    $ nomad operator snapshot restore backup.snap

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace)
	return strings.TrimSpace(helpText)
}

func (c *OperatorSnapshotRestoreCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *OperatorSnapshotRestoreCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSnapshotRestoreCommand) Synopsis() string {
	return "Restore snapshot of Nomad server state"
}

func (c *OperatorSnapshotRestoreCommand) Name() string { return "operator snapshot restore" }

func (c *OperatorSnapshotRestoreCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Check for misuse
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	snap, err := os.Open(args[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening snapshot file: %q", err))
		return 1
	}
	defer snap.Close()

	// Set up a client.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Call snapshot restore API with backup file.
	_, err = client.Operator().SnapshotRestore(snap, &api.WriteOptions{})
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to get restore snapshot: %v", err))
		return 1
	}

	c.Ui.Output("Snapshot Restored")
	return 0
}
