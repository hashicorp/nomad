package stream

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
)

// SinkWriter is the interface used by a ManagedSink to send events to.
type SinkWriter interface {
	Send(ctx context.Context, e *structs.Events) error
}
