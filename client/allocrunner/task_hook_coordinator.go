package allocrunner

import (
	"context"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TaskHookCoordinator helps coordinates when main start tasks can launch
// namely after all Prestart Tasks have run, and after all BlockUntilCompleted have completed
type taskHookCoordinator struct {
	logger hclog.Logger

	closedCh chan struct{}

	mainTaskCtx       context.Context
	mainTaskCtxCancel func()

	prestartTasksUntilRunning   map[string]struct{}
	prestartTasksUntilCompleted map[string]struct{}
}

func newTaskHookCoordinator(logger hclog.Logger, tasks []*structs.Task) *taskHookCoordinator {
	closedCh := make(chan struct{})
	close(closedCh)

	mainTaskCtx, cancelFn := context.WithCancel(context.Background())

	c := &taskHookCoordinator{
		logger:                      logger,
		closedCh:                    closedCh,
		mainTaskCtx:                 mainTaskCtx,
		mainTaskCtxCancel:           cancelFn,
		prestartTasksUntilRunning:   map[string]struct{}{},
		prestartTasksUntilCompleted: map[string]struct{}{},
	}
	c.setTasks(tasks)
	return c
}

func (c *taskHookCoordinator) setTasks(tasks []*structs.Task) {
	for _, task := range tasks {
		if task.Lifecycle == nil || task.Lifecycle.Hook != structs.TaskLifecycleHookPrestart {
			// move nothing
			continue
		}

		// only working with prestart hooks here
		switch task.Lifecycle.BlockUntil {
		case structs.TaskLifecycleBlockUntilRunning:
			c.prestartTasksUntilRunning[task.Name] = struct{}{}
		case structs.TaskLifecycleBlockUntilCompleted:
			c.prestartTasksUntilCompleted[task.Name] = struct{}{}
		default:
			panic(fmt.Sprintf("unexpected block until value: %v", task.Lifecycle.BlockUntil))
		}
	}

	if len(c.prestartTasksUntilRunning)+len(c.prestartTasksUntilCompleted) == 0 {
		c.mainTaskCtxCancel()
	}
}

func (c *taskHookCoordinator) startConditionForTask(task *structs.Task) <-chan struct{} {
	if task.Lifecycle != nil && task.Lifecycle.Hook == structs.TaskLifecycleHookPrestart {
		return c.closedCh
	}

	return c.mainTaskCtx.Done()

}

func (c *taskHookCoordinator) taskStateUpdated(states map[string]*structs.TaskState) {
	if c.mainTaskCtx.Err() != nil {
		// nothing to do here
		return
	}

	for task := range c.prestartTasksUntilRunning {
		st := states[task]
		if st == nil || st.StartedAt.IsZero() {
			continue
		}

		delete(c.prestartTasksUntilRunning, task)
	}

	for task := range c.prestartTasksUntilCompleted {
		st := states[task]
		if st == nil || !st.Successful() {
			continue
		}

		delete(c.prestartTasksUntilCompleted, task)
	}

	// everything well
	if len(c.prestartTasksUntilRunning)+len(c.prestartTasksUntilCompleted) == 0 {
		c.mainTaskCtxCancel()
	}
}
