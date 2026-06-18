// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type Queue interface {
	Enqueue(*structs.Evaluation)
	Start(context.Context) error
	Jobs(map[string]bool) structs.QueueJobsResponse
	Tenants() structs.QueueTenantsResponse
	SetEnabled(bool, *state.StateStore)
}

// Broker is the interface for an evaluation broker
type Broker interface {
	Enqueue(*structs.Evaluation)
}
