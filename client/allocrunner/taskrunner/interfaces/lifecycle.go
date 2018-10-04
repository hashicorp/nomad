package interfaces

import (
	"context"
	"os"

	"github.com/hashicorp/nomad/nomad/structs"
)

type TaskLifecycle interface {
	Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error
	Signal(event *structs.TaskEvent, s os.Signal) error
	Kill(ctx context.Context, event *structs.TaskEvent) error
}
