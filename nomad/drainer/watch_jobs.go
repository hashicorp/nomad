// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drainer

import (
	"context"
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/time/rate"
)

type DrainRequest struct {
	Allocs []*structs.Allocation
	Resp   *structs.BatchFuture
}

func NewDrainRequest(allocs []*structs.Allocation) *DrainRequest {
	return &DrainRequest{
		Allocs: allocs,
		Resp:   structs.NewBatchFuture(),
	}
}

// DrainingJobWatcher is the interface for watching a job drain
type DrainingJobWatcher interface {
	// RegisterJob is used to start watching a draining job
	RegisterJobs(jobs []structs.NamespacedID)

	// Drain is used to emit allocations that should be drained.
	Drain() <-chan *DrainRequest

	// Migrated is allocations for draining jobs that have transitioned to
	// stop. There is no guarantee that duplicates won't be published.
	Migrated() <-chan []*structs.Allocation
}

// drainingJobWatcher is used to watch draining jobs and emit events when
// draining allocations have replacements
type drainingJobWatcher struct {
	ctx    context.Context
	logger log.Logger

	// state is the state that is watched for state changes.
	state *state.StateStore

	// limiter is used to limit the rate of blocking queries
	limiter *rate.Limiter

	// jobs is the set of tracked jobs.
	jobs map[structs.NamespacedID]struct{}

	// queryCtx is used to cancel a blocking query.
	queryCtx    context.Context
	queryCancel context.CancelFunc

	// drainCh and migratedCh are used to emit allocations
	drainCh    chan *DrainRequest
	migratedCh chan []*structs.Allocation

	l sync.RWMutex
}

// NewDrainingJobWatcher returns a new job watcher. The caller is expected to
// cancel the context to clean up the drainer.
func NewDrainingJobWatcher(ctx context.Context, limiter *rate.Limiter, state *state.StateStore, logger log.Logger) *drainingJobWatcher {

	// Create a context that can cancel the blocking query so that when a new
	// job gets registered it is handled.
	queryCtx, queryCancel := context.WithCancel(ctx)

	w := &drainingJobWatcher{
		ctx:         ctx,
		queryCtx:    queryCtx,
		queryCancel: queryCancel,
		limiter:     limiter,
		logger:      logger.Named("job_watcher"),
		state:       state,
		jobs:        make(map[structs.NamespacedID]struct{}, 64),
		drainCh:     make(chan *DrainRequest),
		migratedCh:  make(chan []*structs.Allocation),
	}

	go w.watch()
	return w
}

// RegisterJob marks the given job as draining and adds it to being watched.
func (w *drainingJobWatcher) RegisterJobs(jobs []structs.NamespacedID) {
	w.l.Lock()
	defer w.l.Unlock()

	updated := false
	for _, jns := range jobs {
		if _, ok := w.jobs[jns]; ok {
			continue
		}

		// Add the job and cancel the context
		w.logger.Trace("registering job", "job", jns)
		w.jobs[jns] = struct{}{}
		updated = true
	}

	if updated {
		w.queryCancel()

		// Create a new query context
		w.queryCtx, w.queryCancel = context.WithCancel(w.ctx)
	}
}

// Drain returns the channel that emits allocations to drain.
func (w *drainingJobWatcher) Drain() <-chan *DrainRequest {
	return w.drainCh
}

// Migrated returns the channel that emits allocations for draining jobs that
// have been migrated.
func (w *drainingJobWatcher) Migrated() <-chan []*structs.Allocation {
	return w.migratedCh
}

// deregisterJob removes the job from being watched.
func (w *drainingJobWatcher) deregisterJob(jobID, namespace string) {
	w.l.Lock()
	defer w.l.Unlock()
	jns := structs.NamespacedID{
		ID:        jobID,
		Namespace: namespace,
	}
	delete(w.jobs, jns)
	w.logger.Trace("deregistering job", "job", jns)
}

