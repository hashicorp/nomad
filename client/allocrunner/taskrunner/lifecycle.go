package taskrunner

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Restart a task. Returns immediately if no task is running. Blocks until
// existing task exits or passed-in context is canceled.
func (tr *TaskRunner) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	tr.logger.Trace("Restart requested", "failure", failure)

	// Grab the handle
	handle := tr.getDriverHandle()

	// Check it is running
	if handle == nil {
		return ErrTaskNotRunning
	}

	// Emit the event since it may take a long time to kill
	tr.EmitEvent(event)

	// Run the pre-kill hooks prior to restarting the task
	tr.preKill()

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
	tr.logger.Trace("Kill requested", "event_type", event.Type, "event_reason", event.KillReason)

	// Cancel the task runner to break out of restart delay or the main run
	// loop.
	tr.killCtxCancel()

	// Emit kill event
	tr.EmitEvent(event)

	// Check if the Run method has started yet. If it hasn't we return early,
	// since the task hasn't even started so there is nothing to wait for. This
	// is still correct since the Run method no-op since the kill context has
	// already been cancelled.
	if !tr.hasRunLaunched() {
		return nil
	}

	select {
	case <-tr.WaitCh():
	case <-ctx.Done():
		return ctx.Err()
	}

	return tr.getKillErr()
}
