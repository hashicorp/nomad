// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestCachedtBadNodeTracker(t *testing.T) {
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

func TestCachedBadNodeTracker_isBad(t *testing.T) {
	ci.Parallel(t)

	config := DefaultCachedBadNodeTrackerConfig()
	config.CacheSize = 3
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
	}

	now := time.Now()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read value from cached.
			stats, ok := tracker.cache.Get(tc.nodeID)
			require.True(t, ok)

			// Check if it's bad.
			got := tracker.isBad(now, stats)
			require.Equal(t, tc.bad, got)
		})
	}

	future := time.Now().Add(2 * config.Window)
	nodes := []string{"node-1", "node-2", "node-3"}
	for _, n := range nodes {
		t.Run(fmt.Sprintf("%s cache expires", n), func(t *testing.T) {
			stats, ok := tracker.cache.Get(n)
			require.True(t, ok)

			bad := tracker.isBad(future, stats)
			require.False(t, bad)
		})
	}
}

func TesCachedtBadNodeTracker_rateLimit(t *testing.T) {
	ci.Parallel(t)

	config := DefaultCachedBadNodeTrackerConfig()
	config.Threshold = 3
	config.RateLimit = float64(1) // Get a new token every second.
	config.BurstSize = 3

	tracker, err := NewCachedBadNodeTracker(hclog.NewNullLogger(), config)
	require.NoError(t, err)

	tracker.Add("node-1")
	tracker.Add("node-1")
	tracker.Add("node-1")
	tracker.Add("node-1")
	tracker.Add("node-1")

	stats, ok := tracker.cache.Get("node-1")
	require.True(t, ok)

	// Burst allows for max 3 operations.
	now := time.Now()
	require.True(t, tracker.isBad(now, stats))
	require.True(t, tracker.isBad(now, stats))
	require.True(t, tracker.isBad(now, stats))
	require.False(t, tracker.isBad(now, stats))

	// Wait for a new token.
	time.Sleep(time.Second)
	now = time.Now()
	require.True(t, tracker.isBad(now, stats))
	require.False(t, tracker.isBad(now, stats))
}

func TestBadNodeStats_score(t *testing.T) {
	ci.Parallel(t)

	window := time.Minute
	stats := newBadNodeStats("node-1", window)

	now := time.Now()
	require.Equal(t, 0, stats.score(now))

	stats.record(now)
	stats.record(now)
	stats.record(now)
	require.Equal(t, 3, stats.score(now))
	require.Len(t, stats.history, 3)

	halfWindow := now.Add(window / 2)
	stats.record(halfWindow)
	require.Equal(t, 4, stats.score(halfWindow))
	require.Len(t, stats.history, 4)

	fullWindow := now.Add(window).Add(time.Second)
	require.Equal(t, 1, stats.score(fullWindow))
	require.Len(t, stats.history, 1)

	afterWindow := now.Add(2 * window)
	require.Equal(t, 0, stats.score(afterWindow))
	require.Len(t, stats.history, 0)
}
