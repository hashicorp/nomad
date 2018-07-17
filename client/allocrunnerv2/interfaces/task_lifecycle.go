package interfaces

import (
	"context"

	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/nomad/structs"
)

/*
     prestart   poststart     exited                 stop
        |        |              |                     |
        |        |              |                     |
 --------> run ------> exited ----------> not restart ---------> garbage collect
            |
            |
           kill -> exited -> stop

*/

// TaskHook is a lifecycle hook into the life cycle of a task runner.
type TaskHook interface {
	Name() string
}

type TaskPrestartRequest struct {
	// HookData is previously set data by the hook
	HookData map[string]string

	// Task is the task to run
	Task *structs.Task

	// Vault token may optionally be set if a Vault token is available
	VaultToken string

	// TaskDir is the task's directory on the host
	TaskDir string

	// TaskEnv is the task's environment
	TaskEnv *env.TaskEnv
}

type TaskPrestartResponse struct {
	// Env is the environment variables to set for the task
	Env map[string]string

	// HookData allows the hook to emit data to be passed in the next time it is
	// run
	HookData map[string]string

	// Done lets the hook indicate that it should only be run once
	Done bool
}

type TaskPrestartHook interface {
	TaskHook
	Prestart(context.Context, *TaskPrestartRequest, *TaskPrestartResponse) error
}

type TaskPoststartRequest struct {
	// Network info
}
type TaskPoststartResponse struct{}

type TaskPoststartHook interface {
	TaskHook
	Poststart(context.Context, *TaskPoststartRequest, *TaskPoststartResponse) error
}

type TaskKillRequest struct{}
type TaskKillResponse struct{}

type TaskKillHook interface {
	TaskHook
	Kill(context.Context, *TaskKillRequest, *TaskKillResponse) error
}

type TaskExitedRequest struct{}
type TaskExitedResponse struct{}

type TaskExitedHook interface {
	TaskHook
	Exited(context.Context, *TaskExitedRequest, *TaskExitedResponse) error
}

type TaskUpdateRequest struct {
	VaultToken string
}
type TaskUpdateResponse struct{}

type TaskUpdateHook interface {
	TaskHook
	Update(context.Context, *TaskUpdateRequest, *TaskUpdateResponse) error
}

type TaskStopRequest struct{}
type TaskStopResponse struct{}

type TaskStopHook interface {
	TaskHook
	Stop(context.Context, *TaskStopRequest, *TaskStopResponse) error
}
