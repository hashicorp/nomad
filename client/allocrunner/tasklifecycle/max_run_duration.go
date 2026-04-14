// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package tasklifecycle

import (
	"sync"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

type MaxRunDurationTimeoutFn func(time.Time)

// MaxRunDuration coordinates local enforcement of task group
// `max_run_duration`.
//
// This component is alloc-scoped, not task-scoped. It observes the current
// authoritative allocation plus task state snapshots and arms a timer once the
// allocation is fully running. The timer is canceled and recomputed whenever
// alloc or task state changes are observed.
type MaxRunDuration struct {
	mu sync.Mutex

	alloc     *structs.Allocation
	deadline  time.Time
	timer     *time.Timer
	onTimeout MaxRunDurationTimeoutFn
}

func NewMaxRunDuration(onTimeout MaxRunDurationTimeoutFn) *MaxRunDuration {
	return &MaxRunDuration{onTimeout: onTimeout}
}

// SetAlloc updates the authoritative allocation used for
// `max_run_duration` evaluation.
func (m *MaxRunDuration) SetAlloc(alloc *structs.Allocation) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alloc = alloc
}

// TaskStateUpdated recomputes the `max_run_duration` timer from the latest
// alloc and task state snapshot.
func (m *MaxRunDuration) TaskStateUpdated(taskStates map[string]*structs.TaskState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.setDeadline(taskStates)
	m.resetTimerLocked()
}

// Stop cancels any active timer.
func (m *MaxRunDuration) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopTimerLocked()
}

func (m *MaxRunDuration) setDeadline(taskStates map[string]*structs.TaskState) {
	if m.alloc == nil {
		m.deadline = time.Time{}
		return
	}

	// Only running allocations with a valid max_run_duration and a fully
	// running task set should establish a deadline.
	if m.alloc.TerminalStatus() {
		m.deadline = time.Time{}
		return
	}

	// DesiredStatus defaults to the zero value for normal running allocations.
	// Treat an empty status the same as "run" and only clear the deadline when
	// the alloc is explicitly marked with a non-run desired status.
	if m.alloc.DesiredStatus != "" && m.alloc.DesiredStatus != structs.AllocDesiredStatusRun {
		m.deadline = time.Time{}
		return
	}

	maxRunDuration, ok := m.alloc.MaxRunDuration()
	if !ok {
		m.deadline = time.Time{}
		return
	}

	if !m.deadline.IsZero() {
		return
	}

	startedAt, ok := structs.FullyRunningSince(taskStates)
	if !ok {
		return
	}

	m.deadline = startedAt.Add(maxRunDuration)
}

func (m *MaxRunDuration) resetTimerLocked() {
	m.stopTimerLocked()

	if m.deadline.IsZero() {
		return
	}

	deadline := m.deadline
	remaining := time.Until(deadline)
	if remaining <= 0 {
		go m.onTimeout(deadline)
		return
	}

	timer := time.NewTimer(remaining)
	m.timer = timer

	go func(t *time.Timer, deadline time.Time) {
		<-t.C

		m.mu.Lock()
		if m.timer != t {
			m.mu.Unlock()
			return
		}
		m.timer = nil
		m.mu.Unlock()

		m.onTimeout(deadline)
	}(timer, deadline)
}

func (m *MaxRunDuration) stopTimerLocked() {
	if m.timer == nil {
		return
	}

	if !m.timer.Stop() {
		select {
		case <-m.timer.C:
		default:
		}
	}
	m.timer = nil
}
