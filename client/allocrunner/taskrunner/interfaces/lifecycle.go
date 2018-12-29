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
}
