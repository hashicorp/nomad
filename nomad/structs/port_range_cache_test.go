// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func TestPortRangeCacheEntry(t *testing.T) {
	ports := []uint64{80, 443, 8080}
	entry := PortRangeCacheEntry{
		Ports:     ports,
		Timestamp: time.Now(),
	}

	// Test Get method
	result := entry.Get()
	must.Eq(t, ports, result)

	// Test Age method
	age := entry.Age()
	must.True(t, age >= 0)
	must.True(t, age < time.Second) // Should be very recent
}

func TestNewPortRangeCache(t *testing.T) {
	cache, err := NewPortRangeCache(10)
	must.NoError(t, err)
	must.NotNil(t, cache)
	must.NotNil(t, cache.TwoQueueCache)
}

func TestPortRangeCache_GetAdd(t *testing.T) {
	cache, err := NewPortRangeCache(10)
	must.NoError(t, err)

	key := "80,443,8080"
	ports := []uint64{80, 443, 8080}

	// Test cache miss
	result, found := cache.Get(key)
	must.False(t, found)
	must.Nil(t, result)

	// Add to cache
	cache.Add(key, ports)

	// Test cache hit
	result, found = cache.Get(key)
	must.True(t, found)
	must.Eq(t, ports, result)
}

func TestPortRangeCache_NilSafety(t *testing.T) {
	var cache *PortRangeCache

	// Test nil cache doesn't panic
	result, found := cache.Get("test")
	must.False(t, found)
	must.Nil(t, result)

	// Test nil cache Add doesn't panic
	cache.Add("test", []uint64{80})
}

func TestGetPortRangeCache(t *testing.T) {
	// Reset the global cache for testing
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	cache := getPortRangeCache()
	must.NotNil(t, cache)

	// Test that subsequent calls return the same instance
	cache2 := getPortRangeCache()
	must.Eq(t, cache, cache2)
}

func TestPortRangeCache_ConcurrentAccess(t *testing.T) {
	cache, err := NewPortRangeCache(100)
	must.NoError(t, err)

	// Test concurrent access
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			key := fmt.Sprintf("ports-%d", id)
			ports := []uint64{uint64(8000 + id)}
			
			// Add to cache
			cache.Add(key, ports)
			
			// Read from cache
			result, found := cache.Get(key)
			must.True(t, found)
			must.Eq(t, ports, result)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPortRangeCache_Integration(t *testing.T) {
	// Test that the global cache can be used for actual port range operations
	cache := getPortRangeCache()
	must.NotNil(t, cache)

	// Test with realistic port range strings
	testCases := []struct {
		key   string
		ports []uint64
	}{
		{"80,443", []uint64{80, 443}},
		{"8000-8002", []uint64{8000, 8001, 8002}},
		{"22,80,443,8080", []uint64{22, 80, 443, 8080}},
	}

	for _, tc := range testCases {
		// Cache miss initially
		result, found := cache.Get(tc.key)
		must.False(t, found)
		must.Nil(t, result)

		// Add to cache
		cache.Add(tc.key, tc.ports)

		// Cache hit after adding
		result, found = cache.Get(tc.key)
		must.True(t, found)
		must.Eq(t, tc.ports, result)
	}
}

// TestParsePortRanges_CacheHit tests that cached results are returned correctly
func TestParsePortRanges_CacheHit(t *testing.T) {
	// Reset global cache for clean test
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	testCases := []struct {
		name     string
		spec     string
		expected []uint64
	}{
		{
			name:     "single port",
			spec:     "80",
			expected: []uint64{80},
		},
		{
			name:     "multiple ports",
			spec:     "80,443,8080",
			expected: []uint64{80, 443, 8080},
		},
		{
			name:     "port range",
			spec:     "8000-8002",
			expected: []uint64{8000, 8001, 8002},
		},
		{
			name:     "mixed ports and ranges",
			spec:     "22,80,8000-8002,9000",
			expected: []uint64{22, 80, 8000, 8001, 8002, 9000},
		},
		{
			name:     "empty spec",
			spec:     "",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// First call should parse and cache
			result1, err := ParsePortRanges(tc.spec)
			must.NoError(t, err)
			must.Eq(t, tc.expected, result1)

			// Second call should return cached result
			result2, err := ParsePortRanges(tc.spec)
			must.NoError(t, err)
			must.Eq(t, tc.expected, result2)
			must.Eq(t, result1, result2)

			// Verify it's actually cached by checking the cache directly
			cache := getPortRangeCache()
			if tc.expected != nil || tc.spec != "" {
				cached, found := cache.Get(tc.spec)
				must.True(t, found)
				must.Eq(t, tc.expected, cached)
			}
		})
	}
}

