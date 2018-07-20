package taskrunner

import (
	"context"
	"os"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (tr *TaskRunner) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	// Grab the handle
	handle, _ := tr.getDriverHandle()

	// Check it is running
	if handle == nil {
		return ErrTaskNotRunning
	}

	// Emit the event
	tr.EmitEvent(event)

	// Tell the restart tracker that a restart triggered the exit
	tr.restartTracker.SetRestartTriggered(failure)

	// Kill the task using an exponential backoff in-case of failures.
	destroySuccess, err := tr.handleDestroy(handle)
	if !destroySuccess {
		// We couldn't successfully destroy the resource created.
		tr.logger.Error("failed to kill task. Resources may have been leaked", "error", err)
	}

	// Drain the wait channel or wait for the request context to be cancelled
	select {
	case <-handle.WaitCh():
	case <-ctx.Done():
		return ctx.Err()
	}

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

func (tr *TaskRunner) Kill(ctx context.Context, event *structs.TaskEvent) error {
	// Grab the handle
	handle, _ := tr.getDriverHandle()

	// Check if the handle is running
	if handle == nil {
		return ErrTaskNotRunning
	}

	// Emit the event
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

	// Drain the wait channel or wait for the request context to be cancelled
	select {
	case <-handle.WaitCh():
	case <-ctx.Done():
	}

	// Store that the task has been destroyed and any associated error.
	tr.SetState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled).SetKillError(destroyErr))

	if destroyErr != nil {
		return destroyErr
	} else if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}
