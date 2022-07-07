package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestBadNodeTracker(t *testing.T) {
	ci.Parallel(t)

	config := DefaultCachedBadNodeTrackerConfig()
	config.CacheSize = 3

	tracker, err := NewCachedBadNodeTracker(hclog.NewNullLogger(), config)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		tracker.Add(fmt.Sprintf("node-%d", i+1))
	}

	require.Equal(t, config.CacheSize, tracker.cache.Len())

	// Only track the most recent values.
	expected := []string{"node-8", "node-9", "node-10"}
	require.ElementsMatch(t, expected, tracker.cache.Keys())
}

func TestBadNodeTracker_IsBad(t *testing.T) {
	ci.Parallel(t)

	config := DefaultCachedBadNodeTrackerConfig()
	config.CacheSize = 3
	config.Window = time.Duration(testutil.TestMultiplier()) * time.Second
	config.Threshold = 4

	tracker, err := NewCachedBadNodeTracker(hclog.NewNullLogger(), config)
	require.NoError(t, err)

	// Populate cache.
	tracker.Add("node-1")

	tracker.Add("node-2")
	tracker.Add("node-2")

	tracker.Add("node-3")
	tracker.Add("node-3")
	tracker.Add("node-3")
	tracker.Add("node-3")
	tracker.Add("node-3")
	tracker.Add("node-3")

	testCases := []struct {
		name   string
		nodeID string
		bad    bool
	}{
		{
			name:   "node-1 is not bad",
			nodeID: "node-1",
			bad:    false,
		},
		{
			name:   "node-3 is bad",
			nodeID: "node-3",
			bad:    true,
		},
		{
			name:   "node not tracked is not bad",
			nodeID: "node-1000",
			bad:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tracker.IsBad(tc.nodeID)
			require.Equal(t, tc.bad, got)
		})
	}

	t.Run("cache expires", func(t *testing.T) {
		time.Sleep(config.Window)
		require.False(t, tracker.IsBad("node-1"))
		require.False(t, tracker.IsBad("node-2"))
		require.False(t, tracker.IsBad("node-3"))
	})

	t.Run("IsBad updates cache", func(t *testing.T) {
		// Don't access node-3 so it should be evicted when a new value is
		// added and the tracker size overflows.
		tracker.IsBad("node-1")
		tracker.IsBad("node-2")
		tracker.Add("node-4")

		expected := []string{"node-1", "node-2", "node-4"}
		require.ElementsMatch(t, expected, tracker.cache.Keys())
	})
}

func TestBadNodeTracker_RateLimit(t *testing.T) {
	config := DefaultCachedBadNodeTrackerConfig()
	config.Threshold = 3
	config.RateLimit = float64(testutil.TestMultiplier())
	config.BurstSize = 3

	tracker, err := NewCachedBadNodeTracker(hclog.NewNullLogger(), config)
	require.NoError(t, err)

	tracker.Add("node-1")
	tracker.Add("node-1")
	tracker.Add("node-1")
	tracker.Add("node-1")
	tracker.Add("node-1")

	// Burst allows for max 3 operations.
	require.True(t, tracker.IsBad("node-1"))
	require.True(t, tracker.IsBad("node-1"))
	require.True(t, tracker.IsBad("node-1"))
	require.False(t, tracker.IsBad("node-1"))

	// Wait for a new token.
	time.Sleep(time.Duration(testutil.TestMultiplier()) * time.Second)
	require.True(t, tracker.IsBad("node-1"))
	require.False(t, tracker.IsBad("node-1"))
}

func TestBadNodeStats_score(t *testing.T) {
	ci.Parallel(t)

	window := time.Duration(testutil.TestMultiplier()) * time.Second
	stats := newBadNodeStats(window)

	require.Equal(t, 0, stats.score())

	stats.record()
	stats.record()
	stats.record()
	require.Equal(t, 3, stats.score())

	time.Sleep(window / 2)
	stats.record()
	require.Equal(t, 4, stats.score())

	time.Sleep(window / 2)
	require.Equal(t, 1, stats.score())

	time.Sleep(window / 2)
	require.Equal(t, 0, stats.score())
}
