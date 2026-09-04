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

	newTs := set.NewTreeSet(pq.sortFn)
	for w := range pq.ts.Items() {
		updateFn(w)
		newTs.Insert(w)
	}
	pq.ts = newTs
}

// Iterate does an in order traversal of each item in the queue
// and calls the passed function on the workload.
//
// The callback function should NOT mutate the workload, but use it
// to construct a separate threadsafe object.
func (pq *WorkloadQueue) Iterate(fn func(Workload)) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	for w := range pq.ts.Items() {
		fn(w)
	}
}
