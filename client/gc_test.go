package client

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func gcConfig() *GCConfig {
	return &GCConfig{
		DiskUsageThreshold:  80,
		InodeUsageThreshold: 70,
		Interval:            1 * time.Minute,
		ReservedDiskMB:      0,
		MaxAllocs:           100,
	}
}

func TestIndexedGCAllocPQ(t *testing.T) {
	t.Parallel()
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

// MockAllocCounter implements AllocCounter interface.
type MockAllocCounter struct {
	allocs int
}

func (m *MockAllocCounter) NumAllocs() int {
	return m.allocs
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
	t.Parallel()
	logger := testLogger()
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, &MockAllocCounter{}, gcConfig())

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
	t.Parallel()
	logger := testLogger()
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, &MockAllocCounter{}, gcConfig())

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	if err := gc.MarkForCollection(ar1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := gc.MarkForCollection(ar2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Fake that ar.Run() exits
	close(ar1.waitCh)
	close(ar2.waitCh)

	if err := gc.Collect(ar1.Alloc().ID); err != nil {
		t.Fatalf("err: %v", err)
	}
	gcAlloc := gc.allocRunners.Pop()
	if gcAlloc == nil || gcAlloc.allocRunner != ar2 {
		t.Fatalf("bad gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_CollectAll(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, &MockAllocCounter{}, gcConfig())

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
	t.Parallel()
	logger := testLogger()
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

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
	t.Parallel()
	logger := testLogger()
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

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
	t.Parallel()
	logger := testLogger()
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

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
	t.Parallel()
	logger := testLogger()
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

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

func TestAllocGarbageCollector_MakeRoomForAllocations_MaxAllocs(t *testing.T) {
	t.Parallel()
	const (
		liveAllocs   = 3
		maxAllocs    = 6
		gcAllocs     = 4
		gcAllocsLeft = 1
	)

	logger := testLogger()
	statsCollector := &MockStatsCollector{
		availableValues: []uint64{10 * 1024 * MB},
		usedPercents:    []float64{0},
		inodePercents:   []float64{0},
	}
	allocCounter := &MockAllocCounter{allocs: liveAllocs}
	conf := gcConfig()
	conf.MaxAllocs = maxAllocs
	gc := NewAllocGarbageCollector(logger, statsCollector, allocCounter, conf)

	for i := 0; i < gcAllocs; i++ {
		_, ar := testAllocRunnerFromAlloc(mock.Alloc(), false)
		close(ar.waitCh)
		if err := gc.MarkForCollection(ar); err != nil {
			t.Fatalf("error marking alloc for gc: %v", err)
		}
	}

	if err := gc.MakeRoomFor([]*structs.Allocation{mock.Alloc(), mock.Alloc()}); err != nil {
		t.Fatalf("error making room for 2 new allocs: %v", err)
	}

	// There should be gcAllocsLeft alloc runners left to be collected
	if n := len(gc.allocRunners.index); n != gcAllocsLeft {
		t.Fatalf("expected %d remaining GC-able alloc runners but found %d", gcAllocsLeft, n)
	}
}

func TestAllocGarbageCollector_UsageBelowThreshold(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

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
	t.Parallel()
	logger := testLogger()
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

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

func TestAllocGarbageCollector_MaxAllocsThreshold(t *testing.T) {
	t.Parallel()
	const (
		liveAllocs   = 3
		maxAllocs    = 6
		gcAllocs     = 4
		gcAllocsLeft = 1
	)

	logger := testLogger()
	statsCollector := &MockStatsCollector{
		availableValues: []uint64{1000},
		usedPercents:    []float64{0},
		inodePercents:   []float64{0},
	}
	allocCounter := &MockAllocCounter{allocs: liveAllocs}
	conf := gcConfig()
	conf.MaxAllocs = 4
	gc := NewAllocGarbageCollector(logger, statsCollector, allocCounter, conf)

	for i := 0; i < gcAllocs; i++ {
		_, ar := testAllocRunnerFromAlloc(mock.Alloc(), false)
		close(ar.waitCh)
		if err := gc.MarkForCollection(ar); err != nil {
			t.Fatalf("error marking alloc for gc: %v", err)
		}
	}

	if err := gc.keepUsageBelowThreshold(); err != nil {
		t.Fatalf("error gc'ing: %v", err)
	}

	// We should have gc'd down to MaxAllocs
	if n := len(gc.allocRunners.index); n != gcAllocsLeft {
		t.Fatalf("expected remaining gc allocs (%d) to equal %d", n, gcAllocsLeft)
	}
}
