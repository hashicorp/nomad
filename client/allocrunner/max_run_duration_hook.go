// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	_ interfaces.RunnerPrerunHook  = (*maxRunDurationHook)(nil)
	_ interfaces.RunnerPostrunHook = (*maxRunDurationHook)(nil)
	_ interfaces.RunnerUpdateHook  = (*maxRunDurationHook)(nil)
	_ interfaces.ShutdownHook      = (*maxRunDurationHook)(nil)
)

type maxRunDurationHook struct {
	mu sync.Mutex

	alloc *structs.Allocation

	timer             *time.Timer
	deadline          time.Time
	maxRunDuration    time.Duration
	hasMaxRunDuration bool

	onTimeout func(time.Time)
	logger    hclog.Logger

	baseLabels []metrics.Label

	// taskStatesFn returns task states directly from the task runners.
	// It is used to reconstruct the countdown deadline after a client restart,
	// when the server-side allocation's TaskStates may be absent or stale but
	// each task runner's persisted state still holds the original StartedAt.
	taskStatesFn func() map[string]*structs.TaskState
}

func newMaxRunDurationHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	baseLabels []metrics.Label,
	onTimeout func(time.Time),
	taskStatesFn func() map[string]*structs.TaskState,
) interfaces.RunnerHook {
	return &maxRunDurationHook{
		alloc:        alloc,
		onTimeout:    onTimeout,
		logger:       logger.Named("max_run_duration"),
		baseLabels:   baseLabels,
		taskStatesFn: taskStatesFn,
	}
}

func (h *maxRunDurationHook) Name() string {
	return "max_run_duration"
}

func (h *maxRunDurationHook) Prerun(*taskenv.TaskEnv) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.resetTimer()
	return nil
}

func (h *maxRunDurationHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.alloc = req.Alloc
	h.resetTimer()
	return nil
}

func (h *maxRunDurationHook) Postrun() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.stopTimer()
	return nil
}

func (h *maxRunDurationHook) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.stopTimer()
}

func (h *maxRunDurationHook) resetTimer() {
	deadline, maxRunDuration, ok := h.currentDeadline()
	if !ok {
		h.stopTimer()
		h.deadline = time.Time{}
		h.maxRunDuration = 0
		h.hasMaxRunDuration = false
		return
	}

	// if the duration hasn't changed the timer is already correctly armed —
	// skip it. The deadline can never move earlier with the same duration
	// (StartedAt only advances, and time.Now() only advances).
	if h.hasMaxRunDuration && h.maxRunDuration == maxRunDuration {
		return
	}

	prevMaxRunDuration := h.maxRunDuration
	prevDeadline := h.deadline
	hadMaxRunDuration := h.hasMaxRunDuration

	h.stopTimer()

	h.maxRunDuration = maxRunDuration
	h.hasMaxRunDuration = true
	h.deadline = deadline
	h.emitMetrics(maxRunDuration, deadline)

	remaining := time.Until(deadline)

	if hadMaxRunDuration {
		h.logger.Debug("updated max_run_duration",
			"task_group", h.alloc.TaskGroup,
			"old_configured", prevMaxRunDuration,
			"new_configured", maxRunDuration,
			"old_deadline", prevDeadline,
			"new_deadline", deadline,
			"remaining", remaining,
		)
	}

	if remaining <= 0 {
		h.logger.Debug("allocation exceeded max_run_duration, enforcing immediately",
			"task_group", h.alloc.TaskGroup,
			"configured", maxRunDuration,
			"remaining", remaining,
			"deadline", deadline,
		)
		go h.onTimeout(deadline)
		return
	}

	timer := time.NewTimer(remaining)
	h.timer = timer

	h.logger.Trace("armed max_run_duration timer",
		"task_group", h.alloc.TaskGroup,
		"configured", maxRunDuration,
		"remaining", remaining,
		"deadline", deadline,
	)

	go func(t *time.Timer, deadline time.Time) {
		<-t.C

		h.mu.Lock()
		if h.timer != t {
			h.mu.Unlock()
			return
		}
		h.timer = nil
		h.mu.Unlock()

		h.onTimeout(deadline)
	}(timer, deadline)
}

func (h *maxRunDurationHook) stopTimer() {
	if h.timer == nil {
		return
	}

	if !h.timer.Stop() {
		select {
		case <-h.timer.C:
		default:
		}
	}

	h.timer = nil
}

func (h *maxRunDurationHook) emitMetrics(maxRunDuration time.Duration, deadline time.Time) {
	labels := h.baseLabels
	labels = append(labels, metrics.Label{Name: "task_group", Value: h.alloc.TaskGroup})

	metrics.SetGaugeWithLabels(
		[]string{"client", "allocs", "max_run_duration", "configured_seconds"},
		float32(maxRunDuration.Seconds()),
		labels,
	)
	metrics.SetGaugeWithLabels(
		[]string{"client", "allocs", "max_run_duration", "remaining_seconds"},
		float32(time.Until(deadline).Seconds()),
		labels,
	)
}

func (h *maxRunDurationHook) currentDeadline() (time.Time, time.Duration, bool) {
	if h.alloc.TerminalStatus() {
		return time.Time{}, 0, false
	}

	if h.alloc.DesiredStatus != "" && h.alloc.DesiredStatus != structs.AllocDesiredStatusRun {
		return time.Time{}, 0, false
	}

	maxRunDuration, ok := h.alloc.MaxRunDuration()
	if !ok {
		return time.Time{}, 0, false
	}

	// if task runners report that every task has started at least once, use
	// their StartedAt timestamps to reconstruct the original deadline.
	//
	// Note: the resetTimer guard above ensures this can never *extend* a
	// deadline that was already established, so a slow-starting task cannot
	// gain extra time by eventually setting StartedAt.
	if h.taskStatesFn != nil {
		if startedAt, ok := structs.FullyStartedSince(h.taskStatesFn()); ok {
			return startedAt.Add(maxRunDuration), maxRunDuration, true
		}
	}

	// No task has started yet (fresh alloc). Anchor the deadline to now so
	// that pre-start time counts against the budget.
	return time.Now().Add(maxRunDuration), maxRunDuration, true
}
