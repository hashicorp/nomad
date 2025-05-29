// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

type heartbeatStop struct {
	lastOk       time.Time
	startupGrace time.Time
	allocHookCh  chan *structs.Allocation
	heartbeatCh  chan struct{}
	getRunner    func(string) (interfaces.AllocRunner, error)
	logger       hclog.InterceptLogger
	shutdownCh   chan struct{}
	lock         *sync.RWMutex
}

func newHeartbeatStop(
	getRunner func(string) (interfaces.AllocRunner, error),
	timeout time.Duration,
	logger hclog.InterceptLogger,
	shutdownCh chan struct{}) *heartbeatStop {

	h := &heartbeatStop{
		startupGrace: time.Now().Add(timeout),
		allocHookCh:  make(chan *structs.Allocation, 10),
		heartbeatCh:  make(chan struct{}, 1),
		getRunner:    getRunner,
		logger:       logger,
		shutdownCh:   shutdownCh,
		lock:         &sync.RWMutex{},
	}

	return h
}

// allocHook is called after (re)storing a new AllocRunner in the client. It registers the
// allocation to be stopped if the taskgroup is configured appropriately
func (h *heartbeatStop) allocHook(alloc *structs.Allocation) {
	if _, ok := getDisconnectStopTimeout(alloc); ok {
		h.allocHookCh <- alloc
	}
}

// shouldStop is called on a restored alloc to determine if lastOk is sufficiently in the
// past that it should be prevented from restarting
func (h *heartbeatStop) shouldStop(alloc *structs.Allocation) bool {
	if timeout, ok := getDisconnectStopTimeout(alloc); ok {
		return h.shouldStopAfter(time.Now(), timeout)
	}
	return false
}

func (h *heartbeatStop) shouldStopAfter(now time.Time, interval time.Duration) bool {
	lastOk := h.getLastOk()
	if lastOk.IsZero() {
		return now.After(h.startupGrace)
	}
	return now.After(lastOk.Add(interval))
}

// watch is a loop that checks for allocations that should be stopped. It also manages the
// registration of allocs to be stopped in a single thread.
func (h *heartbeatStop) watch() {
	// If we never manage to successfully contact the server, we want to stop our allocs
	// after duration + start time
	h.setLastOk(time.Now())
	allocIntervals := map[string]time.Duration{}

	timer, stopTimer := helper.NewStoppedTimer()
	defer stopTimer()

	for {
		// we want to fire the ticker only once the shortest
		// stop_on_client_after interval has expired. we'll reset the ticker on
		// every heartbeat and every time a new alloc appears
		var interval time.Duration
		for _, t := range allocIntervals {
			if t < interval || interval == 0 {
				interval = t
			}
		}
		if interval != 0 {
			timer.Reset(interval)
		} else {
			timer.Stop()
		}

		select {
		case <-h.heartbeatCh:
			continue

		case <-h.shutdownCh:
			return

		case alloc := <-h.allocHookCh:
			// receiving a new alloc implies we're still connected, so we'll go
			// back to the top to reset the interval
			if timeout, ok := getDisconnectStopTimeout(alloc); ok {
				allocIntervals[alloc.ID] = timeout
			}

		case now := <-timer.C:
			for allocID, d := range allocIntervals {
				if h.shouldStopAfter(now, d) {
					if err := h.stopAlloc(allocID); err != nil {
						h.logger.Warn("error stopping on heartbeat timeout",
							"alloc", allocID, "error", err)
						continue
					}
					delete(allocIntervals, allocID)
				}
			}

		}
	}
}

// setLastOk sets the last known good heartbeat time to the current time
func (h *heartbeatStop) setLastOk(t time.Time) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.lastOk = t
	select {
	case h.heartbeatCh <- struct{}{}:
	default:
		// if the channel is full then the watch loop has a heartbeat it needs
		// to dequeue to reset its timer anyways, so just drop this one
	}
}

func (h *heartbeatStop) getLastOk() time.Time {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.lastOk
}

// stopAlloc actually stops the allocation
func (h *heartbeatStop) stopAlloc(allocID string) error {
	runner, err := h.getRunner(allocID)
	if err != nil {
		return err
	}

	h.logger.Debug("stopping alloc for stop_after_client_disconnect", "alloc", allocID)

	runner.Destroy()
	return nil
}

// getDisconnectStopTimeout is a helper that gets the alloc's StopOnClientAfter
// timeout and handles the possible nil pointers safely
func getDisconnectStopTimeout(alloc *structs.Allocation) (time.Duration, bool) {
	for _, tg := range alloc.Job.TaskGroups {
		if tg.Name == alloc.TaskGroup {
			timeout := tg.GetDisconnectStopTimeout()
			if timeout != nil {
				return *timeout, true
			}
			break
		}
	}
	return 0, false
}
