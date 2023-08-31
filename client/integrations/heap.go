package integrations

import (
	"container/heap"
	"fmt"
	"time"
)

// Element representing an entry in the renewal heap
type HeapEntry struct {
	Req   *RenewalRequest
	Next  time.Time
	Index int
}

// Wrapper around the actual heap to provide additional semantics on top of
// functions provided by the heap interface. In order to achieve that, an
// additional map is placed beside the actual heap. This map can be used to
// check if an entry is already present in the heap.
type ClientHeap struct {
	HeapMap map[string]*HeapEntry
	Heap    vaultDataHeapImp
}

// Data type of the heap
type vaultDataHeapImp []*HeapEntry

// NewClientHeap returns a new client heap with both the heap and a map which is
// a secondary index for heap elements, both initialized.
func NewClientHeap() *ClientHeap {
	return &ClientHeap{
		HeapMap: make(map[string]*HeapEntry),
		Heap:    make(vaultDataHeapImp, 0),
	}
}

// Length returns the number of elements in the heap
func (h *ClientHeap) Length() int {
	return len(h.Heap)
}

// Returns the root node of the min-heap
func (h *ClientHeap) Peek() *HeapEntry {
	if len(h.Heap) == 0 {
		return nil
	}

	return h.Heap[0]
}

// Push adds the secondary index and inserts an item into the heap
func (h *ClientHeap) Push(req *RenewalRequest, next time.Time) error {
	if req == nil {
		return fmt.Errorf("nil request")
	}

	if _, ok := h.HeapMap[req.ID]; ok {
		return fmt.Errorf("entry %v already exists", req.ID)
	}

	heapEntry := &HeapEntry{
		Req:  req,
		Next: next,
	}
	h.HeapMap[req.ID] = heapEntry
	heap.Push(&h.Heap, heapEntry)
	return nil
}

// Update will modify the existing item in the heap with the new data and the
// time, and fixes the heap.
func (h *ClientHeap) Update(req *RenewalRequest, next time.Time) error {
	if entry, ok := h.HeapMap[req.ID]; ok {
		entry.Req = req
		entry.Next = next
		heap.Fix(&h.Heap, entry.Index)
		return nil
	}

	return fmt.Errorf("heap doesn't contain %v", req.ID)
}

// Remove will remove an identifier from the secondary index and deletes the
// corresponding node from the heap.
func (h *ClientHeap) Remove(id string) error {
	if entry, ok := h.HeapMap[id]; ok {
		heap.Remove(&h.Heap, entry.Index)
		delete(h.HeapMap, id)
		return nil
	}

	return fmt.Errorf("heap doesn't contain entry for %v", id)
}

// The heap interface requires the following methods to be implemented.
// * Push(x interface{}) // add x as element Len()
// * Pop() interface{}   // remove and return element Len() - 1.
// * sort.Interface
//
// sort.Interface comprises of the following methods:
// * Len() int
// * Less(i, j int) bool
// * Swap(i, j int)

// Part of sort.Interface
func (h vaultDataHeapImp) Len() int { return len(h) }

// Part of sort.Interface
func (h vaultDataHeapImp) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	// Sort such that zero times are at the end of the list.
	iZero, jZero := h[i].Next.IsZero(), h[j].Next.IsZero()
	if iZero && jZero {
		return false
	} else if iZero {
		return false
	} else if jZero {
		return true
	}

	return h[i].Next.Before(h[j].Next)
}

// Part of sort.Interface
func (h vaultDataHeapImp) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index = i
	h[j].Index = j
}

// Part of heap.Interface
func (h *vaultDataHeapImp) Push(x interface{}) {
	n := len(*h)
	entry := x.(*HeapEntry)
	entry.Index = n
	*h = append(*h, entry)
}

// Part of heap.Interface
func (h *vaultDataHeapImp) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	entry.Index = -1 // for safety
	*h = old[0 : n-1]
	return entry
}

// randIntn is the function in math/rand needed by renewalTime. A type is used
// to ease deterministic testing.
type randIntn func(int) int

// RenewalTime returns when a token should be renewed given its leaseDuration
// and a randomizer to provide jitter.
//
// Leases < 1m will be not jitter.
func RenewalTime(dice randIntn, leaseDuration int) time.Duration {
	// Start trying to renew at half the lease duration to allow ample time
	// for latency and retries.
	renew := leaseDuration / 2

	// Don't bother about introducing randomness if the
	// leaseDuration is too small.
	const cutoff = 30
	if renew < cutoff {
		return time.Duration(renew) * time.Second
	}

	// jitter is the amount +/- to vary the renewal time
	const jitter = 10
	min := renew - jitter
	renew = min + dice(jitter*2)

	return time.Duration(renew) * time.Second
}
