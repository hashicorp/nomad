package taskrunner

import (
	"context"
	"os"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Restart a task. Returns immediately if no task is running. Blocks until
// existing task exits or passed-in context is canceled.
func (tr *TaskRunner) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	// Grab the handle
	handle, result := tr.getDriverHandle()

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
	result.Wait(ctx)
	return nil
}

func (tr *TaskRunner) Signal(event *structs.TaskEvent, s os.Signal) error {
	// Grab the handle
	handle, _ := tr.getDriverHandle()

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
	tr.ctxCancel()

	// Grab the handle
	handle, result := tr.getDriverHandle()

	// Check if the handle is running
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
	result.Wait(ctx)

	// Store that the task has been destroyed and any associated error.
	tr.UpdateState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled).SetKillError(destroyErr))

	if destroyErr != nil {
		return destroyErr
	} else if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}
