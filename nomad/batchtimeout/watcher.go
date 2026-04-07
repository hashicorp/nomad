// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package batchtimeout

import (
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	defaultScanInterval = 5 * time.Second
)

type RaftApplier interface {
	ApplyPlanResults(req *structs.ApplyPlanResultsRequest) (uint64, error)
}

type Watcher struct {
	logger       log.Logger
	raft         RaftApplier
	state        *state.StateStore
	scanInterval time.Duration

	mu      sync.Mutex
	enabled bool
	stopCh  chan struct{}
	running bool
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
	w.mu.Lock()
	defer w.mu.Unlock()

	if st != nil {
		w.state = st
	}

	if enabled {
		w.enabled = true
		if w.running || w.state == nil {
			return
		}

		w.stopCh = make(chan struct{})
		w.running = true
		go w.watch(w.stopCh)
		return
	}

	w.enabled = false
	if !w.running {
		return
	}

	close(w.stopCh)
	w.stopCh = nil
	w.running = false
}

func (w *Watcher) watch(stopCh <-chan struct{}) {
	ticker := time.NewTicker(w.scanInterval)
	defer ticker.Stop()
	defer w.markStopped(stopCh)

	for {
		w.scan(time.Now().UTC())

		select {
		case <-ticker.C:
		case <-stopCh:
			return
		}
	}
}

func (w *Watcher) markStopped(stopCh <-chan struct{}) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopCh == stopCh {
		w.stopCh = nil
	}
	w.running = false
}

func (w *Watcher) scan(now time.Time) {
	w.mu.Lock()
	st := w.state
	enabled := w.enabled
	w.mu.Unlock()

	if !enabled || st == nil {
		return
	}

	iter, err := st.Allocs(nil, state.SortDefault)
	if err != nil {
		w.logger.Warn("failed to iterate allocations", "error", err)
		return
	}

	var stopped []*structs.AllocationDiff

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)
		if !shouldStopAlloc(now, alloc) {
			continue
		}

		stopped = append(stopped, &structs.AllocationDiff{
			ID:                 alloc.ID,
			DesiredDescription: structs.AllocTimeoutReasonMaxRunDuration,
			ClientStatus:       structs.AllocClientStatusFailed,
		})
	}

	if len(stopped) == 0 {
		return
	}

	req := &structs.ApplyPlanResultsRequest{
		AllocsStopped: stopped,
		UpdatedAt:     now.UnixNano(),
	}

	if _, err := w.raft.ApplyPlanResults(req); err != nil {
		w.logger.Warn("failed to stop timed out allocations", "error", err, "count", len(stopped))
		return
	}

	w.logger.Debug("stopped timed out allocations", "count", len(stopped))
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
