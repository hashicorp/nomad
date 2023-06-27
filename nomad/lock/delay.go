// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lock

import (
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/helper"
)

// DelayTimer is used to mark certain locks as unacquirable. When a locks TTL
// expires, it is subject to the LockDelay configured within the variable. This
// prevents another caller from acquiring the lock for some period of time as a
// protection against split-brains. This is inspired by the lock-delay in
// Chubby.
type DelayTimer struct {

	// delayTimers has the set of active delay expiration times, organized by
	// ID, which the caller dictates when adding entries. The lock should be
	// used for all access.
	delayTimers map[string]time.Time
	lock        sync.RWMutex
}

// NewDelayTimer returns a new delay timer manager.
func NewDelayTimer() *DelayTimer {
	return &DelayTimer{
		delayTimers: make(map[string]time.Time),
	}
}

// Get returns the expiration time of a key lock delay. This must be checked on
// the leader server only due to the variability of clocks.
func (d *DelayTimer) Get(id string) time.Time {
	d.lock.RLock()
	expires := d.delayTimers[id]
	d.lock.RUnlock()
	return expires
}

// Set sets the expiration time for the lock delay to the given delay from the
// given now time.
func (d *DelayTimer) Set(id string, now time.Time, delay time.Duration) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.delayTimers[id] = now.Add(delay)

	// Set up the after func, but ignore the returned timer as we do not need
	// this for cancellation.
	_ = time.AfterFunc(delay, func() {
		d.lock.Lock()
		delete(d.delayTimers, id)
		d.lock.Unlock()
	})
}

// EmitMetrics is a long-running routine used to emit periodic metrics about
// the Delay.
func (d *DelayTimer) EmitMetrics(period time.Duration, shutdownCh chan struct{}) {
	timer, stop := helper.NewSafeTimer(period)
	defer stop()

	for {
		timer.Reset(period)
		select {
		case <-timer.C:
			metrics.SetGauge([]string{"variables", "locks", "delay_timer", "num"}, float32(d.timerNum()))
		case <-shutdownCh:
			return
		}
	}
}

// len returns the number of registered delay timers.
func (d *DelayTimer) timerNum() int {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return len(d.delayTimers)
}
