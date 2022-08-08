package allocrunner

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// taskCoordinatorState represents a state of the task coordinatator FSM.
type taskCoordinatorState string

const (
	taskCoordinatorStateInit      taskCoordinatorState = "init"
	taskCoordinatorStatePrestart  taskCoordinatorState = "prestart"
	taskCoordinatorStateMain      taskCoordinatorState = "main"
	taskCoordinatorStatePoststart taskCoordinatorState = "poststart"
	taskCoordinatorStatePoststop  taskCoordinatorState = "poststop"
)

// taskLifecycle represents a lifecycle configuration of interest for the task
// coordination. Not all possible combinations of hook x sidecar are defined,
// only the ones that are relevant for task coordinator.
type taskLifecycle string

const (
	// taskLifecyclePrestartEphemeral are tasks with the "prestart" hook and
	// sidecar set to "false".
	taskLifecyclePrestartEphemeral taskLifecycle = "prestart_ephemeral"

	// taskLifecyclePrestartSidecar are tasks with the "prestart" hook and
	// sidecar set to "true".
	taskLifecyclePrestartSidecar taskLifecycle = "prestart_sidecar"

	// taskLifecycleMain are tasks without a lifecycle or a lifecycle with an
	// empty hook value.
	taskLifecycleMain taskLifecycle = "main"

	// taskLifecyclePoststart are tasks with the "poststart" hook.
	taskLifecyclePoststart taskLifecycle = "poststart"

	// taskLifecyclePoststop are tasks with the "poststop" hook.
	taskLifecyclePoststop taskLifecycle = "poststop"
)

// taskCoordinator controls when tasks with a given lifecycle configuration are
// allowed to run.
//
// It behaves like a finite state machine where each state transition blocks or
// allows some task lifecycle type to run.
type taskCoordinator struct {
	logger hclog.Logger

	// tasksByLifecycle is an index used to group and quickly access tasks by
	// their lifecycle configuration.
	tasksByLifecycle map[taskLifecycle][]string

	// currentState is the current state of the FSM. It must only be accessed
	// while holding the lock.
	currentState     taskCoordinatorState
	currentStateLock sync.RWMutex

	// controllers store a controller for each task lifecycle.
	controllers map[taskLifecycle]*taskCoordinatorController
}

// newTaskCoordinator returns a new taskCoordinator in the "init" state and
// with all tasks blocked.
func newTaskCoordinator(logger hclog.Logger, tasks []*structs.Task, shutdownCh <-chan struct{}) *taskCoordinator {
	c := &taskCoordinator{
		logger:           logger.Named("task_coordinator"),
		tasksByLifecycle: indexTasksByLifecycle(tasks),
		controllers:      make(map[taskLifecycle]*taskCoordinatorController),
	}

	for lifecycle := range c.tasksByLifecycle {
		ctrl := newTaskCoordinatorController()
		go ctrl.run(shutdownCh)

		c.controllers[lifecycle] = ctrl
	}

	c.enterStateLocked(taskCoordinatorStateInit)
	return c
}

// restore is used to set the coordinatator FSM to the correct state when an
// alloc is restored. Must be called before the allocrunner is running.
func (c *taskCoordinator) restore(states map[string]*structs.TaskState) {
	// Skip the "init" state when restoring since the tasks were likelly
	// already running.
	c.currentStateLock.Lock()
	c.enterStateLocked(taskCoordinatorStatePrestart)
	c.currentStateLock.Unlock()

	c.taskStateUpdated(states)
}

// startConditionForTask returns a channel that a task runner can use to wait
// until it is allowed to run.
func (c *taskCoordinator) startConditionForTask(task *structs.Task) <-chan struct{} {
	var lifecycle taskLifecycle

	if task.IsPrestart() {
		if task.Lifecycle.Sidecar {
			lifecycle = taskLifecyclePrestartSidecar
		} else {
			lifecycle = taskLifecyclePrestartEphemeral
		}
	} else if task.IsPoststart() {
		lifecycle = taskLifecyclePoststart
	} else if task.IsPoststop() {
		lifecycle = taskLifecyclePoststop
	} else {
		// Assume task is "main" by default.
		lifecycle = taskLifecycleMain
	}

	return c.controllers[lifecycle].waitCh()
}

