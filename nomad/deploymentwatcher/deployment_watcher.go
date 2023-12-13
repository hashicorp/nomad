// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/time/rate"
)

const (
	// perJobEvalBatchPeriod is the batching length before creating an evaluation to
	// trigger the scheduler when allocations are marked as healthy.
	perJobEvalBatchPeriod = 1 * time.Second
)

var (
	// allowRescheduleTransition is the transition that allows failed
	// allocations part of a deployment to be rescheduled. We create a one off
	// variable to avoid creating a new object for every request.
	allowRescheduleTransition = &structs.DesiredTransition{
		Reschedule: pointer.Of(true),
	}
)

// deploymentTriggers are the set of functions required to trigger changes on
// behalf of a deployment
type deploymentTriggers interface {
	// createUpdate is used to create allocation desired transition updates and
	// an evaluation.
	createUpdate(allocs map[string]*structs.DesiredTransition, eval *structs.Evaluation) (uint64, error)

	// upsertJob is used to roll back a job when autoreverting for a deployment
	upsertJob(job *structs.Job) (uint64, error)

	// upsertDeploymentStatusUpdate is used to upsert a deployment status update
	// and an optional evaluation and job to upsert
	upsertDeploymentStatusUpdate(u *structs.DeploymentStatusUpdate, eval *structs.Evaluation, job *structs.Job) (uint64, error)

	// upsertDeploymentPromotion is used to promote canaries in a deployment
	upsertDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error)

	// upsertDeploymentAllocHealth is used to set the health of allocations in a
	// deployment
	upsertDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error)
}

// deploymentWatcher is used to watch a single deployment and trigger the
// scheduler when allocation health transitions.
type deploymentWatcher struct {
	// queryLimiter is used to limit the rate of blocking queries
	queryLimiter *rate.Limiter

	// deploymentTriggers holds the methods required to trigger changes on behalf of the
	// deployment
	deploymentTriggers

	// DeploymentRPC holds methods for interacting with peer regions
	// in enterprise edition
	DeploymentRPC

	// JobRPC holds methods for interacting with peer regions
	// in enterprise edition
	JobRPC

	// state is the state that is watched for state changes.
	state *state.StateStore

	// deploymentID is the deployment's ID being watched
	deploymentID string

	// deploymentUpdateCh is triggered when there is an updated deployment
	deploymentUpdateCh chan struct{}

	// d is the deployment being watched
	d *structs.Deployment

	// j is the job the deployment is for
	j *structs.Job

	// outstandingBatch marks whether an outstanding function exists to create
	// the evaluation. Access should be done through the lock.
	outstandingBatch bool

	// outstandingAllowReplacements is the map of allocations that will be
	// marked as allowing a replacement. Access should be done through the lock.
	outstandingAllowReplacements map[string]*structs.DesiredTransition

	// latestEval is the latest eval for the job. It is updated by the watch
	// loop and any time an evaluation is created. The field should be accessed
	// by holding the lock or using the setter and getter methods.
	latestEval uint64

	logger log.Logger
	ctx    context.Context
	exitFn context.CancelFunc
	l      sync.RWMutex
}

// newDeploymentWatcher returns a deployment watcher that is used to watch
// deployments and trigger the scheduler as needed.
func newDeploymentWatcher(parent context.Context, queryLimiter *rate.Limiter,
	logger log.Logger, state *state.StateStore, d *structs.Deployment,
	j *structs.Job, triggers deploymentTriggers,
	deploymentRPC DeploymentRPC, jobRPC JobRPC) *deploymentWatcher {

	ctx, exitFn := context.WithCancel(parent)
	w := &deploymentWatcher{
		queryLimiter:       queryLimiter,
		deploymentID:       d.ID,
		deploymentUpdateCh: make(chan struct{}, 1),
		d:                  d,
		j:                  j,
		state:              state,
		deploymentTriggers: triggers,
		DeploymentRPC:      deploymentRPC,
		JobRPC:             jobRPC,
		logger:             logger.With("deployment_id", d.ID, "job", j.NamespacedID()),
		ctx:                ctx,
		exitFn:             exitFn,
	}

	// Start the long lived watcher that scans for allocation updates
	go w.watch()

	return w
}

