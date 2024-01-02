// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delayheap

import (
	"container/heap"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// DelayHeap wraps a heap and gives operations other than Push/Pop.
// The inner heap is sorted by the time in the WaitUntil field of delayHeapNode
type DelayHeap struct {
	index map[structs.NamespacedID]*delayHeapNode
	heap  delayedHeapImp
}

// HeapNode is an interface type implemented by objects stored in the DelayHeap
type HeapNode interface {
	Data() interface{} // The data object
	ID() string        // ID of the object, used in conjunction with namespace for deduplication
	Namespace() string // Namespace of the object, can be empty
}

// delayHeapNode encapsulates the node stored in DelayHeap
// WaitUntil is used as the sorting criteria
type delayHeapNode struct {
	// Node is the data object stored in the delay heap
	Node HeapNode
	// WaitUntil is the time delay associated with the node
	// Objects in the heap are sorted by WaitUntil
	WaitUntil time.Time

	index int
}

type delayedHeapImp []*delayHeapNode

func (h delayedHeapImp) Len() int {
	return len(h)
}

// Less sorts zero WaitUntil times at the end of the list, and normally
// otherwise
func (h delayedHeapImp) Less(i, j int) bool {
	if h[i].WaitUntil.IsZero() {
		// 0,? => ?,0
		return false
	}

	if h[j].WaitUntil.IsZero() {
		// ?,0 => ?,0
		return true
	}

	return h[i].WaitUntil.Before(h[j].WaitUntil)
}

func (h delayedHeapImp) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *delayedHeapImp) Push(x interface{}) {
	node := x.(*delayHeapNode)
	n := len(*h)
	node.index = n
	*h = append(*h, node)
}

func (h *delayedHeapImp) Pop() interface{} {
	old := *h
	n := len(old)
	node := old[n-1]
	node.index = -1 // for safety
	*h = old[0 : n-1]
	return node
}

func NewDelayHeap() *DelayHeap {
	return &DelayHeap{
		index: make(map[structs.NamespacedID]*delayHeapNode),
		heap:  make(delayedHeapImp, 0),
	}
}

func (p *DelayHeap) Push(dataNode HeapNode, next time.Time) error {
	tuple := structs.NamespacedID{
		ID:        dataNode.ID(),
		Namespace: dataNode.Namespace(),
	}
	if _, ok := p.index[tuple]; ok {
		return fmt.Errorf("node %q (%s) already exists", dataNode.ID(), dataNode.Namespace())
	}

	delayHeapNode := &delayHeapNode{dataNode, next, 0}
	p.index[tuple] = delayHeapNode
	heap.Push(&p.heap, delayHeapNode)
	return nil
}

func (p *DelayHeap) Pop() *delayHeapNode {
	if len(p.heap) == 0 {
		return nil
	}

	delayHeapNode := heap.Pop(&p.heap).(*delayHeapNode)
	tuple := structs.NamespacedID{
		ID:        delayHeapNode.Node.ID(),
		Namespace: delayHeapNode.Node.Namespace(),
	}
	delete(p.index, tuple)
	return delayHeapNode
}

func (p *DelayHeap) Peek() *delayHeapNode {
	if len(p.heap) == 0 {
		return nil
	}

	return p.heap[0]
}

func (p *DelayHeap) Contains(heapNode HeapNode) bool {
	tuple := structs.NamespacedID{
		ID:        heapNode.ID(),
		Namespace: heapNode.Namespace(),
	}
	_, ok := p.index[tuple]
	return ok
}

func (p *DelayHeap) Update(heapNode HeapNode, waitUntil time.Time) error {
	tuple := structs.NamespacedID{
		ID:        heapNode.ID(),
		Namespace: heapNode.Namespace(),
	}
	if existingHeapNode, ok := p.index[tuple]; ok {
		// Need to update the job as well because its spec can change.
		existingHeapNode.Node = heapNode
		existingHeapNode.WaitUntil = waitUntil
		heap.Fix(&p.heap, existingHeapNode.index)
		return nil
	}

	return fmt.Errorf("heap doesn't contain object with ID %q (%s)", heapNode.ID(), heapNode.Namespace())
}

func (p *DelayHeap) Remove(heapNode HeapNode) error {
	tuple := structs.NamespacedID{
		ID:        heapNode.ID(),
		Namespace: heapNode.Namespace(),
	}
	if node, ok := p.index[tuple]; ok {
		heap.Remove(&p.heap, node.index)
		delete(p.index, tuple)
		return nil
	}

	return fmt.Errorf("heap doesn't contain object with ID %q (%s)", heapNode.ID(), heapNode.Namespace())
}

func (p *DelayHeap) Length() int {
	return len(p.heap)
}
