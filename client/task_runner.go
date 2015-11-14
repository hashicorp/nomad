package client

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

// TaskRunner is used to wrap a task within an allocation and provide the execution context.
type TaskRunner struct {
	config         *config.Config
	updater        TaskStateUpdater
	logger         *log.Logger
	ctx            *driver.ExecContext
	allocID        string
	restartTracker restartTracker

	task     *structs.Task
	state    *structs.TaskState
	updateCh chan *structs.Task
	handle   driver.DriverHandle

	destroy     bool
	destroyCh   chan struct{}
	destroyLock sync.Mutex
	waitCh      chan struct{}

	snapshotLock sync.Mutex
}

// taskRunnerState is used to snapshot the state of the task runner
type taskRunnerState struct {
	Task     *structs.Task
	HandleID string
}

// TaskStateUpdater is used to signal that tasks state has changed.
type TaskStateUpdater func(taskName string)

// NewTaskRunner is used to create a new task context
func NewTaskRunner(logger *log.Logger, config *config.Config,
	updater TaskStateUpdater, ctx *driver.ExecContext,
	allocID string, task *structs.Task, state *structs.TaskState,
	restartTracker restartTracker) *TaskRunner {

	tc := &TaskRunner{
		config:         config,
		updater:        updater,
		logger:         logger,
		restartTracker: restartTracker,
		ctx:            ctx,
		allocID:        allocID,
		task:           task,
		state:          state,
		updateCh:       make(chan *structs.Task, 8),
		destroyCh:      make(chan struct{}),
		waitCh:         make(chan struct{}),
	}
	return tc
}

// WaitCh returns a channel to wait for termination
func (r *TaskRunner) WaitCh() <-chan struct{} {
	return r.waitCh
}

// stateFilePath returns the path to our state file
func (r *TaskRunner) stateFilePath() string {
	// Get the MD5 of the task name
	hashVal := md5.Sum([]byte(r.task.Name))
	hashHex := hex.EncodeToString(hashVal[:])
	dirName := fmt.Sprintf("task-%s", hashHex)

	// Generate the path
	path := filepath.Join(r.config.StateDir, "alloc", r.allocID,
		dirName, "state.json")
	return path
}

// RestoreState is used to restore our state
func (r *TaskRunner) RestoreState() error {
	// Load the snapshot
	var snap taskRunnerState
	if err := restoreState(r.stateFilePath(), &snap); err != nil {
		return err
	}

	// Restore fields
	r.task = snap.Task

	// Restore the driver
	if snap.HandleID != "" {
		driver, err := r.createDriver()
		if err != nil {
			return err
		}

		handle, err := driver.Open(r.ctx, snap.HandleID)

		// In the case it fails, we relaunch the task in the Run() method.
		if err != nil {
			r.logger.Printf("[ERR] client: failed to open handle to task '%s' for alloc '%s': %v",
				r.task.Name, r.allocID, err)
			return nil
		}
		r.handle = handle
	}
	return nil
}

// SaveState is used to snapshot our state
func (r *TaskRunner) SaveState() error {
	r.snapshotLock.Lock()
	defer r.snapshotLock.Unlock()
	snap := taskRunnerState{
		Task: r.task,
	}
	if r.handle != nil {
		snap.HandleID = r.handle.ID()
	}
	return persistState(r.stateFilePath(), &snap)
}

// DestroyState is used to cleanup after ourselves
func (r *TaskRunner) DestroyState() error {
	return os.RemoveAll(r.stateFilePath())
}

// setState is used to update the state of the task runner
func (r *TaskRunner) setState(state string, event *structs.TaskEvent) {
	// Update the task.
	r.state.State = state
	r.state.Events = append(r.state.Events, event)

	// Persist our state to disk.
	if err := r.SaveState(); err != nil {
		r.logger.Printf("[ERR] client: failed to save state of Task Runner: %v", r.task.Name)
	}

	// Indicate the task has been updated.
	r.updater(r.task.Name)
}

// createDriver makes a driver for the task
func (r *TaskRunner) createDriver() (driver.Driver, error) {
	driverCtx := driver.NewDriverContext(r.task.Name, r.config, r.config.Node, r.logger)
	driver, err := driver.NewDriver(r.task.Driver, driverCtx)
	if err != nil {
		err = fmt.Errorf("failed to create driver '%s' for alloc %s: %v",
			r.task.Driver, r.allocID, err)
		r.logger.Printf("[ERR] client: %s", err)
	}
	return driver, err
}

