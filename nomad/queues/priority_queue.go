// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"github.com/hashicorp/go-set/v3"
)

// A WorkloadQueue implements heap.Interface and holds *Workload.
type WorkloadQueue struct {
	sortFn func(i, j Workload) int
	*set.TreeSet[Workload]
}

func NewWorkloadQueue(sortFn func(i, j Workload) int) WorkloadQueue {
	return WorkloadQueue{sortFn: sortFn, TreeSet: set.NewTreeSet(sortFn)}
}

func (pq WorkloadQueue) Len() int { return pq.Size() }

func (pq *WorkloadQueue) Push(w Workload) {
	pq.Insert(w)
}

func (pq *WorkloadQueue) Pop() Workload {
	w := pq.Min()
	pq.Remove(w)
	return w
}

// UpdateAll takes a function that mutates a workload and updates
// all workloads in the queue via this function.
func (pq *WorkloadQueue) UpdateAll(updateFn func(w Workload)) {
	newQueue := NewWorkloadQueue(pq.sortFn)
	for _, w := range pq.Slice() {
		updateFn(w)
		newQueue.Push(w)
	}
	*pq = newQueue
}
