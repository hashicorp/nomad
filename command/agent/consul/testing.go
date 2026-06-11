// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"

	"github.com/hashicorp/nomad/v2/client/serviceregistration"
	"github.com/hashicorp/nomad/v2/nomad/structs"
)

func NoopRestarter() serviceregistration.WorkloadRestarter {
	return noopRestarter{}
}

type noopRestarter struct{}

func (noopRestarter) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	return nil
}
