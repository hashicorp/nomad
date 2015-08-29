package client

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// allocSyncRetryIntv is the interval on which we retry updating
	// the status of the allocation
	allocSyncRetryIntv = 15 * time.Second
)

// taskStatus is used to track the status of a task
type taskStatus struct {
	Status      string
	Description string
}

// AllocRunner is used to wrap an allocation and provide the execution context.
type AllocRunner struct {
	client *Client
	logger *log.Logger

	alloc *structs.Allocation

	dirtyCh chan struct{}

	ctx   *driver.ExecContext
	tasks map[string]*TaskRunner

	taskStatus     map[string]taskStatus
	taskStatusLock sync.RWMutex

	updateCh chan *structs.Allocation

	destroy     bool
	destroyCh   chan struct{}
	destroyLock sync.Mutex
}

// NewAllocRunner is used to create a new allocation context
func NewAllocRunner(client *Client, alloc *structs.Allocation) *AllocRunner {
	ctx := &AllocRunner{
		client:     client,
		logger:     client.logger,
		alloc:      alloc,
		dirtyCh:    make(chan struct{}, 1),
		tasks:      make(map[string]*TaskRunner),
		taskStatus: make(map[string]taskStatus),
		updateCh:   make(chan *structs.Allocation, 8),
		destroyCh:  make(chan struct{}),
	}
	return ctx
}

// Alloc returns the associated allocation
func (r *AllocRunner) Alloc() *structs.Allocation {
	return r.alloc
}

// setAlloc is used to update the allocation of the runner
// we preserve the existing client status and description
func (r *AllocRunner) setAlloc(alloc *structs.Allocation) {
	if r.alloc != nil {
		alloc.ClientStatus = r.alloc.ClientStatus
		alloc.ClientDescription = r.alloc.ClientDescription
	}
	r.alloc = alloc
}

// syncStatus is used to run and sync the status when it changes
func (r *AllocRunner) syncStatus() {
	var retryCh <-chan time.Time
	for {
		select {
		case <-retryCh:
		case <-r.dirtyCh:
		case <-r.destroyCh:
			return
		}

		// Scan the task status to termine the status of the alloc
		var pending, running, dead, failed bool
		r.taskStatusLock.RLock()
		pending = len(r.taskStatus) < len(r.tasks)
		for _, status := range r.taskStatus {
			switch status.Status {
			case structs.AllocClientStatusRunning:
				running = true
			case structs.AllocClientStatusDead:
				dead = true
			case structs.AllocClientStatusFailed:
				failed = true
			}
		}
		if len(r.taskStatus) > 0 {
			taskDesc, _ := json.Marshal(r.taskStatus)
			r.alloc.ClientDescription = string(taskDesc)
		}
		r.taskStatusLock.RUnlock()

		// Determine the alloc status
		if failed {
			r.alloc.ClientStatus = structs.AllocClientStatusFailed
		} else if running {
			r.alloc.ClientStatus = structs.AllocClientStatusRunning
		} else if dead && !pending {
			r.alloc.ClientStatus = structs.AllocClientStatusDead
		}

		// Attempt to update the status
		if err := r.client.updateAllocStatus(r.alloc); err != nil {
			r.logger.Printf("[ERR] client: failed to update alloc '%s' status to %s: %s",
				r.alloc.ID, r.alloc.ClientStatus, err)
			retryCh = time.After(allocSyncRetryIntv + randomStagger(allocSyncRetryIntv))
		}
		retryCh = nil
	}
}

// setStatus is used to update the allocation status
func (r *AllocRunner) setStatus(status, desc string) {
	r.alloc.ClientStatus = status
	r.alloc.ClientDescription = desc
	select {
	case r.dirtyCh <- struct{}{}:
	default:
	}
}

// setTaskStatus is used to set the status of a task
func (r *AllocRunner) setTaskStatus(taskName, status, desc string) {
	r.taskStatusLock.Lock()
	defer r.taskStatusLock.Unlock()
	r.taskStatus[taskName] = taskStatus{
		Status:      status,
		Description: desc,
	}
	select {
	case r.dirtyCh <- struct{}{}:
	default:
	}
}

// Run is a long running goroutine used to manage an allocation
func (r *AllocRunner) Run() {
	go r.syncStatus()

	// Check if the allocation is in a terminal status
	alloc := r.alloc
	if alloc.TerminalStatus() {
		r.logger.Printf("[DEBUG] client: aborting runner for alloc '%s', terminal status", r.alloc.ID)
		return
	}
	r.logger.Printf("[DEBUG] client: starting runner for alloc '%s'", r.alloc.ID)

	// Find the task group to run in the allocation
	tg := alloc.Job.LookingTaskGroup(alloc.TaskGroup)
	if tg == nil {
		r.logger.Printf("[ERR] client: alloc '%s' for missing task group '%s'", alloc.ID, alloc.TaskGroup)
		r.setStatus(structs.AllocClientStatusFailed, fmt.Sprintf("missing task group '%s'", alloc.TaskGroup))
		return
	}

	// Create the execution context
	r.ctx = driver.NewExecContext()

	// Start the task runners
	for _, task := range tg.Tasks {
		tr := NewTaskRunner(r, r.ctx, task)
		r.tasks[task.Name] = tr
		go tr.Run()
	}

OUTER:
	// Wait for updates
	for {
		select {
		case update := <-r.updateCh:
			// Check if we're in a terminal status
			if update.TerminalStatus() {
				r.setAlloc(update)
				break OUTER
			}

			// Update the task groups
			for _, task := range tg.Tasks {
				tr := r.tasks[task.Name]
				tr.Update(task)
			}

		case <-r.destroyCh:
			break OUTER
		}
	}

	// Destroy each sub-task
	for _, tr := range r.tasks {
		tr.Destroy()
	}

	// Wait for termination of the task runners
	for _, tr := range r.tasks {
		<-tr.WaitCh()
	}
	r.logger.Printf("[DEBUG] client: terminating runner for alloc '%s'", r.alloc.ID)
}

// Update is used to update the allocation of the context
func (r *AllocRunner) Update(update *structs.Allocation) {
	select {
	case r.updateCh <- update:
	default:
		r.logger.Printf("[ERR] client: dropping update to alloc '%s'", update.ID)
	}
}

// Destroy is used to indicate that the allocation context should be destroyed
func (r *AllocRunner) Destroy() {
	r.destroyLock.Lock()
	defer r.destroyLock.Unlock()

	if r.destroy {
		return
	}
	r.destroy = true
	close(r.destroyCh)
}
