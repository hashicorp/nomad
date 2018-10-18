package taskrunner

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// Restart a task. Returns immediately if no task is running. Blocks until
// existing task exits or passed-in context is canceled.
func (tr *TaskRunner) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	// Grab the handle
	handle := tr.getDriverHandle()
	// Check it is running
	if handle == nil {
		return ErrTaskNotRunning
	}

	// Emit the event since it may take a long time to kill
	tr.EmitEvent(event)

	// Tell the restart tracker that a restart triggered the exit
	tr.restartTracker.SetRestartTriggered(failure)

	// Kill the task using an exponential backoff in-case of failures.
	destroySuccess, err := tr.handleDestroy(handle)
	if !destroySuccess {
		// We couldn't successfully destroy the resource created.
		tr.logger.Error("failed to kill task. Resources may have been leaked", "error", err)
	}

	// Drain the wait channel or wait for the request context to be canceled
	waitCh, err := handle.WaitCh(ctx)
	if err != nil {
		return err
	}

	<-waitCh
	return nil
}

func (tr *TaskRunner) Signal(event *structs.TaskEvent, s string) error {
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
	// Cancel the task runner to break out of restart delay or the main run
	// loop.
	tr.killCtxCancel()

	// Grab the handle
	handle := tr.getDriverHandle()

	// Check it is running
	if handle == nil {
		return ErrTaskNotRunning
	}

	// Emit the event since it may take a long time to kill
	tr.EmitEvent(event)

	// Run the hooks prior to killing the task
	tr.kill()

	// Tell the restart tracker that the task has been killed
	tr.restartTracker.SetKilled()

	// Kill the task using an exponential backoff in-case of failures.
	destroySuccess, destroyErr := tr.handleDestroy(handle)
	if !destroySuccess {
		// We couldn't successfully destroy the resource created.
		tr.logger.Error("failed to kill task. Resources may have been leaked", "error", destroyErr)
	}

	// Block until task has exited.
	waitCh, err := handle.WaitCh(ctx)

	// The error should be nil or TaskNotFound, if it's something else then a
	// failure in the driver or transport layer occurred
	if err != nil {
		if err == drivers.ErrTaskNotFound {
			return nil
		}
		tr.logger.Error("failed to wait on task. Resources may have been leaked", "error", err)
		return err
	}

	<-waitCh

	// Store that the task has been destroyed and any associated error.
	tr.UpdateState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled).SetKillError(destroyErr))

	if destroyErr != nil {
		return destroyErr
	} else if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}
