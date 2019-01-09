package state

import (
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
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

func (t *taskRunnerState08) Upgrade() *state.LocalState {
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

	//TODO How to convert handles?! This does not work.
	ls.TaskHandle = drivers.NewTaskHandle("TODO")

	//TODO where do we get this from?
	ls.TaskHandle.Config = nil

	//TODO do we need to se this accurately? Or will RecoverTask handle it?
	ls.TaskHandle.State = drivers.TaskStateUnknown

	//TODO do we need an envelope so drivers know this is an old state?
	ls.TaskHandle.SetDriverState(t.HandleID)

	return ls
}