// watch is the long lived watching routine that detects job drain changes.
func (w *drainingJobWatcher) watch() {
	timer, stop := helper.NewSafeTimer(stateReadErrorDelay)
	defer stop()

	waitIndex := uint64(1)

	for {
		timer.Reset(stateReadErrorDelay)

		w.logger.Trace("getting job allocs at index", "index", waitIndex)
		jobAllocs, index, err := w.getJobAllocs(w.getQueryCtx(), waitIndex)

		if err != nil {
			if err == context.Canceled {
				// Determine if it is a cancel or a shutdown
				select {
				case <-w.ctx.Done():
					return
				default:
					// The query context was cancelled;
					// reset index so we don't miss past
					// updates to newly registered jobs
					waitIndex = 1
					continue
				}
			}

			w.logger.Error("error watching job allocs updates at index", "index", waitIndex, "error", err)
			select {
			case <-w.ctx.Done():
				w.logger.Trace("shutting down")
				return
			case <-timer.C:
				continue
			}
		}
		w.logger.Trace("retrieved allocs for draining jobs", "num_allocs", len(jobAllocs), "index", index)

		lastHandled := waitIndex
		waitIndex = index

		// Snapshot the state store
		snap, err := w.state.Snapshot()
		if err != nil {
			w.logger.Warn("failed to snapshot statestore", "error", err)
			continue
		}

		currentJobs := w.drainingJobs()
		var allDrain, allMigrated []*structs.Allocation
		for jns, allocs := range jobAllocs {
			// Check if the job is still registered
			if _, ok := currentJobs[jns]; !ok {
				w.logger.Trace("skipping job as it is no longer registered for draining", "job", jns)
				continue
			}

			w.logger.Trace("handling job", "job", jns)

			// Lookup the job
			job, err := snap.JobByID(nil, jns.Namespace, jns.ID)
			if err != nil {
				w.logger.Warn("failed to lookup job", "job", jns, "error", err)
				continue
			}

			// Ignore purged jobs
			if job == nil {
				w.logger.Trace("ignoring garbage collected job", "job", jns)
				w.deregisterJob(jns.ID, jns.Namespace)
				continue
			}

			// Ignore any system jobs
			if job.Type == structs.JobTypeSystem {
				w.deregisterJob(job.ID, job.Namespace)
				continue
			}

			result, err := handleJob(snap, job, allocs, lastHandled)
			if err != nil {
				w.logger.Error("handling drain for job failed", "job", jns, "error", err)
				continue
			}

			w.logger.Trace("received result for job", "job", jns, "result", result)

			allDrain = append(allDrain, result.drain...)
			allMigrated = append(allMigrated, result.migrated...)

			// Stop tracking this job
			if result.done {
				w.deregisterJob(job.ID, job.Namespace)
			}
		}

		if len(allDrain) != 0 {
			// Create the request
			req := NewDrainRequest(allDrain)
			w.logger.Trace("sending drain request for allocs", "num_allocs", len(allDrain))

			select {
			case w.drainCh <- req:
			case <-w.ctx.Done():
				w.logger.Trace("shutting down")
				return
			}

			// Wait for the request to be committed
			select {
			case <-req.Resp.WaitCh():
			case <-w.ctx.Done():
				w.logger.Trace("shutting down")
				return
			}

			// See if it successfully committed
			if err := req.Resp.Error(); err != nil {
				w.logger.Error("failed to transition allocations", "error", err)
			}

			// Wait until the new index
			if index := req.Resp.Index(); index > waitIndex {
				waitIndex = index
			}
		}

		if len(allMigrated) != 0 {
			w.logger.Trace("sending migrated for allocs", "num_allocs", len(allMigrated))
			select {
			case w.migratedCh <- allMigrated:
			case <-w.ctx.Done():
				w.logger.Trace("shutting down")
				return
			}
		}
	}
}

// jobResult is the set of actions to take for a draining job given its current
// state.
type jobResult struct {
	// drain is the set of allocations to emit for draining.
	drain []*structs.Allocation

	// migrated is the set of allocations to emit as migrated
	migrated []*structs.Allocation

	// done marks whether the job has been fully drained.
	done bool
}

// newJobResult returns a jobResult with done=true. It is the responsibility of
// callers to set done=false when a remaining drainable alloc is found.
func newJobResult() *jobResult {
	return &jobResult{
		done: true,
	}
}

func (r *jobResult) String() string {
	return fmt.Sprintf("Drain %d ; Migrate %d ; Done %v", len(r.drain), len(r.migrated), r.done)
}

// handleJob takes the state of a draining job and returns the desired actions.
func handleJob(snap *state.StateSnapshot, job *structs.Job, allocs []*structs.Allocation, lastHandledIndex uint64) (*jobResult, error) {
	r := newJobResult()
	batch := job.Type == structs.JobTypeBatch
	taskGroups := make(map[string]*structs.TaskGroup, len(job.TaskGroups))
	for _, tg := range job.TaskGroups {
		// Only capture the groups that have a migrate strategy or we are just
		// watching batch
		if tg.Migrate != nil || batch {
			taskGroups[tg.Name] = tg
		}
	}

	// Sort the allocations by TG
	tgAllocs := make(map[string][]*structs.Allocation, len(taskGroups))
	for _, alloc := range allocs {
		if _, ok := taskGroups[alloc.TaskGroup]; !ok {
			continue
		}

		tgAllocs[alloc.TaskGroup] = append(tgAllocs[alloc.TaskGroup], alloc)
	}

	for name, tg := range taskGroups {
		allocs := tgAllocs[name]
		if err := handleTaskGroup(snap, batch, tg, allocs, lastHandledIndex, r); err != nil {
			return nil, fmt.Errorf("drain for task group %q failed: %v", name, err)
		}
	}

	return r, nil
}

