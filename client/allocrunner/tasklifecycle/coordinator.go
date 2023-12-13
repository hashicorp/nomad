// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tasklifecycle

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

// coordinatorState represents a state of the task lifecycle Coordinator FSM.
type coordinatorState uint8

const (
	coordinatorStateInit coordinatorState = iota
	coordinatorStatePrestart
	coordinatorStateMain
	coordinatorStatePoststart
	coordinatorStateWaitAlloc
	coordinatorStatePoststop
)

func (s coordinatorState) String() string {
	switch s {
	case coordinatorStateInit:
		return "init"
	case coordinatorStatePrestart:
		return "prestart"
	case coordinatorStateMain:
		return "main"
	case coordinatorStatePoststart:
		return "poststart"
	case coordinatorStateWaitAlloc:
		return "wait_alloc"
	case coordinatorStatePoststop:
		return "poststart"
	}
	panic(fmt.Sprintf("Unexpected task coordinator state %d", s))
}

// lifecycleStage represents a lifecycle configuration used for task
// coordination.
//
// Not all possible combinations of hook X sidecar are defined, only the ones
// that are relevant for coordinating task initialization order. For example, a
// main task with sidecar set to `true` starts at the same time as a
// non-sidecar main task, so there is no need to treat them differently.
type lifecycleStage uint8

const (
	// lifecycleStagePrestartEphemeral are tasks with the "prestart" hook and
	// sidecar set to "false".
	lifecycleStagePrestartEphemeral lifecycleStage = iota

	// lifecycleStagePrestartSidecar are tasks with the "prestart" hook and
	// sidecar set to "true".
	lifecycleStagePrestartSidecar

	// lifecycleStageMain are tasks without a lifecycle or a lifecycle with an
	// empty hook value.
	lifecycleStageMain

	// lifecycleStagePoststartEphemeral are tasks with the "poststart" hook and
	// sidecar set to "false"
	lifecycleStagePoststartEphemeral

	// lifecycleStagePoststartSidecar are tasks with the "poststart" hook and
	// sidecar set to "true".
	lifecycleStagePoststartSidecar

	// lifecycleStagePoststop are tasks with the "poststop" hook.
	lifecycleStagePoststop
)

// Coordinator controls when tasks with a given lifecycle configuration are
// allowed to start and run.
//
// It behaves like a finite state machine where each state transition blocks or
// allows some task lifecycle types to run.
type Coordinator struct {
	logger hclog.Logger

	// tasksByLifecycle is an index used to group and quickly access tasks by
	// their lifecycle stage.
	tasksByLifecycle map[lifecycleStage][]string

	// currentState is the current state of the FSM. It must only be accessed
	// while holding the lock.
	currentState     coordinatorState
	currentStateLock sync.RWMutex

	// gates store the gates that control each task lifecycle stage.
	gates map[lifecycleStage]*Gate
}

// NewCoordinator returns a new Coordinator with all tasks initially blocked.
func NewCoordinator(logger hclog.Logger, tasks []*structs.Task, shutdownCh <-chan struct{}) *Coordinator {
	c := &Coordinator{
		logger:           logger.Named("task_coordinator"),
		tasksByLifecycle: indexTasksByLifecycle(tasks),
		gates:            make(map[lifecycleStage]*Gate),
	}

	for lifecycle := range c.tasksByLifecycle {
		c.gates[lifecycle] = NewGate(shutdownCh)
	}

	c.enterStateLocked(coordinatorStateInit)
	return c
}

// Restart sets the Coordinator state back to "init" and is used to coordinate
// a full alloc restart. Since all tasks will run again they need to be pending
// before they are allowed to proceed.
func (c *Coordinator) Restart() {
	c.currentStateLock.Lock()
	defer c.currentStateLock.Unlock()
	c.enterStateLocked(coordinatorStateInit)
}

// Restore is used to set the Coordinator FSM to the correct state when an
// alloc is restored. Must be called before the allocrunner is running.
func (c *Coordinator) Restore(states map[string]*structs.TaskState) {
	// Skip the "init" state when restoring since the tasks were likely already
	// running, causing the Coordinator to be stuck waiting for them to be
	// "pending".
	c.enterStateLocked(coordinatorStatePrestart)
	c.TaskStateUpdated(states)
}

// StartConditionForTask returns a channel that is unblocked when the task is
// allowed to run.
func (c *Coordinator) StartConditionForTask(task *structs.Task) <-chan struct{} {
	lifecycle := taskLifecycleStage(task)
	return c.gates[lifecycle].WaitCh()
}

