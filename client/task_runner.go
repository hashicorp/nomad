package client

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/discovery"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TaskRunner is used to wrap a task within an allocation and provide the execution context.
type TaskRunner struct {
	config    *config.Config
	updater   TaskStateUpdater
	logger    *log.Logger
	ctx       *driver.ExecContext
	allocID   string
	jobID     string
	taskGroup string

	discovery *discovery.DiscoveryLayer

	task     *structs.Task
	updateCh chan *structs.Task
	handle   driver.DriverHandle

	destroy     bool
	destroyCh   chan struct{}
	destroyLock sync.Mutex
	waitCh      chan struct{}
}

// taskRunnerState is used to snapshot the state of the task runner
type taskRunnerState struct {
	Task     *structs.Task
	HandleID string
}

// TaskStateUpdater is used to update the status of a task
type TaskStateUpdater func(taskName, status, desc string)

// NewTaskRunner is used to create a new task context
func NewTaskRunner(logger *log.Logger, config *config.Config,
	updater TaskStateUpdater, ctx *driver.ExecContext,
	allocID, jobID, taskGroup string, task *structs.Task,
	disc *discovery.DiscoveryLayer) *TaskRunner {

	tc := &TaskRunner{
		config:    config,
		updater:   updater,
		logger:    logger,
		ctx:       ctx,
		allocID:   allocID,
		jobID:     jobID,
		taskGroup: taskGroup,
		discovery: disc,
		task:      task,
		updateCh:  make(chan *structs.Task, 8),
		destroyCh: make(chan struct{}),
		waitCh:    make(chan struct{}),
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
		if err != nil {
			r.logger.Printf("[ERR] client: failed to open handle to task '%s' for alloc '%s': %v",
				r.task.Name, r.allocID, err)
			return err
		}
		r.handle = handle
	}
	return nil
}

// SaveState is used to snapshot our state
func (r *TaskRunner) SaveState() error {
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

// setStatus is used to update the status of the task runner
func (r *TaskRunner) setStatus(status, desc string) {
	r.updater(r.task.Name, status, desc)
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
		r.setStatus(structs.AllocClientStatusFailed, err.Error())
		return err
	}

	// Start the job
	handle, err := driver.Start(r.ctx, r.task)
	if err != nil {
		r.logger.Printf("[ERR] client: failed to start task '%s' for alloc '%s': %v",
			r.task.Name, r.allocID, err)
		r.setStatus(structs.AllocClientStatusFailed,
			fmt.Sprintf("failed to start: %v", err))
		return err
	}
	r.handle = handle
	r.setStatus(structs.AllocClientStatusRunning, "task started")
	return nil
}

// Run is a long running routine used to manage the task
func (r *TaskRunner) Run() {
	defer close(r.waitCh)
	r.logger.Printf("[DEBUG] client: starting task context for '%s' (alloc '%s')",
		r.task.Name, r.allocID)

	// Start the task if not yet started
	if r.handle == nil {
		if err := r.startTask(); err != nil {
			return
		}

		// Register with service discovery
		r.handleDiscovery(false)
	}

OUTER:
	// Wait for updates
	for {
		select {
		case err := <-r.handle.WaitCh():
			if err != nil {
				r.logger.Printf("[ERR] client: failed to complete task '%s' for alloc '%s': %v",
					r.task.Name, r.allocID, err)
				r.setStatus(structs.AllocClientStatusDead,
					fmt.Sprintf("task failed with: %v", err))
			} else {
				r.logger.Printf("[INFO] client: completed task '%s' for alloc '%s'",
					r.task.Name, r.allocID)
				r.setStatus(structs.AllocClientStatusDead,
					"task completed")
			}

			// Deregister from discovery on task completion
			r.handleDiscovery(true)

			break OUTER

		case update := <-r.updateCh:
			// Update
			r.task = update
			if err := r.handle.Update(update); err != nil {
				r.logger.Printf("[ERR] client: failed to update task '%s' for alloc '%s': %v",
					r.task.Name, r.allocID, err)
			}

		case <-r.destroyCh:
			// Send the kill signal, and use the WaitCh to block until complete
			if err := r.handle.Kill(); err != nil {
				r.logger.Printf("[ERR] client: failed to kill task '%s' for alloc '%s': %v",
					r.task.Name, r.allocID, err)
			}
		}
	}

	// Cleanup after ourselves
	r.DestroyState()
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

// handleDiscovery takes care of registering a task and its dynamic ports,
// if any, into service discovery backends. It can also be used to do the
// inverse and perform deregistration.
func (r *TaskRunner) handleDiscovery(deregister bool) {
	// Skip if discovery is not set up. Mainly used to skip for tests.
	if r.discovery == nil {
		return
	}

	// Check if we have a specific discovery name, or apply a default.
	var parts []string
	if r.task.Discover != "" {
		parts = []string{r.task.Discover}
	} else {
		parts = []string{r.jobID, r.taskGroup, r.task.Name}
	}

	// Handle registration of the main task. This can be used
	// to locate apps with static port bindings.
	if deregister {
		r.discovery.Deregister(parts)
	} else {
		r.discovery.Register(parts, 0)
	}

	// Register the dynamic ports. These are named ports which
	// are exposed as individual service entries.
	if networks := r.task.Resources.Networks; networks != nil {
		for _, net := range networks {
			for dynName, port := range net.MapDynamicPorts() {
				dynParts := append(parts, dynName)
				if deregister {
					r.discovery.Deregister(dynParts)
				} else {
					r.discovery.Register(dynParts, port)
				}
			}
		}
	}
}
