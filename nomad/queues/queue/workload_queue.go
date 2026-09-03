// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"sync"

	"github.com/hashicorp/go-set/v3"
)

// A WorkloadQueue implements heap.Interface and holds *Workload.
type WorkloadQueue struct {
	sortFn func(i, j Workload) int
	ts     *set.TreeSet[Workload]
	mux    *sync.Mutex
}

func NewWorkloadQueue(sortFn func(i, j Workload) int) WorkloadQueue {
	return WorkloadQueue{
		sortFn: sortFn,
		ts:     set.NewTreeSet(sortFn),
		mux:    &sync.Mutex{},
	}
}

func (pq WorkloadQueue) Len() int {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	return pq.ts.Size()
}

func (pq *WorkloadQueue) Push(w Workload) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	pq.ts.Insert(w)
}

func (pq *WorkloadQueue) Pop() Workload {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	w := pq.ts.Min()
	pq.ts.Remove(w)
	return w
}

// UpdateAll takes a function that mutates a workload and updates
// all workloads in the queue via this function.
func (pq *WorkloadQueue) UpdateAll(updateFn func(w Workload)) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	newQueue := NewWorkloadQueue(pq.sortFn)
	for w := range pq.ts.Items() {
		updateFn(w)
		newQueue.Push(w)
	}
	*pq = newQueue
}

// Iterate does an in order traversal of each item in the queue
// and called the passed function on the workload.
func (pq *WorkloadQueue) Iterate(fn func(Workload)) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	for w := range pq.ts.Items() {
		fn(w)
	}
}
