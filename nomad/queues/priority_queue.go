// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"github.com/hashicorp/go-set/v3"
)

// A WorkloadQueue implements heap.Interface and holds *Workload.
type WorkloadQueue struct {
	*set.TreeSet[*Workload]
}

func NewWorkloadQueue() WorkloadQueue {
	return WorkloadQueue{set.NewTreeSet(workloadSortFn())}
}

func (pq WorkloadQueue) Len() int { return pq.Size() }

func workloadSortFn() func(i, j *Workload) int {
	return func(a, b *Workload) int {
		// A workload needs to be able to compare with
		// itself and return 0
		if a.waitOnRestore && b.waitOnRestore {
			return 0
		} else if a.waitOnRestore {
			return -1
		} else if b.waitOnRestore {
			return 1
		}

		if a.priority > b.priority {
			return -1
		} else if a.priority < b.priority {
			return 1
		}

		if a.eval.CreateIndex < b.eval.CreateIndex {
			return -1
		} else if a.eval.CreateIndex > b.eval.CreateIndex {
			return 1
		}
		return 0
	}
}

func (pq *WorkloadQueue) Push(w *Workload) {
	pq.Insert(w)
}

func (pq *WorkloadQueue) Pop() *Workload {
	w := pq.Min()
	pq.Remove(w)
	return w
}

// UpdateAll takes a function that mutates a workload and updates
// all workloads in the queue via this function.
func (pq *WorkloadQueue) UpdateAll(updateFn func(w *Workload)) {
	newQueue := NewWorkloadQueue()
	for _, w := range pq.Slice() {
		updateFn(w)
		newQueue.Push(w)
	}
	*pq = newQueue
}
