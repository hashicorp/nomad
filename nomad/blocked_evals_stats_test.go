// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func now(year int) time.Time {
	return time.Date(2000+year, 1, 2, 3, 4, 5, 6, time.UTC)
}

func TestBlockedResourceSummary_Add(t *testing.T) {
	now1 := now(1)
	now2 := now(2)
	a := BlockedResourcesSummary{
		Timestamp: now1,
		CPU:       600,
		MemoryMB:  256,
	}

	b := BlockedResourcesSummary{
		Timestamp: now2,
		CPU:       250,
		MemoryMB:  128,
	}

	result := a.Add(b)

	// a not modified
	require.Equal(t, 600, a.CPU)
	require.Equal(t, 256, a.MemoryMB)
	require.Equal(t, now1, a.Timestamp)

	// b not modified
	require.Equal(t, 250, b.CPU)
	require.Equal(t, 128, b.MemoryMB)
	require.Equal(t, now2, b.Timestamp)

	// result is a + b, using timestamp from b
	require.Equal(t, 850, result.CPU)
	require.Equal(t, 384, result.MemoryMB)
	require.Equal(t, now2, result.Timestamp)
}

func TestBlockedResourceSummary_Add_nil(t *testing.T) {
	now1 := now(1)
	b := BlockedResourcesSummary{
		Timestamp: now1,
		CPU:       250,
		MemoryMB:  128,
	}

	// zero + b == b
	result := (BlockedResourcesSummary{}).Add(b)
	require.Equal(t, now1, result.Timestamp)
	require.Equal(t, 250, result.CPU)
	require.Equal(t, 128, result.MemoryMB)
}

func TestBlockedResourceSummary_Subtract(t *testing.T) {
	now1 := now(1)
	now2 := now(2)
	a := BlockedResourcesSummary{
		Timestamp: now1,
		CPU:       600,
		MemoryMB:  256,
	}

	b := BlockedResourcesSummary{
		Timestamp: now2,
		CPU:       250,
		MemoryMB:  120,
	}

	result := a.Subtract(b)

	// a not modified
	require.Equal(t, 600, a.CPU)
	require.Equal(t, 256, a.MemoryMB)
	require.Equal(t, now1, a.Timestamp)

	// b not modified
	require.Equal(t, 250, b.CPU)
	require.Equal(t, 120, b.MemoryMB)
	require.Equal(t, now2, b.Timestamp)

	// result is a + b, using timestamp from b
	require.Equal(t, 350, result.CPU)
	require.Equal(t, 136, result.MemoryMB)
	require.Equal(t, now2, result.Timestamp)
}

func TestBlockedResourceSummary_IsZero(t *testing.T) {
	now1 := now(1)

	// cpu and mem zero, timestamp is ignored
	require.True(t, (&BlockedResourcesSummary{
		Timestamp: now1,
		CPU:       0,
		MemoryMB:  0,
	}).IsZero())

	// cpu non-zero
	require.False(t, (&BlockedResourcesSummary{
		Timestamp: now1,
		CPU:       1,
		MemoryMB:  0,
	}).IsZero())

	// mem non-zero
	require.False(t, (&BlockedResourcesSummary{
		Timestamp: now1,
		CPU:       0,
		MemoryMB:  1,
	}).IsZero())
}

func TestBlockedResourceStats_New(t *testing.T) {
	a := NewBlockedResourcesStats()
	require.NotNil(t, a.ByJob)
	require.Empty(t, a.ByJob)
	require.NotNil(t, a.ByClassInDC)
	require.Empty(t, a.ByClassInDC)
}

var (
	id1 = structs.NamespacedID{
		ID:        "1",
		Namespace: "one",
	}

	id2 = structs.NamespacedID{
		ID:        "2",
		Namespace: "two",
	}

	node1 = classInDC{
		dc:    "dc1",
		class: "alpha",
	}

	node2 = classInDC{
		dc:    "dc1",
		class: "beta",
	}

	node3 = classInDC{
		dc:    "dc1",
		class: "", // not set
	}
)

