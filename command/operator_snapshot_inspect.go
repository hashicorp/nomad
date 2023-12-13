// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/posener/complete"
)

type OperatorSnapshotInspectCommand struct {
	Meta
}

func (c *OperatorSnapshotInspectCommand) Help() string {
	helpText := `
Usage: nomad operator snapshot inspect [options] <file>

  Displays information about a snapshot file on disk.

  To inspect the file "backup.snap":
    $ nomad operator snapshot inspect backup.snap
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorSnapshotInspectCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorSnapshotInspectCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorSnapshotInspectCommand) Synopsis() string {
	return "Displays information about a Nomad snapshot file"
}

func (c *OperatorSnapshotInspectCommand) Name() string { return "operator snapshot inspect" }

func (c *OperatorSnapshotInspectCommand) Run(args []string) int {
	// Check that we either got no filename or exactly one.
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <filename>")
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

	meta, err := snapshot.Verify(f)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error verifying snapshot: %s", err))
		return 1
	}

	output := []string{
		fmt.Sprintf("ID|%s", meta.ID),
		fmt.Sprintf("Size|%d", meta.Size),
		fmt.Sprintf("Index|%d", meta.Index),
		fmt.Sprintf("Term|%d", meta.Term),
		fmt.Sprintf("Version|%d", meta.Version),
	}

	c.Ui.Output(formatList(output))
	return 0
}
