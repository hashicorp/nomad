package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	lru "github.com/hashicorp/golang-lru"
	"github.com/hashicorp/nomad/helper"
)

// BadNodeTracker keeps a record of nodes marked as bad by the plan applier.
//
// It takes a time window and a threshold value. Plan rejections for a node
// will be registered with its timestamp. If the number of rejections within
// the time window is greater than the threshold the node is reported as bad.
//
// The tracker uses a fixed size cache that evicts old entries based on access
// frequency and recency.
type BadNodeTracker struct {
	logger    hclog.Logger
	cache     *lru.TwoQueueCache
	window    time.Duration
	threshold int
}

// NewBadNodeTracker returns a new BadNodeTracker.
func NewBadNodeTracker(logger hclog.Logger, size int, window time.Duration, threshold int) (*BadNodeTracker, error) {
	cache, err := lru.New2Q(size)
	if err != nil {
		return nil, fmt.Errorf("failed to create new bad node tracker: %v", err)
	}

	return &BadNodeTracker{
		logger: logger.Named("bad_node_tracker").
			With("threshold", threshold).
			With("window", window),
		cache:     cache,
		window:    window,
		threshold: threshold,
	}, nil
}

// IsBad returns true if the node has more rejections than the threshold within
// the time window.
func (t *BadNodeTracker) IsBad(nodeID string) bool {
	value, ok := t.cache.Get(nodeID)
	if !ok {
		return false
	}

	stats := value.(*badNodeStats)
	score := stats.score()

	t.logger.Debug("checking if node is bad", "node_id", nodeID, "score", score)
	return score > t.threshold
}

// Add records a new rejection for node. If it's the first time a node is added
// it will be included in the internal cache. If the cache is full the least
// recently updated or accessed node is evicted.
func (t *BadNodeTracker) Add(nodeID string) {
	value, ok := t.cache.Get(nodeID)
	if !ok {
		value = newBadNodeStats(t.window)
		t.cache.Add(nodeID, value)
	}

	stats := value.(*badNodeStats)
	score := stats.record()
	t.logger.Debug("adding node plan rejection", "node_id", nodeID, "score", score)
}

// EmitStats generates metrics for the bad nodes being currently tracked. Must
// be called in a goroutine.
func (t *BadNodeTracker) EmitStats(period time.Duration, stopCh <-chan struct{}) {
	timer, stop := helper.NewSafeTimer(period)
	defer stop()

	for {
		timer.Reset(period)

		select {
		case <-timer.C:
			t.emitStats()
		case <-stopCh:
			return
		}
	}
}

func (t *BadNodeTracker) emitStats() {
	for _, k := range t.cache.Keys() {
		value, _ := t.cache.Get(k)
		stats := value.(*badNodeStats)
		score := stats.score()

		labels := []metrics.Label{
			{Name: "node_id", Value: k.(string)},
		}
		metrics.SetGaugeWithLabels([]string{"nomad", "plan", "rejection_tracker", "node_score"}, float32(score), labels)
	}
}

// badNodeStats represents a node being tracked by BadNodeTracker.
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