// taskStateUpdated notifies that a task state has changed. This may cause the
// taskCoordinator FSM to progresses to another state.
func (c *taskCoordinator) taskStateUpdated(states map[string]*structs.TaskState) {
	c.currentStateLock.Lock()
	defer c.currentStateLock.Unlock()

	// When restoring an alloc it's likelly that the coordinatator FSM is not
	// in the right state, so we need to loop and advance until we find a
	// stable state that matches the given states of the tasks.
	for {
		nextState := c.nextStateLocked(states)
		if nextState == c.currentState {
			return
		}

		c.enterStateLocked(nextState)
	}
}

// nextState returns the state the FSM should transition to given its current
// internal state and the received states of the tasks.
// The currentStateLock must be held before calling this method.
func (c *taskCoordinator) nextStateLocked(states map[string]*structs.TaskState) taskCoordinatorState {
	switch c.currentState {
	case taskCoordinatorStateInit:
		if !c.isInitDone(states) {
			return taskCoordinatorStateInit
		}

		if c.hasPrestart() {
			return taskCoordinatorStatePrestart
		}
		if c.hasMain() {
			return taskCoordinatorStateMain
		}
		if c.hasPoststart() {
			return taskCoordinatorStatePoststart
		}
		if c.hasPoststop() {
			return taskCoordinatorStatePoststop
		}

	case taskCoordinatorStatePrestart:
		if !c.isPrestartDone(states) {
			return taskCoordinatorStatePrestart
		}

		if c.hasMain() {
			return taskCoordinatorStateMain
		}
		if c.hasPoststart() {
			return taskCoordinatorStatePoststart
		}
		if c.hasPoststop() {
			return taskCoordinatorStatePoststop
		}

	case taskCoordinatorStateMain:
		if !c.isMainDone(states) {
			return taskCoordinatorStateMain
		}

		if c.hasPoststart() {
			return taskCoordinatorStatePoststart
		}
		if c.hasPoststop() {
			return taskCoordinatorStatePoststop
		}

	case taskCoordinatorStatePoststart:
		if !c.isPoststartDone(states) {
			return taskCoordinatorStatePoststart
		}

		if c.hasPoststop() {
			return taskCoordinatorStatePoststop
		}

	case taskCoordinatorStatePoststop:
		return taskCoordinatorStatePoststop
	}

	// If the code reaches here it's a programming error, since the switch
	// statement should cover all possible states.
	c.logger.Warn(fmt.Sprintf("unexpected state %s", c.currentState))
	return c.currentState
}

// enterState updates the current state and executes any action necessary for
// the state transition.
// The currentStateLock must be held before calling this method.
func (c *taskCoordinator) enterStateLocked(state taskCoordinatorState) {
	c.logger.Trace("state transition", "from", c.currentState, "to", state)

	switch state {
	case taskCoordinatorStateInit:
		c.block(taskLifecyclePrestartEphemeral)
		c.block(taskLifecyclePrestartSidecar)
		c.block(taskLifecycleMain)
		c.block(taskLifecyclePoststart)
		c.block(taskLifecyclePoststop)

	case taskCoordinatorStatePrestart:
		c.block(taskLifecycleMain)
		c.block(taskLifecyclePoststart)
		c.block(taskLifecyclePoststop)

		c.allow(taskLifecyclePrestartEphemeral)
		c.allow(taskLifecyclePrestartSidecar)

	case taskCoordinatorStateMain:
		c.block(taskLifecyclePrestartEphemeral)
		c.block(taskLifecyclePoststart)
		c.block(taskLifecyclePoststop)

		c.allow(taskLifecyclePrestartSidecar)
		c.allow(taskLifecycleMain)

	case taskCoordinatorStatePoststart:
		c.block(taskLifecyclePrestartEphemeral)
		c.block(taskLifecyclePoststop)

		c.allow(taskLifecyclePrestartSidecar)
		c.allow(taskLifecycleMain)
		c.allow(taskLifecyclePoststart)

	case taskCoordinatorStatePoststop:
		c.block(taskLifecyclePrestartEphemeral)
		c.block(taskLifecyclePrestartSidecar)
		c.block(taskLifecycleMain)
		c.block(taskLifecyclePoststart)

		c.allow(taskLifecyclePoststop)
	}

	c.currentState = state
}

