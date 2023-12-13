// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// LimitStateQueriesPerSecond is the number of state queries allowed per
	// second
	LimitStateQueriesPerSecond = 100.0

	// CrossDeploymentUpdateBatchDuration is the duration in which allocation
	// desired transition and evaluation creation updates are batched across
	// all deployment watchers before committing to Raft.
	CrossDeploymentUpdateBatchDuration = 250 * time.Millisecond
)

var (
	// notEnabled is the error returned when the deployment watcher is not
	// enabled
	notEnabled = fmt.Errorf("deployment watcher not enabled")
)

// DeploymentRaftEndpoints exposes the deployment watcher to a set of functions
// to apply data transforms via Raft.
type DeploymentRaftEndpoints interface {
	// UpsertJob is used to upsert a job
	UpsertJob(job *structs.Job) (uint64, error)

	// UpdateDeploymentStatus is used to make a deployment status update
	// and potentially create an evaluation.
	UpdateDeploymentStatus(u *structs.DeploymentStatusUpdateRequest) (uint64, error)

	// UpdateDeploymentPromotion is used to promote canaries in a deployment
	UpdateDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error)

	// UpdateDeploymentAllocHealth is used to set the health of allocations in a
	// deployment
	UpdateDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error)

	// UpdateAllocDesiredTransition is used to update the desired transition
	// for allocations.
	UpdateAllocDesiredTransition(req *structs.AllocUpdateDesiredTransitionRequest) (uint64, error)
}

// Watcher is used to watch deployments and their allocations created
// by the scheduler and trigger the scheduler when allocation health
// transitions.
type Watcher struct {
	enabled bool
	logger  log.Logger

	// queryLimiter is used to limit the rate of blocking queries
	queryLimiter *rate.Limiter

	// updateBatchDuration is the duration to batch allocation desired
	// transition and eval creation across all deployment watchers
	updateBatchDuration time.Duration

	// raft contains the set of Raft endpoints that can be used by the
	// deployments watcher
	raft DeploymentRaftEndpoints

	// state is the state that is watched for state changes.
	state *state.StateStore

	// server interface for Deployment RPCs
	deploymentRPC DeploymentRPC

	// server interface for Job RPCs
	jobRPC JobRPC

	// watchers is the set of active watchers, one per deployment
	watchers map[string]*deploymentWatcher

	// allocUpdateBatcher is used to batch the creation of evaluations and
	// allocation desired transition updates
	allocUpdateBatcher *AllocUpdateBatcher

	// ctx and exitFn are used to cancel the watcher
	ctx    context.Context
	exitFn context.CancelFunc

	l sync.RWMutex
}

// NewDeploymentsWatcher returns a deployments watcher that is used to watch
// deployments and trigger the scheduler as needed.
func NewDeploymentsWatcher(logger log.Logger,
	raft DeploymentRaftEndpoints,
	deploymentRPC DeploymentRPC, jobRPC JobRPC,
	stateQueriesPerSecond float64,
	updateBatchDuration time.Duration,
) *Watcher {

	return &Watcher{
		raft:                raft,
		deploymentRPC:       deploymentRPC,
		jobRPC:              jobRPC,
		queryLimiter:        rate.NewLimiter(rate.Limit(stateQueriesPerSecond), 100),
		updateBatchDuration: updateBatchDuration,
		logger:              logger.Named("deployments_watcher"),
	}
}

// SetEnabled is used to control if the watcher is enabled. The watcher
// should only be enabled on the active leader. When being enabled the state is
// passed in as it is no longer valid once a leader election has taken place.
func (w *Watcher) SetEnabled(enabled bool, state *state.StateStore) {
	w.l.Lock()
	defer w.l.Unlock()

	wasEnabled := w.enabled
	w.enabled = enabled

	if state != nil {
		w.state = state
	}

	// Flush the state to create the necessary objects
	w.flush(enabled)

	// If we are starting now, launch the watch daemon
	if enabled && !wasEnabled {
		go w.watchDeployments(w.ctx)
	}
}

