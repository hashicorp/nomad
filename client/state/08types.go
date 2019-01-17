package state

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared"
)

// allocRunnerMutableState08 is state that had to be written on each save as it
// changed over the life-cycle of the alloc_runner in Nomad 0.8.
//
// https://github.com/hashicorp/nomad/blob/v0.8.6/client/alloc_runner.go#L146-L153
//
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
//
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

type taskRunnerHandle08 struct {
	PluginConfig struct {
		Pid      int    `json:"Pid"`
		AddrNet  string `json:"AddrNet"`
		AddrName string `json:"AddrName"`
	} `json:"PluginConfig"`
}

func (t *taskRunnerHandle08) reattachConfig() *shared.ReattachConfig {
	return &shared.ReattachConfig{
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
		PrestartDone: t.TaskDirBuilt,
	}

	// Upgrade dispatch payload state
	ls.Hooks["dispatch_payload"] = &state.HookState{
		PrestartDone: t.PayloadRendered,
	}

	// Add necessary fields to TaskConfig
	ls.TaskHandle = drivers.NewTaskHandle(drivers.Pre09TaskHandleVersion)
	ls.TaskHandle.Config = &drivers.TaskConfig{
		Name:    taskName,
		AllocID: allocID,
	}

	ls.TaskHandle.State = drivers.TaskStateUnknown

	// A ReattachConfig to the pre09 executor is sent
	var raw []byte
	var handle taskRunnerHandle08
	if err := json.Unmarshal([]byte(t.HandleID), &handle); err != nil {
		return nil, fmt.Errorf("failed to decode 0.8 driver state: %v", err)
	}
	raw, err := json.Marshal(handle.reattachConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to encode updated driver state: %v", err)
	}

	ls.TaskHandle.DriverState = raw

	return ls, nil
}
