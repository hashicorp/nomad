// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

// allocRunnerMutableState08 is state that had to be written on each save as it
// changed over the life-cycle of the alloc_runner in Nomad 0.8.
//
// https://github.com/hashicorp/nomad/blob/v0.8.6/client/alloc_runner.go#L146-L153
type allocRunnerMutableState08 struct {
	// AllocClientStatus does not need to be upgraded as it is computed
	// from task states.
	AllocClientStatus string

	// AllocClientDescription does not need to be upgraded as it is computed
	// from task states.
	AllocClientDescription string

	TaskStates       map[string]*structs.TaskState
	DeploymentStatus *structs.AllocDeploymentStatus
}

// taskRunnerState08 was used to snapshot the state of the task runner in Nomad
// 0.8.
//
// https://github.com/hashicorp/nomad/blob/v0.8.6/client/task_runner.go#L188-L197
// COMPAT(0.10): Allows upgrading from 0.8.X to 0.9.0.
type taskRunnerState08 struct {
	Version            string
	HandleID           string
	ArtifactDownloaded bool
	TaskDirBuilt       bool
	PayloadRendered    bool
	DriverNetwork      *drivers.DriverNetwork
	// Created Resources are no longer used.
	//CreatedResources   *driver.CreatedResources
}

type TaskRunnerHandle08 struct {
	// Docker specific handle info
	ContainerID string `json:"ContainerID"`
	Image       string `json:"Image"`

	// LXC specific handle info
	ContainerName string `json:"ContainerName"`
	LxcPath       string `json:"LxcPath"`

	// Executor reattach config
	PluginConfig struct {
		Pid      int    `json:"Pid"`
		AddrNet  string `json:"AddrNet"`
		AddrName string `json:"AddrName"`
	} `json:"PluginConfig"`
}

func (t *TaskRunnerHandle08) ReattachConfig() *pstructs.ReattachConfig {
	return &pstructs.ReattachConfig{
		Network: t.PluginConfig.AddrNet,
		Addr:    t.PluginConfig.AddrName,
		Pid:     t.PluginConfig.Pid,
	}
}

func (t *taskRunnerState08) Upgrade(allocID, taskName string) (*state.LocalState, error) {
	ls := state.NewLocalState()

	// Reuse DriverNetwork
	ls.DriverNetwork = t.DriverNetwork

	// Upgrade artifact state
	ls.Hooks["artifacts"] = &state.HookState{
		PrestartDone: t.ArtifactDownloaded,
	}

	// Upgrade task dir state
	ls.Hooks["task_dir"] = &state.HookState{
		Data: map[string]string{
			// "is_done" is equivalent to task_dir_hook.TaskDirHookIsDoneKey
			// Does not import to avoid import cycle
			"is_done": fmt.Sprintf("%v", t.TaskDirBuilt),
		},
	}

	// Upgrade dispatch payload state
	ls.Hooks["dispatch_payload"] = &state.HookState{
		PrestartDone: t.PayloadRendered,
	}

	// Add necessary fields to TaskConfig
	ls.TaskHandle = drivers.NewTaskHandle(drivers.Pre09TaskHandleVersion)
	ls.TaskHandle.Config = &drivers.TaskConfig{
		ID:      fmt.Sprintf("pre09-%s", uuid.Generate()),
		Name:    taskName,
		AllocID: allocID,
	}

	ls.TaskHandle.State = drivers.TaskStateUnknown

	// The docker driver prefixed the handle with 'DOCKER:'
	// Strip so that it can be unmarshalled
	data := strings.TrimPrefix(t.HandleID, "DOCKER:")

	// The pre09 driver handle ID is given to the driver. It is unmarshalled
	// here to check for errors
	if _, err := UnmarshalPre09HandleID([]byte(data)); err != nil {
		return nil, err
	}

	ls.TaskHandle.DriverState = []byte(data)

	return ls, nil
}

// UnmarshalPre09HandleID decodes the pre09 json encoded handle ID
func UnmarshalPre09HandleID(raw []byte) (*TaskRunnerHandle08, error) {
	var handle TaskRunnerHandle08
	if err := json.Unmarshal(raw, &handle); err != nil {
		return nil, fmt.Errorf("failed to decode 0.8 driver state: %v", err)
	}

	return &handle, nil
}