// flush is used to clear the state of the watcher
func (w *Watcher) flush(enabled bool) {
	// Stop all the watchers and clear it
	for _, watcher := range w.watchers {
		watcher.StopWatch()
	}

	// Kill everything associated with the watcher
	if w.exitFn != nil {
		w.exitFn()
	}

	w.watchers = make(map[string]*deploymentWatcher, 32)
	w.ctx, w.exitFn = context.WithCancel(context.Background())

	if enabled {
		w.allocUpdateBatcher = NewAllocUpdateBatcher(w.ctx, w.updateBatchDuration, w.raft)
	} else {
		w.allocUpdateBatcher = nil
	}
}

// watchDeployments is the long lived go-routine that watches for deployments to
// add and remove watchers on.
func (w *Watcher) watchDeployments(ctx context.Context) {
	dindex := uint64(1)
	for {
		// Block getting all deployments using the last deployment index.
		deployments, idx, err := w.getDeploys(ctx, dindex)
		if err != nil {
			if err == context.Canceled {
				return
			}

			w.logger.Error("failed to retrieve deployments", "error", err)
		}

		// Update the latest index
		dindex = idx

		// Ensure we are tracking the things we should and not tracking what we
		// shouldn't be
		for _, d := range deployments {
			if d.Active() {
				if err := w.add(d); err != nil {
					w.logger.Error("failed to track deployment", "deployment_id", d.ID, "error", err)
				}
			} else {
				w.remove(d)
			}
		}
	}
}

// getDeploys retrieves all deployments blocking at the given index.
func (w *Watcher) getDeploys(ctx context.Context, minIndex uint64) ([]*structs.Deployment, uint64, error) {
	// state can be updated concurrently
	w.l.Lock()
	stateStore := w.state
	w.l.Unlock()

	resp, index, err := stateStore.BlockingQuery(w.getDeploysImpl, minIndex, ctx)
	if err != nil {
		return nil, 0, err
	}

	return resp.([]*structs.Deployment), index, nil
}

// getDeploysImpl retrieves all deployments from the passed state store.
func (w *Watcher) getDeploysImpl(ws memdb.WatchSet, store *state.StateStore) (interface{}, uint64, error) {

	iter, err := store.Deployments(ws, state.SortDefault)
	if err != nil {
		return nil, 0, err
	}

	var deploys []*structs.Deployment
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		deploy := raw.(*structs.Deployment)
		deploys = append(deploys, deploy)
	}

	// Use the last index that affected the deployment table
	index, err := store.Index("deployment")
	if err != nil {
		return nil, 0, err
	}

	return deploys, index, nil
}

// add adds a deployment to the watch list
func (w *Watcher) add(d *structs.Deployment) error {
	w.l.Lock()
	defer w.l.Unlock()
	_, err := w.addLocked(d)
	return err
}

// addLocked adds a deployment to the watch list and should only be called when
// locked. Creating the deploymentWatcher starts a go routine to .watch() it
func (w *Watcher) addLocked(d *structs.Deployment) (*deploymentWatcher, error) {
	// Not enabled so no-op
	if !w.enabled {
		return nil, nil
	}

	if !d.Active() {
		return nil, fmt.Errorf("deployment %q is terminal", d.ID)
	}

	// Already watched so just update the deployment
	if w, ok := w.watchers[d.ID]; ok {
		w.updateDeployment(d)
		return nil, nil
	}

	// Get the job the deployment is referencing
	snap, err := w.state.Snapshot()
	if err != nil {
		return nil, err
	}

	job, err := snap.JobByID(nil, d.Namespace, d.JobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("deployment %q references unknown job %q", d.ID, d.JobID)
	}

	watcher := newDeploymentWatcher(w.ctx, w.queryLimiter, w.logger, w.state, d, job,
		w, w.deploymentRPC, w.jobRPC)
	w.watchers[d.ID] = watcher
	return watcher, nil
}

// remove stops watching a deployment. This can be because the deployment is
// complete or being deleted.
func (w *Watcher) remove(d *structs.Deployment) {
	w.l.Lock()
	defer w.l.Unlock()

	// Not enabled so no-op
	if !w.enabled {
		return
	}

	if watcher, ok := w.watchers[d.ID]; ok {
		watcher.StopWatch()
		delete(w.watchers, d.ID)
	}
}

