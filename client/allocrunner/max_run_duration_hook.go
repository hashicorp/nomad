// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
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
}

func newMaxRunDurationHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	onTimeout func(time.Time),
) interfaces.RunnerHook {
	return &maxRunDurationHook{
		alloc:     alloc,
		onTimeout: onTimeout,
		logger:    logger.Named("max_run_duration"),
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
	maxRunDuration, ok := h.currentMaxRunDuration()
	if !ok {
		h.stopTimer()
		h.deadline = time.Time{}
		h.maxRunDuration = 0
		h.hasMaxRunDuration = false
		return
	}

	if h.hasMaxRunDuration && h.maxRunDuration == maxRunDuration && !h.deadline.IsZero() {
		return
	}

	h.stopTimer()

	h.maxRunDuration = maxRunDuration
	h.hasMaxRunDuration = true
	h.deadline = time.Now().Add(maxRunDuration)

	deadline := h.deadline
	remaining := time.Until(deadline)

	if remaining <= 0 {
		h.logger.Debug("allocation exceeded max_run_duration, enforcing immediately", "deadline", deadline)
		go h.onTimeout(deadline)
		return
	}

	timer := time.NewTimer(remaining)
	h.timer = timer

	h.logger.Trace("armed max_run_duration timer", "deadline", deadline, "remaining", remaining)

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

func (h *maxRunDurationHook) currentMaxRunDuration() (time.Duration, bool) {
	if h.alloc.TerminalStatus() {
		return 0, false
	}

	if h.alloc.DesiredStatus != "" && h.alloc.DesiredStatus != structs.AllocDesiredStatusRun {
		return 0, false
	}

	return h.alloc.MaxRunDuration()
}