// startTask is used to start the task if there is no handle
func (r *TaskRunner) startTask() error {
	// Create a driver
	driver, err := r.createDriver()
	if err != nil {
		e := structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(err)
		r.setState(structs.TaskStateDead, e)
		return err
	}

	// Start the job
	handle, err := driver.Start(r.ctx, r.task)
	if err != nil {
		r.logger.Printf("[ERR] client: failed to start task '%s' for alloc '%s': %v",
			r.task.Name, r.allocID, err)
		e := structs.NewTaskEvent(structs.TaskDriverFailure).
			SetDriverError(fmt.Errorf("failed to start: %v", err))
		r.setState(structs.TaskStateDead, e)
		return err
	}
	r.handle = handle
	r.setState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))
	return nil
}

// Run is a long running routine used to manage the task
func (r *TaskRunner) Run() {
	defer close(r.waitCh)
	r.logger.Printf("[DEBUG] client: starting task context for '%s' (alloc '%s')",
		r.task.Name, r.allocID)

	r.run(false)
	return
}

func (r *TaskRunner) run(forceStart bool) {
	// Start the task if not yet started or it is being forced.
	if r.handle == nil || forceStart {
		if err := r.startTask(); err != nil {
			return
		}
	}

	// Store the errors that caused use to stop waiting for updates.
	var waitRes *cstructs.WaitResult
	var destroyErr error
	destroyed := false

OUTER:
	// Wait for updates
	for {
		select {
		case waitRes = <-r.handle.WaitCh():
			break OUTER
		case update := <-r.updateCh:
			// Update
			r.task = update
			if err := r.handle.Update(update); err != nil {
				r.logger.Printf("[ERR] client: failed to update task '%s' for alloc '%s': %v", r.task.Name, r.allocID, err)
			}
		case <-r.destroyCh:
			// Send the kill signal, and use the WaitCh to block until complete
			if err := r.handle.Kill(); err != nil {
				r.logger.Printf("[ERR] client: failed to kill task '%s' for alloc '%s': %v", r.task.Name, r.allocID, err)
				destroyErr = err
			}
			destroyed = true
		}
	}

	// If the user destroyed the task, we do not attempt to do any restarts.
	if destroyed {
		r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled).SetKillError(destroyErr))
		return
	}

	// Log whether the task was successful or not.
	if !waitRes.Successful() {
		r.logger.Printf("[ERR] client: failed to complete task '%s' for alloc '%s': %v", r.task.Name, r.allocID, waitRes)
	} else {
		r.logger.Printf("[INFO] client: completed task '%s' for alloc '%s'", r.task.Name, r.allocID)
	}

	// Check if we should restart. If not mark task as dead and exit.
	waitEvent := r.waitErrorToEvent(waitRes)
	shouldRestart, when := r.restartTracker.nextRestart()
	if !shouldRestart {
		r.logger.Printf("[INFO] client: Not restarting task: %v for alloc: %v ", r.task.Name, r.allocID)
		r.setState(structs.TaskStateDead, waitEvent)
		return
	}

	r.logger.Printf("[INFO] client: Restarting Task: %v", r.task.Name)
	r.logger.Printf("[DEBUG] client: Sleeping for %v before restarting Task %v", when, r.task.Name)
	r.setState(structs.TaskStatePending, waitEvent)

	// Sleep but watch for destroy events.
	select {
	case <-time.After(when):
	case <-r.destroyCh:
	}

	// Destroyed while we were waiting to restart, so abort.
	r.destroyLock.Lock()
	destroyed = r.destroy
	r.destroyLock.Unlock()
	if destroyed {
		r.logger.Printf("[DEBUG] client: Not restarting task: %v because it's destroyed by user", r.task.Name)
		r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled))
		return
	}

	// Recurse on ourselves and force the start since we are restarting the task.
	r.run(true)
	return
}

// Helper function for converting a WaitResult into a TaskTerminated event.
func (r *TaskRunner) waitErrorToEvent(res *cstructs.WaitResult) *structs.TaskEvent {
	e := structs.NewTaskEvent(structs.TaskTerminated).SetExitCode(res.ExitCode).SetSignal(res.Signal)
	if res.Err != nil {
		e.SetExitMessage(res.Err.Error())
	}
	return e
}

// Update is used to update the task of the context
func (r *TaskRunner) Update(update *structs.Task) {
	select {
	case r.updateCh <- update:
	default:
		r.logger.Printf("[ERR] client: dropping task update '%s' (alloc '%s')",
			update.Name, r.allocID)
	}
}

// Destroy is used to indicate that the task context should be destroyed
func (r *TaskRunner) Destroy() {
	r.destroyLock.Lock()
	defer r.destroyLock.Unlock()

	if r.destroy {
		return
	}
	r.destroy = true
	close(r.destroyCh)
}