// updateDeployment is used to update the tracked deployment.
func (w *deploymentWatcher) updateDeployment(d *structs.Deployment) {
	w.l.Lock()
	defer w.l.Unlock()

	// Update and trigger
	w.d = d
	select {
	case w.deploymentUpdateCh <- struct{}{}:
	default:
	}
}

// getDeployment returns the tracked deployment.
func (w *deploymentWatcher) getDeployment() *structs.Deployment {
	w.l.RLock()
	defer w.l.RUnlock()
	return w.d
}

func (w *deploymentWatcher) SetAllocHealth(
	req *structs.DeploymentAllocHealthRequest,
	resp *structs.DeploymentUpdateResponse) error {

	// If we are failing the deployment, update the status and potentially
	// rollback
	var j *structs.Job
	var u *structs.DeploymentStatusUpdate

	// If there are unhealthy allocations we need to mark the deployment as
	// failed and check if we should roll back to a stable job.
	if l := len(req.UnhealthyAllocationIDs); l != 0 {
		unhealthy := make(map[string]struct{}, l)
		for _, alloc := range req.UnhealthyAllocationIDs {
			unhealthy[alloc] = struct{}{}
		}

		// Get the allocations for the deployment
		snap, err := w.state.Snapshot()
		if err != nil {
			return err
		}

		allocs, err := snap.AllocsByDeployment(nil, req.DeploymentID)
		if err != nil {
			return err
		}

		// Determine if we should autorevert to an older job
		desc := structs.DeploymentStatusDescriptionFailedAllocations
		for _, alloc := range allocs {
			// Check that the alloc has been marked unhealthy
			if _, ok := unhealthy[alloc.ID]; !ok {
				continue
			}

			// Check if the group has autorevert set
			dstate, ok := w.getDeployment().TaskGroups[alloc.TaskGroup]
			if !ok || !dstate.AutoRevert {
				continue
			}

			var err error
			j, err = w.latestStableJob()
			if err != nil {
				return err
			}

			if j != nil {
				j, desc = w.handleRollbackValidity(j, desc)
			}
			break
		}

		u = w.getDeploymentStatusUpdate(structs.DeploymentStatusFailed, desc)
	}

	// Canonicalize the job in case it doesn't have namespace set
	j.Canonicalize()

	// Create the request
	areq := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: *req,
		Timestamp:                    time.Now(),
		Eval:                         w.getEval(),
		DeploymentUpdate:             u,
		Job:                          j,
	}

	index, err := w.upsertDeploymentAllocHealth(areq)
	if err != nil {
		return err
	}

	// Build the response
	resp.EvalID = areq.Eval.ID
	resp.EvalCreateIndex = index
	resp.DeploymentModifyIndex = index
	resp.Index = index
	if j != nil {
		resp.RevertedJobVersion = pointer.Of(j.Version)
	}
	return nil
}

// handleRollbackValidity checks if the job being rolled back to has the same spec as the existing job
// Returns a modified description and job accordingly.
func (w *deploymentWatcher) handleRollbackValidity(rollbackJob *structs.Job, desc string) (*structs.Job, string) {
	// Only rollback if job being changed has a different spec.
	// This prevents an infinite revert cycle when a previously stable version of the job fails to start up during a rollback
	// If the job we are trying to rollback to is identical to the current job, we stop because the rollback will not succeed.
	if w.j.SpecChanged(rollbackJob) {
		desc = structs.DeploymentStatusDescriptionRollback(desc, rollbackJob.Version)
	} else {
		desc = structs.DeploymentStatusDescriptionRollbackNoop(desc, rollbackJob.Version)
		rollbackJob = nil
	}
	return rollbackJob, desc
}

