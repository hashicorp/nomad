// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/hoststats"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
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

// exitAllocRunner is a helper that updates the allocs on the given alloc
// runners to be terminal
func exitAllocRunner(runners ...interfaces.AllocRunner) {
	for _, ar := range runners {
		terminalAlloc := ar.Alloc().Copy()
		terminalAlloc.DesiredStatus = structs.AllocDesiredStatusStop
		ar.Update(terminalAlloc)
	}
}

func TestIndexedGCAllocPQ(t *testing.T) {
	ci.Parallel(t)

	pq := NewIndexedGCAllocPQ()

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup2()
	ar3, cleanup3 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup3()
	ar4, cleanup4 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup4()

	pq.Push(ar1.Alloc().ID, ar1)
	pq.Push(ar2.Alloc().ID, ar2)
	pq.Push(ar3.Alloc().ID, ar3)
	pq.Push(ar4.Alloc().ID, ar4)

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

func (m *MockStatsCollector) Stats() *hoststats.HostStats {
	if len(m.availableValues) == 0 {
		return nil
	}

	available := m.availableValues[m.index]
	usedPercent := m.usedPercents[m.index]
	inodePercent := m.inodePercents[m.index]

	if m.index < len(m.availableValues)-1 {
		m.index = m.index + 1
	}
	return &hoststats.HostStats{
		AllocDirStats: &hoststats.DiskStats{
			Available:         available,
			UsedPercent:       usedPercent,
			InodesUsedPercent: inodePercent,
		},
	}
}

func TestAllocGarbageCollector_MarkForCollection(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, &MockAllocCounter{}, gcConfig())

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)

	gcAlloc := gc.allocRunners.Pop()
	if gcAlloc == nil || gcAlloc.allocRunner != ar1 {
		t.Fatalf("bad gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_Collect(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, &MockAllocCounter{}, gcConfig())

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup2()

	go ar1.Run()
	go ar2.Run()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)
	gc.MarkForCollection(ar2.Alloc().ID, ar2)

	// Exit the alloc runners
	exitAllocRunner(ar1, ar2)

	gc.Collect(ar1.Alloc().ID)
	gcAlloc := gc.allocRunners.Pop()
	if gcAlloc == nil || gcAlloc.allocRunner != ar2 {
		t.Fatalf("bad gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_CollectAll(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	gc := NewAllocGarbageCollector(logger, &MockStatsCollector{}, &MockAllocCounter{}, gcConfig())

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup2()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)
	gc.MarkForCollection(ar2.Alloc().ID, ar2)

	gc.CollectAll()
	gcAlloc := gc.allocRunners.Pop()
	if gcAlloc != nil {
		t.Fatalf("bad gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_MakeRoomForAllocations_EnoughSpace(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup2()

	go ar1.Run()
	go ar2.Run()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)
	gc.MarkForCollection(ar2.Alloc().ID, ar2)

	// Exit the alloc runners
	exitAllocRunner(ar1, ar2)

	// Make stats collector report 200MB free out of which 20MB is reserved
	statsCollector.availableValues = []uint64{200 * MB}
	statsCollector.usedPercents = []float64{0}
	statsCollector.inodePercents = []float64{0}

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.DiskMB = 150
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
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup2()

	go ar1.Run()
	go ar2.Run()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)
	gc.MarkForCollection(ar2.Alloc().ID, ar2)

	// Exit the alloc runners
	exitAllocRunner(ar1, ar2)

	// Make stats collector report 80MB and 175MB free in subsequent calls
	statsCollector.availableValues = []uint64{80 * MB, 80 * MB, 175 * MB}
	statsCollector.usedPercents = []float64{0, 0, 0}
	statsCollector.inodePercents = []float64{0, 0, 0}

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.DiskMB = 150
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
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup2()

	go ar1.Run()
	go ar2.Run()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)
	gc.MarkForCollection(ar2.Alloc().ID, ar2)

	// Exit the alloc runners
	exitAllocRunner(ar1, ar2)

	// Make stats collector report 80MB and 95MB free in subsequent calls
	statsCollector.availableValues = []uint64{80 * MB, 80 * MB, 95 * MB}
	statsCollector.usedPercents = []float64{0, 0, 0}
	statsCollector.inodePercents = []float64{0, 0, 0}

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.DiskMB = 150
	if err := gc.MakeRoomFor([]*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should be GC-ing all the alloc runners
	if gcAlloc := gc.allocRunners.Pop(); gcAlloc != nil {
		t.Fatalf("gcAlloc: %v", gcAlloc)
	}
}

func TestAllocGarbageCollector_MakeRoomForAllocations_GC_Fallback(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	cleanup2()

	go ar1.Run()
	go ar2.Run()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)
	gc.MarkForCollection(ar2.Alloc().ID, ar2)

	// Exit the alloc runners
	exitAllocRunner(ar1, ar2)

	alloc := mock.Alloc()
	alloc.AllocatedResources.Shared.DiskMB = 150
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

// TestAllocGarbageCollector_MakeRoomFor_MaxAllocs asserts that when making room for new
// allocs, terminal allocs are GC'd until old_allocs + new_allocs <= limit
func TestAllocGarbageCollector_MakeRoomFor_MaxAllocs(t *testing.T) {
	ci.Parallel(t)

	const maxAllocs = 6
	require := require.New(t)

	server, serverAddr, cleanupS := testServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, server.RPC)

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.GCMaxAllocs = maxAllocs
		c.GCDiskUsageThreshold = 100
		c.GCInodeUsageThreshold = 100
		c.GCParallelDestroys = 1
		c.GCInterval = time.Hour
		c.RPCHandler = server
		c.Servers = []string{serverAddr}
		c.ConsulConfig.ClientAutoJoin = new(bool)
	})
	defer cleanup()
	waitTilNodeReady(client, t)

	job := mock.Job()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "30s",
	}

	index := uint64(98)
	nextIndex := func() uint64 {
		index++
		return index
	}

	upsertJobFn := func(server *nomad.Server, j *structs.Job) {
		state := server.State()
		require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, nextIndex(), nil, j))
		require.NoError(state.UpsertJobSummary(nextIndex(), mock.JobSummary(j.ID)))
	}

	// Insert the Job
	upsertJobFn(server, job)

	upsertAllocFn := func(server *nomad.Server, a *structs.Allocation) {
		state := server.State()
		require.NoError(state.UpsertAllocs(structs.MsgTypeTestSetup, nextIndex(), []*structs.Allocation{a}))
	}

	upsertNewAllocFn := func(server *nomad.Server, j *structs.Job) *structs.Allocation {
		alloc := mock.Alloc()
		alloc.Job = j
		alloc.JobID = j.ID
		alloc.NodeID = client.NodeID()

		upsertAllocFn(server, alloc)

		return alloc.Copy()
	}

	var allocations []*structs.Allocation

	// Fill the node with allocations
	for i := 0; i < maxAllocs; i++ {
		allocations = append(allocations, upsertNewAllocFn(server, job))
	}

	// Wait until the allocations are ready
	testutil.WaitForResult(func() (bool, error) {
		ar := len(client.getAllocRunners())

		return ar == maxAllocs, fmt.Errorf("Expected %d allocs, got %d", maxAllocs, ar)
	}, func(err error) {
		t.Fatalf("Allocs did not start: %v", err)
	})

	// Mark the first three as terminal
	for i := 0; i < 3; i++ {
		allocations[i].DesiredStatus = structs.AllocDesiredStatusStop
		upsertAllocFn(server, allocations[i].Copy())
	}

	// Wait until the allocations are stopped
	testutil.WaitForResult(func() (bool, error) {
		ar := client.getAllocRunners()
		stopped := 0
		for _, r := range ar {
			if r.Alloc().TerminalStatus() {
				stopped++
			}
		}

		return stopped == 3, fmt.Errorf("Expected %d terminal allocs, got %d", 3, stopped)
	}, func(err error) {
		t.Fatalf("Allocs did not terminate: %v", err)
	})

	// Upsert a new allocation
	// This does not get appended to `allocations` as we do not use them again.
	upsertNewAllocFn(server, job)

	// A single allocation should be GC'd
	testutil.WaitForResult(func() (bool, error) {
		ar := client.getAllocRunners()
		destroyed := 0
		for _, r := range ar {
			if r.IsDestroyed() {
				destroyed++
			}
		}

		return destroyed == 1, fmt.Errorf("Expected %d gc'd ars, got %d", 1, destroyed)
	}, func(err error) {
		t.Fatalf("Allocs did not get GC'd: %v", err)
	})

	// Upsert a new allocation
	// This does not get appended to `allocations` as we do not use them again.
	upsertNewAllocFn(server, job)

	// 2 allocations should be GC'd
	testutil.WaitForResult(func() (bool, error) {
		ar := client.getAllocRunners()
		destroyed := 0
		for _, r := range ar {
			if r.IsDestroyed() {
				destroyed++
			}
		}

		return destroyed == 2, fmt.Errorf("Expected %d gc'd ars, got %d", 2, destroyed)
	}, func(err error) {
		t.Fatalf("Allocs did not get GC'd: %v", err)
	})

	// check that all 8 get run eventually
	testutil.WaitForResult(func() (bool, error) {
		ar := client.getAllocRunners()
		if len(ar) != 8 {
			return false, fmt.Errorf("expected 8 ARs, found %d: %v", len(ar), ar)
		}
		return true, nil
	}, func(err error) {
		require.NoError(err)
	})
}

func TestAllocGarbageCollector_UsageBelowThreshold(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup2()

	go ar1.Run()
	go ar2.Run()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)
	gc.MarkForCollection(ar2.Alloc().ID, ar2)

	// Exit the alloc runners
	exitAllocRunner(ar1, ar2)

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
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	statsCollector := &MockStatsCollector{}
	conf := gcConfig()
	conf.ReservedDiskMB = 20
	gc := NewAllocGarbageCollector(logger, statsCollector, &MockAllocCounter{}, conf)

	ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup1()
	ar2, cleanup2 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
	defer cleanup2()

	go ar1.Run()
	go ar2.Run()

	gc.MarkForCollection(ar1.Alloc().ID, ar1)
	gc.MarkForCollection(ar2.Alloc().ID, ar2)

	// Exit the alloc runners
	exitAllocRunner(ar1, ar2)

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