// TaskStateUpdated notifies that a task state has changed. This may cause the
// Coordinator to transition to another state.
func (c *Coordinator) TaskStateUpdated(states map[string]*structs.TaskState) {
	c.currentStateLock.Lock()
	defer c.currentStateLock.Unlock()

	// We may be able to move directly through some states (for example, when
	// an alloc doesn't have any prestart task we can skip the prestart state),
	// so loop until we stabilize.
	// This is also important when restoring an alloc since we need to find the
	// state where FSM was last positioned.
	for {
		nextState := c.nextStateLocked(states)
		if nextState == c.currentState {
			return
		}

		c.enterStateLocked(nextState)
	}
}

// nextStateLocked returns the state the FSM should transition to given its
// current internal state and the received states of the tasks.
// The currentStateLock must be held before calling this method.
func (c *Coordinator) nextStateLocked(states map[string]*structs.TaskState) coordinatorState {

	// coordinatorStatePoststop is the terminal state of the FSM, and can be
	// reached at any time.
	if c.isAllocDone(states) {
		return coordinatorStatePoststop
	}

	switch c.currentState {
	case coordinatorStateInit:
		if !c.isInitDone(states) {
			return coordinatorStateInit
		}
		return coordinatorStatePrestart

	case coordinatorStatePrestart:
		if !c.isPrestartDone(states) {
			return coordinatorStatePrestart
		}
		return coordinatorStateMain

	case coordinatorStateMain:
		if !c.isMainDone(states) {
			return coordinatorStateMain
		}
		return coordinatorStatePoststart

	case coordinatorStatePoststart:
		if !c.isPoststartDone(states) {
			return coordinatorStatePoststart
		}
		return coordinatorStateWaitAlloc

	case coordinatorStateWaitAlloc:
		if !c.isAllocDone(states) {
			return coordinatorStateWaitAlloc
		}
		return coordinatorStatePoststop

	case coordinatorStatePoststop:
		return coordinatorStatePoststop
	}

	// If the code reaches here it's a programming error, since the switch
	// statement should cover all possible states and return the next state.
	panic(fmt.Sprintf("unexpected state %s", c.currentState))
}

// enterStateLocked updates the current state of the Coordinator FSM and
// executes any action necessary for the state transition.
// The currentStateLock must be held before calling this method.
func (c *Coordinator) enterStateLocked(state coordinatorState) {
	c.logger.Trace("state transition", "from", c.currentState, "to", state)

	switch state {
	case coordinatorStateInit:
		c.block(lifecycleStagePrestartEphemeral)
		c.block(lifecycleStagePrestartSidecar)
		c.block(lifecycleStageMain)
		c.block(lifecycleStagePoststartEphemeral)
		c.block(lifecycleStagePoststartSidecar)
		c.block(lifecycleStagePoststop)

	case coordinatorStatePrestart:
		c.block(lifecycleStageMain)
		c.block(lifecycleStagePoststartEphemeral)
		c.block(lifecycleStagePoststartSidecar)
		c.block(lifecycleStagePoststop)

		c.allow(lifecycleStagePrestartEphemeral)
		c.allow(lifecycleStagePrestartSidecar)

	case coordinatorStateMain:
		c.block(lifecycleStagePrestartEphemeral)
		c.block(lifecycleStagePoststartEphemeral)
		c.block(lifecycleStagePoststartSidecar)
		c.block(lifecycleStagePoststop)

		c.allow(lifecycleStagePrestartSidecar)
		c.allow(lifecycleStageMain)

	case coordinatorStatePoststart:
		c.block(lifecycleStagePrestartEphemeral)
		c.block(lifecycleStagePoststop)

		c.allow(lifecycleStagePrestartSidecar)
		c.allow(lifecycleStageMain)
		c.allow(lifecycleStagePoststartEphemeral)
		c.allow(lifecycleStagePoststartSidecar)

	case coordinatorStateWaitAlloc:
		c.block(lifecycleStagePrestartEphemeral)
		c.block(lifecycleStagePoststartEphemeral)
		c.block(lifecycleStagePoststop)

		c.allow(lifecycleStagePrestartSidecar)
		c.allow(lifecycleStageMain)
		c.allow(lifecycleStagePoststartSidecar)

	case coordinatorStatePoststop:
		c.block(lifecycleStagePrestartEphemeral)
		c.block(lifecycleStagePrestartSidecar)
		c.block(lifecycleStageMain)
		c.block(lifecycleStagePoststartEphemeral)
		c.block(lifecycleStagePoststartSidecar)

		c.allow(lifecycleStagePoststop)
	}

	c.currentState = state
}

