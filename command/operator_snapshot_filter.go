// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/hashicorp/nomad/nomad"
	"github.com/posener/complete"
)

type OperatorSnapshotFilterCommand struct {
	Meta
}

func (c *OperatorSnapshotFilterCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot filter [options] <file>

  Removes selected entries from an existing snapshot file created by the
  operator snapshot save command. This is useful for situations where you need
  to remove objects from a snapshot you want to restore to a cluster, or to
  remove a large amount of pending evals when recovering a cluster from an
  outage. This command edits the snapshot file in place.

Snapshot Filter Options:

  -include
    Specifies a filter expression used to process the snapshot. Only objects
    that match the -include filter will be included. If this is combined with
    -exclude-types, an object must match the -include expression and not be one
    of the -exclude-types to appear in the resulting snapshot.

  -exclude-enterprise
    Specifies removing all Nomad Enterprise objects from the snapshot.
    (Requires a Nomad Enterprise binary.)

  -exclude-types
    Specifies a list of comma-separated numeric internal type IDs to exclude
    from the snapshot. You can find SnapshotType IDs in the Nomad source code at
    nomad/fsm.go. If this is combined with -exclude-types, an object must match
    the -include expression and not be one of the -exclude-types to appear in
    the resulting snapshot.

  `

	return strings.TrimSpace(helpText)
}

func (c *OperatorSnapshotFilterCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-include":            complete.PredictAnything,
		"-exclude-enterprise": complete.PredictNothing,
		"-exclude-types":      complete.PredictAnything,
	}
}

func (c *OperatorSnapshotFilterCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*")
}

func (c *OperatorSnapshotFilterCommand) Synopsis() string {
	return "Filters an existing snapshot of Nomad server state"
}

func (c *OperatorSnapshotFilterCommand) Name() string { return "operator snapshot filter" }

func (c *OperatorSnapshotFilterCommand) Run(args []string) int {
	var includeExpr string
	var excludeEnt bool
	var excludeTypesStr string

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	flagSet.StringVar(&includeExpr, "include", "", "filter expresion of objects to include")
	flagSet.BoolVar(&excludeEnt, "exclude-enterprise", false, "remove all Enterprise objects")
	flagSet.StringVar(&excludeTypesStr, "exclude-types", "", "list of type IDs to exclude")

	if err := flagSet.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	excludedTypes := []nomad.SnapshotType{}
	if excludeTypesStr != "" {
		for part := range strings.SplitSeq(excludeTypesStr, ",") {
			i, err := strconv.Atoi(part)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("Could not convert %q into a numeric type ID: %v", part, err))
				return 1
			}
			if i > 255 {
				c.Ui.Error(fmt.Sprintf("Invalid numeric type ID %s", part))
				return 1
			}
			excludedTypes = append(excludedTypes, nomad.SnapshotType(i))
		}
	}
	if excludeEnt {
		entTypes, err := c.getEntTypes()
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
		excludedTypes = append(excludedTypes, entTypes...)
	}

	args = flagSet.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <file>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	path := args[0]
	f, err := os.Open(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error opening snapshot file: %s", err))
		return 1
	}
	defer f.Close()

	tmpFile, err := os.Create(path + ".tmp")
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create temporary file: %v", err))
		return 1
	}

	_, err = io.Copy(tmpFile, f)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to copy snapshot to temporary file: %v", err))
		return 1
	}

	filter, err := nomad.NewFSMFilter(includeExpr, excludedTypes)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to create snapshot filter expression: %v", err))
		return 1
	}

	err = raftutil.FilterSnapshot(tmpFile, filter)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to redact snapshot: %v", err))
		return 1
	}

	err = os.Rename(tmpFile.Name(), path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to finalize snapshot file: %v", err))
		return 1
	}

	c.Ui.Output("Snapshot filtered")
	return 0
}
