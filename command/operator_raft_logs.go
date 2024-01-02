// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/posener/complete"
)

type OperatorRaftLogsCommand struct {
	Meta
}

func (c *OperatorRaftLogsCommand) Help() string {
	helpText := `
Usage: nomad operator raft logs <path to nomad data dir>

  Display the log entries persisted in the Nomad data directory in JSON
  format.

  This command requires file system permissions to access the data directory on
  disk. The Nomad server locks access to the data directory, so this command
  cannot be run on a data directory that is being used by a running Nomad server.

  This is a low-level debugging tool and not subject to Nomad's usual backward
  compatibility guarantees.

Raft Logs Options:

  -pretty
    By default this command outputs newline delimited JSON. If the -pretty flag
    is passed, each entry will be pretty-printed.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftLogsCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorRaftLogsCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRaftLogsCommand) Synopsis() string {
	return "Display raft log content"
}

func (c *OperatorRaftLogsCommand) Name() string { return "operator raft logs" }

func (c *OperatorRaftLogsCommand) Run(args []string) int {

	var pretty bool
	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	flagSet.BoolVar(&pretty, "pretty", false, "")

	if err := flagSet.Parse(args); err != nil {
		return 1
	}

	args = flagSet.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	raftPath, err := raftutil.FindRaftFile(args[0])
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}

	logChan, warningsChan, err := raftutil.LogEntries(raftPath)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// so that the warnings don't end up mixed into the JSON stream,
	// collect them and print them once we're done
	warnings := []error{}

DONE:
	for {
		select {
		case log := <-logChan:
			if log == nil {
				break DONE // no more logs, but break to print warnings
			}
			if err := enc.Encode(log); err != nil {
				c.Ui.Error(fmt.Sprintf("failed to encode output: %v", err))
				return 1
			}
		case warning := <-warningsChan:
			warnings = append(warnings, warning)
		}
	}

	for _, warning := range warnings {
		c.Ui.Error(warning.Error())
	}

	return 0
}
