// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/posener/complete"
)

type OperatorRaftMigrateCommand struct {
	Meta
}

func (c *OperatorRaftMigrateCommand) Help() string {
	helpText := `
Usage: nomad operator raft migrate-backend <path to nomad data dir>

  Migrate the raft log store from BoltDB to the WAL backend. The Nomad server
  must be stopped before running this command.

  The command copies all raft log entries and stable store keys from the
  existing raft.db (BoltDB) into a new WAL directory. On success the old
  raft.db file is renamed to raft.db.migrated.<timestamp> as a backup.

  If migration fails, any partially written WAL directory is automatically
  removed and the original raft.db file is left untouched. This allows you
  to safely retry the migration after addressing the failure cause.

  After migration completes, configure the server with:

      server {
        raft_logstore {
          backend = "wal"
        }
      }

  Then start the server.

  This command requires file system permissions to access the data directory on
  disk. The Nomad server locks access to the data directory, so this command
  cannot be run on a data directory that is being used by a running Nomad server.

Options:

  -yes
    Skip the confirmation prompt.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftMigrateCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-yes": complete.PredictNothing,
	}
}

func (c *OperatorRaftMigrateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictDirs("*")
}

func (c *OperatorRaftMigrateCommand) Synopsis() string {
	return "Migrate raft log store from BoltDB to WAL"
}

func (c *OperatorRaftMigrateCommand) Name() string { return "operator raft migrate-backend" }

func (c *OperatorRaftMigrateCommand) Run(args []string) int {
	var yes bool

	flags := c.Meta.FlagSet(c.Name(), 0)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&yes, "yes", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	raftDir, err := raftutil.FindRaftDir(args[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to locate raft directory: %v", err))
		return 1
	}

	if !yes {
		c.Ui.Output(fmt.Sprintf("This will migrate the raft log store in %s from BoltDB to WAL.", raftDir))
		c.Ui.Output("The Nomad server must be stopped. The old raft.db will be preserved as raft.db.migrated.<timestamp>.")
		c.Ui.Output("")

		confirm, err := c.Ui.Ask("Type 'yes' to confirm migration: ")
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read confirmation: %v", err))
			return 1
		}
		if confirm != "yes" {
			c.Ui.Output("Migration cancelled.")
			return 0
		}
	}

	// Buffer of 1 is sufficient since sendProgress() already handles slow
	// consumers by dropping messages with a non-blocking select.
	progress := make(chan string, 1)
	done := make(chan error, 1)

	go func() {
		done <- raftutil.MigrateToWAL(context.Background(), raftDir, progress)
	}()

	for msg := range progress {
		c.Ui.Output(msg)
	}

	if err := <-done; err != nil {
		c.Ui.Error(fmt.Sprintf("Migration failed: %v", err))
		return 1
	}

	c.Ui.Output("Migration succeeded. You may now start the server with backend = \"wal\".")
	return 0
}