// TestParsePortRanges_CacheMiss tests that cache misses trigger original parsing
func TestParsePortRanges_CacheMiss(t *testing.T) {
	// Reset global cache for clean test
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	cache := getPortRangeCache()
	must.NotNil(t, cache)

	// Verify cache is empty initially
	result, found := cache.Get("80,443")
	must.False(t, found)
	must.Nil(t, result)

	// Parse should work even with empty cache
	parsed, err := ParsePortRanges("80,443")
	must.NoError(t, err)
	must.Eq(t, []uint64{80, 443}, parsed)

	// Now it should be cached
	result, found = cache.Get("80,443")
	must.True(t, found)
	must.Eq(t, []uint64{80, 443}, result)
}

// TestParsePortRanges_ConcurrentAccess tests thread safety with multiple goroutines
func TestParsePortRanges_ConcurrentAccess(t *testing.T) {
	// Reset global cache for clean test
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	const numGoroutines = 50
	const numIterations = 10

	// Test data - mix of different port specs
	testSpecs := []struct {
		spec     string
		expected []uint64
	}{
		{"80", []uint64{80}},
		{"443", []uint64{443}},
		{"80,443", []uint64{80, 443}},
		{"8000-8002", []uint64{8000, 8001, 8002}},
		{"22,80,443", []uint64{22, 80, 443}},
		{"9000-9001,9003", []uint64{9000, 9001, 9003}},
	}

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numIterations*len(testSpecs))

	// Launch multiple goroutines that concurrently parse port ranges
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numIterations; j++ {
				for _, tc := range testSpecs {
					result, err := ParsePortRanges(tc.spec)
					if err != nil {
						errors <- fmt.Errorf("goroutine %d iteration %d spec %s: %v", goroutineID, j, tc.spec, err)
						continue
					}
					
					// Verify result matches expected
					if len(result) != len(tc.expected) {
						errors <- fmt.Errorf("goroutine %d iteration %d spec %s: length mismatch got %d expected %d", 
							goroutineID, j, tc.spec, len(result), len(tc.expected))
						continue
					}
					
					for k, port := range result {
						if port != tc.expected[k] {
							errors <- fmt.Errorf("goroutine %d iteration %d spec %s: port mismatch at index %d got %d expected %d", 
								goroutineID, j, tc.spec, k, port, tc.expected[k])
							break
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}

	// Verify all test specs are now cached
	cache := getPortRangeCache()
	for _, tc := range testSpecs {
		cached, found := cache.Get(tc.spec)
		must.True(t, found)
		must.Eq(t, tc.expected, cached)
	}
}

// TestPortRangeCache_SizeLimitsAndEviction tests cache size limits and LRU eviction
func TestPortRangeCache_SizeLimitsAndEviction(t *testing.T) {
	// Create a small cache for testing eviction
	cache, err := NewPortRangeCache(3)
	must.NoError(t, err)

	// Add entries up to the limit
	cache.Add("spec1", []uint64{80})
	cache.Add("spec2", []uint64{443})
	cache.Add("spec3", []uint64{8080})

	// Verify all entries are present
	result, found := cache.Get("spec1")
	must.True(t, found)
	must.Eq(t, []uint64{80}, result)

	result, found = cache.Get("spec2")
	must.True(t, found)
	must.Eq(t, []uint64{443}, result)

	result, found = cache.Get("spec3")
	must.True(t, found)
	must.Eq(t, []uint64{8080}, result)

	// Add one more entry, which should trigger eviction
	cache.Add("spec4", []uint64{9000})

	// spec4 should be present
	result, found = cache.Get("spec4")
	must.True(t, found)
	must.Eq(t, []uint64{9000}, result)

	// At least one of the original entries should have been evicted
	// Due to 2Q LRU behavior, we can't predict exactly which one
	foundCount := 0
	if _, found := cache.Get("spec1"); found {
		foundCount++
	}
	if _, found := cache.Get("spec2"); found {
		foundCount++
	}
	if _, found := cache.Get("spec3"); found {
		foundCount++
	}

	// We should have at most 2 of the original 3 entries (since cache size is 3 and we added spec4)
	must.True(t, foundCount <= 2)
}

// TestParsePortRanges_ErrorHandling tests that error cases preserve original error messages
func TestParsePortRanges_ErrorHandling(t *testing.T) {
	// Reset global cache for clean test
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	testCases := []struct {
		name        string
		spec        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty port",
			spec:        ",80",
			expectError: true,
			errorMsg:    "can't specify empty port",
		},
		{
			name:        "invalid port number",
			spec:        "abc",
			expectError: true,
			errorMsg:    "invalid syntax",
		},
		{
			name:        "port too large",
			spec:        "70000",
			expectError: true,
			errorMsg:    "port must be < 65536 but found 70000",
		},
		{
			name:        "zero port",
			spec:        "0",
			expectError: true,
			errorMsg:    "port must be > 0",
		},
		{
			name:        "invalid range",
			spec:        "100-50",
			expectError: true,
			errorMsg:    "invalid range: starting value (50) less than ending (100) value",
		},
		{
			name:        "range with invalid port",
			spec:        "80-abc",
			expectError: true,
			errorMsg:    "invalid syntax",
		},
		{
			name:        "too many hyphens",
			spec:        "80-90-100",
			expectError: true,
			errorMsg:    "can only parse single port numbers or port ranges",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that ParsePortRanges returns the expected error
			result, err := ParsePortRanges(tc.spec)
			if tc.expectError {
				must.Error(t, err)
				must.StrContains(t, err.Error(), tc.errorMsg)
				must.Nil(t, result)

				// Verify error cases are not cached
				cache := getPortRangeCache()
				cached, found := cache.Get(tc.spec)
				must.False(t, found)
				must.Nil(t, cached)

				// Test that calling again produces the same error
				result2, err2 := ParsePortRanges(tc.spec)
				must.Error(t, err2)
				must.StrContains(t, err2.Error(), tc.errorMsg)
				must.Nil(t, result2)
			} else {
				must.NoError(t, err)
				must.NotNil(t, result)
			}
		})
	}
}

