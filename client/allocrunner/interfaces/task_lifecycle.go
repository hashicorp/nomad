// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
*/

// TaskHook is a lifecycle hook into the life cycle of a task runner.
type TaskHook interface {
	Name() string
}

type TaskPrestartRequest struct {
	// PreviousState is previously set data by the hook. It must be copied
	// to State below to be maintained across restarts.
	PreviousState map[string]string

	// Task is the task to run
	Task *structs.Task

	// TaskResources is the resources assigned to the task
	TaskResources *structs.AllocatedTaskResources

	// Vault token may optionally be set if a Vault token is available
	VaultToken string

	// NomadToken token may optionally be set if a Nomad token is available
	NomadToken string

	// TaskDir contains the task's directory tree on the host
	TaskDir *allocdir.TaskDir

	// TaskEnv is the task's environment
	TaskEnv *taskenv.TaskEnv

	// Alloc is the current version of the allocation
	Alloc *structs.Allocation
}

type TaskPrestartResponse struct {
	// Env is the environment variables to set for the task
	Env map[string]string

	// Mounts is the set of host volumes to mount into the task
	Mounts []*drivers.MountConfig

	// Devices are the set of devices to mount into the task
	Devices []*drivers.DeviceConfig

	// State allows the hook to emit data to be passed in the next time it is
	// run. Hooks must copy relevant PreviousState to State to maintain it
	// across restarts.
	State map[string]string

	// Done lets the hook indicate that it completed successfully and
	// should not be run again.
	//
	// Use sparringly! In general hooks should be idempotent and therefore Done
	// is unneeded. You never know at what point an agent might crash, and it can
	// be hard to reason about how Done=true impacts agent restarts and node
	// reboots. See #19787 for example.
	//
	// Done is useful for expensive operations such as downloading artifacts, or
	// for operations which might fail needlessly if rerun while a node is
	// disconnected.
	Done bool
}

type TaskPrestartHook interface {
	TaskHook

	// Prestart is called before the task is started including after every
	// restart. Prestart is not called if the allocation is terminal.
	//
	// The context is cancelled if the task is killed or shutdown but
	// should not be stored any persistent goroutines this Prestart
	// creates.
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

	// Poststart is called after the task has started. Poststart is not
	// called if the allocation is terminal.
	//
	// The context is cancelled if the task is killed.
	Poststart(context.Context, *TaskPoststartRequest, *TaskPoststartResponse) error
}

type TaskPreKillRequest struct{}
type TaskPreKillResponse struct{}

type TaskPreKillHook interface {
	TaskHook

	// PreKilling is called right before a task is going to be killed or
	// restarted. They are called concurrently with TaskRunner.Run and may
	// be called without Prestart being called.
	PreKilling(context.Context, *TaskPreKillRequest, *TaskPreKillResponse) error
}

type TaskExitedRequest struct{}
type TaskExitedResponse struct{}

type TaskExitedHook interface {
	TaskHook

	// Exited is called after a task exits and may or may not be restarted.
	// Prestart may or may not have been called.
	//
	// The context is cancelled if the task is killed.
	Exited(context.Context, *TaskExitedRequest, *TaskExitedResponse) error
}

type TaskUpdateRequest struct {
	VaultToken string

	NomadToken string

	// Alloc is the current version of the allocation (may have been
	// updated since the hook was created)
	Alloc *structs.Allocation

	// TaskEnv is the task's environment
	TaskEnv *taskenv.TaskEnv
}
type TaskUpdateResponse struct{}

type TaskUpdateHook interface {
	TaskHook

	// Update is called when the servers have updated the Allocation for
	// this task. Updates are concurrent with all other task hooks and
	// therefore hooks that implement this interface must be completely
	// safe for concurrent access.
	//
	// The context is cancelled if the task is killed.
	Update(context.Context, *TaskUpdateRequest, *TaskUpdateResponse) error
}

type TaskStopRequest struct {
	// ExistingState is previously set hook data and should only be
	// read. Stop hooks cannot alter state.
	ExistingState map[string]string

	// TaskDir contains the task's directory tree on the host
	TaskDir *allocdir.TaskDir
}

type TaskStopResponse struct{}

type TaskStopHook interface {
	TaskHook

	// Stop is called after the task has exited and will not be started
	// again. It is the only hook guaranteed to be executed whenever
	// TaskRunner.Run is called (and not gracefully shutting down).
	// Therefore it may be called even when prestart and the other hooks
	// have not.
	//
	// Stop hooks must be idempotent. The context is cancelled if the task
	// is killed.
	Stop(context.Context, *TaskStopRequest, *TaskStopResponse) error
}