func TestBlockedResourceStats_Copy(t *testing.T) {
	now1 := now(1)
	now2 := now(2)

	a := NewBlockedResourcesStats()
	a.ByJob = map[structs.NamespacedID]BlockedResourcesSummary{
		id1: {
			Timestamp: now1,
			CPU:       100,
			MemoryMB:  256,
		},
	}
	a.ByClassInDC = map[classInDC]BlockedResourcesSummary{
		node1: {
			Timestamp: now1,
			CPU:       300,
			MemoryMB:  333,
		},
	}

	c := a.Copy()
	c.ByJob[id1] = BlockedResourcesSummary{
		Timestamp: now2,
		CPU:       888,
		MemoryMB:  888,
	}
	c.ByClassInDC[node1] = BlockedResourcesSummary{
		Timestamp: now2,
		CPU:       999,
		MemoryMB:  999,
	}

	// underlying data should have been deep copied
	require.Equal(t, 100, a.ByJob[id1].CPU)
	require.Equal(t, 300, a.ByClassInDC[node1].CPU)
}

func TestBlockedResourcesStats_Add(t *testing.T) {
	a := NewBlockedResourcesStats()
	a.ByJob = map[structs.NamespacedID]BlockedResourcesSummary{
		id1: {Timestamp: now(1), CPU: 111, MemoryMB: 222},
	}
	a.ByClassInDC = map[classInDC]BlockedResourcesSummary{
		node1: {Timestamp: now(2), CPU: 333, MemoryMB: 444},
	}

	b := NewBlockedResourcesStats()
	b.ByJob = map[structs.NamespacedID]BlockedResourcesSummary{
		id1: {Timestamp: now(3), CPU: 200, MemoryMB: 300},
		id2: {Timestamp: now(4), CPU: 400, MemoryMB: 500},
	}
	b.ByClassInDC = map[classInDC]BlockedResourcesSummary{
		node1: {Timestamp: now(5), CPU: 600, MemoryMB: 700},
		node2: {Timestamp: now(6), CPU: 800, MemoryMB: 900},
	}

	t.Run("a add b", func(t *testing.T) {
		result := a.Add(b)

		require.Equal(t, map[structs.NamespacedID]BlockedResourcesSummary{
			id1: {Timestamp: now(3), CPU: 311, MemoryMB: 522},
			id2: {Timestamp: now(4), CPU: 400, MemoryMB: 500},
		}, result.ByJob)

		require.Equal(t, map[classInDC]BlockedResourcesSummary{
			node1: {Timestamp: now(5), CPU: 933, MemoryMB: 1144},
			node2: {Timestamp: now(6), CPU: 800, MemoryMB: 900},
		}, result.ByClassInDC)
	})

	// make sure we handle zeros in both directions
	// and timestamps originate from rhs
	t.Run("b add a", func(t *testing.T) {
		result := b.Add(a)
		require.Equal(t, map[structs.NamespacedID]BlockedResourcesSummary{
			id1: {Timestamp: now(1), CPU: 311, MemoryMB: 522},
			id2: {Timestamp: now(4), CPU: 400, MemoryMB: 500},
		}, result.ByJob)

		require.Equal(t, map[classInDC]BlockedResourcesSummary{
			node1: {Timestamp: now(2), CPU: 933, MemoryMB: 1144},
			node2: {Timestamp: now(6), CPU: 800, MemoryMB: 900},
		}, result.ByClassInDC)
	})
}

func TestBlockedResourcesStats_Add_NoClass(t *testing.T) {
	a := NewBlockedResourcesStats()
	a.ByClassInDC = map[classInDC]BlockedResourcesSummary{
		node3: {Timestamp: now(1), CPU: 111, MemoryMB: 1111},
	}
	result := a.Add(a)
	require.Equal(t, map[classInDC]BlockedResourcesSummary{
		node3: {Timestamp: now(1), CPU: 222, MemoryMB: 2222},
	}, result.ByClassInDC)
}

