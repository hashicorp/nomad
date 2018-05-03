package deploymentwatcher

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
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
		Reschedule: helper.BoolToPtr(true),
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

	logger *log.Logger
	ctx    context.Context
	exitFn context.CancelFunc
	l      sync.RWMutex
}

// newDeploymentWatcher returns a deployment watcher that is used to watch
// deployments and trigger the scheduler as needed.
func newDeploymentWatcher(parent context.Context, queryLimiter *rate.Limiter,
	logger *log.Logger, state *state.StateStore, d *structs.Deployment,
	j *structs.Job, triggers deploymentTriggers) *deploymentWatcher {

	ctx, exitFn := context.WithCancel(parent)
	w := &deploymentWatcher{
		queryLimiter:       queryLimiter,
		deploymentID:       d.ID,
		deploymentUpdateCh: make(chan struct{}, 1),
		d:                  d,
		j:                  j,
		state:              state,
		deploymentTriggers: triggers,
		logger:             logger,
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
			group, ok := w.getDeployment().TaskGroups[alloc.TaskGroup]
			if !ok || !group.AutoRevert {
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
		resp.RevertedJobVersion = helper.Uint64ToPtr(j.Version)
	}
	w.setLatestEval(index)
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
		Eval: w.getEval(),
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
	w.setLatestEval(index)
	return nil
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
	w.setLatestEval(i)
	return nil
}

func (w *deploymentWatcher) FailDeployment(
	req *structs.DeploymentFailRequest,
	resp *structs.DeploymentUpdateResponse) error {

	status, desc := structs.DeploymentStatusFailed, structs.DeploymentStatusDescriptionFailedByUser

	// Determine if we should rollback
	rollback := false
	for _, state := range w.getDeployment().TaskGroups {
		if state.AutoRevert {
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
		resp.RevertedJobVersion = helper.Uint64ToPtr(rollbackJob.Version)
	}
	w.setLatestEval(i)
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
	currentDeadline := getDeploymentProgressCutoff(w.getDeployment())
	var deadlineTimer *time.Timer
	if currentDeadline.IsZero() {
		deadlineTimer = time.NewTimer(0)
		if !deadlineTimer.Stop() {
			<-deadlineTimer.C
		}
	} else {
		deadlineTimer = time.NewTimer(currentDeadline.Sub(time.Now()))
	}

	allocIndex := uint64(1)
	var updates *allocUpdates

	rollback, deadlineHit := false, false

FAIL:
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-deadlineTimer.C:
			// We have hit the progress deadline so fail the deployment. We need
			// to determine whether we should roll back the job by inspecting
			// which allocs as part of the deployment are healthy and which
			// aren't.
			deadlineHit = true
			fail, rback, err := w.shouldFail()
			if err != nil {
				w.logger.Printf("[ERR] nomad.deployment_watcher: failed to determine whether to rollback job for deployment %q: %v", w.deploymentID, err)
			}
			if !fail {
				w.logger.Printf("[DEBUG] nomad.deployment_watcher: skipping deadline for deployment %q", w.deploymentID)
				continue
			}

			w.logger.Printf("[DEBUG] nomad.deployment_watcher: deadline for deployment %q hit and rollback is %v", w.deploymentID, rback)
			rollback = rback
			break FAIL
		case <-w.deploymentUpdateCh:
			// Get the updated deployment and check if we should change the
			// deadline timer
			next := getDeploymentProgressCutoff(w.getDeployment())
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
				deadlineTimer.Reset(next.Sub(time.Now()))
			}

		case updates = <-w.getAllocsCh(allocIndex):
			if err := updates.err; err != nil {
				if err == context.Canceled || w.ctx.Err() == context.Canceled {
					return
				}

				w.logger.Printf("[ERR] nomad.deployment_watcher: failed to retrieve allocations for deployment %q: %v", w.deploymentID, err)
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

				w.logger.Printf("[ERR] nomad.deployment_watcher: failed handling allocation updates: %v", err)
				return
			}

			// The deployment has failed, so break out of the watch loop and
			// handle the failure
			if res.failDeployment {
				rollback = res.rollback
				break FAIL
			}

			// Create an eval to push the deployment along
			if res.createEval || len(res.allowReplacements) != 0 {
				w.createBatchedUpdate(res.allowReplacements, allocIndex)
			}
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
			w.logger.Printf("[ERR] nomad.deployment_watcher: failed to lookup latest stable job for %q: %v", w.j.ID, err)
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
	if index, err := w.upsertDeploymentStatusUpdate(u, e, j); err != nil {
		w.logger.Printf("[ERR] nomad.deployment_watcher: failed to update deployment %q status: %v", w.deploymentID, err)
	} else {
		w.setLatestEval(index)
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
	latestEval, err := w.latestEvalIndex()
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

		// Nothing to do for this allocation
		if alloc.DeploymentStatus == nil || alloc.DeploymentStatus.ModifyIndex <= latestEval {
			continue
		}

		// Determine if the update stanza for this group is progress based
		progressBased := dstate.ProgressDeadline != 0

		// We need to create an eval so the job can progress.
		if alloc.DeploymentStatus.IsHealthy() {
			res.createEval = true
		} else if progressBased && alloc.DeploymentStatus.IsUnhealthy() && deployment.Active() && !alloc.DesiredTransition.ShouldReschedule() {
			res.allowReplacements = append(res.allowReplacements, alloc.ID)
		}

		// If the group is using a progress deadline, we don't have to do anything.
		if progressBased {
			continue
		}

		// Fail on the first bad allocation
		if alloc.DeploymentStatus.IsUnhealthy() {
			// Check if the group has autorevert set
			if dstate.AutoRevert {
				res.rollback = true
			}

			// Since we have an unhealthy allocation, fail the deployment
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
	for tg, state := range d.TaskGroups {
		// If we are in a canary state we fail if there aren't enough healthy
		// allocs to satisfy DesiredCanaries
		if state.DesiredCanaries > 0 && !state.Promoted {
			if state.HealthyAllocs >= state.DesiredCanaries {
				continue
			}
		} else if state.HealthyAllocs >= state.DesiredTotal {
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
		return true, true, nil
	}

	return fail, false, nil
}

// getDeploymentProgressCutoff returns the progress cutoff for the given
// deployment
func getDeploymentProgressCutoff(d *structs.Deployment) time.Time {
	var next time.Time
	for _, state := range d.TaskGroups {
		if next.IsZero() || state.RequireProgressBy.Before(next) {
			next = state.RequireProgressBy
		}
	}
	return next
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
		if index, err := w.createUpdate(replacements, w.getEval()); err != nil {
			w.logger.Printf("[ERR] nomad.deployment_watcher: failed to create evaluation for deployment %q: %v", w.deploymentID, err)
		} else {
			w.setLatestEval(index)
		}
	})
}

// getEval returns an evaluation suitable for the deployment
func (w *deploymentWatcher) getEval() *structs.Evaluation {
	return &structs.Evaluation{
		ID:           uuid.Generate(),
		Namespace:    w.j.Namespace,
		Priority:     w.j.Priority,
		Type:         w.j.Type,
		TriggeredBy:  structs.EvalTriggerDeploymentWatcher,
		JobID:        w.j.ID,
		DeploymentID: w.deploymentID,
		Status:       structs.EvalStatusPending,
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

type allocUpdates struct {
	allocs []*structs.AllocListStub
	index  uint64
	err    error
}

// getAllocsCh retrieves the allocations that are part of the deployment blocking
// at the given index.
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

	stubs := make([]*structs.AllocListStub, 0, len(allocs))
	for _, alloc := range allocs {
		stubs = append(stubs, alloc.Stub())
	}

	// Use the last index that affected the jobs table
	index, err := state.Index("allocs")
	if err != nil {
		return nil, index, err
	}

	return stubs, index, nil
}

// latestEvalIndex returns the index of the last evaluation created for
// the job. The index is used to determine if an allocation update requires an
// evaluation to be triggered.
func (w *deploymentWatcher) latestEvalIndex() (uint64, error) {
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

	if len(evals) == 0 {
		idx, err := snap.Index("evals")
		if err != nil {
			w.setLatestEval(idx)
		}

		return idx, err
	}

	// Prefer using the snapshot index. Otherwise use the create index
	e := evals[0]
	if e.SnapshotIndex != 0 {
		w.setLatestEval(e.SnapshotIndex)
		return e.SnapshotIndex, nil
	}

	w.setLatestEval(e.CreateIndex)
	return e.CreateIndex, nil
}

// setLatestEval sets the given index as the latest eval unless the currently
// stored index is higher.
func (w *deploymentWatcher) setLatestEval(index uint64) {
	w.l.Lock()
	defer w.l.Unlock()
	if index > w.latestEval {
		w.latestEval = index
	}
}

// getLatestEval returns the latest eval index.
func (w *deploymentWatcher) getLatestEval() uint64 {
	w.l.Lock()
	defer w.l.Unlock()
	return w.latestEval
}