func (w *deploymentWatcher) PromoteDeployment(
	req *structs.DeploymentPromoteRequest,
	resp *structs.DeploymentUpdateResponse) error {

	// Create the request
	areq := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: *req,
		Eval:                     w.getEval(),
	}

	index, err := w.upsertDeploymentPromotion(areq)
	if err != nil {
		return err
	}

	// Build the response
	resp.EvalID = areq.Eval.ID
	resp.EvalCreateIndex = index
	resp.DeploymentModifyIndex = index
	resp.Index = index
	return nil
}

// autoPromoteDeployment creates a synthetic promotion request, and upserts it for processing
func (w *deploymentWatcher) autoPromoteDeployment(allocs []*structs.AllocListStub) error {
	d := w.getDeployment()
	if !d.HasPlacedCanaries() || !d.RequiresPromotion() {
		return nil
	}

	// AutoPromote iff every task group with canaries is marked auto_promote and is healthy. The whole
	// job version has been incremented, so we promote together. See also AutoRevert
	for _, dstate := range d.TaskGroups {

		// skip auto promote canary validation if the task group has no canaries
		// to prevent auto promote hanging on mixed canary/non-canary taskgroup deploys
		if dstate.DesiredCanaries < 1 {
			continue
		}

		if !dstate.AutoPromote || len(dstate.PlacedCanaries) < dstate.DesiredCanaries {
			return nil
		}

		healthyCanaries := 0
		// Find the health status of each canary
		for _, c := range dstate.PlacedCanaries {
			for _, a := range allocs {
				if c == a.ID && a.DeploymentStatus.IsHealthy() {
					healthyCanaries += 1
				}
			}
		}
		if healthyCanaries != dstate.DesiredCanaries {
			return nil
		}
	}

	// Send the request
	_, err := w.upsertDeploymentPromotion(&structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{DeploymentID: d.GetID(), All: true},
		Eval:                     w.getEval(),
	})
	return err
}

func (w *deploymentWatcher) PauseDeployment(
	req *structs.DeploymentPauseRequest,
	resp *structs.DeploymentUpdateResponse) error {
	// Determine the status we should transition to and if we need to create an
	// evaluation
	status, desc := structs.DeploymentStatusPaused, structs.DeploymentStatusDescriptionPaused
	var eval *structs.Evaluation
	evalID := ""
	if !req.Pause {
		status, desc = structs.DeploymentStatusRunning, structs.DeploymentStatusDescriptionRunning
		eval = w.getEval()
		evalID = eval.ID
	}
	update := w.getDeploymentStatusUpdate(status, desc)

	// Commit the change
	i, err := w.upsertDeploymentStatusUpdate(update, eval, nil)
	if err != nil {
		return err
	}

	// Build the response
	if evalID != "" {
		resp.EvalID = evalID
		resp.EvalCreateIndex = i
	}
	resp.DeploymentModifyIndex = i
	resp.Index = i
	return nil
}

func (w *deploymentWatcher) FailDeployment(
	req *structs.DeploymentFailRequest,
	resp *structs.DeploymentUpdateResponse) error {

	status, desc := structs.DeploymentStatusFailed, structs.DeploymentStatusDescriptionFailedByUser

	// Determine if we should rollback
	rollback := false
	for _, dstate := range w.getDeployment().TaskGroups {
		if dstate.AutoRevert {
			rollback = true
			break
		}
	}

	var rollbackJob *structs.Job
	if rollback {
		var err error
		rollbackJob, err = w.latestStableJob()
		if err != nil {
			return err
		}

		if rollbackJob != nil {
			rollbackJob, desc = w.handleRollbackValidity(rollbackJob, desc)
		} else {
			desc = structs.DeploymentStatusDescriptionNoRollbackTarget(desc)
		}
	}

	// Commit the change
	update := w.getDeploymentStatusUpdate(status, desc)
	eval := w.getEval()
	i, err := w.upsertDeploymentStatusUpdate(update, eval, rollbackJob)
	if err != nil {
		return err
	}

	// Build the response
	resp.EvalID = eval.ID
	resp.EvalCreateIndex = i
	resp.DeploymentModifyIndex = i
	resp.Index = i
	if rollbackJob != nil {
		resp.RevertedJobVersion = pointer.Of(rollbackJob.Version)
	}
	return nil
}

