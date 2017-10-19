package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
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
	gc.MarkForCollection(ar1)

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
	gc.MarkForCollection(ar1)
	gc.MarkForCollection(ar2)

	// Fake that ar.Run() exits
	close(ar1.waitCh)
	close(ar2.waitCh)

	gc.Collect(ar1.Alloc().ID)
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
	gc.MarkForCollection(ar1)
	gc.MarkForCollection(ar2)

	gc.CollectAll()
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
	gc.MarkForCollection(ar1)
	gc.MarkForCollection(ar2)

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
	gc.MarkForCollection(ar1)
	gc.MarkForCollection(ar2)

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
	gc.MarkForCollection(ar1)
	gc.MarkForCollection(ar2)

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
	gc.MarkForCollection(ar1)
	gc.MarkForCollection(ar2)

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

// TestAllocGarbageCollector_MakeRoomForAllocations_MaxAllocs asserts that when
// making room for new allocs, terminal allocs are GC'd until old_allocs +
// new_allocs <= limit
func TestAllocGarbageCollector_MakeRoomForAllocations_MaxAllocs(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	server, serverAddr := testServer(t, nil)
	defer server.Shutdown()
	testutil.WaitForLeader(t, server.RPC)

	const maxAllocs = 6
	client := testClient(t, func(c *config.Config) {
		c.GCMaxAllocs = maxAllocs
		c.RPCHandler = server
		c.Servers = []string{serverAddr}
		c.ConsulConfig.ClientAutoJoin = new(bool) // squelch logs
	})
	defer client.Shutdown()
	waitTilNodeReady(client, t)

	assertAllocs := func(expectedAll, expectedDestroyed int) {
		// Wait for allocs to be started
		testutil.WaitForResult(func() (bool, error) {
			all, destroyed := 0, 0
			for _, ar := range client.getAllocRunners() {
				all++
				if ar.IsDestroyed() {
					destroyed++
				}
			}
			return all == expectedAll && destroyed == expectedDestroyed, fmt.Errorf(
				"expected %d allocs (found %d); expected %d destroy (found %d)",
				expectedAll, all, expectedDestroyed, destroyed,
			)
		}, func(err error) {
			t.Fatalf("alloc state: %v", err)
		})
	}

	// Create a job
	state := server.State()
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	job.TaskGroups[0].Tasks[0].Config["run_for"] = "30s"
	nodeID := client.Node().ID
	if err := state.UpsertJob(98, job); err != nil {
		t.Fatalf("error upserting job: %v", err)
	}
	if err := state.UpsertJobSummary(99, mock.JobSummary(job.ID)); err != nil {
		t.Fatalf("error upserting job summary: %v", err)
	}

	newAlloc := func() *structs.Allocation {
		alloc := mock.Alloc()
		alloc.JobID = job.ID
		alloc.Job = job
		alloc.NodeID = nodeID
		return alloc
	}

	// Create the allocations
	allocs := make([]*structs.Allocation, 7)
	for i := 0; i < len(allocs); i++ {
		allocs[i] = newAlloc()
	}
	if err := state.UpsertAllocs(100, allocs); err != nil {
		t.Fatalf("error upserting initial allocs: %v", err)
	}

	// 7 total, 0 GC'd
	assertAllocs(7, 0)

	// Set the first few as terminal so they're marked for gc
	const terminalN = 4
	for i := 0; i < terminalN; i++ {
		// Copy the alloc so the pointers aren't shared
		alloc := allocs[i].Copy()
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		allocs[i] = alloc
	}
	if err := state.UpsertAllocs(101, allocs[:terminalN]); err != nil {
		t.Fatalf("error upserting stopped allocs: %v", err)
	}

	// 7 total, 0 GC'd still, but 4 should be marked for GC
	assertAllocs(7, 0)

	// Add one more alloc
	if err := state.UpsertAllocs(102, []*structs.Allocation{newAlloc()}); err != nil {
		t.Fatalf("error upserting new alloc: %v", err)
	}

	// 8 total, 2 GC'd to get down to limit of 6
	assertAllocs(8, 2)

	// Add new allocs to cause the gc of old terminal ones
	newAllocs := make([]*structs.Allocation, 4)
	for i := 0; i < len(newAllocs); i++ {
		newAllocs[i] = newAlloc()
	}
	assert.Nil(state.UpsertAllocs(200, newAllocs))

	// 12 total, 4 GC'd total because all other allocs are alive
	assertAllocs(12, 4)
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
	gc.MarkForCollection(ar1)
	gc.MarkForCollection(ar2)

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
	gc.MarkForCollection(ar1)
	gc.MarkForCollection(ar2)

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
