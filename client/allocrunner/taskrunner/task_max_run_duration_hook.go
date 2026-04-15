// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	_ interfaces.TaskPrestartHook = (*taskMaxRunDurationHook)(nil)
	_ interfaces.TaskUpdateHook   = (*taskMaxRunDurationHook)(nil)
	_ interfaces.TaskExitedHook   = (*taskMaxRunDurationHook)(nil)
)

type taskMaxRunDurationHook struct {
	tr *TaskRunner

	mu sync.Mutex

	timer             *time.Timer
	deadline          time.Time
	maxRunDuration    time.Duration
	hasMaxRunDuration bool

	logger log.Logger
}

func newTaskMaxRunDurationHook(tr *TaskRunner, logger log.Logger) interfaces.TaskHook {
	return &taskMaxRunDurationHook{
		tr:     tr,
		logger: logger.Named("task_max_run_duration"),
	}
}

func (h *taskMaxRunDurationHook) Name() string {
	return "task_max_run_duration"
}

func (h *taskMaxRunDurationHook) Prestart(context.Context, *interfaces.TaskPrestartRequest, *interfaces.TaskPrestartResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.resetTimer()
	return nil
}

func (h *taskMaxRunDurationHook) Update(_ context.Context, req *interfaces.TaskUpdateRequest, _ *interfaces.TaskUpdateResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if req != nil && req.Alloc != nil {
		task := req.Alloc.LookupTask(h.tr.taskName)
		if task != nil {
			h.tr.setAlloc(req.Alloc, task)
		}
	}

	h.resetTimer()
	return nil
}

func (h *taskMaxRunDurationHook) Exited(context.Context, *interfaces.TaskExitedRequest, *interfaces.TaskExitedResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.stopTimer()
	return nil
}

func (h *taskMaxRunDurationHook) resetTimer() {
	deadline, maxRunDuration, ok := h.currentDeadline()
	if !ok {
		h.stopTimer()
		h.deadline = time.Time{}
		h.maxRunDuration = 0
		h.hasMaxRunDuration = false
		return
	}

	if h.hasMaxRunDuration && h.maxRunDuration == maxRunDuration && h.deadline.Equal(deadline) {
		return
	}

	h.stopTimer()

	h.deadline = deadline
	h.maxRunDuration = maxRunDuration
	h.hasMaxRunDuration = true

	remaining := time.Until(deadline)
	if remaining <= 0 {
		h.logger.Debug("task exceeded max_run_duration, enforcing immediately", "task", h.tr.taskName, "deadline", deadline)
		go h.enforce(deadline)
		return
	}

	timer := time.NewTimer(remaining)
	h.timer = timer

	h.logger.Trace("armed task max_run_duration timer", "task", h.tr.taskName, "deadline", deadline, "remaining", remaining)

	go func(t *time.Timer, deadline time.Time) {
		<-t.C

		h.mu.Lock()
		if h.timer != t {
			h.mu.Unlock()
			return
		}
		h.timer = nil
		h.mu.Unlock()

		h.enforce(deadline)
	}(timer, deadline)
}

func (h *taskMaxRunDurationHook) stopTimer() {
	if h.timer != nil {
		if !h.timer.Stop() {
			select {
			case <-h.timer.C:
			default:
			}
		}

		h.timer = nil
	}

	h.deadline = time.Time{}
	h.maxRunDuration = 0
	h.hasMaxRunDuration = false
}

func (h *taskMaxRunDurationHook) currentDeadline() (time.Time, time.Duration, bool) {
	task := h.tr.Task()
	if task == nil || task.MaxRunDuration == nil || *task.MaxRunDuration <= 0 {
		return time.Time{}, 0, false
	}

	alloc := h.tr.Alloc()
	if alloc == nil || alloc.TerminalStatus() {
		return time.Time{}, 0, false
	}

	if alloc.DesiredStatus != "" && alloc.DesiredStatus != structs.AllocDesiredStatusRun {
		return time.Time{}, 0, false
	}

	state := h.tr.TaskState()
	if state == nil || state.State != structs.TaskStateRunning || state.StartedAt.IsZero() {
		return time.Time{}, 0, false
	}

	return state.StartedAt.Add(*task.MaxRunDuration), *task.MaxRunDuration, true
}

func (h *taskMaxRunDurationHook) enforce(deadline time.Time) {
	now := time.Now()
	if now.Before(deadline) {
		return
	}

	task := h.tr.Task()
	if task == nil || task.MaxRunDuration == nil || *task.MaxRunDuration <= 0 {
		return
	}

	state := h.tr.TaskState()
	if state == nil || state.State != structs.TaskStateRunning || state.StartedAt.IsZero() {
		return
	}

	if state.StartedAt.Add(*task.MaxRunDuration).After(now) {
		return
	}

	h.logger.Debug("task exceeded max_run_duration, killing task", "task", h.tr.taskName, "deadline", deadline)

	event := structs.NewTaskEvent(structs.TaskKilling).
		SetKillTimeout(task.KillTimeout, h.tr.clientConfig.MaxKillTimeout).
		SetDisplayMessage(structs.AllocTimeoutReasonMaxRunDuration)

	_ = h.tr.Kill(context.Background(), event)
}
