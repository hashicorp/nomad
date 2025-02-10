// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Restart restarts a task that is already running. Returns an error if the
// task is not running. Blocks until existing task exits or passed-in context
// is canceled.
func (tr *TaskRunner) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	tr.logger.Trace("Restart requested", "failure", failure, "event", event.GoString())

	taskState := tr.TaskState()
	if taskState == nil {
		return ErrTaskNotRunning
	}

	switch taskState.State {
	case structs.TaskStatePending, structs.TaskStateDead:
		return ErrTaskNotRunning
	}

	return tr.restartImpl(ctx, event, failure)
}

// ForceRestart restarts a task that is already running or reruns it if dead.
// Returns an error if the task is not able to rerun. Blocks until existing
// task exits or passed-in context is canceled.
//
// Callers must restart the AllocRuner taskCoordinator beforehand to make sure
// the task will be able to run again.
func (tr *TaskRunner) ForceRestart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	tr.logger.Trace("Force restart requested", "failure", failure, "event", event.GoString())

	taskState := tr.TaskState()
	if taskState == nil {
		return ErrTaskNotRunning
	}

	tr.stateLock.Lock()
	localState := tr.localState.Copy()
	tr.stateLock.Unlock()

	if localState == nil {
		return ErrTaskNotRunning
	}

	switch taskState.State {
	case structs.TaskStatePending:
		return ErrTaskNotRunning

	case structs.TaskStateDead:
		// Tasks that are in the "dead" state are only allowed to restart if
		// their Run() method is still active.
		if localState.RunComplete {
			return ErrTaskNotRunning
		}
	}

	return tr.restartImpl(ctx, event, failure)
}

// restartImpl implements to task restart process.
//
// It should never be called directly as it doesn't verify if the task state
// allows for a restart.
func (tr *TaskRunner) restartImpl(ctx context.Context, event *structs.TaskEvent, failure bool) error {

	// Check if the task is able to restart based on its state and the type of
	// restart event that was triggered.
	taskState := tr.TaskState()
	if taskState == nil {
		return ErrTaskNotRunning
	}

	// Emit the event since it may take a long time to kill
	tr.EmitEvent(event)

	// Tell the restart tracker that a restart triggered the exit
	tr.restartTracker.SetRestartTriggered(failure)

	// Signal a restart to unblock tasks that are in the "dead" state, but
	// don't block since the channel is buffered. Only one signal is enough to
	// notify the tr.Run() loop.
	// The channel must be signaled after SetRestartTriggered is called so the
	// tr.Run() loop runs again.
	if taskState.State == structs.TaskStateDead {
		select {
		case tr.restartCh <- struct{}{}:
		default:
		}
	}

	// Grab the handle to see if the task is still running and needs to be
	// killed.
	handle := tr.getDriverHandle()
	if handle == nil {
		return nil
	}

	// Run the pre-kill hooks prior to restarting the task
	tr.preKill()

	// Grab a handle to the wait channel that will timeout with context cancelation
	// _before_ killing the task.
	waitCh, err := handle.WaitCh(ctx)
	if err != nil {
		return err
	}

	// Kill the task using an exponential backoff in-case of failures.
	if _, err := tr.killTask(handle, waitCh); err != nil {
		// We couldn't successfully destroy the resource created.
		tr.logger.Error("failed to kill task. Resources may have been leaked", "error", err)
	}

	select {
	case <-waitCh:
	case <-ctx.Done():
	}
	return nil
}

func (tr *TaskRunner) Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error) {
	tr.logger.Trace("Exec requested")

	handle := tr.getDriverHandle()
	if handle == nil {
		return nil, 0, ErrTaskNotRunning
	}

	out, code, err := handle.Exec(timeout, cmd, args)
	return out, code, err
}

func (tr *TaskRunner) Signal(event *structs.TaskEvent, s string) error {
	tr.logger.Trace("Signal requested", "signal", s)

	// Grab the handle
	handle := tr.getDriverHandle()

	// Check it is running
	if handle == nil {
		return ErrTaskNotRunning
	}

	// Emit the event
	tr.EmitEvent(event)

	// Send the signal
	return handle.Signal(s)
}

// Kill a task. Blocks until task exits or context is canceled. State is set to
// dead.
func (tr *TaskRunner) Kill(ctx context.Context, event *structs.TaskEvent) error {
	tr.logger.Trace("Kill requested")

	// Cancel the task runner to break out of restart delay or the main run
	// loop.
	tr.killCtxCancel()

	// Emit kill event
	if event != nil {
		tr.logger.Trace("Kill event", "event_type", event.Type, "event_reason", event.KillReason)
		tr.EmitEvent(event)
	}

	select {
	case <-tr.WaitCh():
	case <-ctx.Done():
		return ctx.Err()
	}

	return tr.getKillErr()
}

func (tr *TaskRunner) IsRunning() bool {
	return tr.getDriverHandle() != nil
}
