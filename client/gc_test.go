// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/hoststats"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
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

func TestAllocGarbageCollector_KeepUsageBelowThreshold(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name    string
		counter *MockAllocCounter
		stats   *MockStatsCollector
		expGC   bool
	}{
		{
			name:    "garbage collects alloc when disk usage above threshold",
			counter: &MockAllocCounter{},
			stats: &MockStatsCollector{
				availableValues: []uint64{0, 0},
				usedPercents:    []float64{85, 85}, // above threshold
				inodePercents:   []float64{0, 0},
			},
			expGC: true,
		},
		{
			name:    "garbage collects alloc when inode usage above threshold",
			counter: &MockAllocCounter{},
			stats: &MockStatsCollector{
				availableValues: []uint64{0, 0},
				usedPercents:    []float64{0, 0},
				inodePercents:   []float64{90, 90}, // above threshold
			},
			expGC: true,
		},
		{
			name: "garbage collects alloc when liveAllocs above maxAllocs threshold",
			counter: &MockAllocCounter{
				allocs: 150, // above threshold
			},
			stats: &MockStatsCollector{
				availableValues: []uint64{0, 0},
				usedPercents:    []float64{0, 0},
				inodePercents:   []float64{0, 0},
			},
			expGC: true,
		},
		{
			name: "exits when there is no reason to GC",
			counter: &MockAllocCounter{
				allocs: 0,
			},
			stats: &MockStatsCollector{
				availableValues: []uint64{0, 0},
				usedPercents:    []float64{0, 0},
				inodePercents:   []float64{0, 0},
			},
			expGC: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testlog.HCLogger(t)
			gc := NewAllocGarbageCollector(logger, tc.stats, tc.counter, gcConfig())

			// add a single alloc for garbage collection
			ar1, cleanup1 := allocrunner.TestAllocRunnerFromAlloc(t, mock.Alloc())
			defer cleanup1()
			exitAllocRunner(ar1)
			gc.MarkForCollection(ar1.Alloc().ID, ar1)

			// gc
			err := gc.keepUsageBelowThreshold()
			must.NoError(t, err)

			gcAlloc := gc.allocRunners.Pop()
			if tc.expGC {
				must.Nil(t, gcAlloc)
			} else {
				must.NotNil(t, gcAlloc)
			}
		})
	}
}