// isInitDone returns true when the following conditions are met:
//   - all tasks are in the "pending" state.
func (c *Coordinator) isInitDone(states map[string]*structs.TaskState) bool {
	for _, task := range states {
		if task.State != structs.TaskStatePending {
			return false
		}
	}
	return true
}

// isPrestartDone returns true when the following conditions are met:
//   - there is at least one prestart task
//   - all ephemeral prestart tasks are successful.
//   - no ephemeral prestart task has failed.
//   - all prestart sidecar tasks are running.
func (c *Coordinator) isPrestartDone(states map[string]*structs.TaskState) bool {
	if !c.hasPrestart() {
		return true
	}

	for _, task := range c.tasksByLifecycle[lifecycleStagePrestartEphemeral] {
		if !states[task].Successful() {
			return false
		}
	}
	for _, task := range c.tasksByLifecycle[lifecycleStagePrestartSidecar] {
		if states[task].State != structs.TaskStateRunning {
			return false
		}
	}
	return true
}

// isMainDone returns true when the following conditions are met:
//   - there is at least one main task.
//   - all main tasks are no longer "pending".
func (c *Coordinator) isMainDone(states map[string]*structs.TaskState) bool {
	if !c.hasMain() {
		return true
	}

	for _, task := range c.tasksByLifecycle[lifecycleStageMain] {
		if states[task].State == structs.TaskStatePending {
			return false
		}
	}
	return true
}

// isPoststartDone returns true when the following conditions are met:
//   - there is at least one poststart task.
//   - all ephemeral poststart tasks are in the "dead" state.
func (c *Coordinator) isPoststartDone(states map[string]*structs.TaskState) bool {
	if !c.hasPoststart() {
		return true
	}

	for _, task := range c.tasksByLifecycle[lifecycleStagePoststartEphemeral] {
		if states[task].State != structs.TaskStateDead {
			return false
		}
	}
	return true
}

// isAllocDone returns true when the following conditions are met:
//   - all non-poststop tasks are in the "dead" state.
func (c *Coordinator) isAllocDone(states map[string]*structs.TaskState) bool {
	for lifecycle, tasks := range c.tasksByLifecycle {
		if lifecycle == lifecycleStagePoststop {
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

func (c *Coordinator) hasPrestart() bool {
	return len(c.tasksByLifecycle[lifecycleStagePrestartEphemeral])+
		len(c.tasksByLifecycle[lifecycleStagePrestartSidecar]) > 0
}

func (c *Coordinator) hasMain() bool {
	return len(c.tasksByLifecycle[lifecycleStageMain]) > 0
}

func (c *Coordinator) hasPoststart() bool {
	return len(c.tasksByLifecycle[lifecycleStagePoststartEphemeral])+
		len(c.tasksByLifecycle[lifecycleStagePoststartSidecar]) > 0
}

func (c *Coordinator) hasPoststop() bool {
	return len(c.tasksByLifecycle[lifecycleStagePoststop]) > 0
}

// block is used to block the execution of tasks in the given lifecycle stage.
func (c *Coordinator) block(lifecycle lifecycleStage) {
	gate := c.gates[lifecycle]
	if gate != nil {
		gate.Close()
	}
}

// allows is used to allow the execution of tasks in the given lifecycle stage.
func (c *Coordinator) allow(lifecycle lifecycleStage) {
	gate := c.gates[lifecycle]
	if gate != nil {
		gate.Open()
	}
}

// indexTasksByLifecycle generates a map that groups tasks by their lifecycle
// configuration. This makes it easier to retrieve tasks by these groups or to
// determine if a task has a certain lifecycle configuration.
func indexTasksByLifecycle(tasks []*structs.Task) map[lifecycleStage][]string {
	index := make(map[lifecycleStage][]string)

	for _, task := range tasks {
		lifecycle := taskLifecycleStage(task)

		if _, ok := index[lifecycle]; !ok {
			index[lifecycle] = []string{}
		}
		index[lifecycle] = append(index[lifecycle], task.Name)
	}

	return index
}

// taskLifecycleStage returns the relevant lifecycle stage for a given task.
func taskLifecycleStage(task *structs.Task) lifecycleStage {
	if task.IsPrestart() {
		if task.Lifecycle.Sidecar {
			return lifecycleStagePrestartSidecar
		}
		return lifecycleStagePrestartEphemeral
	} else if task.IsPoststart() {
		if task.Lifecycle.Sidecar {
			return lifecycleStagePoststartSidecar
		}
		return lifecycleStagePoststartEphemeral
	} else if task.IsPoststop() {
		return lifecycleStagePoststop
	}

	// Assume task is "main" by default.
	return lifecycleStageMain
}
