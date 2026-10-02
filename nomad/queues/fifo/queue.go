// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"cmp"

	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/structs"
)

// FifoQueue orders workloads by the create index of their evaluation.
type FifoQueue struct {
	queue queue.WorkloadQueue
}

// New returns the workload storage and ordering for a FIFO queue.
func New() *FifoQueue {
	return &FifoQueue{
		queue: queue.NewWorkloadQueue(workloadSortFn()),
	}
}

func workloadSortFn() func(i, j queue.Workload) int {
	return func(i, j queue.Workload) int {
		return cmp.Compare(i.Eval().CreateIndex, j.Eval().CreateIndex)
	}
}

// NewWorkload implements base.Queue. All evals are accepted.
func (f *FifoQueue) NewWorkload(e *structs.Evaluation, j *structs.Job) (queue.Workload, bool) {
	return newFifoWorkload(e, j), true
}

func (f *FifoQueue) Type() structs.BatchQueueType {
	return structs.BatchQueueTypeFifo
}

// Push implements base.Queue. The workload must have been created by
// NewWorkload.
func (f *FifoQueue) Push(w queue.Workload) {
	f.queue.Push(w.(*fifoWorkload))
}

func (f *FifoQueue) Pop() (queue.Workload, bool) {
	return f.queue.Pop()
}

func (f *FifoQueue) Get(id structs.NamespacedID) (queue.Workload, bool) {
	w, ok := f.queue.Get(id)
	if !ok {
		return nil, false
	}
	return w, true
}

func (f *FifoQueue) Remove(id structs.NamespacedID) (queue.Workload, bool) {
	w, ok := f.queue.Remove(id)
	if !ok {
		return nil, false
	}
	return w, true
}

// Update implements base.Queue. The workload must have been created by
// NewWorkload.
func (f *FifoQueue) Update(w queue.Workload) (queue.Workload, bool) {
	old, ok := f.queue.UpdateByID(w.(*fifoWorkload))
	if !ok {
		return nil, false
	}
	return old, true
}

// Jobs implements base.Viewable.
func (f *FifoQueue) Jobs(sortOrder structs.SortOrder, inProgress []queue.Workload) *queue.WorkloadIter {
	pos := 0
	workloads := []structs.QueueWorkload{}

	newWorkloadStruct := func(w *fifoWorkload, position int) *structs.Workload {
		eval := w.Eval()
		return &structs.Workload{
			JobID:       eval.JobID,
			Namespace:   eval.Namespace,
			Position:    position,
			Status:      w.Status(),
			CreatedAt:   eval.CreateTime,
			CreateIndex: eval.CreateIndex,
		}
	}

	for _, workload := range inProgress {
		if w, ok := workload.(*fifoWorkload); ok {
			workloads = append(workloads, newWorkloadStruct(w, 0))
		}
	}

	f.queue.Iterate(func(workload queue.Workload) {
		pos++
		workloads = append(workloads, newWorkloadStruct(workload.(*fifoWorkload), pos))
	})

	iter := queue.NewWorkloadIter(workloads)

	if sortOrder != structs.SortByPriority {
		iter.SortByJobId()
	}

	return iter
}

// Tenants implements base.Viewable.
func (f *FifoQueue) Tenants() structs.QueueTenantsResponse {
	return structs.QueueTenantsResponse{Type: structs.BatchQueueTypeFifo}
}
