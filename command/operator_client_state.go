// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
	trstate "github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/client/state"
	"github.com/posener/complete"
)

type OperatorClientStateCommand struct {
	Meta
}

func (c *OperatorClientStateCommand) Help() string {
	helpText := `
Usage: nomad operator client-state <path_to_nomad_dir>

  Emits a representation of the stored client state in JSON format.
`
	return strings.TrimSpace(helpText)
}
func (c *OperatorClientStateCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{}
}

func (c *OperatorClientStateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorClientStateCommand) Synopsis() string {
	return "Dump the nomad client state"
}
func (c *OperatorClientStateCommand) Name() string { return "operator client-state" }

func (c *OperatorClientStateCommand) Run(args []string) int {
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <nomad-data-dir>")
		c.Ui.Error(commandErrorText(c))

		return 1
	}

	logger := hclog.L()
	db, err := state.NewBoltStateDB(logger, args[0])
	if err != nil {
		c.Ui.Error(fmt.Sprintf("failed to open client state: %v", err))
		return 1
	}
	defer db.Close()

	allocs, _, err := db.GetAllAllocations()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("failed to get allocations: %v", err))
		return 1
	}

	data := map[string]*clientStateAlloc{}
	for _, alloc := range allocs {
		allocID := alloc.ID
		deployState, err := db.GetDeploymentStatus(allocID)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("failed to get deployment status for %s: %v", allocID, err))
			return 1
		}

		tasks := map[string]*taskState{}
		tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
		for _, jt := range tg.Tasks {
			ls, rs, err := db.GetTaskRunnerState(allocID, jt.Name)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("failed to get task runner state %s: %v", allocID, err))
				return 1
			}

			var ds interface{}
			if ls.TaskHandle == nil {
				continue
			}
			err = ls.TaskHandle.GetDriverState(&ds)
			if err != nil {
				c.Ui.Error(fmt.Sprintf("failed to parse driver state %s: %v", allocID, err))
				return 1
			}

			tasks[jt.Name] = &taskState{
				LocalState:  ls,
				RemoteState: rs,
				DriverState: ds,
			}
		}

		data[allocID] = &clientStateAlloc{
			Alloc:        alloc,
			DeployStatus: deployState,
			Tasks:        tasks,
		}
	}
	output := debugOutput{
		Allocations: data,
	}
	bytes, err := json.Marshal(output)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("failed to serialize client state: %v", err))
		return 1
	}
	c.Ui.Output(string(bytes))

	return 0
}

type debugOutput struct {
	Allocations map[string]*clientStateAlloc
}

type clientStateAlloc struct {
	Alloc        any
	DeployStatus any
	Tasks        map[string]*taskState
}

type taskState struct {
	LocalState  *trstate.LocalState
	RemoteState any
	DriverState interface{}
}
