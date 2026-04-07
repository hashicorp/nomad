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

// Watcher scans running batch and sysbatch allocations for max_run_duration
// violations and stops expired allocations via plan results.
//
// The watcher is leader-managed. The enabled flag tracks desired state, while
// running indicates whether the background watch loop is currently active.
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

// SetEnabled updates the desired watcher state and starts or stops the
// background loop as needed.
func (w *Watcher) SetEnabled(enabled bool, st *state.StateStore) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if st != nil {
		w.state = st
	}

	if enabled {
		w.enableLocked()
		return
	}

	w.disableLocked()
}

func (w *Watcher) enableLocked() {
	w.enabled = true
	if w.running || w.state == nil {
		return
	}

	w.stopCh = make(chan struct{})
	w.running = true
	go w.watch(w.stopCh)
}

func (w *Watcher) disableLocked() {
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
		// Scan immediately so leadership changes do not wait a full interval
		// before enforcing timeouts.
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
		w.running = false
	}
}

// scan evaluates all allocations in state and stops any that have exceeded
// max_run_duration.
func (w *Watcher) scan(now time.Time) {
	st, enabled := w.snapshot()
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

func (w *Watcher) snapshot() (*state.StateStore, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.state, w.enabled
}

func shouldStopAlloc(now time.Time, alloc *structs.Allocation) bool {
	// Allocation must exist and still be actively running.
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

	// Job and task group must opt into max_run_duration and support timeout
	// enforcement.
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil || tg.MaxRunDuration == nil || *tg.MaxRunDuration <= 0 {
		return false
	}
	switch alloc.Job.Type {
	case structs.JobTypeBatch, structs.JobTypeSysBatch:
	default:
		return false
	}

	// Allocation must be fully running and past its deadline.
	startedAt, ok := allocFullyRunningSince(alloc)
	if !ok {
		return false
	}

	return !startedAt.Add(*tg.MaxRunDuration).After(now)
}

// allocFullyRunningSince returns the latest StartedAt timestamp across all task
// states, but only when every known task state is running with a non-zero start
// time.
func allocFullyRunningSince(alloc *structs.Allocation) (time.Time, bool) {
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
