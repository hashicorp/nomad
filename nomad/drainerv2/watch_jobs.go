package drainerv2

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

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
	RegisterJob(jobID, namespace string)

	// Drain is used to emit allocations that should be drained.
	Drain() <-chan *DrainRequest

	// Migrated is allocations for draining jobs that have transistioned to
	// stop. There is no guarantee that duplicates won't be published.
	Migrated() <-chan []*structs.Allocation
}

// drainingJobWatcher is used to watch draining jobs and emit events when
// draining allocations have replacements
type drainingJobWatcher struct {
	ctx    context.Context
	logger *log.Logger

	// state is the state that is watched for state changes.
	state *state.StateStore

	// limiter is used to limit the rate of blocking queries
	limiter *rate.Limiter

	// jobs is the set of tracked jobs.
	jobs map[structs.JobNs]struct{}

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
func NewDrainingJobWatcher(ctx context.Context, limiter *rate.Limiter, state *state.StateStore, logger *log.Logger) *drainingJobWatcher {

	// Create a context that can cancel the blocking query so that when a new
	// job gets registered it is handled.
	queryCtx, queryCancel := context.WithCancel(ctx)

	w := &drainingJobWatcher{
		ctx:         ctx,
		queryCtx:    queryCtx,
		queryCancel: queryCancel,
		limiter:     limiter,
		logger:      logger,
		state:       state,
		jobs:        make(map[structs.JobNs]struct{}, 64),
		drainCh:     make(chan *DrainRequest, 8),
		migratedCh:  make(chan []*structs.Allocation, 8),
	}

	go w.watch()
	return w
}

// RegisterJob marks the given job as draining and adds it to being watched.
func (w *drainingJobWatcher) RegisterJob(jobID, namespace string) {
	w.l.Lock()
	defer w.l.Unlock()

	jns := structs.JobNs{
		ID:        jobID,
		Namespace: namespace,
	}
	if _, ok := w.jobs[jns]; ok {
		return
	}

	// Add the job and cancel the context
	w.jobs[jns] = struct{}{}
	w.queryCancel()

	// Create a new query context
	w.queryCtx, w.queryCancel = context.WithCancel(w.ctx)
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
	jns := structs.JobNs{
		ID:        jobID,
		Namespace: namespace,
	}
	delete(w.jobs, jns)
	w.logger.Printf("[TRACE] nomad.drain.job_watcher: deregistering job %v", jns)
}

// watch is the long lived watching routine that detects job drain changes.
func (w *drainingJobWatcher) watch() {
	jindex := uint64(1)
	for {
		w.logger.Printf("[TRACE] nomad.drain.job_watcher: getting job allocs at index %d", jindex)
		jobAllocs, index, err := w.getJobAllocs(w.getQueryCtx(), jindex)
		if err != nil {
			if err == context.Canceled {
				// Determine if it is a cancel or a shutdown
				select {
				case <-w.ctx.Done():
					w.logger.Printf("[TRACE] nomad.drain.job_watcher: shutting down")
					return
				default:
					// The query context was cancelled
					continue
				}
			}

			w.logger.Printf("[ERR] nomad.drain.job_watcher: error watching job allocs updates at index %d: %v", jindex, err)
			select {
			case <-w.ctx.Done():
				w.logger.Printf("[TRACE] nomad.drain.job_watcher: shutting down")
				return
			case <-time.After(stateReadErrorDelay):
				continue
			}
		}

		// update index for next run
		lastHandled := jindex
		jindex = index

		// Snapshot the state store
		snap, err := w.state.Snapshot()
		if err != nil {
			w.logger.Printf("[WARN] nomad.drain.job_watcher: failed to snapshot statestore: %v", err)
			continue
		}

		currentJobs := w.drainingJobs()
		var allDrain, allMigrated []*structs.Allocation
		for job, allocs := range jobAllocs {
			// Check if the job is still registered
			if _, ok := currentJobs[job]; !ok {
				continue
			}

			w.logger.Printf("[TRACE] nomad.drain.job_watcher: handling job %v", job)

			// Lookup the job
			job, err := w.state.JobByID(nil, job.Namespace, job.ID)
			if err != nil {
				w.logger.Printf("[WARN] nomad.drain.job_watcher: failed to lookup job %v: %v", job, err)
				continue
			}

			// Ignore all non-service jobs
			if job.Type != structs.JobTypeService {
				w.deregisterJob(job.ID, job.Namespace)
				continue
			}

			result, err := handleJob(snap, job, allocs, lastHandled)
			if err != nil {
				w.logger.Printf("[ERR] nomad.drain.job_watcher: handling drain for job %v failed: %v", job, err)
				continue
			}

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

			select {
			case w.drainCh <- req:
			case <-w.ctx.Done():
				w.logger.Printf("[TRACE] nomad.drain.job_watcher: shutting down")
				return
			}

			// Wait for the request to be commited
			select {
			case <-req.Resp.WaitCh():
			case <-w.ctx.Done():
				w.logger.Printf("[TRACE] nomad.drain.job_watcher: shutting down")
				return
			}

			// See if it successfully committed
			if err := req.Resp.Error(); err != nil {
				w.logger.Printf("[ERR] nomad.drain.job_watcher: failed to transistion allocations: %v", err)
			}

			// TODO Probably want to wait till the new index
		}

		if len(allMigrated) != 0 {
			select {
			case w.migratedCh <- allMigrated:
			case <-w.ctx.Done():
				w.logger.Printf("[TRACE] nomad.drain.job_watcher: shutting down")
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

// newJobResult returns an initialized jobResult
func newJobResult() *jobResult {
	return &jobResult{
		done: true,
	}
}

// handleJob takes the state of a draining job and returns the desired actions.
func handleJob(snap *state.StateSnapshot, job *structs.Job, allocs []*structs.Allocation, lastHandledIndex uint64) (*jobResult, error) {
	r := newJobResult()
	taskGroups := make(map[string]*structs.TaskGroup, len(job.TaskGroups))
	for _, tg := range job.TaskGroups {
		if tg.Migrate != nil {
			// TODO handle the upgrade path
			// Only capture the groups that have a migrate strategy
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
		if err := handleTaskGroup(snap, tg, allocs, lastHandledIndex, r); err != nil {
			return nil, fmt.Errorf("drain for task group %q failed: %v", name, err)
		}
	}

	return r, nil
}

// handleTaskGroup takes the state of a draining task group and computes the desired actions.
func handleTaskGroup(snap *state.StateSnapshot, tg *structs.TaskGroup,
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

			onDrainingNode = node.DrainStrategy != nil
			drainingNodes[node.ID] = onDrainingNode
		}

		// Check if the alloc should be considered migrated. A migrated
		// allocation is one that is terminal, is on a draining
		// allocation, and has only happened since our last handled index to
		// avoid emitting many duplicate migrate events.
		if alloc.TerminalStatus() &&
			onDrainingNode &&
			alloc.ModifyIndex > lastHandledIndex {
			result.migrated = append(result.migrated, alloc)
			continue
		}

		// If the alloc is running and has its deployment status set, it is
		// considered healthy from a migration standpoint.
		if !alloc.TerminalStatus() &&
			alloc.DeploymentStatus != nil &&
			alloc.DeploymentStatus.Healthy != nil {
			healthy++
		}

		// An alloc can't be considered for migration if:
		// - It isn't on a draining node
		// - It is already terminal
		// - It has already been marked for draining
		if !onDrainingNode || alloc.TerminalStatus() || alloc.DesiredTransition.ShouldMigrate() {
			continue
		}

		// This alloc is drainable, so capture it and the fact that the job
		// isn't done draining yet.
		remainingDrainingAlloc = true
		drainable = append(drainable, alloc)
	}

	// Update the done status
	if remainingDrainingAlloc {
		result.done = false
	}

	// Determine how many we can drain
	thresholdCount := tg.Count - tg.Migrate.MaxParallel
	numToDrain := healthy - thresholdCount
	numToDrain = helper.IntMin(len(drainable), numToDrain)
	if numToDrain <= 0 {
		return nil
	}

	result.drain = append(result.drain, drainable[0:numToDrain]...)
	return nil
}

// getJobAllocs returns all allocations for draining jobs
func (w *drainingJobWatcher) getJobAllocs(ctx context.Context, minIndex uint64) (map[structs.JobNs][]*structs.Allocation, uint64, error) {
	if err := w.limiter.Wait(ctx); err != nil {
		return nil, 0, err
	}

	resp, index, err := w.state.BlockingQuery(w.getJobAllocsImpl, minIndex, ctx)
	if err != nil {
		return nil, 0, err
	}

	return resp.(map[structs.JobNs][]*structs.Allocation), index, nil
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
	resp := make(map[structs.JobNs][]*structs.Allocation, l)
	for jns := range draining {
		allocs, err := state.AllocsByJob(ws, jns.Namespace, jns.ID, false)
		if err != nil {
			return nil, index, err
		}

		resp[jns] = allocs
	}

	return resp, index, nil
}

// drainingJobs captures the set of draining jobs.
func (w *drainingJobWatcher) drainingJobs() map[structs.JobNs]struct{} {
	w.l.RLock()
	defer w.l.RUnlock()

	l := len(w.jobs)
	if l == 0 {
		return nil
	}

	draining := make(map[structs.JobNs]struct{}, l)
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
