// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/nomad/scheduler/tests"
	"github.com/shoenig/test/must"
)

// TestNetworkIndexSetNodePortRangeCaching tests that NetworkIndex.SetNode
// benefits from port range caching when processing nodes with repeated
// port range configurations.
func TestNetworkIndexSetNodePortRangeCaching(t *testing.T) {
	// Don't run in parallel to avoid cache interference
	
	// Clear cache before test to ensure clean state
	cache := structs.GetPortRangeCache()
	if cache != nil {
		cache.Clear()
	}

	// Common port range configurations that would be repeated across nodes
	commonPortRanges := []string{
		"22,80,443",
		"1000-2000",
		"8080,8443,9000-9100",
		"3000,5432,6379",
		"22,80,443,8080", // Overlapping with first to test cache hits
	}

	// Create multiple nodes with repeated port range configurations
	nodeCount := 100
	nodes := make([]*structs.Node, nodeCount)

	for i := 0; i < nodeCount; i++ {
		node := mock.Node()
		
		// Assign port ranges in a pattern that creates cache hits
		portRangeIndex := i % len(commonPortRanges)
		portRange := commonPortRanges[portRangeIndex]
		
		// Set up node with reserved ports
		node.ReservedResources = &structs.NodeReservedResources{
			Networks: structs.NodeReservedNetworkResources{
				ReservedHostPorts: portRange,
			},
		}
		
		// Add host networks with reserved ports
		node.NodeResources = &structs.NodeResources{
			NodeNetworks: []*structs.NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Speed:  1000,
					Addresses: []structs.NodeNetworkAddress{
						{
							Family:        structs.NodeNetworkAF_IPv4,
							Alias:         "default",
							Address:       fmt.Sprintf("192.168.1.%d", i+1),
							ReservedPorts: portRange,
						},
					},
				},
			},
		}
		
		nodes[i] = node
	}

	// Record initial cache stats
	initialHits, initialMisses := int64(0), int64(0)
	if cache != nil {
		initialHits, initialMisses = cache.Stats()
	}

	// Process all nodes through NetworkIndex.SetNode
	for _, node := range nodes {
		idx := structs.NewNetworkIndex()
		err := idx.SetNode(node)
		must.NoError(t, err)
	}

	// Verify cache hit rates
	if cache != nil {
		finalHits, finalMisses := cache.Stats()
		totalHits := finalHits - initialHits
		totalMisses := finalMisses - initialMisses
		totalRequests := totalHits + totalMisses

		t.Logf("Cache stats - Hits: %d, Misses: %d, Total: %d", totalHits, totalMisses, totalRequests)

		// We expect significant cache hits since we're reusing port ranges
		// Each node processes 2 port ranges (ReservedHostPorts + host network ReservedPorts)
		// With 100 nodes and 5 unique port ranges, we expect few misses and many hits
		
		// Verify we have reasonable cache performance
		// We should have few misses (only for unique port ranges) and many hits
		if totalMisses == 0 {
			t.Errorf("Expected some cache misses, got %d", totalMisses)
		}
		if totalMisses >= 20 {
			t.Errorf("Expected fewer than 20 cache misses, got %d", totalMisses)
		}
		if totalHits <= 50 {
			t.Errorf("Expected more than 50 cache hits, got %d", totalHits)
		}

		// Verify cache hit rate is reasonable (should be > 80%)
		hitRate := float64(totalHits) / float64(totalRequests)
		if hitRate < 0.8 {
			t.Errorf("Cache hit rate should be > 80%%, got %.2f%%", hitRate*100)
		}
	}
}

