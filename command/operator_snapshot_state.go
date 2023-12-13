// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	flaghelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/hashicorp/nomad/nomad"
	"github.com/posener/complete"
)

type OperatorSnapshotStateCommand struct {
	Meta
}

func (c *OperatorSnapshotStateCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot state [options] <file>

  Displays a JSON representation of state in the snapshot.

  To inspect the file "backup.snap":

    $ nomad operator snapshot state backup.snap

Snapshot State Options:

  -filter
    Specifies an expression used to filter query results.

`
	return strings.TrimSpace(helpText)
}

func (c *OperatorSnapshotStateCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorSnapshotStateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSnapshotStateCommand) Synopsis() string {
	return "Displays information about a Nomad snapshot file"
}

func (c *OperatorSnapshotStateCommand) Name() string { return "operator snapshot state" }

func (c *OperatorSnapshotStateCommand) Run(args []string) int {
	var filterExpr flaghelper.StringFlag

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	flags.Var(&filterExpr, "filter", "")
	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	filter, err := nomad.NewFSMFilter(filterExpr.String())
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Invalid filter expression %q: %s", filterExpr, err))
		return 1
	}

	// Check that we either got no filename or exactly one.
	if len(flags.Args()) != 1 {
		c.Ui.Error("This command takes one argument: <file>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	path := flags.Args()[0]
	f, err := os.Open(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	state, meta, err := raftutil.RestoreFromArchive(f, filter)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to read archive file: %s", err))
		return 1
	}

	sm := raftutil.StateAsMap(state)
	sm["SnapshotMeta"] = []interface{}{meta}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sm); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to encode output: %v", err))
		return 1
	}

	return 0
}
