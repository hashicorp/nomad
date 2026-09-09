// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queue

import (
	"sync"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/nomad/structs"
)

// A WorkloadQueue implements heap.Interface and holds *Workload.
type WorkloadQueue struct {
	// sortFn is the function used to compare workloads in the treeset. It is stored
	// in order to easily create a new treeset during UpdateAll.
	sortFn func(i, j Workload) int

	// ts is the underlying datastructure for storing workloads.
	ts *set.TreeSet[Workload]

	// wl is a k/v store of workloads used to remove or check or existence
	// of a workload via it's namespaced JobID
	//
	// TODO: Golang maps never shrink capacity, maybe need to recreate this
	// when the queue shrinks
	wl map[structs.NamespacedID]Workload

	// mux is the lock for the queue, making it safe for concurrent access.
	mux *sync.Mutex
}

func NewWorkloadQueue(sortFn func(i, j Workload) int) WorkloadQueue {
	return WorkloadQueue{
		sortFn: sortFn,
		ts:     set.NewTreeSet(sortFn),
		wl:     map[structs.NamespacedID]Workload{},
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

	e := w.GetEval()

	pq.wl[structs.NewNamespacedID(e.JobID, e.Namespace)] = w

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

func (pq *WorkloadQueue) Remove(j *structs.Job) Workload {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	if wl, ok := pq.wl[j.NamespacedID()]; ok {
		pq.ts.Remove(wl)
		delete(pq.wl, j.NamespacedID())
		return wl
	}

	return nil
}
