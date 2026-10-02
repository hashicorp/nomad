// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"cmp"

	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Queue orders workloads by the create index of their evaluation.
type Queue struct {
	queue queue.WorkloadQueue
}

// New returns the workload storage and ordering for a FIFO queue.
func New() *Queue {
	return &Queue{
		queue: queue.NewWorkloadQueue(workloadSortFn()),
	}
}

func workloadSortFn() func(i, j queue.Workload) int {
	return func(i, j queue.Workload) int {
		return cmp.Compare(i.Eval().CreateIndex, j.Eval().CreateIndex)
	}
}

// NewWorkload implements base.Queue. All evals are accepted.
func (f *Queue) NewWorkload(e *structs.Evaluation, j *structs.Job) (queue.Workload, bool) {
	return newFifoWorkload(e, j), true
}

func (f *Queue) Type() structs.BatchQueueType {
	return structs.BatchQueueTypeFifo
}

// Push implements base.Queue. The workload must have been created by
// NewWorkload.
func (f *Queue) Push(w queue.Workload) {
	f.queue.Push(w)
}

func (f *Queue) Pop() (queue.Workload, bool) {
	return f.queue.Pop()
}

func (f *Queue) Get(id structs.NamespacedID) (queue.Workload, bool) {
	w, ok := f.queue.Get(id)
	if !ok {
		return nil, false
	}
	return w, true
}

func (f *Queue) Remove(id structs.NamespacedID) (queue.Workload, bool) {
	w, ok := f.queue.Remove(id)
	if !ok {
		return nil, false
	}
	return w, true
}

// Swap implements base.Queue. The workload must have been created by
// NewWorkload.
func (f *Queue) Swap(w queue.Workload) (queue.Workload, bool) {
	old, ok := f.queue.Swap(w)
	if !ok {
		return nil, false
	}
	return old, true
}

// Jobs implements base.Viewable.
func (f *Queue) Jobs(sortOrder structs.SortOrder, inProgress []queue.Workload) *queue.WorkloadIter {
	pos := 0
	workloads := []structs.QueueWorkload{}

	for _, workload := range inProgress {
		if w, ok := workload.(*fifoWorkload); ok {
			workloads = append(workloads, w.toStruct(pos))
		}
	}

	f.queue.Iterate(func(workload queue.Workload) {
		if w, ok := workload.(*fifoWorkload); ok {
			pos++
			workloads = append(workloads, w.toStruct(pos))
		}
	})

	iter := queue.NewWorkloadIter(workloads)

	if sortOrder != structs.SortByPriority {
		iter.SortByJobId()
	}

	return iter
}

// Tenants implements base.Viewable.
func (f *Queue) Tenants() structs.QueueTenantsResponse {
	return structs.QueueTenantsResponse{Type: structs.BatchQueueTypeFifo}
}