// isInitDone returns true when the following conditions are met:
//   - all tasks are in the "pending" state.
func (c *taskCoordinator) isInitDone(states map[string]*structs.TaskState) bool {
	for _, task := range states {
		if task.State != structs.TaskStatePending {
			return false
		}
	}
	return true
}

// isPrestartDone returns true when the following conditions are met:
//   - all ephemeral prestart tasks are in the "dead" state.
//   - no ephemeral prestart task has failed.
//   - all prestart sidecar tasks are running.
func (c *taskCoordinator) isPrestartDone(states map[string]*structs.TaskState) bool {
	for _, task := range c.tasksByLifecycle[taskLifecyclePrestartEphemeral] {
		if states[task].State != structs.TaskStateDead || states[task].Failed {
			return false
		}
	}
	for _, task := range c.tasksByLifecycle[taskLifecyclePrestartSidecar] {
		if states[task].State != structs.TaskStateRunning {
			return false
		}
	}
	return true
}

// isMainDone returns true when the following conditions are met:
//   - all main tasks are no longer "pending".
func (c *taskCoordinator) isMainDone(states map[string]*structs.TaskState) bool {
	for _, task := range c.tasksByLifecycle[taskLifecycleMain] {
		if states[task].State == structs.TaskStatePending {
			return false
		}
	}
	return true
}

// isPoststartDone returns true when the following conditions are met:
//   - all tasks that are not poststop are in the "dead" state.
func (c *taskCoordinator) isPoststartDone(states map[string]*structs.TaskState) bool {
	for lifecycle, tasks := range c.tasksByLifecycle {
		if lifecycle == taskLifecyclePoststop {
			continue
		}

		for _, task := range tasks {
			if states[task].State != structs.TaskStateDead {
				return false
			}
		}
	}
	return true
}

func (c *taskCoordinator) hasPrestart() bool {
	return len(c.tasksByLifecycle[taskLifecyclePrestartEphemeral])+
		len(c.tasksByLifecycle[taskLifecyclePrestartSidecar]) > 0
}

func (c *taskCoordinator) hasMain() bool {
	return len(c.tasksByLifecycle[taskLifecycleMain]) > 0
}

func (c *taskCoordinator) hasPoststart() bool {
	return len(c.tasksByLifecycle[taskLifecyclePoststart]) > 0
}

func (c *taskCoordinator) hasPoststop() bool {
	return len(c.tasksByLifecycle[taskLifecyclePoststop]) > 0
}

func (c *taskCoordinator) block(lifecycle taskLifecycle) {
	ctrl := c.controllers[lifecycle]
	if ctrl != nil {
		ctrl.block()
	}
}

func (c *taskCoordinator) allow(lifecycle taskLifecycle) {
	ctrl := c.controllers[lifecycle]
	if ctrl != nil {
		ctrl.allow()
	}
}

// indexTasksByLifecycle generates a map that groups tasks by their lifecycle
// configuration. This makes it easier to retrieve tasks by these groups or to
// determine if a task has a certain lifecycle configuration.
func indexTasksByLifecycle(tasks []*structs.Task) map[taskLifecycle][]string {
	index := make(map[taskLifecycle][]string)

	for _, task := range tasks {
		var hook string
		var sidecar bool
		if task.Lifecycle != nil {
			hook = task.Lifecycle.Hook
			sidecar = task.Lifecycle.Sidecar
		}

		var key taskLifecycle
		switch hook {
		case structs.TaskLifecycleHookPrestart:
			if sidecar {
				key = taskLifecyclePrestartSidecar
			} else {
				key = taskLifecyclePrestartEphemeral
			}
		case structs.TaskLifecycleHookPoststart:
			key = taskLifecyclePoststart
		case structs.TaskLifecycleHookPoststop:
			key = taskLifecyclePoststop
		case "":
			key = taskLifecycleMain
		}

		if _, ok := index[key]; !ok {
			index[key] = []string{}
		}
		index[key] = append(index[key], task.Name)
	}

	return index
}
