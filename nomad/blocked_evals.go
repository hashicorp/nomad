// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// unblockBuffer is the buffer size for the unblock channel. The buffer
	// should be large to ensure that the FSM doesn't block when calling Unblock
	// as this would apply back-pressure on Raft.
	unblockBuffer = 8096

	// pruneInterval is the interval at which we prune objects from the
	// BlockedEvals tracker
	pruneInterval = 5 * time.Minute

	// pruneThreshold is the threshold after which objects will be pruned.
	pruneThreshold = 15 * time.Minute
)

// BlockedEvals is used to track evaluations that shouldn't be queued until a
// certain class of nodes becomes available. An evaluation is put into the
// blocked state when it is run through the scheduler and produced failed
// allocations. It is unblocked when the capacity of a node that could run the
// failed allocation becomes available.
type BlockedEvals struct {
	// logger is the logger to use by the blocked eval tracker.
	logger hclog.Logger

	evalBroker *EvalBroker
	enabled    bool
	stats      *BlockedStats
	l          sync.RWMutex

	// captured is the set of evaluations that are captured by computed node
	// classes.
	captured map[string]wrappedEval

	// escaped is the set of evaluations that have escaped computed node
	// classes.
	escaped map[string]wrappedEval

	// system is the set of system evaluations that failed to start on nodes because of
	// resource constraints.
	system *systemEvals

	// unblockCh is used to buffer unblocking of evaluations.
	capacityChangeCh chan *capacityUpdate

	// jobs is the map of blocked job and is used to ensure that only one
	// blocked eval exists for each job. The value is the blocked evaluation ID.
	jobs map[structs.NamespacedID]string

	// unblockIndexes maps computed node classes or quota name to the index in
	// which they were unblocked. This is used to check if an evaluation could
	// have been unblocked between the time they were in the scheduler and the
	// time they are being blocked.
	unblockIndexes map[string]uint64

	// duplicates is the set of evaluations for jobs that had pre-existing
	// blocked evaluations. These should be marked as cancelled since only one
	// blocked eval is needed per job.
	duplicates []*structs.Evaluation

	// duplicateCh is used to signal that a duplicate eval was added to the
	// duplicate set. It can be used to unblock waiting callers looking for
	// duplicates.
	duplicateCh chan struct{}

	// timetable is used to correlate indexes with their insertion time. This
	// allows us to prune based on time.
	timetable *TimeTable

	// stopCh is used to stop any created goroutines.
	stopCh chan struct{}
}

// capacityUpdate stores unblock data.
type capacityUpdate struct {
	computedClass string
	quotaChange   string
	index         uint64
}

// wrappedEval captures both the evaluation and the optional token
type wrappedEval struct {
	eval  *structs.Evaluation
	token string
}

// NewBlockedEvals creates a new blocked eval tracker that will enqueue
// unblocked evals into the passed broker.
func NewBlockedEvals(evalBroker *EvalBroker, logger hclog.Logger) *BlockedEvals {
	return &BlockedEvals{
		logger:           logger.Named("blocked_evals"),
		evalBroker:       evalBroker,
		captured:         make(map[string]wrappedEval),
		escaped:          make(map[string]wrappedEval),
		system:           newSystemEvals(),
		jobs:             make(map[structs.NamespacedID]string),
		unblockIndexes:   make(map[string]uint64),
		capacityChangeCh: make(chan *capacityUpdate, unblockBuffer),
		duplicateCh:      make(chan struct{}, 1),
		stopCh:           make(chan struct{}),
		stats:            NewBlockedStats(),
	}
}

// Enabled is used to check if the broker is enabled.
func (b *BlockedEvals) Enabled() bool {
	b.l.RLock()
	defer b.l.RUnlock()
	return b.enabled
}

// SetEnabled is used to control if the blocked eval tracker is enabled. The
// tracker should only be enabled on the active leader.
func (b *BlockedEvals) SetEnabled(enabled bool) {
	b.l.Lock()
	if b.enabled == enabled {
		// No-op
		b.l.Unlock()
		return
	} else if enabled {
		go b.watchCapacity(b.stopCh, b.capacityChangeCh)
		go b.prune(b.stopCh)
	} else {
		close(b.stopCh)
	}
	b.enabled = enabled
	b.l.Unlock()
	if !enabled {
		b.Flush()
	}
}

