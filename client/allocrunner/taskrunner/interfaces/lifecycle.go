package interfaces

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
)

type TaskLifecycle interface {
	// Restart a task in place. If failure=false then the restart does not
	// count as an attempt in the restart policy.
	Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error

	// Sends a signal to a task.
	Signal(event *structs.TaskEvent, signal string) error

	// Kill a task permanently.
	Kill(ctx context.Context, event *structs.TaskEvent) error

	// IsRunning returns true if the task runner has a handle to the task
	// driver, which is useful for distinguishing restored tasks during
	// prestart hooks. But note that the driver handle could go away after you
	// check this, so callers should make sure they're handling that case
	// safely. Ideally prestart hooks should be idempotnent whenever possible
	// to handle restored tasks; use this as an escape hatch.
	IsRunning() bool
}
