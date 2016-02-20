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

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/hashstructure"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

const (
	// killBackoffBaseline is the baseline time for exponential backoff while
	// killing a task.
	killBackoffBaseline = 5 * time.Second

	// killBackoffLimit is the the limit of the exponential backoff for killing
	// the task.
	killBackoffLimit = 5 * time.Minute

	// killFailureLimit is how many times we will attempt to kill a task before
	// giving up and potentially leaking resources.
	killFailureLimit = 10
)

// TaskRunner is used to wrap a task within an allocation and provide the execution context.
type TaskRunner struct {
	config         *config.Config
	updater        TaskStateUpdater
	logger         *log.Logger
	ctx            *driver.ExecContext
	alloc          *structs.Allocation
	restartTracker *RestartTracker
	consulService  *ConsulService

	task       *structs.Task
	updateCh   chan *structs.Allocation
	handle     driver.DriverHandle
	handleLock sync.Mutex

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
type TaskStateUpdater func(taskName, state string, event *structs.TaskEvent)

// NewTaskRunner is used to create a new task context
func NewTaskRunner(logger *log.Logger, config *config.Config,
	updater TaskStateUpdater, ctx *driver.ExecContext,
	alloc *structs.Allocation, task *structs.Task,
	consulService *ConsulService) *TaskRunner {

	// Merge in the task resources
	task.Resources = alloc.TaskResources[task.Name]

	// Build the restart tracker.
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		logger.Printf("[ERR] client: alloc '%s' for missing task group '%s'", alloc.ID, alloc.TaskGroup)
		return nil
	}
	restartTracker := newRestartTracker(tg.RestartPolicy, alloc.Job.Type)

	tc := &TaskRunner{
		config:         config,
		updater:        updater,
		logger:         logger,
		restartTracker: restartTracker,
		consulService:  consulService,
		ctx:            ctx,
		alloc:          alloc,
		task:           task,
		updateCh:       make(chan *structs.Allocation, 8),
		destroyCh:      make(chan struct{}),
		waitCh:         make(chan struct{}),
	}

	// Set the state to pending.
	tc.updater(task.Name, structs.TaskStatePending, structs.NewTaskEvent(structs.TaskReceived))
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
	path := filepath.Join(r.config.StateDir, "alloc", r.alloc.ID,
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
				r.task.Name, r.alloc.ID, err)
			return nil
		}
		r.handleLock.Lock()
		r.handle = handle
		r.handleLock.Unlock()
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
	r.handleLock.Lock()
	if r.handle != nil {
		snap.HandleID = r.handle.ID()
	}
	r.handleLock.Unlock()
	return persistState(r.stateFilePath(), &snap)
}

// DestroyState is used to cleanup after ourselves
func (r *TaskRunner) DestroyState() error {
	return os.RemoveAll(r.stateFilePath())
}

// setState is used to update the state of the task runner
func (r *TaskRunner) setState(state string, event *structs.TaskEvent) {
	// Persist our state to disk.
	if err := r.SaveState(); err != nil {
		r.logger.Printf("[ERR] client: failed to save state of Task Runner: %v", r.task.Name)
	}

	// Indicate the task has been updated.
	r.updater(r.task.Name, state, event)
}

// createDriver makes a driver for the task
func (r *TaskRunner) createDriver() (driver.Driver, error) {
	taskEnv, err := driver.GetTaskEnv(r.ctx.AllocDir, r.config.Node, r.task)
	if err != nil {
		err = fmt.Errorf("failed to create driver '%s' for alloc %s: %v",
			r.task.Driver, r.alloc.ID, err)
		r.logger.Printf("[ERR] client: %s", err)
		return nil, err

	}

	driverCtx := driver.NewDriverContext(r.task.Name, r.config, r.config.Node, r.logger, taskEnv)
	driver, err := driver.NewDriver(r.task.Driver, driverCtx)
	if err != nil {
		err = fmt.Errorf("failed to create driver '%s' for alloc %s: %v",
			r.task.Driver, r.alloc.ID, err)
		r.logger.Printf("[ERR] client: %s", err)
		return nil, err
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
			r.task.Name, r.alloc.ID, err)
		e := structs.NewTaskEvent(structs.TaskDriverFailure).
			SetDriverError(fmt.Errorf("failed to start: %v", err))
		r.setState(structs.TaskStateDead, e)
		return err
	}
	r.handleLock.Lock()
	r.handle = handle
	r.handleLock.Unlock()
	r.setState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))
	return nil
}

// Run is a long running routine used to manage the task
func (r *TaskRunner) Run() {
	defer close(r.waitCh)
	r.logger.Printf("[DEBUG] client: starting task context for '%s' (alloc '%s')",
		r.task.Name, r.alloc.ID)

	r.run()
	return
}