// StopWatch stops watching the deployment. This should be called whenever a
// deployment is completed or the watcher is no longer needed.
func (w *deploymentWatcher) StopWatch() {
	w.exitFn()
}

// watch is the long running watcher that watches for both allocation and
// deployment changes. Its function is to create evaluations to trigger the
// scheduler when more progress can be made, to fail the deployment if it has
// failed and potentially rolling back the job. Progress can be made when an
// allocation transitions to healthy, so we create an eval.
func (w *deploymentWatcher) watch() {
	// Get the deadline. This is likely a zero time to begin with but we need to
	// handle the case that the deployment has already progressed and we are now
	// just starting to watch it. This must likely would occur if there was a
	// leader transition and we are now starting our watcher.
	currentDeadline := w.getDeploymentProgressCutoff(w.getDeployment())
	var deadlineTimer *time.Timer
	if currentDeadline.IsZero() {
		deadlineTimer = time.NewTimer(0)
		if !deadlineTimer.Stop() {
			<-deadlineTimer.C
		}
	} else {
		deadlineTimer = time.NewTimer(time.Until(currentDeadline))
	}

	allocIndex := uint64(1)
	allocsCh := w.getAllocsCh(allocIndex)
	var updates *allocUpdates

	rollback, deadlineHit := false, false

FAIL:
	for {
		select {
		case <-w.ctx.Done():
			// This is the successful case, and we stop the loop
			return
		case <-deadlineTimer.C:
			// We have hit the progress deadline, so fail the deployment
			// unless we're waiting for manual promotion. We need to determine
			// whether we should roll back the job by inspecting which allocs
			// as part of the deployment are healthy and which aren't. The
			// deadlineHit flag is never reset, so even in the case of a
			// manual promotion, we'll describe any failure as a progress
			// deadline failure at this point.
			deadlineHit = true
			fail, rback, err := w.shouldFail()
			if err != nil {
				w.logger.Error("failed to determine whether to rollback job", "error", err)
			}
			if !fail {
				w.logger.Debug("skipping deadline")
				continue
			}

			w.logger.Debug("deadline hit", "rollback", rback)
			rollback = rback
			err = w.nextRegion(structs.DeploymentStatusFailed)
			if err != nil {
				w.logger.Error("multiregion deployment error", "error", err)
			}
			break FAIL
		case <-w.deploymentUpdateCh:
			// Get the updated deployment and check if we should change the
			// deadline timer
			next := w.getDeploymentProgressCutoff(w.getDeployment())
			if !next.Equal(currentDeadline) {
				prevDeadlineZero := currentDeadline.IsZero()
				currentDeadline = next
				// The most recent deadline can be zero if no allocs were created for this deployment.
				// The deadline timer would have already been stopped once in that case. To prevent
				// deadlocking on the already stopped deadline timer, we only drain the channel if
				// the previous deadline was not zero.
				if !prevDeadlineZero && !deadlineTimer.Stop() {
					select {
					case <-deadlineTimer.C:
					default:
					}
				}

				// If the next deadline is zero, we should not reset the timer
				// as we aren't tracking towards a progress deadline yet. This
				// can happen if you have multiple task groups with progress
				// deadlines and one of the task groups hasn't made any
				// placements. As soon as the other task group finishes its
				// rollout, the next progress deadline becomes zero, so we want
				// to avoid resetting, causing a deployment failure.
				if !next.IsZero() {
					deadlineTimer.Reset(time.Until(next))
					w.logger.Trace("resetting deadline")
				}
			}

			err := w.nextRegion(w.getStatus())
			if err != nil {
				break FAIL
			}

		case updates = <-allocsCh:
			if err := updates.err; err != nil {
				if err == context.Canceled || w.ctx.Err() == context.Canceled {
					return
				}

				w.logger.Error("failed to retrieve allocations", "error", err)
				return
			}
			allocIndex = updates.index

			// We have allocation changes for this deployment so determine the
			// steps to take.
			res, err := w.handleAllocUpdate(updates.allocs)
			if err != nil {
				if err == context.Canceled || w.ctx.Err() == context.Canceled {
					return
				}

				w.logger.Error("failed handling allocation updates", "error", err)
				return
			}

			// The deployment has failed, so break out of the watch loop and
			// handle the failure
			if res.failDeployment {
				rollback = res.rollback
				err := w.nextRegion(structs.DeploymentStatusFailed)
				if err != nil {
					w.logger.Error("multiregion deployment error", "error", err)
				}
				break FAIL
			}

			// If permitted, automatically promote this canary deployment
			err = w.autoPromoteDeployment(updates.allocs)
			if err != nil {
				w.logger.Error("failed to auto promote deployment", "error", err)
			}

			// Create an eval to push the deployment along
			if res.createEval || len(res.allowReplacements) != 0 {
				w.createBatchedUpdate(res.allowReplacements, allocIndex)
			}

			// only start a new blocking query if we haven't returned early
			allocsCh = w.getAllocsCh(allocIndex)
		}
	}

	// Change the deployments status to failed
	desc := structs.DeploymentStatusDescriptionFailedAllocations
	if deadlineHit {
		desc = structs.DeploymentStatusDescriptionProgressDeadline
	}

	// Rollback to the old job if necessary
	var j *structs.Job
	if rollback {
		var err error
		j, err = w.latestStableJob()
		if err != nil {
			w.logger.Error("failed to lookup latest stable job", "error", err)
		}

		// Description should include that the job is being rolled back to
		// version N
		if j != nil {
			j, desc = w.handleRollbackValidity(j, desc)
		} else {
			desc = structs.DeploymentStatusDescriptionNoRollbackTarget(desc)
		}
	}

	// Update the status of the deployment to failed and create an evaluation.
	e := w.getEval()
	u := w.getDeploymentStatusUpdate(structs.DeploymentStatusFailed, desc)
	if _, err := w.upsertDeploymentStatusUpdate(u, e, j); err != nil {
		w.logger.Error("failed to update deployment status", "error", err)
	}
}

