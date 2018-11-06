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

	// Run the hooks prior to restarting the task
	tr.killing()

	// Tell the restart tracker that a restart triggered the exit
	tr.restartTracker.SetRestartTriggered(failure)

	// Kill the task using an exponential backoff in-case of failures.
	if err := tr.killTask(handle); err != nil {
		// We couldn't successfully destroy the resource created.
		tr.logger.Error("failed to kill task. Resources may have been leaked", "error", err)
	}

	// Drain the wait channel or wait for the request context to be canceled
	waitCh, err := handle.WaitCh(ctx)
	if err != nil {
		return err
	}

	select {
	case <-waitCh:
	case <-ctx.Done():
	}
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
	tr.killing()

	// Tell the restart tracker that the task has been killed so it doesn't
	// attempt to restart it.
	tr.restartTracker.SetKilled()

	// Kill the task using an exponential backoff in-case of failures.
	killErr := tr.killTask(handle)
	if killErr != nil {
		// We couldn't successfully destroy the resource created.
		tr.logger.Error("failed to kill task. Resources may have been leaked", "error", killErr)
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

	select {
	case <-waitCh:
	case <-ctx.Done():
	}

	// Store that the task has been destroyed and any associated error.
	tr.UpdateState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled).SetKillError(killErr))

	if killErr != nil {
		return killErr
	} else if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}