// TestParsePortRanges_CacheConsistency tests that cached results are identical to uncached results
func TestParsePortRanges_CacheConsistency(t *testing.T) {
	testCases := []string{
		"80",
		"80,443",
		"8000-8002",
		"22,80,443,8080",
		"1000-1002,2000,3000-3001",
		"",
		"1,2,3,4,5,6,7,8,9,10",
	}

	for _, spec := range testCases {
		t.Run(fmt.Sprintf("spec_%s", spec), func(t *testing.T) {
			// Get uncached result
			uncachedResult, uncachedErr := parsePortRangesUncached(spec)

			// Reset cache to ensure clean state
			portRangeCache = nil
			portRangeCacheOnce = sync.Once{}

			// Get cached result (first call will cache it)
			cachedResult1, cachedErr1 := ParsePortRanges(spec)

			// Get cached result again (should come from cache)
			cachedResult2, cachedErr2 := ParsePortRanges(spec)

			// All results should be identical
			if uncachedErr != nil {
				must.Error(t, cachedErr1)
				must.Error(t, cachedErr2)
				must.Eq(t, uncachedErr.Error(), cachedErr1.Error())
				must.Eq(t, uncachedErr.Error(), cachedErr2.Error())
			} else {
				must.NoError(t, cachedErr1)
				must.NoError(t, cachedErr2)
				must.Eq(t, uncachedResult, cachedResult1)
				must.Eq(t, uncachedResult, cachedResult2)
				must.Eq(t, cachedResult1, cachedResult2)
			}
		})
	}
}