// allocUpdateResult is used to return the desired actions given the newest set
// of allocations for the deployment.
type allocUpdateResult struct {
	createEval        bool
	failDeployment    bool
	rollback          bool
	allowReplacements []string
}

// handleAllocUpdate is used to compute the set of actions to take based on the
// updated allocations for the deployment.
func (w *deploymentWatcher) handleAllocUpdate(allocs []*structs.AllocListStub) (allocUpdateResult, error) {
	var res allocUpdateResult

	// Get the latest evaluation index
	latestEval, err := w.jobEvalStatus()
	if err != nil {
		if err == context.Canceled || w.ctx.Err() == context.Canceled {
			return res, err
		}

		return res, fmt.Errorf("failed to determine last evaluation index for job %q: %v", w.j.ID, err)
	}

	deployment := w.getDeployment()
	for _, alloc := range allocs {
		dstate, ok := deployment.TaskGroups[alloc.TaskGroup]
		if !ok {
			continue
		}

		// Check if we can already fail the deployment
		failDeployment := w.shouldFailEarly(deployment, alloc, dstate)

		// Check if the allocation has failed and we need to mark it for allow
		// replacements
		if alloc.DeploymentStatus.IsUnhealthy() && !failDeployment &&
			deployment.Active() && !alloc.DesiredTransition.ShouldReschedule() {
			res.allowReplacements = append(res.allowReplacements, alloc.ID)
			continue
		}

		// We need to create an eval so the job can progress.
		if alloc.DeploymentStatus.IsHealthy() && alloc.DeploymentStatus.ModifyIndex > latestEval {
			res.createEval = true
		}

		if failDeployment {
			// Check if the group has autorevert set
			if dstate.AutoRevert {
				res.rollback = true
			}

			res.failDeployment = true
		}

		// All conditions have been hit so we can break
		if res.createEval && res.failDeployment && res.rollback {
			break
		}
	}

	return res, nil
}

