// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
)

type PassthroughQueue struct {
	broker Broker
}

func NewPassthroughQueue(b Broker) *PassthroughQueue {
	return &PassthroughQueue{
		broker: b,
	}
}

func (p *PassthroughQueue) Type() structs.BatchQueueType {
	return "unset"
}

// Start is a noop for the passthrough implementation
func (p *PassthroughQueue) Start(context.Context) error { return nil }

func (p *PassthroughQueue) Stop() {}

func (p *PassthroughQueue) Enqueue(e *structs.Evaluation) { p.broker.Enqueue(e) }

func (p *PassthroughQueue) Jobs(structs.SortOrder) *WorkloadIter {
	return &WorkloadIter{}
}

func (p *PassthroughQueue) Tenants() structs.QueueTenantsResponse {
	return structs.QueueTenantsResponse{Type: "unset"}
}
