// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lock

import (
	"sync"
	"time"

	"github.com/hashicorp/go-metrics"
	"github.com/hashicorp/nomad/helper"
)

// TTLTimer provides a map of named timers which is safe for concurrent use.
// Each timer is created using time.AfterFunc which will be triggered once the
// timer fires.
type TTLTimer struct {

	// timers is a mapping of timers which represent when a lock TTL will
	// expire. The lock should be used for all access.
	ttlTimers map[string]*time.Timer
	lock      sync.RWMutex
}

// NewTTLTimer initializes a new TTLTimer.
func NewTTLTimer() *TTLTimer {
	return &TTLTimer{
		ttlTimers: make(map[string]*time.Timer),
	}
}

// Get returns the timer with the given ID. If the timer is not found, nil is
// returned, so callers should be expected to handle this case.
func (t *TTLTimer) Get(id string) *time.Timer {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.ttlTimers[id]
}

// Delete removes the timer with the given ID from the tracking. If the timer
// is not found, the call is noop.
func (t *TTLTimer) Delete(id string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.ttlTimers, id)
}

// Create sets the TTL of the timer with the given ID or creates a new
// one if it does not exist.
func (t *TTLTimer) Create(id string, ttl time.Duration, afterFn func()) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if tm := t.ttlTimers[id]; tm != nil {
		tm.Reset(ttl)
		return
	}
	t.ttlTimers[id] = time.AfterFunc(ttl, afterFn)
}

// StopAndRemove stops the timer with the given ID and removes it from
// tracking.
func (t *TTLTimer) StopAndRemove(id string) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if tm := t.ttlTimers[id]; tm != nil {
		tm.Stop()
		delete(t.ttlTimers, id)
	}
}

// StopAndRemoveAll stops and removes all registered timers.
func (t *TTLTimer) StopAndRemoveAll() {
	t.lock.Lock()
	defer t.lock.Unlock()

	for _, tm := range t.ttlTimers {
		tm.Stop()
	}
	t.ttlTimers = make(map[string]*time.Timer)
}

// EmitMetrics is a long-running routine used to emit periodic metrics about
// the Timer.
func (t *TTLTimer) EmitMetrics(period time.Duration, shutdownCh chan struct{}) {
	timer, stop := helper.NewSafeTimer(period)
	defer stop()

	for {
		timer.Reset(period)
		select {
		case <-timer.C:
			metrics.SetGauge([]string{"variables", "locks", "ttl_timer", "num"}, float32(t.TimerNum()))
		case <-shutdownCh:
			return
		}
	}
}

// timerNum returns the number of registered timers.
func (t *TTLTimer) TimerNum() int {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return len(t.ttlTimers)
}
