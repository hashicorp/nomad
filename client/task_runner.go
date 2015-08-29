package client

import (
	"fmt"
	"log"
	"sync"

	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TaskRunner is used to wrap a task within an allocation and provide the execution context.
type TaskRunner struct {
	allocRunner *AllocRunner
	logger      *log.Logger
	ctx         *driver.ExecContext

	allocID string
	task    *structs.Task

	updateCh chan *structs.Task

	destroy     bool
	destroyCh   chan struct{}
	destroyLock sync.Mutex

	waitCh chan struct{}
}

// NewTaskRunner is used to create a new task context
func NewTaskRunner(allocRunner *AllocRunner, ctx *driver.ExecContext, task *structs.Task) *TaskRunner {
	tc := &TaskRunner{
		allocRunner: allocRunner,
		logger:      allocRunner.logger,
		ctx:         ctx,
		allocID:     allocRunner.Alloc().ID,
		task:        task,
		updateCh:    make(chan *structs.Task, 8),
		destroyCh:   make(chan struct{}),
		waitCh:      make(chan struct{}),
	}
	return tc
}

// WaitCh returns a channel to wait for termination
func (r *TaskRunner) WaitCh() <-chan struct{} {
	return r.waitCh
}

// setStatus is used to update the status of the task runner
func (r *TaskRunner) setStatus(status, desc string) {
	r.allocRunner.setTaskStatus(r.task.Name, status, desc)
}

// Run is a long running routine used to manage the task
func (r *TaskRunner) Run() {
	defer close(r.waitCh)
	r.logger.Printf("[DEBUG] client: starting task context for '%s' (alloc '%s')",
		r.task.Name, r.allocID)

	// Create the driver
	driver, err := driver.NewDriver(r.task.Driver, r.logger)
	if err != nil {
		r.logger.Printf("[ERR] client: failed to create driver '%s' for alloc '%s'",
			r.task.Driver, r.allocID)
		r.setStatus(structs.AllocClientStatusFailed,
			fmt.Sprintf("failed to create driver '%s'", r.task.Driver))
		return
	}

	// Start the job
	handle, err := driver.Start(r.ctx, r.task)
	if err != nil {
		r.logger.Printf("[ERR] client: failed to start task '%s' for alloc '%s': %v",
			r.task.Name, r.allocID, err)
		r.setStatus(structs.AllocClientStatusFailed,
			fmt.Sprintf("failed to start: %v", err))
		return
	}
	r.setStatus(structs.AllocClientStatusRunning, "task started")

	// Wait for updates
	for {
		select {
		case err := <-handle.WaitCh():
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
			return

		case update := <-r.updateCh:
			// Update
			r.task = update
			if err := handle.Update(update); err != nil {
				r.logger.Printf("[ERR] client: failed to update task '%s' for alloc '%s': %v",
					r.task.Name, r.allocID, err)
			}

		case <-r.destroyCh:
			// Send the kill signal, and use the WaitCh to block until complete
			if err := handle.Kill(); err != nil {
				r.logger.Printf("[ERR] client: failed to kill task '%s' for alloc '%s': %v",
					r.task.Name, r.allocID, err)
			}
		}
	}
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
