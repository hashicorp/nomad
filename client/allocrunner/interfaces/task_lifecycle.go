package interfaces

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
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

	// TaskResources is the resources assigned to the task
	TaskResources *structs.AllocatedTaskResources

	// Vault token may optionally be set if a Vault token is available
	VaultToken string

	// TaskDir contains the task's directory tree on the host
	TaskDir *allocdir.TaskDir

	// TaskEnv is the task's environment
	TaskEnv *taskenv.TaskEnv
}

type TaskPrestartResponse struct {
	// Env is the environment variables to set for the task
	Env map[string]string

	// Mounts is the set of host volumes to mount into the task
	Mounts []*drivers.MountConfig

	// Devices are the set of devices to mount into the task
	Devices []*drivers.DeviceConfig

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

// DriverStats is the interface implemented by DriverHandles to return task stats.
type DriverStats interface {
	Stats(context.Context, time.Duration) (<-chan *cstructs.TaskResourceUsage, error)
}

type TaskPoststartRequest struct {
	// Exec hook (may be nil)
	DriverExec interfaces.ScriptExecutor

	// Network info (may be nil)
	DriverNetwork *drivers.DriverNetwork

	// TaskEnv is the task's environment
	TaskEnv *taskenv.TaskEnv

	// Stats collector
	DriverStats DriverStats
}
type TaskPoststartResponse struct{}

type TaskPoststartHook interface {
	TaskHook

	// Poststart is called after the task has started.
	Poststart(context.Context, *TaskPoststartRequest, *TaskPoststartResponse) error
}

type TaskPreKillRequest struct{}
type TaskPreKillResponse struct{}

type TaskPreKillHook interface {
	TaskHook

	// PreKilling is called right before a task is going to be killed or restarted.
	PreKilling(context.Context, *TaskPreKillRequest, *TaskPreKillResponse) error
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
	TaskEnv *taskenv.TaskEnv
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
