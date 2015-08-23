package client

import (
	"log"
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
)

// TaskRunner is used to wrap a task within an allocation and provide the execution context.
type TaskRunner struct {
	ctx    *AllocRunner
	logger *log.Logger

	task *structs.Task

	updateCh chan *structs.Task

	destroy     bool
	destroyCh   chan struct{}
	destroyLock sync.Mutex
}

// NewTaskRunner is used to create a new task context
func NewTaskRunner(ctx *AllocRunner, task *structs.Task) *TaskRunner {
	tc := &TaskRunner{
		ctx:       ctx,
		logger:    ctx.logger,
		task:      task,
		updateCh:  make(chan *structs.Task, 8),
		destroyCh: make(chan struct{}),
	}
	return tc
}

// Run is a long running routine used to manage the task
func (r *TaskRunner) Run() {
	r.logger.Printf("[DEBUG] client: starting task context for '%s' (alloc '%s')", r.task.Name, r.ctx.Alloc().ID)

	// TODO: Start
	for {
		select {
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
		r.logger.Printf("[ERR] client: dropping task update '%s' (alloc '%s')", update.Name, r.ctx.Alloc().ID)
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
