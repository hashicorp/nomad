// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/posener/complete"
)

type OperatorRaftInfoCommand struct {
	Meta
}

func (c *OperatorRaftInfoCommand) Help() string {
	helpText := `
Usage: nomad operator raft info <path to nomad data dir>

  Displays summary information about the raft logs in the data directory.

  This command requires file system permissions to access the data directory on
  disk. The Nomad server locks access to the data directory, so this command
  cannot be run on a data directory that is being used by a running Nomad server.

  This is a low-level debugging tool and not subject to Nomad's usual backward
  compatibility guarantees.
`
	return strings.TrimSpace(helpText)
}

func (c *OperatorRaftInfoCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorRaftInfoCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorRaftInfoCommand) Synopsis() string {
	return "Display info of the raft log"
}

func (c *OperatorRaftInfoCommand) Name() string { return "operator raft info" }

func (c *OperatorRaftInfoCommand) Run(args []string) int {
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))

		return 1
	}

	// Find raft.db
	raftPath, err := raftutil.FindRaftFile(args[0])
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	store, firstIdx, lastIdx, err := raftutil.RaftStateInfo(raftPath)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("failed to open raft logs: %v", err))
		return 1
	}
	defer store.Close()

	c.Ui.Output(fmt.Sprintf("path:        %v", raftPath))
	c.Ui.Output(fmt.Sprintf("length:      %v", lastIdx-firstIdx+1))
	c.Ui.Output(fmt.Sprintf("first index: %v", firstIdx))
	c.Ui.Output(fmt.Sprintf("last index:  %v", lastIdx))

	return 0
}