func (b *BlockedEvals) SetTimetable(timetable *TimeTable) {
	b.l.Lock()
	b.timetable = timetable
	b.l.Unlock()
}

// Block tracks the passed evaluation and enqueues it into the eval broker when
// a suitable node calls unblock.
func (b *BlockedEvals) Block(eval *structs.Evaluation) {
	b.processBlock(eval, "")
}

// Reblock tracks the passed evaluation and enqueues it into the eval broker when
// a suitable node calls unblock. Reblock should be used over Block when the
// blocking is occurring by an outstanding evaluation. The token is the
// evaluation's token.
func (b *BlockedEvals) Reblock(eval *structs.Evaluation, token string) {
	b.processBlock(eval, token)
}

// processBlock is the implementation of blocking an evaluation. It supports
// taking an optional evaluation token to use when reblocking an evaluation that
// may be outstanding.
func (b *BlockedEvals) processBlock(eval *structs.Evaluation, token string) {
	b.l.Lock()
	defer b.l.Unlock()

	// Do nothing if not enabled
	if !b.enabled {
		return
	}

	// Handle the new evaluation being for a job we are already tracking.
	if b.processBlockJobDuplicate(eval) {
		// If process block job duplicate returns true, the new evaluation has
		// been marked as a duplicate and we have nothing to do, so return
		// early.
		return
	}

	// Check if the eval missed an unblock while it was in the scheduler at an
	// older index. The scheduler could have been invoked with a snapshot of
	// state that was prior to additional capacity being added or allocations
	// becoming terminal.
	if b.missedUnblock(eval) {
		// Just re-enqueue the eval immediately. We pass the token so that the
		// eval_broker can properly handle the case in which the evaluation is
		// still outstanding.
		b.evalBroker.EnqueueAll(map[*structs.Evaluation]string{eval: token})
		return
	}

	// Mark the job as tracked.
	b.jobs[structs.NewNamespacedID(eval.JobID, eval.Namespace)] = eval.ID
	b.stats.Block(eval)

	// Track that the evaluation is being added due to reaching the quota limit
	if eval.QuotaLimitReached != "" {
		b.stats.TotalQuotaLimit++
	}

	// Wrap the evaluation, capturing its token.
	wrapped := wrappedEval{
		eval:  eval,
		token: token,
	}

	// If the eval has escaped, meaning computed node classes could not capture
	// the constraints of the job, we store the eval separately as we have to
	// unblock it whenever node capacity changes. This is because we don't know
	// what node class is feasible for the jobs constraints.
	if eval.EscapedComputedClass {
		b.escaped[eval.ID] = wrapped
		b.stats.TotalEscaped++
		return
	}

	// System evals are indexed by node and re-processed on utilization changes in
	// existing nodes
	if eval.Type == structs.JobTypeSystem {
		b.system.Add(eval, token)
	}

	// Add the eval to the set of blocked evals whose jobs constraints are
	// captured by computed node class.
	b.captured[eval.ID] = wrapped
}

// processBlockJobDuplicate handles the case where the new eval is for a job
// that we are already tracking. If the eval is a duplicate, we add the older
// evaluation by Raft index to the list of duplicates such that it can be
// cancelled. We only ever want one blocked evaluation per job, otherwise we
// would create unnecessary work for the scheduler as multiple evals for the
// same job would be run, all producing the same outcome. It is critical to
// prefer the newer evaluation, since it will contain the most up to date set of
// class eligibility. The return value is set to true, if the passed evaluation
// is cancelled. This should be called with the lock held.
func (b *BlockedEvals) processBlockJobDuplicate(eval *structs.Evaluation) (newCancelled bool) {
	existingID, hasExisting := b.jobs[structs.NewNamespacedID(eval.JobID, eval.Namespace)]
	if !hasExisting {
		return
	}

	var dup *structs.Evaluation
	existingW, ok := b.captured[existingID]
	if ok {
		if latestEvalIndex(existingW.eval) <= latestEvalIndex(eval) {
			delete(b.captured, existingID)
			dup = existingW.eval
			b.stats.Unblock(dup)
		} else {
			dup = eval
			newCancelled = true
		}
	} else {
		existingW, ok = b.escaped[existingID]
		if !ok {
			// This is a programming error
			b.logger.Error("existing blocked evaluation is neither tracked as captured or escaped", "existing_id", existingID)
			delete(b.jobs, structs.NewNamespacedID(eval.JobID, eval.Namespace))
			return
		}

		if latestEvalIndex(existingW.eval) <= latestEvalIndex(eval) {
			delete(b.escaped, existingID)
			b.stats.TotalEscaped--
			dup = existingW.eval
		} else {
			dup = eval
			newCancelled = true
		}
	}

	b.duplicates = append(b.duplicates, dup)

	// Unblock any waiter.
	select {
	case b.duplicateCh <- struct{}{}:
	default:
	}

	return
}