func TestBlockedResourcesStats_Subtract(t *testing.T) {
	a := NewBlockedResourcesStats()
	a.ByJob = map[structs.NamespacedID]BlockedResourcesSummary{
		id1: {Timestamp: now(1), CPU: 100, MemoryMB: 100},
		id2: {Timestamp: now(2), CPU: 200, MemoryMB: 200},
	}
	a.ByClassInDC = map[classInDC]BlockedResourcesSummary{
		node1: {Timestamp: now(3), CPU: 300, MemoryMB: 300},
		node2: {Timestamp: now(4), CPU: 400, MemoryMB: 400},
	}

	b := NewBlockedResourcesStats()
	b.ByJob = map[structs.NamespacedID]BlockedResourcesSummary{
		id1: {Timestamp: now(5), CPU: 10, MemoryMB: 11},
		id2: {Timestamp: now(6), CPU: 12, MemoryMB: 13},
	}
	b.ByClassInDC = map[classInDC]BlockedResourcesSummary{
		node1: {Timestamp: now(7), CPU: 14, MemoryMB: 15},
		node2: {Timestamp: now(8), CPU: 16, MemoryMB: 17},
	}

	result := a.Subtract(b)

	// id1
	require.Equal(t, now(5), result.ByJob[id1].Timestamp)
	require.Equal(t, 90, result.ByJob[id1].CPU)
	require.Equal(t, 89, result.ByJob[id1].MemoryMB)

	// id2
	require.Equal(t, now(6), result.ByJob[id2].Timestamp)
	require.Equal(t, 188, result.ByJob[id2].CPU)
	require.Equal(t, 187, result.ByJob[id2].MemoryMB)

	// node1
	require.Equal(t, now(7), result.ByClassInDC[node1].Timestamp)
	require.Equal(t, 286, result.ByClassInDC[node1].CPU)
	require.Equal(t, 285, result.ByClassInDC[node1].MemoryMB)

	// node2
	require.Equal(t, now(8), result.ByClassInDC[node2].Timestamp)
	require.Equal(t, 384, result.ByClassInDC[node2].CPU)
	require.Equal(t, 383, result.ByClassInDC[node2].MemoryMB)
}

// testBlockedEvalsRandomBlockedEval wraps an eval that is randomly generated.
type testBlockedEvalsRandomBlockedEval struct {
	eval *structs.Evaluation
}

// Generate returns a random eval.
func (t testBlockedEvalsRandomBlockedEval) Generate(rand *rand.Rand, _ int) reflect.Value {
	resourceTypes := []string{"cpu", "memory"}

	// Start with a mock eval.
	e := mock.BlockedEval()

	// Get how many task groups, datacenters and node classes to generate.
	// Add 1 to avoid 0.
	jobCount := rand.Intn(3) + 1
	tgCount := rand.Intn(10) + 1
	dcCount := rand.Intn(3) + 1
	nodeClassCount := rand.Intn(3) + 1

	failedTGAllocs := map[string]*structs.AllocMetric{}

	e.JobID = fmt.Sprintf("job-%d", jobCount)
	for tg := 1; tg <= tgCount; tg++ {
		tgName := fmt.Sprintf("group-%d", tg)

		// Get which resource type to use for this task group.
		// Nomad stops at the first dimension that is exhausted, so only 1 is
		// added per task group.
		i := rand.Int() % len(resourceTypes)
		resourceType := resourceTypes[i]

		failedTGAllocs[tgName] = &structs.AllocMetric{
			DimensionExhausted: map[string]int{
				resourceType: 1,
			},
			NodesAvailable: map[string]int{},
			ClassExhausted: map[string]int{},
		}

		for dc := 1; dc <= dcCount; dc++ {
			dcName := fmt.Sprintf("dc%d", dc)
			failedTGAllocs[tgName].NodesAvailable[dcName] = 1
		}

		for nc := 1; nc <= nodeClassCount; nc++ {
			nodeClassName := fmt.Sprintf("node-class-%d", nc)
			failedTGAllocs[tgName].ClassExhausted[nodeClassName] = 1
		}

		// Generate resources for each task.
		taskCount := rand.Intn(5) + 1
		resourcesExhausted := map[string]*structs.Resources{}

		for t := 1; t <= taskCount; t++ {
			task := fmt.Sprintf("task-%d", t)
			resourcesExhausted[task] = &structs.Resources{}

			resourceAmount := rand.Intn(1000)
			switch resourceType {
			case "cpu":
				resourcesExhausted[task].CPU = resourceAmount
			case "memory":
				resourcesExhausted[task].MemoryMB = resourceAmount
			}
		}
		failedTGAllocs[tgName].ResourcesExhausted = resourcesExhausted
	}
	e.FailedTGAllocs = failedTGAllocs
	t.eval = e
	return reflect.ValueOf(t)
}

