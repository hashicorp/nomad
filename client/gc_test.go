package client

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestIndexedGCAllocPQ(t *testing.T) {
	pq := NewIndexedGCAllocPQ()

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar3 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar4 := testAllocRunnerFromAlloc(mock.Alloc(), false)

	pq.Push(ar1)
	pq.Push(ar2)
	pq.Push(ar3)
	pq.Push(ar4)

	allocID := pq.Pop().allocRunner.Alloc().ID
	if allocID != ar1.Alloc().ID {
		t.Fatalf("expected alloc %v, got %v", allocID, ar1.Alloc().ID)
	}

	allocID = pq.Pop().allocRunner.Alloc().ID
	if allocID != ar2.Alloc().ID {
		t.Fatalf("expected alloc %v, got %v", allocID, ar1.Alloc().ID)
	}

	allocID = pq.Pop().allocRunner.Alloc().ID
	if allocID != ar3.Alloc().ID {
		t.Fatalf("expected alloc %v, got %v", allocID, ar1.Alloc().ID)
	}

	allocID = pq.Pop().allocRunner.Alloc().ID
	if allocID != ar4.Alloc().ID {
		t.Fatalf("expected alloc %v, got %v", allocID, ar1.Alloc().ID)
	}

	gcAlloc := pq.Pop()
	if gcAlloc != nil {
		t.Fatalf("expected nil, got %v", gcAlloc)
	}
}

type MockStatsCollector struct {
	availableValues []uint64
	usedPercents    []float64
	inodePercents   []float64
	index           int
}

func (m *MockStatsCollector) Collect() error {
	return nil
}

func (m *MockStatsCollector) Stats() *stats.HostStats {
	if len(m.availableValues) == 0 {
		return nil
	}

	available := m.availableValues[m.index]
	usedPercent := m.usedPercents[m.index]
	inodePercent := m.inodePercents[m.index]

	if m.index < len(m.availableValues)-1 {
		m.index = m.index + 1
	}
	return &stats.HostStats{
		AllocDirStats: &stats.DiskStats{
			Available:         available,
			UsedPercent:       usedPercent,
			InodesUsedPercent: inodePercent,
		},
	}
}