// shouldFail returns whether the job should be failed and whether it should
// rolled back to an earlier stable version by examining the allocations in the
// deployment.
func (w *deploymentWatcher) shouldFail() (fail, rollback bool, err error) {
	snap, err := w.state.Snapshot()
	if err != nil {
		return false, false, err
	}

	d, err := snap.DeploymentByID(nil, w.deploymentID)
	if err != nil {
		return false, false, err
	}
	if d == nil {
		// The deployment wasn't in the state store, possibly due to a system gc
		return false, false, fmt.Errorf("deployment id not found: %q", w.deploymentID)
	}

	fail = false
	for tg, dstate := range d.TaskGroups {
		// If we are in a canary state we fail if there aren't enough healthy
		// allocs to satisfy DesiredCanaries
		if dstate.DesiredCanaries > 0 && !dstate.Promoted {
			if dstate.HealthyAllocs >= dstate.DesiredCanaries {
				continue
			}
		} else if dstate.HealthyAllocs >= dstate.DesiredTotal {
			continue
		}

		// We have failed this TG
		fail = true

		// We don't need to autorevert this group
		upd := w.j.LookupTaskGroup(tg).Update
		if upd == nil || !upd.AutoRevert {
			continue
		}

		// Unhealthy allocs and we need to autorevert
		return fail, true, nil
	}

	return fail, false, nil
}

func (w *deploymentWatcher) shouldFailEarly(deployment *structs.Deployment, alloc *structs.AllocListStub, dstate *structs.DeploymentState) bool {
	if !alloc.DeploymentStatus.IsUnhealthy() {
		return false
	}

	// Fail on the first unhealthy allocation if no progress deadline is specified.
	if dstate.ProgressDeadline == 0 {
		w.logger.Debug("failing deployment because an allocation failed and the deployment is not progress based", "alloc", alloc.ID)
		return true
	}

	if deployment.Active() {
		reschedulePolicy := w.j.LookupTaskGroup(alloc.TaskGroup).ReschedulePolicy
		isRescheduleEligible := alloc.RescheduleEligible(reschedulePolicy, time.Now())
		if !isRescheduleEligible {
			// We have run out of reschedule attempts: do not wait for the progress deadline to expire because
			// we know that we will not be able to try to get another allocation healthy
			w.logger.Debug("failing deployment because an allocation has failed and the task group has run out of reschedule attempts", "alloc", alloc.ID)
			return true
		}
	}

	return false
}

// getDeploymentProgressCutoff returns the progress cutoff for the given
// deployment
func (w *deploymentWatcher) getDeploymentProgressCutoff(d *structs.Deployment) time.Time {
	var next time.Time
	doneTGs := w.doneGroups(d)
	for name, dstate := range d.TaskGroups {
		// This task group is done so we don't have to concern ourselves with
		// its progress deadline.
		if done, ok := doneTGs[name]; ok && done {
			continue
		}

		if dstate.RequireProgressBy.IsZero() {
			continue
		}

		if next.IsZero() || dstate.RequireProgressBy.Before(next) {
			next = dstate.RequireProgressBy
		}
	}
	return next
}