// latestEvalIndex returns the max of the evaluations create and snapshot index
func latestEvalIndex(eval *structs.Evaluation) uint64 {
	if eval == nil {
		return 0
	}

	return max(eval.CreateIndex, eval.SnapshotIndex)
}

// missedUnblock returns whether an evaluation missed an unblock while it was in
// the scheduler. Since the scheduler can operate at an index in the past, the
// evaluation may have been processed missing data that would allow it to
// complete. This method returns if that is the case and should be called with
// the lock held.
func (b *BlockedEvals) missedUnblock(eval *structs.Evaluation) bool {
	var max uint64 = 0
	for id, index := range b.unblockIndexes {
		// Calculate the max unblock index
		if max < index {
			max = index
		}

		// The evaluation is blocked because it has hit a quota limit not class
		// eligibility
		if eval.QuotaLimitReached != "" {
			if eval.QuotaLimitReached != id {
				// Not a match
				continue
			} else if eval.SnapshotIndex < index {
				// The evaluation was processed before the quota specification was
				// updated, so unblock the evaluation.
				return true
			}

			// The evaluation was processed having seen all changes to the quota
			return false
		}

		elig, ok := eval.ClassEligibility[id]
		if !ok && eval.SnapshotIndex < index {
			// The evaluation was processed and did not encounter this class
			// because it was added after it was processed. Thus for correctness
			// we need to unblock it.
			return true
		}

		// The evaluation could use the computed node class and the eval was
		// processed before the last unblock.
		if elig && eval.SnapshotIndex < index {
			return true
		}
	}

	// If the evaluation has escaped, and the map contains an index older than
	// the evaluations, it should be unblocked.
	if eval.EscapedComputedClass && eval.SnapshotIndex < max {
		return true
	}

	// The evaluation is ahead of all recent unblocks.
	return false
}

// Untrack causes any blocked evaluation for the passed job to be no longer
// tracked. Untrack is called when there is a successful evaluation for the job
// and a blocked evaluation is no longer needed.
func (b *BlockedEvals) Untrack(jobID, namespace string) {
	b.l.Lock()
	defer b.l.Unlock()

	// Do nothing if not enabled
	if !b.enabled {
		return
	}

	nsID := structs.NewNamespacedID(jobID, namespace)

	if evals, ok := b.system.JobEvals(nsID); ok {
		for _, e := range evals {
			b.system.Remove(e)
			b.stats.Unblock(e)
		}
		return
	}

	// Get the evaluation ID to cancel
	evalID, ok := b.jobs[nsID]
	if !ok {
		// No blocked evaluation so exit
		return
	}

	// Attempt to delete the evaluation
	if w, ok := b.captured[evalID]; ok {
		delete(b.jobs, nsID)
		delete(b.captured, evalID)
		b.stats.Unblock(w.eval)
		if w.eval.QuotaLimitReached != "" {
			b.stats.TotalQuotaLimit--
		}
	}

	if w, ok := b.escaped[evalID]; ok {
		delete(b.jobs, nsID)
		delete(b.escaped, evalID)
		b.stats.TotalEscaped--
		b.stats.Unblock(w.eval)
		if w.eval.QuotaLimitReached != "" {
			b.stats.TotalQuotaLimit--
		}
	}
}

