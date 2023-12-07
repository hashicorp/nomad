// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/nomad/helper"
	"golang.org/x/time/rate"
)

type BadNodeTracker interface {
	Add(string) bool
	EmitStats(time.Duration, <-chan struct{})
}

// NoopBadNodeTracker is a no-op implementation of bad node tracker that is
// used when tracking is disabled.
type NoopBadNodeTracker struct{}

func (n *NoopBadNodeTracker) EmitStats(time.Duration, <-chan struct{}) {}
func (n *NoopBadNodeTracker) Add(string) bool {
	return false
}

// CachedBadNodeTracker keeps a record of nodes marked as bad by the plan
// applier in a LRU cache.
//
// It takes a time window and a threshold value. Plan rejections for a node
// will be registered with its timestamp. If the number of rejections within
// the time window is greater than the threshold the node is reported as bad.
//
// The tracker uses a fixed size cache that evicts old entries based on access
// frequency and recency.
type CachedBadNodeTracker struct {
	logger    hclog.Logger
	cache     *lru.TwoQueueCache[string, *badNodeStats]
	limiter   *rate.Limiter
	window    time.Duration
	threshold int
}

type CachedBadNodeTrackerConfig struct {
	CacheSize int
	RateLimit float64
	BurstSize int
	Window    time.Duration
	Threshold int
}

func DefaultCachedBadNodeTrackerConfig() CachedBadNodeTrackerConfig {
	return CachedBadNodeTrackerConfig{
		CacheSize: 50,

		// Limit marking 5 nodes per 30min as ineligible with an initial
		// burst of 10 nodes.
		RateLimit: 5.0 / (30 * 60),
		BurstSize: 10,

		// Consider a node as bad if it is added more than 100 times in a 5min
		// window period.
		Window:    5 * time.Minute,
		Threshold: 100,
	}
}

// NewCachedBadNodeTracker returns a new CachedBadNodeTracker.
func NewCachedBadNodeTracker(logger hclog.Logger, config CachedBadNodeTrackerConfig) (*CachedBadNodeTracker, error) {
	log := logger.Named("bad_node_tracker").
		With("window", config.Window).
		With("threshold", config.Threshold)

	cache, err := lru.New2Q[string, *badNodeStats](config.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create new bad node tracker: %v", err)
	}

	return &CachedBadNodeTracker{
		logger:    log,
		cache:     cache,
		limiter:   rate.NewLimiter(rate.Limit(config.RateLimit), config.BurstSize),
		window:    config.Window,
		threshold: config.Threshold,
	}, nil
}

// Add records a new rejection for a node and returns true if the number of
// rejections reaches the threshold.
//
// If it's the first time the node is added it will be included in the internal
// cache. If the cache is full the least recently updated or accessed node is
// evicted.
func (c *CachedBadNodeTracker) Add(nodeID string) bool {
	stats, ok := c.cache.Get(nodeID)
	if !ok {
		stats = newBadNodeStats(nodeID, c.window)
		c.cache.Add(nodeID, stats)
	}

	now := time.Now()
	stats.record(now)

	return c.isBad(now, stats)
}

// EmitStats generates metrics for the bad nodes being currently tracked. Must
// be called in a goroutine.
func (c *CachedBadNodeTracker) EmitStats(period time.Duration, stopCh <-chan struct{}) {
	timer, stop := helper.NewSafeTimer(period)
	defer stop()

	for {
		timer.Reset(period)

		select {
		case <-timer.C:
			c.emitStats()
		case <-stopCh:
			return
		}
	}
}

// isBad returns true if the node has more rejections than the threshold within
// the time window.
func (c *CachedBadNodeTracker) isBad(t time.Time, stats *badNodeStats) bool {
	score := stats.score(t)
	logger := c.logger.With("node_id", stats.id, "score", score)

	logger.Trace("checking if node is bad")
	if score >= c.threshold {
		// Limit the number of nodes we report as bad to avoid mass assigning
		// nodes as ineligible, but do it after Get to keep the cache entry
		// fresh.
		if !c.limiter.Allow() {
			logger.Trace("node is bad, but returning false due to rate limiting")
			return false
		}
		return true
	}

	return false
}

func (c *CachedBadNodeTracker) emitStats() {
	now := time.Now()
	for _, nodeID := range c.cache.Keys() {
		stats, _ := c.cache.Get(nodeID)
		score := stats.score(now)

		labels := []metrics.Label{
			{Name: "node_id", Value: nodeID},
		}
		metrics.SetGaugeWithLabels([]string{"nomad", "plan", "rejection_tracker", "node_score"}, float32(score), labels)
	}
}

// badNodeStats represents a node being tracked by a BadNodeTracker.
type badNodeStats struct {
	id      string
	history []time.Time
	window  time.Duration
}

// newBadNodeStats returns an empty badNodeStats.
func newBadNodeStats(id string, window time.Duration) *badNodeStats {
	return &badNodeStats{
		id:     id,
		window: window,
	}
}

// score returns the number of rejections within the past time window.
func (s *badNodeStats) score(t time.Time) int {
	active, expired := s.countActive(t)

	// Remove expired records.
	if expired > 0 {
		s.history = s.history[expired:]
	}

	return active
}

// record adds a new entry to the stats history and returns the new score.
func (s *badNodeStats) record(t time.Time) {
	s.history = append(s.history, t)
}

// countActive returns the number of records that happened after the time
// window started (active) and before (expired).
func (s *badNodeStats) countActive(t time.Time) (int, int) {
	windowStart := t.Add(-s.window)

	// Assume all values are expired and move back from history until we find
	// a record that actually happened before the window started.
	expired := len(s.history)
	for ; expired > 0; expired-- {
		i := expired - 1
		ts := s.history[i]
		if ts.Before(windowStart) {
			break
		}
	}

	active := len(s.history) - expired
	return active, expired
}
