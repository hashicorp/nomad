// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/hoststats"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// MB is a constant which converts values in bytes to MB
	MB = 1024 * 1024
)

// GCConfig allows changing the behaviour of the garbage collector
type GCConfig struct {
	// MaxAllocs is the maximum number of allocations to track before a GC
	// is triggered.
	MaxAllocs           int
	DiskUsageThreshold  float64
	InodeUsageThreshold float64
	Interval            time.Duration
	ReservedDiskMB      int
	ParallelDestroys    int
}

// AllocCounter is used by AllocGarbageCollector to discover how many un-GC'd
// allocations a client has and is generally fulfilled by the Client.
type AllocCounter interface {
	NumAllocs() int
}

// AllocGarbageCollector garbage collects terminated allocations on a node
type AllocGarbageCollector struct {
	config *GCConfig

	// allocRunners marked for GC
	allocRunners *IndexedGCAllocPQ

	// statsCollector for node based thresholds (eg disk)
	statsCollector hoststats.NodeStatsCollector

	// allocCounter return the number of un-GC'd allocs on this node
	allocCounter AllocCounter

	// destroyCh is a semaphore for rate limiting concurrent garbage
	// collections
	destroyCh chan struct{}

	// shutdownCh is closed when the GC's run method should exit
	shutdownCh chan struct{}

	// triggerCh is ticked by the Trigger method to cause a GC
	triggerCh chan struct{}

	logger hclog.Logger
}

// NewAllocGarbageCollector returns a garbage collector for terminated
// allocations on a node. Must call Run() in a goroutine enable periodic
// garbage collection.
func NewAllocGarbageCollector(logger hclog.Logger, statsCollector hoststats.NodeStatsCollector, ac AllocCounter, config *GCConfig) *AllocGarbageCollector {
	logger = logger.Named("gc")
	// Require at least 1 to make progress
	if config.ParallelDestroys <= 0 {
		logger.Warn("garbage collector defaulting parallelism to 1 due to invalid input value", "gc_parallel_destroys", config.ParallelDestroys)
		config.ParallelDestroys = 1
	}

	gc := &AllocGarbageCollector{
		allocRunners:   NewIndexedGCAllocPQ(),
		statsCollector: statsCollector,
		allocCounter:   ac,
		config:         config,
		logger:         logger,
		destroyCh:      make(chan struct{}, config.ParallelDestroys),
		shutdownCh:     make(chan struct{}),
		triggerCh:      make(chan struct{}, 1),
	}

	return gc
}

// Run the periodic garbage collector.
func (a *AllocGarbageCollector) Run() {
	ticker := time.NewTicker(a.config.Interval)
	for {
		select {
		case <-a.triggerCh:
		case <-ticker.C:
		case <-a.shutdownCh:
			ticker.Stop()
			return
		}

		if err := a.keepUsageBelowThreshold(); err != nil {
			a.logger.Error("error garbage collecting allocations", "error", err)
		}
	}
}

// Trigger forces the garbage collector to run.
func (a *AllocGarbageCollector) Trigger() {
	select {
	case a.triggerCh <- struct{}{}:
	default:
		// already triggered
	}
}

// keepUsageBelowThreshold collects disk usage information and garbage collects
// allocations to make disk space available.
func (a *AllocGarbageCollector) keepUsageBelowThreshold() error {
	for {
		select {
		case <-a.shutdownCh:
			return nil
		default:
		}

		// Check if we have enough free space
		if err := a.statsCollector.Collect(); err != nil {
			return err
		}

		// See if we are below thresholds for used disk space and inode usage
		diskStats := a.statsCollector.Stats().AllocDirStats
		reason := ""
		logf := a.logger.Warn

		liveAllocs := a.allocCounter.NumAllocs()

		switch {
		case diskStats.UsedPercent > a.config.DiskUsageThreshold:
			reason = fmt.Sprintf("disk usage of %.0f is over gc threshold of %.0f",
				diskStats.UsedPercent, a.config.DiskUsageThreshold)
		case diskStats.InodesUsedPercent > a.config.InodeUsageThreshold:
			reason = fmt.Sprintf("inode usage of %.0f is over gc threshold of %.0f",
				diskStats.InodesUsedPercent, a.config.InodeUsageThreshold)
		case liveAllocs > a.config.MaxAllocs:
			// if we're unable to gc, don't WARN until at least 2x over limit
			if liveAllocs < (a.config.MaxAllocs * 2) {
				logf = a.logger.Info
			}
			reason = fmt.Sprintf("number of allocations (%d) is over the limit (%d)", liveAllocs, a.config.MaxAllocs)
		}

		if reason == "" {
			// No reason to gc, exit
			break
		}

		// Collect an allocation
		gcAlloc := a.allocRunners.Pop()
		if gcAlloc == nil {
			logf("garbage collection skipped because no terminal allocations", "reason", reason)
			break
		}

		// Destroy the alloc runner and wait until it exits
		a.destroyAllocRunner(gcAlloc.allocID, gcAlloc.allocRunner, reason)
	}
	return nil
}

// destroyAllocRunner is used to destroy an allocation runner. It will acquire a
// lock to restrict parallelism and then destroy the alloc runner, returning
// once the allocation has been destroyed.
func (a *AllocGarbageCollector) destroyAllocRunner(allocID string, ar interfaces.AllocRunner, reason string) {
	a.logger.Info("garbage collecting allocation", "alloc_id", allocID, "reason", reason)

	// Acquire the destroy lock
	select {
	case <-a.shutdownCh:
		return
	case a.destroyCh <- struct{}{}:
	}

	ar.Destroy()

	select {
	case <-ar.DestroyCh():
	case <-a.shutdownCh:
	}

	a.logger.Debug("alloc garbage collected", "alloc_id", allocID)

	// Release the lock
	<-a.destroyCh
}