func TestAllocGarbageCollector_MarkForCollection(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, 0)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}

	gcAlloc := gc.allocRunners.Pop()
	if gcAlloc == nil || gcAlloc.allocRunner != ar1 {
		t.Fatalf("bad gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_Collect(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, 0)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := gc.Collect(ar1.Alloc().ID); err != nil {
		t.Fatalf("err: %v", err)
	}
	gcAlloc := gc.allocRunners.Pop()
	if gcAlloc == nil || gcAlloc.allocRunner != ar2 {
		t.Fatalf("bad gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_CollectAll(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, 0)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := gc.CollectAll(); err != nil {
		t.Fatalf("err: %v", err)
	}
	gcAlloc := gc.allocRunners.Pop()
	if gcAlloc != nil {
		t.Fatalf("bad gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_MakeRoomForAllocations_EnoughSpace(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	statsCollector := &MockStatsCollector{}
	gc := NewAllocGarbageCollector(logger, statsCollector, 20)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar1.waitCh)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar2.waitCh)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make stats collector report 200MB free out of which 20MB is reserved
	statsCollector.availableValues = []uint64{200 * MB}
	statsCollector.usedPercents = []float64{0}
	statsCollector.inodePercents = []float64{0}

	alloc := mock.Alloc()
	alloc.Resources.DiskMB = 150
	if err := gc.MakeRoomFor([]*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// When we have enough disk available and don't need to do any GC so we
	// should have two ARs in the GC queue
	for i := 0; i < 2; i++ {
		if gcAlloc := gc.allocRunners.Pop(); gcAlloc == nil {
			t.Fatalf("err: %v", gcAlloc)
		}
	}
}

func TestAllocGarbageCollector_MakeRoomForAllocations_GC_Partial(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	statsCollector := &MockStatsCollector{}
	gc := NewAllocGarbageCollector(logger, statsCollector, 20)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar1.waitCh)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar2.waitCh)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make stats collector report 80MB and 175MB free in subsequent calls
	statsCollector.availableValues = []uint64{80 * MB, 80 * MB, 175 * MB}
	statsCollector.usedPercents = []float64{0, 0, 0}
	statsCollector.inodePercents = []float64{0, 0, 0}

	alloc := mock.Alloc()
	alloc.Resources.DiskMB = 150
	if err := gc.MakeRoomFor([]*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should be GC-ing one alloc
	if gcAlloc := gc.allocRunners.Pop(); gcAlloc == nil {
		t.Fatalf("err: %v", gcAlloc)
	}

	if gcAlloc := gc.allocRunners.Pop(); gcAlloc != nil {
		t.Fatalf("gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_MakeRoomForAllocations_GC_All(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	statsCollector := &MockStatsCollector{}
	gc := NewAllocGarbageCollector(logger, statsCollector, 20)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar1.waitCh)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar2.waitCh)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make stats collector report 80MB and 95MB free in subsequent calls
	statsCollector.availableValues = []uint64{80 * MB, 80 * MB, 95 * MB}
	statsCollector.usedPercents = []float64{0, 0, 0}
	statsCollector.inodePercents = []float64{0, 0, 0}

	alloc := mock.Alloc()
	alloc.Resources.DiskMB = 150
	if err := gc.MakeRoomFor([]*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should be GC-ing all the alloc runners
	if gcAlloc := gc.allocRunners.Pop(); gcAlloc != nil {
		t.Fatalf("gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_MakeRoomForAllocations_GC_Fallback(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	statsCollector := &MockStatsCollector{}
	gc := NewAllocGarbageCollector(logger, statsCollector, 20)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar1.waitCh)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar2.waitCh)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc := mock.Alloc()
	alloc.Resources.DiskMB = 150
	if err := gc.MakeRoomFor([]*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should be GC-ing one alloc
	if gcAlloc := gc.allocRunners.Pop(); gcAlloc == nil {
		t.Fatalf("err: %v", gcAlloc)
	}

	if gcAlloc := gc.allocRunners.Pop(); gcAlloc != nil {
		t.Fatalf("gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_UsageBelowThreshold(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	statsCollector := &MockStatsCollector{}
	gc := NewAllocGarbageCollector(logger, statsCollector, 20)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar1.waitCh)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar2.waitCh)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	statsCollector.availableValues = []uint64{1000}
	statsCollector.usedPercents = []float64{20}
	statsCollector.inodePercents = []float64{10}

	if err := gc.keepUsageBelowThreshold(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We shouldn't GC any of the allocs since the used percent values are below
	// threshold
	for i := 0; i < 2; i++ {
		if gcAlloc := gc.allocRunners.Pop(); gcAlloc == nil {
			t.Fatalf("err: %v", gcAlloc)
		}
	}
}

func TestAllocGarbageCollector_UsedPercentThreshold(t *testing.T) {
	logger := log.New(os.Stdout, "", 0)
	statsCollector := &MockStatsCollector{}
	gc := NewAllocGarbageCollector(logger, statsCollector, 20)

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar1.waitCh)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	close(ar2.waitCh)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	statsCollector.availableValues = []uint64{1000, 800}
	statsCollector.usedPercents = []float64{85, 60}
	statsCollector.inodePercents = []float64{50, 30}

	if err := gc.keepUsageBelowThreshold(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should be GC-ing only one of the alloc runners since the second time
	// used percent returns a number below threshold.
	if gcAlloc := gc.allocRunners.Pop(); gcAlloc == nil {
		t.Fatalf("err: %v", gcAlloc)
	}

	if gcAlloc := gc.allocRunners.Pop(); gcAlloc != nil {
		t.Fatalf("gcAlloc: %v", gcAlloc)
	}
}
