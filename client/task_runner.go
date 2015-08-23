package client

import (
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

	task *structs.Task

	updateCh chan *structs.Task

	destroy     bool
	destroyCh   chan struct{}
	destroyLock sync.Mutex
}

// NewTaskRunner is used to create a new task context
func NewTaskRunner(allocRunner *AllocRunner, ctx *driver.ExecContext, task *structs.Task) *TaskRunner {
	tc := &TaskRunner{
		allocRunner: allocRunner,
		logger:      allocRunner.logger,
		ctx:         ctx,
		task:        task,
		updateCh:    make(chan *structs.Task, 8),
		destroyCh:   make(chan struct{}),
	}
	return tc
}

// Alloc returns the associated alloc
func (r *TaskRunner) Alloc() *structs.Allocation {
	return r.allocRunner.Alloc()
}

// Run is a long running routine used to manage the task
func (r *TaskRunner) Run() {
	r.logger.Printf("[DEBUG] client: starting task context for '%s' (alloc '%s')", r.task.Name, r.Alloc().ID)

	// Create the driver
	driver, err := driver.NewDriver(r.task.Driver, r.logger)
	if err != nil {
		r.logger.Printf("[ERR] client: failed to create driver '%s' for alloc '%s'", r.task.Driver, r.Alloc().ID)
		// TODO Err return
		return
	}

	// Start the job
	handle, err := driver.Start(r.ctx, r.task)
	if err != nil {
		r.logger.Printf("[ERR] client: failed to start task '%s' for alloc '%s'", r.task.Name, r.Alloc().ID)
		// TODO Err return
		return
	}

	// Wait for updates
	for {
		select {
		case err := <-handle.WaitCh():
			if err != nil {
				r.logger.Printf("[ERR] client: failed to complete task '%s' for alloc '%s': %v",
					r.task.Name, r.Alloc().ID, err)
			} else {
				r.logger.Printf("[INFO] client: completed task '%s' for alloc '%s'",
					r.task.Name, r.Alloc().ID)
			}
			return
			// TODO:
		case update := <-r.updateCh:
			// TODO: Update
			r.task = update
		case <-r.destroyCh:
			// TODO: Destroy
			return
		}
	}
}

// Update is used to update the task of the context
func (r *TaskRunner) Update(update *structs.Task) {
	select {
	case r.updateCh <- update:
	default:
		r.logger.Printf("[ERR] client: dropping task update '%s' (alloc '%s')", update.Name, r.Alloc().ID)
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