// doneGroups returns a map of task group to whether the deployment appears to
// be done for the group. A true value doesn't mean no more action will be taken
// in the life time of the deployment because there could always be node
// failures, or rescheduling events.
func (w *deploymentWatcher) doneGroups(d *structs.Deployment) map[string]bool {
	if d == nil {
		return nil
	}

	// Collect the allocations by the task group
	snap, err := w.state.Snapshot()
	if err != nil {
		return nil
	}

	allocs, err := snap.AllocsByDeployment(nil, d.ID)
	if err != nil {
		return nil
	}

	// Go through the allocs and count up how many healthy allocs we have
	healthy := make(map[string]int, len(d.TaskGroups))
	for _, a := range allocs {
		if a.TerminalStatus() || !a.DeploymentStatus.IsHealthy() {
			continue
		}
		healthy[a.TaskGroup]++
	}

	// Go through each group and check if it done
	groups := make(map[string]bool, len(d.TaskGroups))
	for name, dstate := range d.TaskGroups {
		// Requires promotion
		if dstate.DesiredCanaries != 0 && !dstate.Promoted {
			groups[name] = false
			continue
		}

		// Check we have enough healthy currently running allocations
		groups[name] = healthy[name] >= dstate.DesiredTotal
	}

	return groups
}

// latestStableJob returns the latest stable job. It may be nil if none exist
func (w *deploymentWatcher) latestStableJob() (*structs.Job, error) {
	snap, err := w.state.Snapshot()
	if err != nil {
		return nil, err
	}

	versions, err := snap.JobVersionsByID(nil, w.j.Namespace, w.j.ID)
	if err != nil {
		return nil, err
	}

	var stable *structs.Job
	for _, job := range versions {
		if job.Stable {
			stable = job
			break
		}
	}

	return stable, nil
}

// createBatchedUpdate creates an eval for the given index as well as updating
// the given allocations to allow them to reschedule.
func (w *deploymentWatcher) createBatchedUpdate(allowReplacements []string, forIndex uint64) {
	w.l.Lock()
	defer w.l.Unlock()

	// Store the allocations that can be replaced
	for _, allocID := range allowReplacements {
		if w.outstandingAllowReplacements == nil {
			w.outstandingAllowReplacements = make(map[string]*structs.DesiredTransition, len(allowReplacements))
		}
		w.outstandingAllowReplacements[allocID] = allowRescheduleTransition
	}

	if w.outstandingBatch || (forIndex < w.latestEval && len(allowReplacements) == 0) {
		return
	}

	w.outstandingBatch = true

	time.AfterFunc(perJobEvalBatchPeriod, func() {
		// If the timer has been created and then we shutdown, we need to no-op
		// the evaluation creation.
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		w.l.Lock()
		replacements := w.outstandingAllowReplacements
		w.outstandingAllowReplacements = nil
		w.outstandingBatch = false
		w.l.Unlock()

		// Create the eval
		if _, err := w.createUpdate(replacements, w.getEval()); err != nil {
			w.logger.Error("failed to create evaluation for deployment", "deployment_id", w.deploymentID, "error", err)
		}
	})
}

// getEval returns an evaluation suitable for the deployment
func (w *deploymentWatcher) getEval() *structs.Evaluation {
	now := time.Now().UTC().UnixNano()

	// During a server upgrade it's possible we end up with deployments created
	// on the previous version that are then "watched" on a leader that's on
	// the new version. This would result in an eval with its priority set to
	// zero which would be bad. This therefore protects against that.
	w.l.Lock()
	priority := w.d.EvalPriority
	if priority == 0 {
		priority = w.j.Priority
	}
	w.l.Unlock()

	return &structs.Evaluation{
		ID:           uuid.Generate(),
		Namespace:    w.j.Namespace,
		Priority:     priority,
		Type:         w.j.Type,
		TriggeredBy:  structs.EvalTriggerDeploymentWatcher,
		JobID:        w.j.ID,
		DeploymentID: w.deploymentID,
		Status:       structs.EvalStatusPending,
		CreateTime:   now,
		ModifyTime:   now,
	}
}