// clearTimestampFromBlockedResourceStats set timestamp metrics to zero to
// avoid invalid comparisons.
func clearTimestampFromBlockedResourceStats(b *BlockedResourcesStats) {
	for k, v := range b.ByJob {
		v.Timestamp = time.Time{}
		b.ByJob[k] = v
	}
	for k, v := range b.ByClassInDC {
		v.Timestamp = time.Time{}
		b.ByClassInDC[k] = v
	}
}

// TestBlockedEvalsStats_BlockedResources generates random evals and processes
// them using the expected code paths and a manual check of the expeceted result.
func TestBlockedEvalsStats_BlockedResources(t *testing.T) {
	ci.Parallel(t)
	blocked, _ := testBlockedEvals(t)

	// evalHistory stores all evals generated during the test.
	var evalHistory []*structs.Evaluation

	// blockedEvals keeps track if evals are blocked or unblocked.
	blockedEvals := map[string]bool{}

	// blockAndUntrack processes the generated evals in order using a
	// BlockedEvals instance.
	blockAndUntrack := func(testEval testBlockedEvalsRandomBlockedEval, block bool, unblockIdx uint16) *BlockedResourcesStats {
		if block || len(evalHistory) == 0 {
			blocked.Block(testEval.eval)
		} else {
			i := int(unblockIdx) % len(evalHistory)
			eval := evalHistory[i]
			blocked.Untrack(eval.JobID, eval.Namespace)
		}

		// Remove zero stats from unblocked evals.
		blocked.pruneStats(time.Now().UTC())

		result := blocked.Stats().BlockedResources
		clearTimestampFromBlockedResourceStats(result)
		return result
	}

	// manualCount processes only the blocked evals and generate a
	// BlockedResourcesStats result directly from the eval history.
	manualCount := func(testEval testBlockedEvalsRandomBlockedEval, block bool, unblockIdx uint16) *BlockedResourcesStats {
		if block || len(evalHistory) == 0 {
			evalHistory = append(evalHistory, testEval.eval)

			// Find and unblock evals for the same job.
			for _, e := range evalHistory {
				if e.Namespace == testEval.eval.Namespace && e.JobID == testEval.eval.JobID {
					blockedEvals[e.ID] = false
				}
			}
			blockedEvals[testEval.eval.ID] = true
		} else {
			i := int(unblockIdx) % len(evalHistory)
			eval := evalHistory[i]

			// Find and unlock all evals for this job.
			for _, e := range evalHistory {
				if e.Namespace == eval.Namespace && e.JobID == eval.JobID {
					blockedEvals[e.ID] = false
				}
			}
		}

		result := NewBlockedResourcesStats()
		for _, e := range evalHistory {
			if !blockedEvals[e.ID] {
				continue
			}
			result = result.Add(generateResourceStats(e))
		}
		clearTimestampFromBlockedResourceStats(result)
		return result
	}

	err := quick.CheckEqual(blockAndUntrack, manualCount, nil)
	if err != nil {
		t.Error(err)
	}
}