func (a *AllocGarbageCollector) Stop() {
	close(a.shutdownCh)
}

// Collect garbage collects a single allocation on a node. Returns true if
// alloc was found and garbage collected; otherwise false.
func (a *AllocGarbageCollector) Collect(allocID string) bool {
	gcAlloc := a.allocRunners.Remove(allocID)
	if gcAlloc == nil {
		a.logger.Debug("alloc was already garbage collected", "alloc_id", allocID)
		return false
	}

	a.destroyAllocRunner(allocID, gcAlloc.allocRunner, "forced collection")
	return true
}

// CollectAll garbage collects all terminated allocations on a node
func (a *AllocGarbageCollector) CollectAll() {
	for {
		select {
		case <-a.shutdownCh:
			return
		default:
		}

		gcAlloc := a.allocRunners.Pop()
		if gcAlloc == nil {
			return
		}

		go a.destroyAllocRunner(gcAlloc.allocID, gcAlloc.allocRunner, "forced full node collection")
	}
}

// MakeRoomFor garbage collects enough number of allocations in the terminal
// state to make room for new allocations
func (a *AllocGarbageCollector) MakeRoomFor(allocations []*structs.Allocation) error {
	if len(allocations) == 0 {
		// Nothing to make room for!
		return nil
	}

	// GC allocs until below the max limit + the new allocations
	max := a.config.MaxAllocs - len(allocations)
	for a.allocCounter.NumAllocs() > max {
		select {
		case <-a.shutdownCh:
			return nil
		default:
		}

		gcAlloc := a.allocRunners.Pop()
		if gcAlloc == nil {
			// It's fine if we can't lower below the limit here as
			// we'll keep trying to drop below the limit with each
			// periodic gc
			break
		}

		// Destroy the alloc runner and wait until it exits
		a.destroyAllocRunner(gcAlloc.allocID, gcAlloc.allocRunner, fmt.Sprintf("new allocations and over max (%d)", a.config.MaxAllocs))
	}

	totalResource := &structs.AllocatedSharedResources{}
	for _, alloc := range allocations {
		// COMPAT(0.11): Remove in 0.11
		if alloc.AllocatedResources != nil {
			totalResource.Add(&alloc.AllocatedResources.Shared)
		} else {
			totalResource.DiskMB += int64(alloc.Resources.DiskMB)
		}
	}

	// If the host has enough free space to accommodate the new allocations then
	// we don't need to garbage collect terminated allocations
	if hostStats := a.statsCollector.Stats(); hostStats != nil {
		var availableForAllocations uint64
		if hostStats.AllocDirStats.Available < uint64(a.config.ReservedDiskMB*MB) {
			availableForAllocations = 0
		} else {
			availableForAllocations = hostStats.AllocDirStats.Available - uint64(a.config.ReservedDiskMB*MB)
		}
		if uint64(totalResource.DiskMB*MB) < availableForAllocations {
			return nil
		}
	}

	var diskCleared int64
	for {
		select {
		case <-a.shutdownCh:
			return nil
		default:
		}

		// Collect host stats and see if we still need to remove older
		// allocations
		var allocDirStats *hoststats.DiskStats
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

		// COMPAT(0.11): Remove in 0.11
		var allocDiskMB int64
		if alloc.AllocatedResources != nil {
			allocDiskMB = alloc.AllocatedResources.Shared.DiskMB
		} else {
			allocDiskMB = int64(alloc.Resources.DiskMB)
		}

		// Destroy the alloc runner and wait until it exits
		a.destroyAllocRunner(gcAlloc.allocID, ar, fmt.Sprintf("freeing %d MB for new allocations", allocDiskMB))

		diskCleared += allocDiskMB
	}
	return nil
}

// MarkForCollection starts tracking an allocation for Garbage Collection
func (a *AllocGarbageCollector) MarkForCollection(allocID string, ar interfaces.AllocRunner) {
	if a.allocRunners.Push(allocID, ar) {
		a.logger.Info("marking allocation for GC", "alloc_id", allocID)
	}
}

// GCAlloc wraps an allocation runner and an index enabling it to be used within
// a PQ
type GCAlloc struct {
	timeStamp   time.Time
	allocID     string
	allocRunner interfaces.AllocRunner
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

// Push an alloc runner into the GC queue. Returns true if alloc was added,
// false if the alloc already existed.
func (i *IndexedGCAllocPQ) Push(allocID string, ar interfaces.AllocRunner) bool {
	i.pqLock.Lock()
	defer i.pqLock.Unlock()

	if _, ok := i.index[allocID]; ok {
		// No work to do
		return false
	}
	gcAlloc := &GCAlloc{
		timeStamp:   time.Now(),
		allocID:     allocID,
		allocRunner: ar,
	}
	i.index[allocID] = gcAlloc
	heap.Push(&i.heap, gcAlloc)
	return true
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

// Remove alloc from GC. Returns nil if alloc doesn't exist.
func (i *IndexedGCAllocPQ) Remove(allocID string) *GCAlloc {
	i.pqLock.Lock()
	defer i.pqLock.Unlock()

	if gcAlloc, ok := i.index[allocID]; ok {
		heap.Remove(&i.heap, gcAlloc.index)
		delete(i.index, allocID)
		return gcAlloc
	}

	return nil
}

func (i *IndexedGCAllocPQ) Length() int {
	i.pqLock.Lock()
	defer i.pqLock.Unlock()

	return len(i.heap)
}