// Unblock causes any evaluation that could potentially make progress on a
// capacity change on the passed computed node class to be enqueued into the
// eval broker.
func (b *BlockedEvals) Unblock(computedClass string, index uint64) {
	b.l.Lock()

	// Do nothing if not enabled
	if !b.enabled {
		b.l.Unlock()
		return
	}

	// Store the index in which the unblock happened. We use this on subsequent
	// block calls in case the evaluation was in the scheduler when a trigger
	// occurred.
	b.unblockIndexes[computedClass] = index

	// Capture chan in lock as Flush overwrites it
	ch := b.capacityChangeCh
	done := b.stopCh
	b.l.Unlock()

	select {
	case <-done:
	case ch <- &capacityUpdate{
		computedClass: computedClass,
		index:         index,
	}:
	}
}

// UnblockQuota causes any evaluation that could potentially make progress on a
// capacity change on the passed quota to be enqueued into the eval broker.
func (b *BlockedEvals) UnblockQuota(quota string, index uint64) {
	// Nothing to do
	if quota == "" {
		return
	}

	b.l.Lock()

	// Do nothing if not enabled
	if !b.enabled {
		b.l.Unlock()
		return
	}

	// Store the index in which the unblock happened. We use this on subsequent
	// block calls in case the evaluation was in the scheduler when a trigger
	// occurred.
	b.unblockIndexes[quota] = index
	ch := b.capacityChangeCh
	done := b.stopCh
	b.l.Unlock()

	select {
	case <-done:
	case ch <- &capacityUpdate{
		quotaChange: quota,
		index:       index,
	}:
	}
}

// UnblockClassAndQuota causes any evaluation that could potentially make
// progress on a capacity change on the passed computed node class or quota to
// be enqueued into the eval broker.
func (b *BlockedEvals) UnblockClassAndQuota(class, quota string, index uint64) {
	b.l.Lock()

	// Do nothing if not enabled
	if !b.enabled {
		b.l.Unlock()
		return
	}

	// Store the index in which the unblock happened. We use this on subsequent
	// block calls in case the evaluation was in the scheduler when a trigger
	// occurred.
	if quota != "" {
		b.unblockIndexes[quota] = index
	}
	b.unblockIndexes[class] = index

	// Capture chan inside the lock to prevent a race with it getting reset
	// in Flush.
	ch := b.capacityChangeCh
	done := b.stopCh
	b.l.Unlock()

	select {
	case <-done:
	case ch <- &capacityUpdate{
		computedClass: class,
		quotaChange:   quota,
		index:         index,
	}:
	}
}

// UnblockNode finds any blocked evalution that's node specific (system jobs) and enqueues
// it on the eval broker
func (b *BlockedEvals) UnblockNode(nodeID string, index uint64) {
	b.l.Lock()
	defer b.l.Unlock()

	evals, ok := b.system.NodeEvals(nodeID)

	// Do nothing if not enabled
	if !b.enabled || !ok || len(evals) == 0 {
		return
	}

	for e := range evals {
		b.system.Remove(e)
		b.stats.Unblock(e)
	}

	b.evalBroker.EnqueueAll(evals)
}

// watchCapacity is a long lived function that watches for capacity changes in
// nodes and unblocks the correct set of evals.
func (b *BlockedEvals) watchCapacity(stopCh <-chan struct{}, changeCh <-chan *capacityUpdate) {
	for {
		select {
		case <-stopCh:
			return
		case update := <-changeCh:
			b.unblock(update.computedClass, update.quotaChange, update.index)
		}
	}
}