func (r *TaskRunner) run() {
	var forceStart bool
	for {
		// Start the task if not yet started or it is being forced.
		r.handleLock.Lock()
		handleEmpty := r.handle == nil
		r.handleLock.Unlock()
		if handleEmpty || forceStart {
			forceStart = false
			if err := r.startTask(); err != nil {
				return
			}
		}

		// Store the errors that caused use to stop waiting for updates.
		var waitRes *cstructs.WaitResult
		var destroyErr error
		destroyed := false

		// Register the services defined by the task with Consil
		r.consulService.Register(r.task, r.alloc)

	OUTER:
		// Wait for updates
		for {
			select {
			case waitRes = <-r.handle.WaitCh():
				break OUTER
			case update := <-r.updateCh:
				if err := r.handleUpdate(update); err != nil {
					r.logger.Printf("[ERR] client: update to task %q failed: %v", r.task.Name, err)
				}
			case <-r.destroyCh:
				// Kill the task using an exponential backoff in-case of failures.
				destroySuccess, err := r.handleDestroy()
				if !destroySuccess {
					// We couldn't successfully destroy the resource created.
					r.logger.Printf("[ERR] client: failed to kill task %q. Resources may have been leaked: %v", r.task.Name, err)
				} else {
					// Wait for the task to exit but cap the time to ensure we don't block.
					select {
					case waitRes = <-r.handle.WaitCh():
					case <-time.After(3 * time.Second):
					}
				}

				// Store that the task has been destroyed and any associated error.
				destroyed = true
				destroyErr = err
				break OUTER
			}
		}

		// De-Register the services belonging to the task from consul
		r.consulService.Deregister(r.task, r.alloc)

		// If the user destroyed the task, we do not attempt to do any restarts.
		if destroyed {
			r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled).SetKillError(destroyErr))
			return
		}

		// Log whether the task was successful or not.
		if !waitRes.Successful() {
			r.logger.Printf("[ERR] client: failed to complete task '%s' for alloc '%s': %v", r.task.Name, r.alloc.ID, waitRes)
		} else {
			r.logger.Printf("[INFO] client: completed task '%s' for alloc '%s'", r.task.Name, r.alloc.ID)
		}

		// Check if we should restart. If not mark task as dead and exit.
		shouldRestart, when := r.restartTracker.NextRestart(waitRes.ExitCode)
		waitEvent := r.waitErrorToEvent(waitRes)
		if !shouldRestart {
			r.logger.Printf("[INFO] client: Not restarting task: %v for alloc: %v ", r.task.Name, r.alloc.ID)
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

		// Set force start because we are restarting the task.
		forceStart = true
	}
}

// handleUpdate takes an updated allocation and updates internal state to
// reflect the new config for the task.
func (r *TaskRunner) handleUpdate(update *structs.Allocation) error {
	// Extract the task group from the alloc.
	tg := update.Job.LookupTaskGroup(update.TaskGroup)
	if tg == nil {
		return fmt.Errorf("alloc '%s' missing task group '%s'", update.ID, update.TaskGroup)
	}

	// Extract the task.
	var updatedTask *structs.Task
	for _, t := range tg.Tasks {
		if t.Name == r.task.Name {
			updatedTask = t
		}
	}
	if updatedTask == nil {
		return fmt.Errorf("task group %q doesn't contain task %q", tg.Name, r.task.Name)
	}

	// Merge in the task resources
	updatedTask.Resources = update.TaskResources[updatedTask.Name]

	// Update will update resources and store the new kill timeout.
	var mErr multierror.Error
	r.handleLock.Lock()
	if r.handle != nil {
		if err := r.handle.Update(updatedTask); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("updating task resources failed: %v", err))
		}
	}
	r.handleLock.Unlock()

	// Update the restart policy.
	if r.restartTracker != nil {
		r.restartTracker.SetPolicy(tg.RestartPolicy)
	}

	// Hash services returns the hash of the task's services
	hashServices := func(task *structs.Task) uint64 {
		h, err := hashstructure.Hash(task.Services, nil)
		if err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("hashing services failed %#v: %v", task.Services, err))
		}
		return h
	}

	// Re-register the task to consul if any of the services have changed.
	if hashServices(updatedTask) != hashServices(r.task) {
		if err := r.consulService.Register(updatedTask, update); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("updating services with consul failed: %v", err))
		}
	}

	// Store the updated alloc.
	r.alloc = update
	r.task = updatedTask
	return mErr.ErrorOrNil()
}

// handleDestroy kills the task handle. In the case that killing fails,
// handleDestroy will retry with an exponential backoff and will give up at a
// given limit. It returns whether the task was destroyed and the error
// associated with the last kill attempt.
func (r *TaskRunner) handleDestroy() (destroyed bool, err error) {
	// Cap the number of times we attempt to kill the task.
	for i := 0; i < killFailureLimit; i++ {
		if err = r.handle.Kill(); err != nil {
			// Calculate the new backoff
			backoff := (1 << (2 * uint64(i))) * killBackoffBaseline
			if backoff > killBackoffLimit {
				backoff = killBackoffLimit
			}

			r.logger.Printf("[ERR] client: failed to kill task '%s' for alloc %q. Retrying in %v: %v",
				r.task.Name, r.alloc.ID, backoff, err)
			time.Sleep(time.Duration(backoff))
		} else {
			// Kill was successful
			return true, nil
		}
	}
	return
}

// Helper function for converting a WaitResult into a TaskTerminated event.
func (r *TaskRunner) waitErrorToEvent(res *cstructs.WaitResult) *structs.TaskEvent {
	return structs.NewTaskEvent(structs.TaskTerminated).
		SetExitCode(res.ExitCode).
		SetSignal(res.Signal).
		SetExitMessage(res.Err)
}

// Update is used to update the task of the context
func (r *TaskRunner) Update(update *structs.Allocation) {
	select {
	case r.updateCh <- update:
	default:
		r.logger.Printf("[ERR] client: dropping task update '%s' (alloc '%s')",
			r.task.Name, r.alloc.ID)
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
