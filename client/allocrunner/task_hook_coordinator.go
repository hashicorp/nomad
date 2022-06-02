package allocrunner

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner"
	"github.com/hashicorp/nomad/nomad/structs"
)

// TODO: killed deployment while prestart was stuck in running state>>>
//        [ERROR] client.alloc_runner.task_runner.task_hook.stats_hook:
//        failed to start stats collection for task: alloc_id=24546e00-09c1-6582-d589-85b63b5b9548
//        task=prestart error="task not found for given id"

// TODO: investigate when a task is marked as Dead
//       - prestart will never get to Dead if it's being blocked
//       - this blocks all tasks from being executed

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

	restartAllocCtx       context.Context
	restartAllocCtxCancel context.CancelFunc

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
	restartAllocCtx, restartAllocCancelFn := context.WithCancel(context.Background())

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
		restartAllocCtx:        restartAllocCtx,
		restartAllocCtxCancel:  restartAllocCancelFn,
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

// Tasks are able to exit the taskrunner Run() loop when poststop tasks are ready to start
// TODO: in case of a restart, poststop tasks will not be ready to run
//  - need to refresh the taskhookcoordinator so execution flow happens sequentially
func (c *taskHookCoordinator) endConditionForTask(task *structs.Task) <-chan struct{} {
	return c.restartAllocCtx.Done()
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

	// This unblocks the main tasks
	for task := range c.mainTasksRunning {
		st := states[task]

		if st == nil || st.State != structs.TaskStateDead {
			continue
		}

		delete(c.mainTasksRunning, task)
	}

	// This unblocks the PostStart tasks
	for task := range c.mainTasksPending {
		st := states[task]
		if st == nil || st.StartedAt.IsZero() {
			continue
		}

		delete(c.mainTasksPending, task)
	}

	// This unblocks the Main tasks
	if !c.hasPrestartTasks() {
		c.mainTaskCtxCancel()
	}

	// This unblocks the PostStart tasks
	if !c.hasPendingMainTasks() {
		c.poststartTaskCtxCancel()
	}

	// This unblocks the PostStop tasks
	if !c.hasRunningMainTasks() {
		c.poststopTaskCtxCancel()

		// TODO: realizing now that the naming is bad,
		//       you can unblock the endCondition in case of
		//       regular execution or in case of a restart
		c.restartAllocCtxCancel()
	}

}

func (c *taskHookCoordinator) RestartTaskHookCoordinator() {
	c.restartAllocCtxCancel()
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