func (b *BlockedEvals) unblock(computedClass, quota string, index uint64) {
	b.l.Lock()
	defer b.l.Unlock()

	// Protect against the case of a flush.
	if !b.enabled {
		return
	}

	// Every eval that has escaped computed node class has to be unblocked
	// because any node could potentially be feasible.
	numQuotaLimit := 0
	numEscaped := len(b.escaped)
	unblocked := make(map[*structs.Evaluation]string, max(uint64(numEscaped), 4))

	if numEscaped != 0 && computedClass != "" {
		for id, wrapped := range b.escaped {
			unblocked[wrapped.eval] = wrapped.token
			delete(b.escaped, id)
			delete(b.jobs, structs.NewNamespacedID(wrapped.eval.JobID, wrapped.eval.Namespace))

			if wrapped.eval.QuotaLimitReached != "" {
				numQuotaLimit++
			}
		}
	}

	// We unblock any eval that is explicitly eligible for the computed class
	// and also any eval that is not eligible or uneligible. This signifies that
	// when the evaluation was originally run through the scheduler, that it
	// never saw a node with the given computed class and thus needs to be
	// unblocked for correctness.
	for id, wrapped := range b.captured {
		if quota != "" && wrapped.eval.QuotaLimitReached != quota {
			// We are unblocking based on quota and this eval doesn't match
			continue
		} else if elig, ok := wrapped.eval.ClassEligibility[computedClass]; ok && !elig {
			// Can skip because the eval has explicitly marked the node class
			// as ineligible.
			continue
		}

		// Unblock the evaluation because it is either for the matching quota,
		// is eligible based on the computed node class, or never seen the
		// computed node class.
		unblocked[wrapped.eval] = wrapped.token
		delete(b.jobs, structs.NewNamespacedID(wrapped.eval.JobID, wrapped.eval.Namespace))
		delete(b.captured, id)
		if wrapped.eval.QuotaLimitReached != "" {
			numQuotaLimit++
		}
	}

	if len(unblocked) != 0 {
		// Update the counters
		b.stats.TotalEscaped = 0
		b.stats.TotalQuotaLimit -= numQuotaLimit
		for eval := range unblocked {
			b.stats.Unblock(eval)
		}

		// Enqueue all the unblocked evals into the broker.
		b.evalBroker.EnqueueAll(unblocked)
	}
}

// UnblockFailed unblocks all blocked evaluation that were due to scheduler
// failure.
func (b *BlockedEvals) UnblockFailed() {
	b.l.Lock()
	defer b.l.Unlock()

	// Do nothing if not enabled
	if !b.enabled {
		return
	}

	quotaLimit := 0
	unblocked := make(map[*structs.Evaluation]string, 4)
	for id, wrapped := range b.captured {
		if wrapped.eval.TriggeredBy == structs.EvalTriggerMaxPlans {
			unblocked[wrapped.eval] = wrapped.token
			delete(b.captured, id)
			delete(b.jobs, structs.NewNamespacedID(wrapped.eval.JobID, wrapped.eval.Namespace))
			if wrapped.eval.QuotaLimitReached != "" {
				quotaLimit++
			}
		}
	}

	for id, wrapped := range b.escaped {
		if wrapped.eval.TriggeredBy == structs.EvalTriggerMaxPlans {
			unblocked[wrapped.eval] = wrapped.token
			delete(b.escaped, id)
			delete(b.jobs, structs.NewNamespacedID(wrapped.eval.JobID, wrapped.eval.Namespace))
			b.stats.TotalEscaped -= 1
			if wrapped.eval.QuotaLimitReached != "" {
				quotaLimit++
			}
		}
	}

	if len(unblocked) > 0 {
		b.stats.TotalQuotaLimit -= quotaLimit
		for eval := range unblocked {
			b.stats.Unblock(eval)
		}

		b.evalBroker.EnqueueAll(unblocked)
	}
}

// GetDuplicates returns all the duplicate evaluations and blocks until the
// passed timeout.
func (b *BlockedEvals) GetDuplicates(timeout time.Duration) []*structs.Evaluation {
	var timeoutTimer *time.Timer
	var timeoutCh <-chan time.Time
SCAN:
	b.l.Lock()
	if len(b.duplicates) != 0 {
		dups := b.duplicates
		b.duplicates = nil
		b.l.Unlock()
		return dups
	}

	// Capture chans inside the lock to prevent a race with them getting
	// reset in Flush
	dupCh := b.duplicateCh
	stopCh := b.stopCh
	b.l.Unlock()

	// Create the timer
	if timeoutTimer == nil && timeout != 0 {
		timeoutTimer = time.NewTimer(timeout)
		timeoutCh = timeoutTimer.C
		defer timeoutTimer.Stop()
	}

	select {
	case <-stopCh:
		return nil
	case <-timeoutCh:
		return nil
	case <-dupCh:
		goto SCAN
	}
}

