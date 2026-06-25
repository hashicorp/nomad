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
		if a.priority > b.priority {
			return -1
		} else if a.priority < b.priority {
			return 1
		} else {
			if a.eval.CreateIndex < b.eval.CreateIndex {
				return -1
			} else if a.eval.CreateIndex > b.eval.CreateIndex {
				return 1
			}
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

func (pq *WorkloadQueue) Workloads() []*Workload {
	return pq.Slice()
}

func (pq *WorkloadQueue) UpdateAll(workloadFn func(w *Workload) *Workload) {
	newQueue := NewWorkloadQueue()
	for _, w := range pq.Workloads() {
		newQueue.Push(workloadFn(w))
	}
	*pq = newQueue
}
