// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package passthrough

import (
	"context"

	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/structs"
)

type PassthroughQueue struct {
	// evalBroker is the injected broker for passing an evaluation
	// on to be scheduled by Nomad
	evalBroker queue.Broker
}

func NewPassthroughQueue(b queue.Broker) *PassthroughQueue {
	return &PassthroughQueue{
		evalBroker: b,
	}
}

func (p *PassthroughQueue) Type() structs.BatchQueueType {
	return structs.BatchQueueTypePassthrough
}

// Start is a noop for the passthrough implementation
func (p *PassthroughQueue) Start(context.Context) error { return nil }

func (p *PassthroughQueue) Stop() {}

func (p *PassthroughQueue) Enqueue(e *structs.Evaluation) { p.evalBroker.Enqueue(e) }

func (p *PassthroughQueue) Jobs(structs.SortOrder) *queue.WorkloadIter {
	return &queue.WorkloadIter{}
}

func (p *PassthroughQueue) Tenants() structs.QueueTenantsResponse {
	return structs.QueueTenantsResponse{Type: structs.BatchQueueTypePassthrough}
}