// handleTaskGroup takes the state of a draining task group and computes the
// desired actions. For batch jobs we only notify when they have been migrated
// and never mark them for drain. Batch jobs are allowed to complete up until
// the deadline, after which they are force killed.
func handleTaskGroup(snap *state.StateSnapshot, batch bool, tg *structs.TaskGroup,
	allocs []*structs.Allocation, lastHandledIndex uint64, result *jobResult) error {

	// Determine how many allocations can be drained
	drainingNodes := make(map[string]bool, 4)
	healthy := 0
	remainingDrainingAlloc := false
	var drainable []*structs.Allocation

	for _, alloc := range allocs {
		// Check if the alloc is on a draining node.
		onDrainingNode, ok := drainingNodes[alloc.NodeID]
		if !ok {
			// Look up the node
			node, err := snap.NodeByID(nil, alloc.NodeID)
			if err != nil {
				return err
			}

			// Check if the node exists and whether it has a drain strategy
			onDrainingNode = node != nil && node.DrainStrategy != nil
			drainingNodes[alloc.NodeID] = onDrainingNode
		}

		// Check if the alloc should be considered migrated. A migrated
		// allocation is one that is terminal on the client, is on a draining
		// allocation, and has been updated since our last handled index to
		// avoid emitting many duplicate migrate events.
		if alloc.ClientTerminalStatus() &&
			onDrainingNode &&
			alloc.ModifyIndex > lastHandledIndex {
			result.migrated = append(result.migrated, alloc)
			continue
		}

		// If the service alloc is running and has its deployment status set, it
		// is considered healthy from a migration standpoint.
		if !batch && !alloc.TerminalStatus() && alloc.DeploymentStatus.HasHealth() {
			healthy++
		}

		// An alloc can't be considered for migration if:
		// - It isn't on a draining node
		// - It is already terminal on the client
		if !onDrainingNode || alloc.ClientTerminalStatus() {
			continue
		}

		// Capture the fact that there is an allocation that is still draining
		// for this job.
		remainingDrainingAlloc = true

		// If we haven't marked this allocation for migration already, capture
		// it as eligible for draining.
		if !batch && !alloc.DesiredTransition.ShouldMigrate() {
			drainable = append(drainable, alloc)
		}
	}

	// Update the done status
	if remainingDrainingAlloc {
		result.done = false
	}

	// We don't mark batch for drain so exit
	if batch {
		return nil
	}

	// Determine how many we can drain
	thresholdCount := tg.Count - tg.Migrate.MaxParallel
	numToDrain := healthy - thresholdCount
	numToDrain = min(len(drainable), numToDrain)
	if numToDrain <= 0 {
		return nil
	}

	result.drain = append(result.drain, drainable[0:numToDrain]...)
	return nil
}

// getJobAllocs returns all allocations for draining jobs
func (w *drainingJobWatcher) getJobAllocs(ctx context.Context, minIndex uint64) (map[structs.NamespacedID][]*structs.Allocation, uint64, error) {
	if err := w.limiter.Wait(ctx); err != nil {
		return nil, 0, err
	}

	resp, index, err := w.state.BlockingQuery(w.getJobAllocsImpl, minIndex, ctx)
	if err != nil {
		return nil, 0, err
	}
	if resp == nil {
		return nil, index, nil
	}

	return resp.(map[structs.NamespacedID][]*structs.Allocation), index, nil
}

// getJobAllocsImpl returns a map of draining jobs to their allocations.
func (w *drainingJobWatcher) getJobAllocsImpl(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	index, err := state.Index("allocs")
	if err != nil {
		return nil, 0, err
	}

	// Capture the draining jobs.
	draining := w.drainingJobs()
	l := len(draining)
	if l == 0 {
		return nil, index, nil
	}

	// Capture the allocs for each draining job.
	var maxIndex uint64 = 0
	resp := make(map[structs.NamespacedID][]*structs.Allocation, l)
	for jns := range draining {
		allocs, err := state.AllocsByJob(ws, jns.Namespace, jns.ID, false)
		if err != nil {
			return nil, index, err
		}

		resp[jns] = allocs
		for _, alloc := range allocs {
			if maxIndex < alloc.ModifyIndex {
				maxIndex = alloc.ModifyIndex
			}
		}
	}

	// Prefer using the actual max index of affected allocs since it means less
	// unblocking
	if maxIndex != 0 {
		index = maxIndex
	}

	return resp, index, nil
}

// drainingJobs captures the set of draining jobs.
func (w *drainingJobWatcher) drainingJobs() map[structs.NamespacedID]struct{} {
	w.l.RLock()
	defer w.l.RUnlock()

	l := len(w.jobs)
	if l == 0 {
		return nil
	}

	draining := make(map[structs.NamespacedID]struct{}, l)
	for k := range w.jobs {
		draining[k] = struct{}{}
	}

	return draining
}

// getQueryCtx is a helper for getting the query context.
func (w *drainingJobWatcher) getQueryCtx() context.Context {
	w.l.RLock()
	defer w.l.RUnlock()
	return w.queryCtx
}
