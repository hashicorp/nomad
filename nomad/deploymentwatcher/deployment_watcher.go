package deploymentwatcher

import (
	"context"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// perJobEvalBatchPeriod is the batching length before creating an evaluation to
	// trigger the scheduler when allocations are marked as healthy.
	perJobEvalBatchPeriod = 1 * time.Second
)

// deploymentTriggers are the set of functions required to trigger changes on
// behalf of a deployment
type deploymentTriggers interface {
	// createEvaluation is used to create an evaluation.
	createEvaluation(eval *structs.Evaluation) (uint64, error)

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
// scheduler when allocation health transistions.
type deploymentWatcher struct {
	// queryLimiter is used to limit the rate of blocking queries
	queryLimiter *rate.Limiter

	// deploymentTriggers holds the methods required to trigger changes on behalf of the
	// deployment
	deploymentTriggers

	// state is the state that is watched for state changes.
	state *state.StateStore

	// d is the deployment being watched
	d *structs.Deployment

	// j is the job the deployment is for
	j *structs.Job

	// outstandingBatch marks whether an outstanding function exists to create
	// the evaluation. Access should be done through the lock
	outstandingBatch bool

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
			group, ok := w.d.TaskGroups[alloc.TaskGroup]
			if !ok || !group.AutoRevert {
				continue
			}

			var err error
			j, err = w.latestStableJob()
			if err != nil {
				return err
			}

			if j != nil {
				desc = structs.DeploymentStatusDescriptionRollback(desc, j.Version)
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
		Eval:             w.getEval(),
		DeploymentUpdate: u,
		Job:              j,
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
	for _, state := range w.d.TaskGroups {
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
			desc = structs.DeploymentStatusDescriptionRollback(desc, rollbackJob.Version)
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

// watch is the long running watcher that takes actions upon allocation changes
func (w *deploymentWatcher) watch() {
	allocIndex := uint64(1)
	for {
		// Block getting all allocations that are part of the deployment using
		// the last evaluation index. This will have us block waiting for
		// something to change past what the scheduler has evaluated.
		allocs, index, err := w.getAllocs(allocIndex)
		if err != nil {
			if err == context.Canceled || w.ctx.Err() == context.Canceled {
				return
			}

			w.logger.Printf("[ERR] nomad.deployment_watcher: failed to retrieve allocations for deployment %q: %v", w.d.ID, err)
			return
		}
		allocIndex = index

		// Get the latest evaluation index
		latestEval, err := w.latestEvalIndex()
		if err != nil {
			if err == context.Canceled || w.ctx.Err() == context.Canceled {
				return
			}

			w.logger.Printf("[ERR] nomad.deployment_watcher: failed to determine last evaluation index for job %q: %v", w.d.JobID, err)
			return
		}

		// Create an evaluation trigger if there is any allocation whose
		// deployment status has been updated past the latest eval index.
		createEval, failDeployment, rollback := false, false, false
		for _, alloc := range allocs {
			if alloc.DeploymentStatus == nil || alloc.DeploymentStatus.ModifyIndex <= latestEval {
				continue
			}

			// We need to create an eval
			createEval = true

			if alloc.DeploymentStatus.IsUnhealthy() {
				// Check if the group has autorevert set
				group, ok := w.d.TaskGroups[alloc.TaskGroup]
				if ok && group.AutoRevert {
					rollback = true
				}

				// Since we have an unhealthy allocation, fail the deployment
				failDeployment = true
			}

			// All conditions have been hit so we can break
			if createEval && failDeployment && rollback {
				break
			}
		}

		// Change the deployments status to failed
		if failDeployment {
			// Default description
			desc := structs.DeploymentStatusDescriptionFailedAllocations

			// Rollback to the old job if necessary
			var j *structs.Job
			if rollback {
				var err error
				j, err = w.latestStableJob()
				if err != nil {
					w.logger.Printf("[ERR] nomad.deployment_watcher: failed to lookup latest stable job for %q: %v", w.d.JobID, err)
				}

				// Description should include that the job is being rolled back to
				// version N
				if j != nil {
					desc = structs.DeploymentStatusDescriptionRollback(desc, j.Version)
				} else {
					desc = structs.DeploymentStatusDescriptionNoRollbackTarget(desc)
				}
			}

			// Update the status of the deployment to failed and create an
			// evaluation.
			e := w.getEval()
			u := w.getDeploymentStatusUpdate(structs.DeploymentStatusFailed, desc)
			if index, err := w.upsertDeploymentStatusUpdate(u, e, j); err != nil {
				w.logger.Printf("[ERR] nomad.deployment_watcher: failed to update deployment %q status: %v", w.d.ID, err)
			} else {
				w.setLatestEval(index)
			}
		} else if createEval {
			// Create an eval to push the deployment along
			w.createEvalBatched(index)
		}
	}
}

// latestStableJob returns the latest stable job. It may be nil if none exist
func (w *deploymentWatcher) latestStableJob() (*structs.Job, error) {
	snap, err := w.state.Snapshot()
	if err != nil {
		return nil, err
	}

	versions, err := snap.JobVersionsByID(nil, w.d.Namespace, w.d.JobID)
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

// createEvalBatched creates an eval but batches calls together
func (w *deploymentWatcher) createEvalBatched(forIndex uint64) {
	w.l.Lock()
	defer w.l.Unlock()

	if w.outstandingBatch || forIndex < w.latestEval {
		return
	}

	w.outstandingBatch = true

	time.AfterFunc(perJobEvalBatchPeriod, func() {
		// Create the eval
		evalCreateIndex, err := w.createEvaluation(w.getEval())
		if err != nil {
			w.logger.Printf("[ERR] nomad.deployment_watcher: failed to create evaluation for deployment %q: %v", w.d.ID, err)
		} else {
			w.setLatestEval(evalCreateIndex)
		}

		w.l.Lock()
		w.outstandingBatch = false
		w.l.Unlock()

	})
}

// getEval returns an evaluation suitable for the deployment
func (w *deploymentWatcher) getEval() *structs.Evaluation {
	return &structs.Evaluation{
		ID:           structs.GenerateUUID(),
		Namespace:    w.j.Namespace,
		Priority:     w.j.Priority,
		Type:         w.j.Type,
		TriggeredBy:  structs.EvalTriggerDeploymentWatcher,
		JobID:        w.j.ID,
		DeploymentID: w.d.ID,
		Status:       structs.EvalStatusPending,
	}
}

// getDeploymentStatusUpdate returns a deployment status update
func (w *deploymentWatcher) getDeploymentStatusUpdate(status, desc string) *structs.DeploymentStatusUpdate {
	return &structs.DeploymentStatusUpdate{
		DeploymentID:      w.d.ID,
		Status:            status,
		StatusDescription: desc,
	}
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
	allocs, err := state.AllocsByDeployment(ws, w.d.ID)
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

	evals, err := snap.EvalsByJob(nil, w.d.Namespace, w.d.JobID)
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