// Flush is used to clear the state of blocked evaluations.
func (b *BlockedEvals) Flush() {
	b.l.Lock()
	defer b.l.Unlock()

	// Reset the blocked eval tracker.
	b.stats.TotalEscaped = 0
	b.stats.TotalBlocked = 0
	b.stats.TotalQuotaLimit = 0
	b.stats.BlockedResources = NewBlockedResourcesStats()
	b.captured = make(map[string]wrappedEval)
	b.escaped = make(map[string]wrappedEval)
	b.jobs = make(map[structs.NamespacedID]string)
	b.unblockIndexes = make(map[string]uint64)
	b.timetable = nil
	b.duplicates = nil
	b.capacityChangeCh = make(chan *capacityUpdate, unblockBuffer)
	b.stopCh = make(chan struct{})
	b.duplicateCh = make(chan struct{}, 1)
	b.system = newSystemEvals()
}

// Stats is used to query the state of the blocked eval tracker.
func (b *BlockedEvals) Stats() *BlockedStats {
	// Allocate a new stats struct
	stats := NewBlockedStats()

	b.l.RLock()
	defer b.l.RUnlock()

	// Copy all the stats
	stats.TotalEscaped = b.stats.TotalEscaped
	stats.TotalBlocked = b.stats.TotalBlocked
	stats.TotalQuotaLimit = b.stats.TotalQuotaLimit
	stats.BlockedResources = b.stats.BlockedResources.Copy()

	return stats
}

// EmitStats is used to export metrics about the blocked eval tracker while enabled
func (b *BlockedEvals) EmitStats(period time.Duration, stopCh <-chan struct{}) {
	timer, stop := helper.NewSafeTimer(period)
	defer stop()

	for {
		timer.Reset(period)

		select {
		case <-timer.C:
			stats := b.Stats()
			metrics.SetGauge([]string{"nomad", "blocked_evals", "total_quota_limit"}, float32(stats.TotalQuotaLimit))
			metrics.SetGauge([]string{"nomad", "blocked_evals", "total_blocked"}, float32(stats.TotalBlocked))
			metrics.SetGauge([]string{"nomad", "blocked_evals", "total_escaped"}, float32(stats.TotalEscaped))

			for k, v := range stats.BlockedResources.ByJob {
				labels := []metrics.Label{
					{Name: "namespace", Value: k.Namespace},
					{Name: "job", Value: k.ID},
				}
				metrics.SetGaugeWithLabels([]string{"nomad", "blocked_evals", "job", "cpu"}, float32(v.CPU), labels)
				metrics.SetGaugeWithLabels([]string{"nomad", "blocked_evals", "job", "memory"}, float32(v.MemoryMB), labels)
			}

			for k, v := range stats.BlockedResources.ByClassInDC {
				labels := []metrics.Label{
					{Name: "datacenter", Value: k.dc},
					{Name: "node_class", Value: k.class},
				}
				metrics.SetGaugeWithLabels([]string{"nomad", "blocked_evals", "cpu"}, float32(v.CPU), labels)
				metrics.SetGaugeWithLabels([]string{"nomad", "blocked_evals", "memory"}, float32(v.MemoryMB), labels)
			}
		case <-stopCh:
			return
		}
	}
}

// prune is a long lived function that prunes unnecessary objects on a timer.
func (b *BlockedEvals) prune(stopCh <-chan struct{}) {
	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case t := <-ticker.C:
			cutoff := t.UTC().Add(-1 * pruneThreshold)
			b.pruneUnblockIndexes(cutoff)
			b.pruneStats(cutoff)
		}
	}
}

// pruneUnblockIndexes is used to prune any tracked entry that is excessively
// old. This protects againsts unbounded growth of the map.
func (b *BlockedEvals) pruneUnblockIndexes(cutoff time.Time) {
	b.l.Lock()
	defer b.l.Unlock()

	if b.timetable == nil {
		return
	}

	oldThreshold := b.timetable.NearestIndex(cutoff)
	for key, index := range b.unblockIndexes {
		if index < oldThreshold {
			delete(b.unblockIndexes, key)
		}
	}
}

// pruneStats is used to prune any zero value stats that are excessively old.
func (b *BlockedEvals) pruneStats(cutoff time.Time) {
	b.l.Lock()
	defer b.l.Unlock()

	b.stats.prune(cutoff)
}
