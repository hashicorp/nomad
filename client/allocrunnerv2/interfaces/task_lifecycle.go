package interfaces

import (
	"context"

	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/env"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

/*

                            Restart
      +--------------------------------------------------------+
      |                                                        |
      |                      *Update                           |
      |                     +-------+                          |
      |                     |       |                          |
      |                     |       |                          |
      |                  +---v-------+----+                    |
 +----v----+             |    Running     |               +----+-----+           +--------------+
 |         | *Prestart   |----------------|      *Exited  |          |  *Stop    |              |
 | Pending +-------------> *Poststart run +---^-----------> Exited   +----------->  Terminal    |
 |         |             |  upon entering |   |           |          | NoRestart |              |
 +---------+             |  running       |   |           +----------+           +--------------+
                         |                |   |
                         +--------+-------+   |
                                  |           |
                                  +-----------+
                                     *Kill
                                (forces terminal)

Link: http://stable.ascii-flow.appspot.com/#Draw4489375405966393064/1824429135
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

	// Prestart is called before the task is started.
	Prestart(context.Context, *TaskPrestartRequest, *TaskPrestartResponse) error
}

type TaskPoststartRequest struct {
	// Exec hook (may be nil)
	DriverExec driver.ScriptExecutor

	// Network info (may be nil)
	DriverNetwork *cstructs.DriverNetwork

	// TaskEnv is the task's environment
	TaskEnv *env.TaskEnv
}
type TaskPoststartResponse struct{}

type TaskPoststartHook interface {
	TaskHook

	// Poststart is called after the task has started.
	Poststart(context.Context, *TaskPoststartRequest, *TaskPoststartResponse) error
}

type TaskKillRequest struct{}
type TaskKillResponse struct{}

type TaskKillHook interface {
	TaskHook

	// Kill is called when a task is going to be killed.
	Kill(context.Context, *TaskKillRequest, *TaskKillResponse) error
}

type TaskExitedRequest struct{}
type TaskExitedResponse struct{}

type TaskExitedHook interface {
	TaskHook

	// Exited is called when a task exits and may or may not be restarted.
	Exited(context.Context, *TaskExitedRequest, *TaskExitedResponse) error
}

type TaskUpdateRequest struct {
	VaultToken string

	// Alloc is the current version of the allocation (may have been
	// updated since the hook was created)
	Alloc *structs.Allocation

	// TaskEnv is the task's environment
	TaskEnv *env.TaskEnv
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

	// Stop is called after the task has exited and will not be started again.
	Stop(context.Context, *TaskStopRequest, *TaskStopResponse) error
}
