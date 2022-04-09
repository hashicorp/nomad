package allocrunner

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TaskHookCoordinator helps coordinate when mainTasks start tasks can launch
// namely after all Prestart Tasks have run, and after all BlockUntilCompleted have completed
type taskHookCoordinator struct {
	logger hclog.Logger

	// constant for quickly starting all prestart tasks
	closedCh chan struct{}

	// Each context is used to gate task runners launching the tasks. A task
	// runner waits until the context associated its lifecycle context is
	// done/cancelled.
	mainTaskCtx       context.Context
	mainTaskCtxCancel func()

	poststartTaskCtx       context.Context
	poststartTaskCtxCancel func()
	poststopTaskCtx        context.Context
	poststopTaskCtxCancel  context.CancelFunc

	prestartSidecar   map[string]struct{}
	prestartEphemeral map[string]struct{}
	mainTasksRunning  map[string]struct{} // poststop: main tasks running -> finished
	mainTasksPending  map[string]struct{} // poststart: main tasks pending -> running
}

func newTaskHookCoordinator(logger hclog.Logger, tasks []*structs.Task) *taskHookCoordinator {
	closedCh := make(chan struct{})
	close(closedCh)

	mainTaskCtx, mainCancelFn := context.WithCancel(context.Background())
	poststartTaskCtx, poststartCancelFn := context.WithCancel(context.Background())
	poststopTaskCtx, poststopTaskCancelFn := context.WithCancel(context.Background())

	c := &taskHookCoordinator{
		logger:                 logger,
		closedCh:               closedCh,
		mainTaskCtx:            mainTaskCtx,
		mainTaskCtxCancel:      mainCancelFn,
		prestartSidecar:        map[string]struct{}{},
		prestartEphemeral:      map[string]struct{}{},
		mainTasksRunning:       map[string]struct{}{},
		mainTasksPending:       map[string]struct{}{},
		poststartTaskCtx:       poststartTaskCtx,
		poststartTaskCtxCancel: poststartCancelFn,
		poststopTaskCtx:        poststopTaskCtx,
		poststopTaskCtxCancel:  poststopTaskCancelFn,
	}
	c.setTasks(tasks)
	return c
}

func (c *taskHookCoordinator) setTasks(tasks []*structs.Task) {
	for _, task := range tasks {

		if task.Lifecycle == nil {
			c.mainTasksPending[task.Name] = struct{}{}
			c.mainTasksRunning[task.Name] = struct{}{}
			continue
		}

		switch task.Lifecycle.Hook {
		case structs.TaskLifecycleHookPrestart:
			if task.Lifecycle.Sidecar {
				c.prestartSidecar[task.Name] = struct{}{}
			} else {
				c.prestartEphemeral[task.Name] = struct{}{}
			}
		case structs.TaskLifecycleHookPoststart:
			// Poststart hooks don't need to be tracked.
		case structs.TaskLifecycleHookPoststop:
			// Poststop hooks don't need to be tracked.
		default:
			c.logger.Error("invalid lifecycle hook", "task", task.Name, "hook", task.Lifecycle.Hook)
		}
	}

	if !c.hasPrestartTasks() {
		c.mainTaskCtxCancel()
	}
}

func (c *taskHookCoordinator) hasPrestartTasks() bool {
	return len(c.prestartSidecar)+len(c.prestartEphemeral) > 0
}

func (c *taskHookCoordinator) hasRunningMainTasks() bool {
	return len(c.mainTasksRunning) > 0
}

func (c *taskHookCoordinator) hasPendingMainTasks() bool {
	return len(c.mainTasksPending) > 0
}

func (c *taskHookCoordinator) startConditionForTask(task *structs.Task) <-chan struct{} {
	if task.Lifecycle == nil {
		return c.mainTaskCtx.Done()
	}

	switch task.Lifecycle.Hook {
	case structs.TaskLifecycleHookPrestart:
		// Prestart tasks start without checking status of other tasks
		return c.closedCh
	case structs.TaskLifecycleHookPoststart:
		return c.poststartTaskCtx.Done()
	case structs.TaskLifecycleHookPoststop:
		return c.poststopTaskCtx.Done()
	default:
		// it should never have a lifecycle stanza w/o a hook, so report an error but allow the task to start normally
		c.logger.Error("invalid lifecycle hook", "task", task.Name, "hook", task.Lifecycle.Hook)
		return c.mainTaskCtx.Done()
	}
}

// This is not thread safe! This must only be called from one thread per alloc runner.
func (c *taskHookCoordinator) taskStateUpdated(states map[string]*structs.TaskState) {
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

	for task := range c.mainTasksRunning {
		st := states[task]

		if st == nil || st.State != structs.TaskStateDead {
			continue
		}

		delete(c.mainTasksRunning, task)
	}

	for task := range c.mainTasksPending {
		st := states[task]
		if st == nil || st.StartedAt.IsZero() {
			continue
		}

		delete(c.mainTasksPending, task)
	}

	if !c.hasPrestartTasks() {
		c.mainTaskCtxCancel()
	}

	if !c.hasPendingMainTasks() {
		c.poststartTaskCtxCancel()
	}
	if !c.hasRunningMainTasks() {
		c.poststopTaskCtxCancel()
	}
}

func (c *taskHookCoordinator) StartPoststopTasks() {
	c.poststopTaskCtxCancel()
}

// hasNonSidecarTasks returns false if all the passed tasks are sidecar tasks
func hasNonSidecarTasks(tasks []*taskrunner.TaskRunner) bool {
	for _, tr := range tasks {
		lc := tr.Task().Lifecycle
		if lc == nil || !lc.Sidecar {
			return true
		}
	}

	return false
}

// hasSidecarTasks returns true if all the passed tasks are sidecar tasks
func hasSidecarTasks(tasks map[string]*taskrunner.TaskRunner) bool {
	for _, tr := range tasks {
		lc := tr.Task().Lifecycle
		if lc != nil && lc.Sidecar {
			return true
		}
	}

	return false
}
