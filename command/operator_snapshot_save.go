// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type OperatorSnapshotSaveCommand struct {
	Meta
}

func (c *OperatorSnapshotSaveCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot save [options] <file>

  Retrieves an atomic, point-in-time snapshot of the state of the Nomad servers
  which includes jobs, nodes, allocations, periodic jobs, and ACLs.

  If ACLs are enabled, a management token must be supplied in order to perform
  snapshot operations.

  To create a snapshot from the leader server and save it to "backup.snap":

    $ nomad snapshot save backup.snap

  To create a potentially stale snapshot from any available server (useful if no
  leader is available):

    $ nomad snapshot save -stale backup.snap

  This is useful for situations where a cluster is in a degraded state and no
  leader is available. To target a specific server for a snapshot, you can run
  the 'nomad operator snapshot save' command on that specific server.


General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Snapshot Save Options:

  -stale=[true|false]
    The -stale argument defaults to "false" which means the leader provides the
    result. If the cluster is in an outage state without a leader, you may need
    to set -stale to "true" to get the configuration from a non-leader server.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorSnapshotSaveCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-stale": complete.PredictAnything,
		})
}

func (c *OperatorSnapshotSaveCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSnapshotSaveCommand) Synopsis() string {
	return "Saves snapshot of Nomad server state"
}

func (c *OperatorSnapshotSaveCommand) Name() string { return "operator snapshot save" }

func (c *OperatorSnapshotSaveCommand) Run(args []string) int {
	var stale bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.BoolVar(&stale, "stale", false, "")
	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Check for misuse
	// Check that we either got no filename or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	now := time.Now()
	filename := fmt.Sprintf("nomad-state-%04d%02d%0d-%d.snap", now.Year(), now.Month(), now.Day(), now.Unix())

	if len(args) == 1 {
		filename = args[0]
	}

	if _, err := os.Lstat(filename); err == nil {
		c.Ui.Error(fmt.Sprintf("Destination file already exists: %q", filename))
		c.Ui.Error(commandErrorText(c))
		return 1
	} else if !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Unexpected failure checking %q: %v", filename, err))
		return 1
	}

	// Set up a client.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	tmpFile, err := os.Create(filename + ".tmp")
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create file: %v", err))
		return 1
	}

	// Fetch the current configuration.
	q := &api.QueryOptions{
		AllowStale: stale,
	}
	snapIn, err := client.Operator().Snapshot(q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to get snapshot file: %v", err))
		return 1
	}

	defer snapIn.Close()

	_, err = io.Copy(tmpFile, snapIn)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Filed to download snapshot file: %v", err))
		return 1
	}

	err = os.Rename(tmpFile.Name(), filename)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Filed to finalize snapshot file: %v", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("State file written to %v", filename))
	return 0
}