// forceAdd is used to force a lookup of the given deployment object and create
// a watcher. If the deployment does not exist or is terminal an error is
// returned.
func (w *Watcher) forceAdd(dID string) (*deploymentWatcher, error) {
	snap, err := w.state.Snapshot()
	if err != nil {
		return nil, err
	}

	deployment, err := snap.DeploymentByID(nil, dID)
	if err != nil {
		return nil, err
	}

	if deployment == nil {
		return nil, fmt.Errorf("unknown deployment %q", dID)
	}

	return w.addLocked(deployment)
}

// getOrCreateWatcher returns the deployment watcher for the given deployment ID.
func (w *Watcher) getOrCreateWatcher(dID string) (*deploymentWatcher, error) {
	w.l.Lock()
	defer w.l.Unlock()

	// Not enabled so no-op
	if !w.enabled {
		return nil, notEnabled
	}

	watcher, ok := w.watchers[dID]
	if ok {
		return watcher, nil
	}

	return w.forceAdd(dID)
}

// SetAllocHealth is used to set the health of allocations for a deployment. If
// there are any unhealthy allocations, the deployment is updated to be failed.
// Otherwise the allocations are updated and an evaluation is created.
func (w *Watcher) SetAllocHealth(req *structs.DeploymentAllocHealthRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getOrCreateWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.SetAllocHealth(req, resp)
}

// PromoteDeployment is used to promote a deployment. If promote is false,
// deployment is marked as failed. Otherwise the deployment is updated and an
// evaluation is created.
func (w *Watcher) PromoteDeployment(req *structs.DeploymentPromoteRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getOrCreateWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.PromoteDeployment(req, resp)
}

// PauseDeployment is used to toggle the pause state on a deployment. If the
// deployment is being unpaused, an evaluation is created.
func (w *Watcher) PauseDeployment(req *structs.DeploymentPauseRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getOrCreateWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.PauseDeployment(req, resp)
}

// FailDeployment is used to fail the deployment.
func (w *Watcher) FailDeployment(req *structs.DeploymentFailRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getOrCreateWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.FailDeployment(req, resp)
}

// RunDeployment is used to run a pending multiregion deployment.  In
// single-region deployments, the pending state is unused.
func (w *Watcher) RunDeployment(req *structs.DeploymentRunRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getOrCreateWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.RunDeployment(req, resp)
}

// UnblockDeployment is used to unblock a multiregion deployment.  In
// single-region deployments, the blocked state is unused.
func (w *Watcher) UnblockDeployment(req *structs.DeploymentUnblockRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getOrCreateWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.UnblockDeployment(req, resp)
}

// CancelDeployment is used to cancel a multiregion deployment.  In
// single-region deployments, the deploymentwatcher has sole responsibility to
// cancel deployments so this RPC is never used.
func (w *Watcher) CancelDeployment(req *structs.DeploymentCancelRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getOrCreateWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.CancelDeployment(req, resp)
}

// createUpdate commits the given allocation desired transition and evaluation
// to Raft but batches the commit with other calls.
func (w *Watcher) createUpdate(allocs map[string]*structs.DesiredTransition, eval *structs.Evaluation) (uint64, error) {
	b := w.allocUpdateBatcher
	if b == nil {
		return 0, notEnabled
	}
	return b.CreateUpdate(allocs, eval).Results()
}

// upsertJob commits the given job to Raft
func (w *Watcher) upsertJob(job *structs.Job) (uint64, error) {
	return w.raft.UpsertJob(job)
}

// upsertDeploymentStatusUpdate commits the given deployment update and optional
// evaluation to Raft
func (w *Watcher) upsertDeploymentStatusUpdate(
	u *structs.DeploymentStatusUpdate,
	e *structs.Evaluation,
	j *structs.Job) (uint64, error) {
	return w.raft.UpdateDeploymentStatus(&structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: u,
		Eval:             e,
		Job:              j,
	})
}

// upsertDeploymentPromotion commits the given deployment promotion to Raft
func (w *Watcher) upsertDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error) {
	return w.raft.UpdateDeploymentPromotion(req)
}

// upsertDeploymentAllocHealth commits the given allocation health changes to
// Raft
func (w *Watcher) upsertDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error) {
	return w.raft.UpdateDeploymentAllocHealth(req)
}