// TestSchedulerPerformanceWithPortRangeCaching tests scheduler performance
// improvements when using port range caching in realistic scheduling scenarios.
func TestSchedulerPerformanceWithPortRangeCaching(t *testing.T) {
	// Don't run in parallel to avoid cache interference
	
	// Clear cache before test
	cache := structs.GetPortRangeCache()
	if cache != nil {
		cache.Clear()
	}

	// Create a test harness
	h := tests.NewHarness(t)

	// Create nodes with common port range configurations
	nodeCount := 50
	commonPortRanges := []string{
		"22,80,443",
		"1000-2000,8080",
		"3000,5432,6379,9000-9100",
	}

	for i := 0; i < nodeCount; i++ {
		node := mock.Node()
		portRange := commonPortRanges[i%len(commonPortRanges)]
		
		node.ReservedResources = &structs.NodeReservedResources{
			Networks: structs.NodeReservedNetworkResources{
				ReservedHostPorts: portRange,
			},
		}
		
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job that will trigger scheduler evaluation
	job := mock.Job()
	job.TaskGroups[0].Count = 10
	job.TaskGroups[0].Tasks[0].Resources.Networks = []*structs.NetworkResource{
		{
			MBits:        10,
			DynamicPorts: []structs.Port{{Label: "http"}},
		},
	}

	// Record initial cache stats and memory usage
	var initialHits, initialMisses int64
	if cache != nil {
		initialHits, initialMisses = cache.Stats()
	}
	
	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// Submit job and measure scheduling performance
	startTime := time.Now()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	
	// Create and process evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	must.NoError(t, h.Process(scheduler.NewServiceScheduler, eval))
	schedulingDuration := time.Since(startTime)

	// Measure memory usage after scheduling
	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	// Verify cache performance
	if cache != nil {
		finalHits, finalMisses := cache.Stats()
		totalHits := finalHits - initialHits
		totalMisses := finalMisses - initialMisses
		totalRequests := totalHits + totalMisses

		t.Logf("Scheduling completed in %v", schedulingDuration)
		t.Logf("Cache stats - Hits: %d, Misses: %d, Total: %d", totalHits, totalMisses, totalRequests)
		t.Logf("Memory usage - Before: %d KB, After: %d KB, Delta: %d KB", 
			memBefore.Alloc/1024, memAfter.Alloc/1024, (memAfter.Alloc-memBefore.Alloc)/1024)

		// Verify we had cache activity (scheduler evaluated nodes)
		if totalRequests == 0 {
			t.Error("Expected cache activity during scheduling")
		}

		// Verify reasonable cache hit rate for repeated port ranges
		if totalRequests > 0 {
			hitRate := float64(totalHits) / float64(totalRequests)
			if hitRate < 0.5 {
				t.Errorf("Expected reasonable cache hit rate, got %.2f%%", hitRate*100)
			}
		}

		// Verify cache size is bounded
		cacheSize := cache.Len()
		if cacheSize > 256 {
			t.Errorf("Cache size should be bounded, got %d entries", cacheSize)
		}
	}

	// Verify scheduling was successful
	ws := memdb.NewWatchSet()
	allocs, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)
	must.Len(t, 10, allocs, must.Sprint("Expected 10 allocations to be scheduled"))

	// Verify no functional regressions - all allocs should be running
	for _, alloc := range allocs {
		must.Eq(t, structs.AllocDesiredStatusRun, alloc.DesiredStatus)
	}
}

// TestPortRangeCacheMemoryBounds tests that the port range cache
// maintains bounded memory usage under sustained load.
func TestPortRangeCacheMemoryBounds(t *testing.T) {
	// Don't run in parallel to avoid cache interference
	
	cache := structs.GetPortRangeCache()
	if cache == nil {
		t.Skip("Port range cache not available")
	}

	// Clear cache before test
	cache.Clear()

	// Generate many unique port ranges to test cache eviction
	uniquePortRanges := make([]string, 500) // More than cache size (256)
	for i := 0; i < 500; i++ {
		// Create unique port ranges to force cache eviction
		uniquePortRanges[i] = fmt.Sprintf("%d,%d-%d", 1000+i, 2000+i, 2010+i)
	}

	// Process port ranges to fill and overflow cache
	for _, portRange := range uniquePortRanges {
		_, err := structs.ParsePortRanges(portRange)
		must.NoError(t, err)
	}

	// Verify cache size is bounded
	cacheSize := cache.Len()
	if cacheSize > 256 {
		t.Errorf("Cache size should be bounded to 256, got %d", cacheSize)
	}

	// Verify cache statistics
	hits, misses := cache.Stats()
	t.Logf("Final cache stats - Hits: %d, Misses: %d, Size: %d", hits, misses, cacheSize)

	// All should be misses since we used unique port ranges
	if misses != 500 {
		t.Errorf("Expected 500 cache misses for unique port ranges, got %d", misses)
	}
	if hits != 0 {
		t.Errorf("Expected 0 cache hits for unique port ranges, got %d", hits)
	}
}

// TestPortRangeCacheConcurrentAccess tests that the port range cache
// handles concurrent access safely without data races.
func TestPortRangeCacheConcurrentAccess(t *testing.T) {
	// Don't run in parallel to avoid cache interference
	
	cache := structs.GetPortRangeCache()
	if cache == nil {
		t.Skip("Port range cache not available")
	}

	// Clear cache before test
	cache.Clear()

	// Common port ranges for concurrent access
	portRanges := []string{
		"22,80,443",
		"1000-2000",
		"8080,8443,9000-9100",
		"3000,5432,6379",
	}

	// Run concurrent goroutines accessing the cache
	const numGoroutines = 50
	const operationsPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for j := 0; j < operationsPerGoroutine; j++ {
				portRange := portRanges[j%len(portRanges)]
				
				// Parse port range (which uses cache)
				result, err := structs.ParsePortRanges(portRange)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if len(result) == 0 {
					t.Error("Expected non-empty result")
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify cache statistics
	hits, misses := cache.Stats()
	totalRequests := hits + misses
	expectedRequests := int64(numGoroutines * operationsPerGoroutine)

	// Allow some tolerance for concurrent execution variations
	if totalRequests < expectedRequests-50 || totalRequests > expectedRequests+50 {
		t.Errorf("Expected approximately %d total requests, got %d", expectedRequests, totalRequests)
	}

	// Should have significant cache hits due to repeated port ranges
	hitRate := float64(hits) / float64(totalRequests)
	if hitRate < 0.9 {
		t.Errorf("Expected high cache hit rate with concurrent access, got %.2f%%", hitRate*100)
	}

	t.Logf("Concurrent access test - Hits: %d, Misses: %d, Hit Rate: %.2f%%", hits, misses, hitRate*100)
}