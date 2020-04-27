package consul

import (
	"context"

	"github.com/hashicorp/nomad/sdk/structs"
)

func NoopRestarter() WorkloadRestarter {
	return noopRestarter{}
}

type noopRestarter struct{}

func (noopRestarter) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	return nil
}
