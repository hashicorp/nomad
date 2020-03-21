package allocrunner

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TaskHookCoordinator helps coordinate when main start tasks can launch
// namely after all Prestart Tasks have run, and after all BlockUntilCompleted have completed
type taskHookCoordinator struct {
	logger hclog.Logger

	closedCh chan struct{}

	mainTaskCtx       context.Context
	mainTaskCtxCancel func()

	prestartSidecar   map[string]struct{}
	prestartEphemeral map[string]struct{}
}

func newTaskHookCoordinator(logger hclog.Logger, tasks []*structs.Task) *taskHookCoordinator {
	closedCh := make(chan struct{})
	close(closedCh)

	mainTaskCtx, cancelFn := context.WithCancel(context.Background())

	c := &taskHookCoordinator{
		logger:            logger,
		closedCh:          closedCh,
		mainTaskCtx:       mainTaskCtx,
		mainTaskCtxCancel: cancelFn,
		prestartSidecar:   map[string]struct{}{},
		prestartEphemeral: map[string]struct{}{},
	}
	c.setTasks(tasks)
	return c
}

func (c *taskHookCoordinator) setTasks(tasks []*structs.Task) {
	for _, task := range tasks {

		if task.Lifecycle == nil {
			// move nothing
			continue
		}

		switch task.Lifecycle.Hook {
		case structs.TaskLifecycleHookPrestart:
			if task.Lifecycle.Sidecar {
				c.prestartSidecar[task.Name] = struct{}{}
			} else {
				c.prestartEphemeral[task.Name] = struct{}{}
			}

		default:
			c.logger.Error("invalid lifecycle hook", "hook", task.Lifecycle.Hook)
		}
	}

	if !c.hasPrestartTasks() {
		c.mainTaskCtxCancel()
	}
}

func (c *taskHookCoordinator) hasPrestartTasks() bool {
	return len(c.prestartSidecar)+len(c.prestartEphemeral) > 0
}

func (c *taskHookCoordinator) startConditionForTask(task *structs.Task) <-chan struct{} {
	if task.Lifecycle != nil && task.Lifecycle.Hook == structs.TaskLifecycleHookPrestart {
		return c.closedCh
	}

	return c.mainTaskCtx.Done()

}

// This is not thread safe! This must only be called from one thread per alloc runner.
func (c *taskHookCoordinator) taskStateUpdated(states map[string]*structs.TaskState) {
	if c.mainTaskCtx.Err() != nil {
		// nothing to do here
		return
	}

	for task := range c.prestartSidecar {
		st := states[task]
		if st == nil || st.StartedAt.IsZero() {
			continue
		}

		delete(c.prestartSidecar, task)
	}

	for task := range c.prestartEphemeral {
		st := states[task]
		if st == nil || !st.Successful() {
			continue
		}

		delete(c.prestartEphemeral, task)
	}

	// everything well
	if !c.hasPrestartTasks() {
		c.mainTaskCtxCancel()
	}
}
