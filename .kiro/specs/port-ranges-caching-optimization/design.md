# Design Document

## Overview

This design implements a caching layer for the `ParsePortRanges` function to eliminate redundant parsing operations during scheduler evaluations. The solution leverages Nomad's existing caching patterns, specifically the `ACLCache` implementation, to provide thread-safe, memory-bounded caching of parsed port ranges.

The core insight is that port range strings (like "22,80,443" or "1000-2000") are typically static configuration values that rarely change but are parsed repeatedly during every node evaluation in the scheduler. By caching the parsed results, we can convert expensive string parsing operations into fast hash table lookups.

## Architecture

### Cache Implementation

The design introduces a `PortRangeCache` that follows the same pattern as the existing `ACLCache`:

```go
type PortRangeCache struct {
    *lru.TwoQueueCache[string, PortRangeCacheEntry]
    clock libtime.Clock
}

type PortRangeCacheEntry struct {
    Ports []uint64
    Timestamp time.Time
}
```

The cache uses a 2Q LRU eviction policy (same as ACL cache) which provides better hit rates than simple LRU for workloads with both temporal and spatial locality.

### Integration Points

The caching will be integrated at two key locations:

1. **Global Cache Instance**: A package-level cache instance in `nomad/structs/funcs.go` that can be shared across all parsing operations
2. **ParsePortRanges Function**: Modified to check cache first, then fall back to parsing if cache miss occurs

### Cache Key Strategy

The cache key will be the raw port range string itself. This provides:
- Simple, deterministic key generation
- Direct mapping from input to cached result
- No additional hashing overhead for short strings

### Memory Management

- **Cache Size**: 256 entries (smaller than ACL cache since port ranges are less diverse)
- **Entry Size**: Approximately 50-100 bytes per entry (string key + slice of uint64)
- **Total Memory**: ~25KB maximum memory footprint
- **Eviction**: 2Q LRU automatically handles memory bounds

## Components and Interfaces

### PortRangeCache

```go
type PortRangeCache struct {
    *lru.TwoQueueCache[string, PortRangeCacheEntry]
    clock libtime.Clock
}

func NewPortRangeCache(size int) *PortRangeCache
func (c *PortRangeCache) Get(key string) ([]uint64, bool)
func (c *PortRangeCache) Add(key string, ports []uint64)
```

### Modified ParsePortRanges Function

```go
func ParsePortRanges(spec string) ([]uint64, error) {
    // Check cache first
    if cached, found := portRangeCache.Get(spec); found {
        return cached, nil
    }
    
    // Parse if not cached
    result, err := parsePortRangesUncached(spec)
    if err != nil {
        return nil, err
    }
    
    // Cache successful results
    portRangeCache.Add(spec, result)
    return result, nil
}
```

### Cache Entry Structure

```go
type PortRangeCacheEntry struct {
    Ports     []uint64
    Timestamp time.Time
}

func (e PortRangeCacheEntry) Age() time.Duration
func (e PortRangeCacheEntry) Get() []uint64
```

## Data Models

### Cache Storage

- **Key**: Raw port range string (e.g., "22,80,443", "1000-2000")
- **Value**: `PortRangeCacheEntry` containing parsed ports and timestamp
- **Capacity**: 256 entries maximum
- **Eviction**: 2Q LRU policy

### Thread Safety

The cache uses the thread-safe `lru.TwoQueueCache` from `github.com/hashicorp/golang-lru/v2`, ensuring safe concurrent access without additional synchronization.

## Error Handling

### Cache Miss Behavior

When a cache miss occurs, the system falls back to the original parsing logic. This ensures:
- No change in error behavior
- Identical error messages for invalid inputs
- Graceful degradation if cache is unavailable

### Invalid Input Handling

Invalid port range strings are not cached, ensuring that:
- Error conditions don't consume cache space
- Repeated invalid inputs still produce appropriate errors
- Cache only stores valid, successful parse results

### Cache Failure Handling

If the cache itself fails (memory allocation, etc.), the system falls back to direct parsing without caching, maintaining system availability.

## Testing Strategy

### Unit Tests

1. **Cache Hit/Miss Scenarios**
   - Verify cache stores and retrieves parsed results correctly
   - Test cache miss triggers original parsing logic
   - Validate cache hit returns identical results to parsing

2. **Concurrency Tests**
   - Multiple goroutines accessing cache simultaneously
   - Race condition detection using `go test -race`
   - Stress testing with high concurrent load

3. **Memory Bounds Tests**
   - Verify cache respects size limits
   - Test LRU eviction behavior
   - Memory usage validation under sustained load

4. **Error Handling Tests**
   - Invalid port ranges produce same errors as original
   - Cache failures don't break parsing functionality
   - Edge cases (empty strings, malformed ranges)

### Integration Tests

1. **Scheduler Performance Tests**
   - Benchmark scheduler operations with and without caching
   - Measure cache hit rates in realistic scenarios
   - Validate no functional regressions in scheduling

2. **Memory Usage Tests**
   - Long-running tests to verify no memory leaks
   - Cache behavior under memory pressure
   - Integration with existing memory management

### Benchmarks

```go
func BenchmarkParsePortRanges_Cached(b *testing.B)
func BenchmarkParsePortRanges_Uncached(b *testing.B)
func BenchmarkParsePortRanges_Mixed(b *testing.B)
```

Expected performance improvements:
- 90%+ reduction in CPU time for cached entries
- 50-80% overall improvement in scheduler node evaluation performance
- Minimal memory overhead (< 25KB)