// getDeploymentStatusUpdate returns a deployment status update
func (w *deploymentWatcher) getDeploymentStatusUpdate(status, desc string) *structs.DeploymentStatusUpdate {
	return &structs.DeploymentStatusUpdate{
		DeploymentID:      w.deploymentID,
		Status:            status,
		StatusDescription: desc,
	}
}

// getStatus returns the current status of the deployment
func (w *deploymentWatcher) getStatus() string {
	w.l.RLock()
	defer w.l.RUnlock()
	return w.d.Status
}

type allocUpdates struct {
	allocs []*structs.AllocListStub
	index  uint64
	err    error
}

// getAllocsCh creates a channel and starts a goroutine that
// 1. parks a blocking query for allocations on the state
// 2. reads those and drops them on the channel
// This query runs once here, but watch calls it in a loop
func (w *deploymentWatcher) getAllocsCh(index uint64) <-chan *allocUpdates {
	out := make(chan *allocUpdates, 1)
	go func() {
		allocs, index, err := w.getAllocs(index)
		out <- &allocUpdates{
			allocs: allocs,
			index:  index,
			err:    err,
		}
	}()

	return out
}

// getAllocs retrieves the allocations that are part of the deployment blocking
// at the given index.
func (w *deploymentWatcher) getAllocs(index uint64) ([]*structs.AllocListStub, uint64, error) {
	resp, index, err := w.state.BlockingQuery(w.getAllocsImpl, index, w.ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := w.ctx.Err(); err != nil {
		return nil, 0, err
	}

	return resp.([]*structs.AllocListStub), index, nil
}

// getDeploysImpl retrieves all deployments from the passed state store.
func (w *deploymentWatcher) getAllocsImpl(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	if err := w.queryLimiter.Wait(w.ctx); err != nil {
		return nil, 0, err
	}

	// Capture all the allocations
	allocs, err := state.AllocsByDeployment(ws, w.deploymentID)
	if err != nil {
		return nil, 0, err
	}

	maxIndex := uint64(0)
	stubs := make([]*structs.AllocListStub, 0, len(allocs))
	for _, alloc := range allocs {
		stubs = append(stubs, alloc.Stub(nil))

		if maxIndex < alloc.ModifyIndex {
			maxIndex = alloc.ModifyIndex
		}
	}

	// Use the last index that affected the allocs table
	if len(stubs) == 0 {
		index, err := state.Index("allocs")
		if err != nil {
			return nil, index, err
		}
		maxIndex = index
	}

	return stubs, maxIndex, nil
}

// jobEvalStatus returns the latest eval index for a job. The index is used to
// determine if an allocation update requires an evaluation to be triggered.
func (w *deploymentWatcher) jobEvalStatus() (latestIndex uint64, err error) {
	if err := w.queryLimiter.Wait(w.ctx); err != nil {
		return 0, err
	}

	snap, err := w.state.Snapshot()
	if err != nil {
		return 0, err
	}

	evals, err := snap.EvalsByJob(nil, w.j.Namespace, w.j.ID)
	if err != nil {
		return 0, err
	}

	// If there are no evals for the job, return zero, since we want any
	// allocation change to trigger an evaluation.
	if len(evals) == 0 {
		return 0, nil
	}

	var max uint64
	for _, eval := range evals {
		// A cancelled eval never impacts what the scheduler has saw, so do not
		// use it's indexes.
		if eval.Status == structs.EvalStatusCancelled {
			continue
		}

		// Prefer using the snapshot index. Otherwise use the create index
		if eval.SnapshotIndex != 0 && max < eval.SnapshotIndex {
			max = eval.SnapshotIndex
		} else if max < eval.CreateIndex {
			max = eval.CreateIndex
		}
	}

	return max, nil
}
