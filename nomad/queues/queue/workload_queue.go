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

func (pq *WorkloadQueue) Len() int {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	return pq.ts.Size()
}

func (pq *WorkloadQueue) Push(w Workload) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	pq.wl[w.ID()] = w

	pq.ts.Insert(w)
}

func (pq *WorkloadQueue) Pop() (res Workload, found bool) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	if pq.ts.Size() == 0 {
		return res, false
	}

	w := pq.ts.Min()
	pq.ts.Remove(w)

	delete(pq.wl, w.ID())
	return w, true
}

func (pq *WorkloadQueue) Get(id structs.NamespacedID) (Workload, bool) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	wl, ok := pq.wl[id]
	return wl, ok
}

// UpdateByID atomically replaces the workload with the same ID as updated,
// returning the replaced workload. If no workload with that ID is in the
// queue, updated is not inserted and false is returned.
func (pq *WorkloadQueue) UpdateByID(updated Workload) (Workload, bool) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	old, ok := pq.wl[updated.ID()]
	if !ok {
		return old, false
	}

	pq.ts.Remove(old)
	pq.wl[updated.ID()] = updated
	pq.ts.Insert(updated)
	return old, true
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

func (pq *WorkloadQueue) Remove(id structs.NamespacedID) (Workload, bool) {
	pq.mux.Lock()
	defer pq.mux.Unlock()

	wl, ok := pq.wl[id]
	if ok {
		pq.ts.Remove(wl)
		delete(pq.wl, id)
	}

	return wl, ok
}