// TestPortRangeCache_NilCacheGracefulDegradation tests behavior when cache creation fails
func TestPortRangeCache_NilCacheGracefulDegradation(t *testing.T) {
	// Simulate cache creation failure by setting global cache to nil
	originalCache := portRangeCache
	originalOnce := portRangeCacheOnce
	defer func() {
		portRangeCache = originalCache
		portRangeCacheOnce = originalOnce
	}()

	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	// Force the once to be used with a nil cache
	portRangeCacheOnce.Do(func() {
		portRangeCache = nil // Simulate failure
	})

	// ParsePortRanges should still work without caching
	result, err := ParsePortRanges("80,443")
	must.NoError(t, err)
	must.Eq(t, []uint64{80, 443}, result)

	// Multiple calls should still work (though not cached)
	result2, err := ParsePortRanges("80,443")
	must.NoError(t, err)
	must.Eq(t, []uint64{80, 443}, result2)
	must.Eq(t, result, result2)
}

// TestPortRangeCache_MemoryBounds tests that cache respects memory bounds
func TestPortRangeCache_MemoryBounds(t *testing.T) {
	// Create cache with very small size to test bounds
	cache, err := NewPortRangeCache(2)
	must.NoError(t, err)

	// Add more entries than the cache can hold
	entries := []struct {
		key   string
		ports []uint64
	}{
		{"spec1", []uint64{80}},
		{"spec2", []uint64{443}},
		{"spec3", []uint64{8080}},
		{"spec4", []uint64{9000}},
		{"spec5", []uint64{9001}},
	}

	for _, entry := range entries {
		cache.Add(entry.key, entry.ports)
	}

	// Count how many entries are actually in the cache
	foundCount := 0
	for _, entry := range entries {
		if _, found := cache.Get(entry.key); found {
			foundCount++
		}
	}

	// Should not exceed cache size
	must.True(t, foundCount <= 2)
}

// TestPortRangeCache_LRUBehavior tests that LRU eviction works correctly
func TestPortRangeCache_LRUBehavior(t *testing.T) {
	cache, err := NewPortRangeCache(3)
	must.NoError(t, err)

	// Add initial entries
	cache.Add("old1", []uint64{80})
	cache.Add("old2", []uint64{443})
	cache.Add("old3", []uint64{8080})

	// Access old1 to make it recently used
	_, found := cache.Get("old1")
	must.True(t, found)

	// Add new entry, which should evict one of the less recently used entries
	cache.Add("new1", []uint64{9000})

	// old1 should still be present (was recently accessed)
	_, found = cache.Get("old1")
	must.True(t, found)

	// new1 should be present
	_, found = cache.Get("new1")
	must.True(t, found)

	// At least one of old2 or old3 should have been evicted
	old2Found := false
	old3Found := false
	if _, found := cache.Get("old2"); found {
		old2Found = true
	}
	if _, found := cache.Get("old3"); found {
		old3Found = true
	}

	// We can't guarantee which specific entry was evicted due to 2Q algorithm,
	// but we know that not all original entries can still be present
	totalFound := 2 // old1 and new1 are guaranteed to be present
	if old2Found {
		totalFound++
	}
	if old3Found {
		totalFound++
	}

	must.True(t, totalFound <= 3)
}

