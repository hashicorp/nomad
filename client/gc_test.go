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

// TODO: (mismithhisler) refactor to table driven test UseBelowThreshold
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
