// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

type maxRunDurationSetter interface {
	EnforceMaxRunDurationTimeout(time.Time)
}

type maxRunDurationHook struct {
	logger log.Logger
	setter maxRunDurationSetter

	hookLock sync.Mutex

	alloc        *structs.Allocation
	originalJob  *structs.Job
	originalTG   string
	originalWant string
	timer        *time.Timer
}

func newMaxRunDurationHook(
	logger log.Logger,
	alloc *structs.Allocation,
	setter maxRunDurationSetter,
) *maxRunDurationHook {
	h := &maxRunDurationHook{
		alloc:  alloc,
		setter: setter,
	}
	if alloc != nil {
		h.originalTG = alloc.TaskGroup
		h.originalWant = alloc.DesiredStatus
		if alloc.Job != nil {
			h.originalJob = alloc.Job.Copy()
		}
	}
	h.logger = logger.Named(h.Name())
	return h
}

var (
	_ interfaces.RunnerPrerunHook    = (*maxRunDurationHook)(nil)
	_ interfaces.RunnerUpdateHook    = (*maxRunDurationHook)(nil)
	_ interfaces.RunnerTaskStateHook = (*maxRunDurationHook)(nil)
	_ interfaces.RunnerPostrunHook   = (*maxRunDurationHook)(nil)
	_ interfaces.ShutdownHook        = (*maxRunDurationHook)(nil)
)

func (h *maxRunDurationHook) Name() string {
	return "max_run_duration"
}

func (h *maxRunDurationHook) Prerun(_ *taskenv.TaskEnv) error {
	h.hookLock.Lock()
	defer h.hookLock.Unlock()

	h.resetTimerLocked(nil)
	return nil
}

func (h *maxRunDurationHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.hookLock.Lock()
	defer h.hookLock.Unlock()

	h.alloc = h.mergeAllocLocked(req.Alloc)
	h.resetTimerLocked(req.TaskStates)
	return nil
}

func (h *maxRunDurationHook) TaskStateUpdated(req *interfaces.RunnerUpdateRequest) error {
	h.hookLock.Lock()
	defer h.hookLock.Unlock()

	if req.Alloc != nil {
		h.alloc = h.mergeAllocLocked(req.Alloc)
	}

	h.resetTimerLocked(req.TaskStates)
	return nil
}

func (h *maxRunDurationHook) Postrun() error {
	h.hookLock.Lock()
	defer h.hookLock.Unlock()

	h.stopTimerLocked()
	return nil
}

func (h *maxRunDurationHook) Shutdown() {
	_ = h.Postrun()
}

func (h *maxRunDurationHook) allocWithTaskStatesLocked(taskStates map[string]*structs.TaskState) *structs.Allocation {
	if h.alloc == nil {
		return nil
	}

	alloc := h.alloc.Copy()
	if h.originalJob != nil {
		alloc.Job = h.originalJob.Copy()
	}
	if alloc.TaskGroup == "" {
		alloc.TaskGroup = h.originalTG
	}
	if alloc.DesiredStatus == "" {
		alloc.DesiredStatus = h.originalWant
	}

	alloc.TaskStates = make(map[string]*structs.TaskState, len(taskStates))
	for name, ts := range taskStates {
		if ts == nil {
			alloc.TaskStates[name] = nil
			continue
		}
		alloc.TaskStates[name] = ts.Copy()
	}

	return alloc
}

func (h *maxRunDurationHook) mergeAllocLocked(update *structs.Allocation) *structs.Allocation {
	if update == nil {
		return h.alloc
	}

	merged := update.Copy()

	if merged.TaskGroup != "" {
		h.originalTG = merged.TaskGroup
	}
	if merged.DesiredStatus != "" {
		h.originalWant = merged.DesiredStatus
	}

	if h.originalJob != nil {
		merged.Job = h.originalJob.Copy()
	}
	if merged.TaskGroup == "" {
		merged.TaskGroup = h.originalTG
	}
	if merged.DesiredStatus == "" {
		merged.DesiredStatus = h.originalWant
	}

	return merged
}

func (h *maxRunDurationHook) resetTimerLocked(taskStates map[string]*structs.TaskState) {
	h.stopTimerLocked()

	alloc := h.allocWithTaskStatesLocked(taskStates)
	if alloc == nil {
		return
	}

	if alloc.TerminalStatus() {
		return
	}

	if alloc.DesiredStatus != "" && alloc.DesiredStatus != structs.AllocDesiredStatusRun {
		return
	}

	maxRunDuration, ok := alloc.MaxRunDuration()
	if !ok {
		return
	}

	startedAt, ok := fullyRunningSince(taskStates)
	if !ok {
		return
	}

	deadline := startedAt.Add(maxRunDuration)
	remaining := time.Until(deadline)
	if remaining <= 0 {
		go h.setter.EnforceMaxRunDurationTimeout(deadline)
		return
	}

	timer := time.NewTimer(remaining)
	h.timer = timer

	go func(t *time.Timer, deadline time.Time) {
		<-t.C

		h.hookLock.Lock()
		if h.timer != t {
			h.hookLock.Unlock()
			return
		}
		h.timer = nil
		h.hookLock.Unlock()

		h.setter.EnforceMaxRunDurationTimeout(deadline)
	}(timer, deadline)
}

func (h *maxRunDurationHook) stopTimerLocked() {
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

func fullyRunningSince(taskStates map[string]*structs.TaskState) (time.Time, bool) {
	if len(taskStates) == 0 {
		return time.Time{}, false
	}

	var latest time.Time
	for _, ts := range taskStates {
		if ts == nil || ts.State != structs.TaskStateRunning || ts.StartedAt.IsZero() {
			return time.Time{}, false
		}
		if ts.StartedAt.After(latest) {
			latest = ts.StartedAt
		}
	}

	if latest.IsZero() {
		return time.Time{}, false
	}

	return latest, true
}