// BenchmarkParsePortRanges_Cached benchmarks cached ParsePortRanges calls
func BenchmarkParsePortRanges_Cached(b *testing.B) {
	// Reset global cache for clean benchmark
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	// Pre-populate cache with test data
	testSpecs := []string{
		"80",
		"80,443",
		"8000-8002",
		"22,80,443,8080",
		"1000-1002,2000,3000-3001",
		"9000-9010",
		"5000,5001,5002,5003,5004",
		"443,8080,9000-9002",
	}

	// Warm up the cache
	for _, spec := range testSpecs {
		_, _ = ParsePortRanges(spec)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Use modulo to cycle through test specs
		spec := testSpecs[i%len(testSpecs)]
		_, err := ParsePortRanges(spec)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkParsePortRanges_Uncached benchmarks uncached ParsePortRanges calls
func BenchmarkParsePortRanges_Uncached(b *testing.B) {
	testSpecs := []string{
		"80",
		"80,443",
		"8000-8002",
		"22,80,443,8080",
		"1000-1002,2000,3000-3001",
		"9000-9010",
		"5000,5001,5002,5003,5004",
		"443,8080,9000-9002",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Use modulo to cycle through test specs
		spec := testSpecs[i%len(testSpecs)]
		_, err := parsePortRangesUncached(spec)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkParsePortRanges_Mixed benchmarks mixed workload simulating real usage
func BenchmarkParsePortRanges_Mixed(b *testing.B) {
	// Reset global cache for clean benchmark
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	// Test data representing realistic port configurations
	// Some specs will be repeated (cache hits), others unique (cache misses)
	testSpecs := []string{
		// Common configurations (will be cached after first use)
		"22,80,443",           // Common web server ports
		"22,80,443",           // Repeated - cache hit
		"8080,8443",           // Common app server ports
		"8080,8443",           // Repeated - cache hit
		"3000-3010",           // Development port range
		"3000-3010",           // Repeated - cache hit
		"5432,5433",           // Database ports
		"5432,5433",           // Repeated - cache hit
		
		// Less common configurations (cache misses)
		"9000-9005,9010",      // Unique config 1
		"7000,7001,7002",      // Unique config 2
		"6379,6380",           // Unique config 3
		"4000-4002,4010",      // Unique config 4
		"1234,5678,9012",      // Unique config 5
		"8000-8003,8010-8012", // Unique config 6
		"2000,2001,2002,2003", // Unique config 7
		"10000-10010",         // Unique config 8
		
		// More cache hits
		"22,80,443",           // Cache hit
		"8080,8443",           // Cache hit
		"3000-3010",           // Cache hit
		"5432,5433",           // Cache hit
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Use modulo to cycle through test specs
		spec := testSpecs[i%len(testSpecs)]
		_, err := ParsePortRanges(spec)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkParsePortRanges_SinglePort benchmarks single port parsing
func BenchmarkParsePortRanges_SinglePort(b *testing.B) {
	// Reset global cache for clean benchmark
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	spec := "80"
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ParsePortRanges(spec)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkParsePortRanges_MultiplePortsCommaDelimited benchmarks multiple comma-delimited ports
func BenchmarkParsePortRanges_MultiplePortsCommaDelimited(b *testing.B) {
	// Reset global cache for clean benchmark
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	spec := "22,80,443,8080,9000"
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ParsePortRanges(spec)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkParsePortRanges_PortRange benchmarks port range parsing
func BenchmarkParsePortRanges_PortRange(b *testing.B) {
	// Reset global cache for clean benchmark
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	spec := "8000-8010"
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ParsePortRanges(spec)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkParsePortRanges_ComplexMixed benchmarks complex mixed port specifications
func BenchmarkParsePortRanges_ComplexMixed(b *testing.B) {
	// Reset global cache for clean benchmark
	portRangeCache = nil
	portRangeCacheOnce = sync.Once{}

	spec := "22,80,443,8000-8010,9000,9001,9002,10000-10005"
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ParsePortRanges(spec)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkParsePortRanges_CacheVsUncached compares cached vs uncached performance
func BenchmarkParsePortRanges_CacheVsUncached(b *testing.B) {
	spec := "22,80,443,8000-8010,9000"

	b.Run("Cached", func(b *testing.B) {
		// Reset global cache for clean benchmark
		portRangeCache = nil
		portRangeCacheOnce = sync.Once{}

		// Warm up cache
		_, _ = ParsePortRanges(spec)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := ParsePortRanges(spec)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})

	b.Run("Uncached", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := parsePortRangesUncached(spec)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})
}