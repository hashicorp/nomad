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

const (
	// diskUsageThreshold is the percent of used disk space beyond which Nomad
	// GCs terminated allocations
	diskUsageThreshold = 80

	// gcInterval is the interval at which Nomad runs the garbage collector
	gcInterval = 1 * time.Minute

	// inodeUsageThreshold is the percent of inode usage that Nomad tries to
	// maintain, whenever we are over it we will attempt to GC terminal
	// allocations
	inodeUsageThreshold = 70

	// MB is a constant which converts values in bytes to MB
	MB = 1024 * 1024
)

// GCAlloc wraps an allocation runner and an index enabling it to be used within
// a PQ
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

	pqLock sync.Mutex
}

func NewIndexedGCAllocPQ() *IndexedGCAllocPQ {
	return &IndexedGCAllocPQ{
		index: make(map[string]*GCAlloc),
		heap:  make(GCAllocPQImpl, 0),
	}
}

func (i *IndexedGCAllocPQ) Push(ar *AllocRunner) error {
	i.pqLock.Lock()
	defer i.pqLock.Unlock()

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
	i.pqLock.Lock()
	defer i.pqLock.Unlock()

	if len(i.heap) == 0 {
		return nil
	}

	gcAlloc := heap.Pop(&i.heap).(*GCAlloc)
	delete(i.index, gcAlloc.allocRunner.Alloc().ID)
	return gcAlloc
}

func (i *IndexedGCAllocPQ) Remove(allocID string) (*GCAlloc, error) {
	i.pqLock.Lock()
	defer i.pqLock.Unlock()

	if gcAlloc, ok := i.index[allocID]; ok {
		heap.Remove(&i.heap, gcAlloc.index)
		delete(i.index, allocID)
		return gcAlloc, nil
	}

	return nil, fmt.Errorf("alloc %q not present", allocID)
}

func (i *IndexedGCAllocPQ) Length() int {
	i.pqLock.Lock()
	defer i.pqLock.Unlock()

	return len(i.heap)
}

// AllocGarbageCollector garbage collects terminated allocations on a node
type AllocGarbageCollector struct {
	allocRunners   *IndexedGCAllocPQ
	statsCollector stats.NodeStatsCollector
	reservedDiskMB int
	logger         *log.Logger
	shutdownCh     chan struct{}
}

// NewAllocGarbageCollector returns a garbage collector for terminated
// allocations on a node.
func NewAllocGarbageCollector(logger *log.Logger, statsCollector stats.NodeStatsCollector, reservedDiskMB int) *AllocGarbageCollector {
	gc := &AllocGarbageCollector{
		allocRunners:   NewIndexedGCAllocPQ(),
		statsCollector: statsCollector,
		reservedDiskMB: reservedDiskMB,
		logger:         logger,
		shutdownCh:     make(chan struct{}),
	}
	go gc.run()

	return gc
}

func (a *AllocGarbageCollector) run() {
	ticker := time.NewTicker(gcInterval)
	for {
		select {
		case <-ticker.C:
			if err := a.keepUsageBelowThreshold(); err != nil {
				a.logger.Printf("[ERR] client: error garbage collecting allocation: %v", err)
			}
		case <-a.shutdownCh:
			ticker.Stop()
			return
		}
	}
}

// keepUsageBelowThreshold collects disk usage information and garbage collects
// allocations to make disk space available.
func (a *AllocGarbageCollector) keepUsageBelowThreshold() error {
	for {
		// Check if we have enough free space
		err := a.statsCollector.Collect()
		if err != nil {
			return err
		}

		// See if we are below thresholds for used disk space and inode usage
		// TODO(diptanu) figure out why this is nil
		stats := a.statsCollector.Stats()
		if stats == nil {
			break
		}

		diskStats := stats.AllocDirStats
		if diskStats == nil {
			break
		}

		if diskStats.UsedPercent <= diskUsageThreshold &&
			diskStats.InodesUsedPercent <= inodeUsageThreshold {
			break
		}

		// Collect an allocation
		gcAlloc := a.allocRunners.Pop()
		if gcAlloc == nil {
			break
		}

		ar := gcAlloc.allocRunner
		alloc := ar.Alloc()
		a.logger.Printf("[INFO] client: garbage collecting allocation %v", alloc.ID)

		// Destroy the alloc runner and wait until it exits
		ar.Destroy()
		select {
		case <-ar.WaitCh():
		case <-a.shutdownCh:
		}
	}
	return nil
}

func (a *AllocGarbageCollector) Stop() {
	close(a.shutdownCh)
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

	// If the host has enough free space to accomodate the new allocations then
	// we don't need to garbage collect terminated allocations
	if hostStats := a.statsCollector.Stats(); hostStats != nil {
		var availableForAllocations uint64
		if hostStats.AllocDirStats.Available < uint64(a.reservedDiskMB*MB) {
			availableForAllocations = 0
		} else {
			availableForAllocations = hostStats.AllocDirStats.Available - uint64(a.reservedDiskMB*MB)
		}
		if uint64(totalResource.DiskMB*MB) < availableForAllocations {
			return nil
		}
	}

	var diskCleared int
	for {
		// Collect host stats and see if we still need to remove older
		// allocations
		var allocDirStats *stats.DiskStats
		if err := a.statsCollector.Collect(); err == nil {
			if hostStats := a.statsCollector.Stats(); hostStats != nil {
				allocDirStats = hostStats.AllocDirStats
			}
		}

		if allocDirStats != nil {
			if allocDirStats.Available >= uint64(totalResource.DiskMB*MB) {
				break
			}
		} else {
			// Falling back to a simpler model to know if we have enough disk
			// space if stats collection fails
			if diskCleared >= totalResource.DiskMB {
				break
			}
		}

		gcAlloc := a.allocRunners.Pop()
		if gcAlloc == nil {
			break
		}

		ar := gcAlloc.allocRunner
		alloc := ar.Alloc()
		a.logger.Printf("[INFO] client: garbage collecting allocation %v", alloc.ID)

		// Destroy the alloc runner and wait until it exits
		ar.Destroy()
		select {
		case <-ar.WaitCh():
		case <-a.shutdownCh:
		}

		// Call stats collect again
		diskCleared += alloc.Resources.DiskMB
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
	return a.allocRunners.Push(ar)
}

// Remove removes an alloc runner without garbage collecting it
func (a *AllocGarbageCollector) Remove(ar *AllocRunner) {
	if ar == nil || ar.Alloc() == nil {
		return
	}

	alloc := ar.Alloc()
	if _, err := a.allocRunners.Remove(alloc.ID); err == nil {
		a.logger.Printf("[INFO] client: removed alloc runner %v from garbage collector", alloc.ID)
	}
}
