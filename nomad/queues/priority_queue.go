// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"container/heap"
)

// A WorkloadQueue implements heap.Interface and holds *Workload.
type WorkloadQueue []*Workload

func (pq WorkloadQueue) Len() int { return len(pq) }

func (pq WorkloadQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].priority > pq[j].priority
}

func (pq WorkloadQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *WorkloadQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Workload)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *WorkloadQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // don't stop the GC from reclaiming the item eventually
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// update modifies the priority and value of an Item in the queue.
func (pq *WorkloadQueue) update(item *Workload, priority int) {
	item.priority = priority
	heap.Fix(pq, item.index)
}

func (pq *WorkloadQueue) Workloads() []Workload {
	workloads := make([]Workload, len(*pq))
	for i, w := range *pq {
		workloads[i] = *w
	}
	return workloads
}
