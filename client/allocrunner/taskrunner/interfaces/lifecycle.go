package interfaces

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
)

type TaskLifecycle interface {
	Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error
	Signal(event *structs.TaskEvent, signal string) error
	Kill(ctx context.Context, event *structs.TaskEvent) error
}
