// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"

	"github.com/hashicorp/nomad/nomad/structs"
)

type Queue interface {
	Start(context.Context) error
	Stop()
	Enqueue(*structs.Evaluation)
	Jobs() *WorkloadIter
	Tenants() structs.QueueTenantsResponse
	Type() structs.BatchQueueType
}

// Broker is the interface for an evaluation broker
type Broker interface {
	Enqueue(*structs.Evaluation)
}

type WorkloadIter struct {
	Workloads []structs.QueueWorkload
	index     int
}

func (i *WorkloadIter) Next() interface{} {
	if i.index >= len(i.Workloads) {
		return nil
	}
	w := i.Workloads[i.index]
	i.index++
	return w
}
