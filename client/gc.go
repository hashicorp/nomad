package client

import (
	"container/heap"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/nomad/structs"
)

type GCAlloc struct {
	timeStamp   time.Time
	allocRunner *AllocRunner
	index       int
}

type GCAllocPQImpl []*GCAlloc

func (pq GCAllocPQImpl) Len() int {
	return len(pq)
}

func (pq GCAllocPQImpl) Less(i, j int) bool {
	return pq[i].timeStamp.Before(pq[j].timeStamp)
}

func (pq GCAllocPQImpl) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *GCAllocPQImpl) Push(x interface{}) {
	n := len(*pq)
	item := x.(*GCAlloc)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *GCAllocPQImpl) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// IndexedGCAllocPQ is an indexed PQ which maintains a list of allocation runner
// based on their termination time.
type IndexedGCAllocPQ struct {
	index map[string]*GCAlloc
	heap  GCAllocPQImpl
}

func NewIndexedGCAllocPQ() *IndexedGCAllocPQ {
	return &IndexedGCAllocPQ{
		index: make(map[string]*GCAlloc),
		heap:  make(GCAllocPQImpl, 0),
	}
}

func (i *IndexedGCAllocPQ) Push(ar *AllocRunner) error {
	alloc := ar.Alloc()
	if _, ok := i.index[alloc.ID]; ok {
		return fmt.Errorf("alloc %v already being tracked for GC", alloc.ID)
	}
	gcAlloc := &GCAlloc{
		timeStamp:   time.Now(),
		allocRunner: ar,
	}
	i.index[alloc.ID] = gcAlloc
	heap.Push(&i.heap, gcAlloc)
	return nil
}

func (i *IndexedGCAllocPQ) Pop() *GCAlloc {
	if len(i.heap) == 0 {
		return nil
	}

	gcAlloc := heap.Pop(&i.heap).(*GCAlloc)
	delete(i.index, gcAlloc.allocRunner.Alloc().ID)
	return gcAlloc
}

func (i *IndexedGCAllocPQ) Remove(allocID string) (*GCAlloc, error) {
	if gcAlloc, ok := i.index[allocID]; ok {
		heap.Remove(&i.heap, gcAlloc.index)
		delete(i.index, allocID)
		return gcAlloc, nil
	}

	return nil, fmt.Errorf("alloc %q not present", allocID)
}

func (i *IndexedGCAllocPQ) Length() int {
	return len(i.heap)
}

// AllocGarbageCollector garbage collects terminated allocations on a node
type AllocGarbageCollector struct {
	allocRunners   *IndexedGCAllocPQ
	allocsLock     sync.Mutex
	statsCollector *stats.HostStatsCollector
	logger         *log.Logger
}

// NewAllocGarbageCollector returns a garbage collector for terminated
// allocations on a node.
func NewAllocGarbageCollector(logger *log.Logger, statsCollector *stats.HostStatsCollector) *AllocGarbageCollector {
	return &AllocGarbageCollector{
		allocRunners:   NewIndexedGCAllocPQ(),
		statsCollector: statsCollector,
		logger:         logger,
	}
}

// Collect garbage collects a single allocation on a node
func (a *AllocGarbageCollector) Collect(allocID string) error {
	gcAlloc, err := a.allocRunners.Remove(allocID)
	if err != nil {
		return fmt.Errorf("unable to collect allocation %q: %v", allocID, err)
	}

	ar := gcAlloc.allocRunner
	a.logger.Printf("[INFO] client: garbage collecting allocation %q", ar.Alloc().ID)
	ar.Destroy()

	return nil
}

// CollectAll garbage collects all termianated allocations on a node
func (a *AllocGarbageCollector) CollectAll() error {
	for {
		gcAlloc := a.allocRunners.Pop()
		if gcAlloc == nil {
			break
		}
		ar := gcAlloc.allocRunner
		a.logger.Printf("[INFO] client: garbage collecting alloc runner for alloc %q", ar.Alloc().ID)
		ar.Destroy()
	}
	return nil
}

// MakeRoomFor garbage collects enough number of allocations in the terminal
// state to make room for new allocations
func (a *AllocGarbageCollector) MakeRoomFor(allocations []*structs.Allocation) error {
	totalResource := &structs.Resources{}
	for _, alloc := range allocations {
		if err := totalResource.Add(alloc.Resources); err != nil {
			return err
		}
	}

	var diskCleared int
	for {
		gcAlloc := a.allocRunners.Pop()
		if gcAlloc == nil {
			break
		}

		ar := gcAlloc.allocRunner
		alloc := ar.Alloc()
		a.logger.Printf("[INFO] client: garbage collecting allocation %v", alloc.ID)
		ar.Destroy()
		diskCleared += alloc.Resources.DiskMB
		if diskCleared >= totalResource.DiskMB {
			break
		}
	}
	return nil
}

// MarkForCollection starts tracking an allocation for Garbage Collection
func (a *AllocGarbageCollector) MarkForCollection(ar *AllocRunner) error {
	if ar == nil {
		return fmt.Errorf("nil allocation runner inserted for garbage collection")
	}
	if ar.Alloc() == nil {
		a.logger.Printf("[INFO] client: alloc is nil, so garbage collecting")
		ar.Destroy()
	}

	a.logger.Printf("[INFO] client: marking allocation %v for GC", ar.Alloc().ID)
	a.allocsLock.Lock()
	defer a.allocsLock.Unlock()
	return a.allocRunners.Push(ar)
}
