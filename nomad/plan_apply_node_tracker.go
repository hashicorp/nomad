package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/helper"
	"golang.org/x/time/rate"
)

type BadNodeTracker interface {
	IsBad(string) bool
	Add(string)
	EmitStats(time.Duration, <-chan struct{})
}

// NoopBadNodeTracker is a no-op implementation of bad node tracker that is
// used when tracking is disabled.
type NoopBadNodeTracker struct{}

func (n *NoopBadNodeTracker) Add(string)                               {}
func (n *NoopBadNodeTracker) EmitStats(time.Duration, <-chan struct{}) {}
func (n *NoopBadNodeTracker) IsBad(string) bool {
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
	cache     *lru.TwoQueueCache
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
		RateLimit: 5 / (30 * 60),
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
		With("threshold", config.Threshold).
		With("window", config.Window)

	cache, err := lru.New2Q(config.CacheSize)
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

// IsBad returns true if the node has more rejections than the threshold within
// the time window.
func (c *CachedBadNodeTracker) IsBad(nodeID string) bool {
	// Limit the number of nodes we report as bad to avoid mass assigning nodes
	// as ineligible, but still call Get to keep the cache entry fresh.
	value, ok := c.cache.Get(nodeID)
	if !ok || !c.limiter.Allow() {
		return false
	}

	stats := value.(*badNodeStats)
	score := stats.score()

	c.logger.Debug("checking if node is bad", "node_id", nodeID, "score", score)
	return score > c.threshold
}

// Add records a new rejection for node. If it's the first time a node is added
// it will be included in the internal cache. If the cache is full the least
// recently updated or accessed node is evicted.
func (c *CachedBadNodeTracker) Add(nodeID string) {
	value, ok := c.cache.Get(nodeID)
	if !ok {
		value = newBadNodeStats(c.window)
		c.cache.Add(nodeID, value)
	}

	stats := value.(*badNodeStats)
	score := stats.record()
	c.logger.Debug("adding node plan rejection", "node_id", nodeID, "score", score)
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

func (c *CachedBadNodeTracker) emitStats() {
	for _, k := range c.cache.Keys() {
		value, _ := c.cache.Get(k)
		stats := value.(*badNodeStats)
		score := stats.score()

		labels := []metrics.Label{
			{Name: "node_id", Value: k.(string)},
		}
		metrics.SetGaugeWithLabels([]string{"nomad", "plan", "rejection_tracker", "node_score"}, float32(score), labels)
	}
}

// badNodeStats represents a node being tracked by a BadNodeTracker.
type badNodeStats struct {
	history []time.Time
	window  time.Duration
}

// newBadNodeStats returns an empty badNodeStats.
func newBadNodeStats(window time.Duration) *badNodeStats {
	return &badNodeStats{
		window: window,
	}
}

// score returns the number of rejections within the past time window.
func (s *badNodeStats) score() int {
	count := 0
	windowStart := time.Now().Add(-s.window)

	for i := len(s.history) - 1; i >= 0; i-- {
		ts := s.history[i]
		if ts.Before(windowStart) {
			// Since we start from the end of the history list, anything past
			// this point will have happened before the time window.
			break
		}
		count += 1
	}
	return count
}

// record adds a new entry to the stats history and returns the new score.
func (s *badNodeStats) record() int {
	now := time.Now()
	s.history = append(s.history, now)
	return s.score()
}
