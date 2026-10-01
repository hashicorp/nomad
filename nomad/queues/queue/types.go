// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"context"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type EvalCancelFn func(*structs.Evaluation) error

// Queue is the main interface that must be implemented to create a
// new queue. This queue will be run by a QueueRunner so it shares
// common Nomad application logic with other queue types.
type Queue interface {
	NewWorkload(*structs.Evaluation, *structs.Job) (Workload, bool)
	Push(Workload)
	Pop() (Workload, bool)
	Get(structs.NamespacedID) (Workload, bool)
	Remove(structs.NamespacedID) (Workload, bool)
	// TODO: this doesnt really need to exist? Should Push just replace if
	// the workload already exists on the
	Update(Workload) (Workload, bool)
	Type() structs.BatchQueueType
}

type QueueManager interface {
	SetEnabled(bool, *state.StateStore)
	Enqueue(*structs.Evaluation)
	Dequeue(*structs.Job) *structs.Evaluation
	Queue(string) QueueRunner
	UpdateQueue(*structs.NodePool) error
}

type QueueRunner interface {
	Start(context.Context) error
	Stop()
	Enqueue(*structs.Evaluation, *structs.Job)
	Restore(*structs.Evaluation, *structs.Job) error
	Dequeue(structs.NamespacedID) *structs.Evaluation
	Jobs(structs.SortOrder) *WorkloadIter
	Tenants() structs.QueueTenantsResponse
	Type() structs.BatchQueueType
}

type Workload interface {
	ID() structs.NamespacedID
	Eval() *structs.Evaluation
	SetEval(*structs.Evaluation)
	Job() *structs.Job
	Status() string
	SetStatus(string, string)
	JobVersion() uint64
}

// Broker is the interface for an evaluation broker
type Broker interface {
	Enqueue(*structs.Evaluation)
}

type Snapshotter interface {
	Snapshot() (*state.StateSnapshot, error)
}
