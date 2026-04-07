// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package batchtimeout

import (
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	defaultScanInterval = 5 * time.Second

	timeoutDescription = "allocation exceeded max_run_duration"
)

type RaftApplier interface {
	UpdateAllocDesiredTransition(req *structs.AllocUpdateDesiredTransitionRequest) (uint64, error)
}

type Watcher struct {
	logger       log.Logger
	raft         RaftApplier
	state        *state.StateStore
	scanInterval time.Duration
	enabled      bool
}

func NewWatcher(logger log.Logger, raft RaftApplier, scanInterval time.Duration) *Watcher {
	if scanInterval <= 0 {
		scanInterval = defaultScanInterval
	}

	return &Watcher{
		logger:       logger.Named("batch_timeout_watcher"),
		raft:         raft,
		scanInterval: scanInterval,
	}
}

func (w *Watcher) SetEnabled(enabled bool, st *state.StateStore) {
	w.enabled = enabled
	if st != nil {
		w.state = st
	}

	if enabled && w.state != nil {
		go w.watch()
	}
}

func (w *Watcher) watch() {
	ticker := time.NewTicker(w.scanInterval)
	defer ticker.Stop()

	for {
		if !w.enabled {
			return
		}

		w.scan(time.Now().UTC())

		<-ticker.C
	}
}

func (w *Watcher) scan(now time.Time) {
	if w.state == nil {
		return
	}

	iter, err := w.state.Allocs(nil, state.SortDefault)
	if err != nil {
		w.logger.Warn("failed to iterate allocations", "error", err)
		return
	}

	transitions := make(map[string]*structs.DesiredTransition)

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)
		if !shouldStopAlloc(now, alloc) {
			continue
		}

		transitions[alloc.ID] = &structs.DesiredTransition{
			NoShutdownDelay: pointer.Of(false),
		}
	}

	if len(transitions) == 0 {
		return
	}

	req := &structs.AllocUpdateDesiredTransitionRequest{
		Allocs: transitions,
	}

	if _, err := w.raft.UpdateAllocDesiredTransition(req); err != nil {
		w.logger.Warn("failed to stop timed out allocations", "error", err, "count", len(transitions))
		return
	}

	w.logger.Debug("stopped timed out allocations", "count", len(transitions))
}

func shouldStopAlloc(now time.Time, alloc *structs.Allocation) bool {
	if alloc == nil || alloc.Job == nil {
		return false
	}

	if alloc.DesiredStatus != structs.AllocDesiredStatusRun {
		return false
	}

	if alloc.ClientStatus != structs.AllocClientStatusRunning {
		return false
	}

	if alloc.ClientTerminalStatus() || alloc.ServerTerminalStatus() {
		return false
	}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil || tg.MaxRunDuration == nil || *tg.MaxRunDuration <= 0 {
		return false
	}

	switch alloc.Job.Type {
	case structs.JobTypeBatch, structs.JobTypeSysBatch:
	default:
		return false
	}

	startedAt, ok := allocRunningSince(alloc)
	if !ok {
		return false
	}

	return !startedAt.Add(*tg.MaxRunDuration).After(now)
}

func allocRunningSince(alloc *structs.Allocation) (time.Time, bool) {
	var latest time.Time

	for _, ts := range alloc.TaskStates {
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
