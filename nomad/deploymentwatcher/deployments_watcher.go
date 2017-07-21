package deploymentwatcher

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// LimitStateQueriesPerSecond is the number of state queries allowed per
	// second
	LimitStateQueriesPerSecond = 100.0

	// CrossDeploymentEvalBatchDuration is the duration in which evaluations are
	// batched across all deployment watchers before commiting to Raft.
	CrossDeploymentEvalBatchDuration = 250 * time.Millisecond
)

var (
	// notEnabled is the error returned when the deployment watcher is not
	// enabled
	notEnabled = fmt.Errorf("deployment watcher not enabled")
)

// DeploymentRaftEndpoints exposes the deployment watcher to a set of functions
// to apply data transforms via Raft.
type DeploymentRaftEndpoints interface {
	// UpsertEvals is used to upsert a set of evaluations
	UpsertEvals([]*structs.Evaluation) (uint64, error)

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
}

// DeploymentStateWatchers are the set of functions required to watch objects on
// behalf of a deployment
type DeploymentStateWatchers interface {
	// Evaluations returns the set of evaluations for the given job
	Evaluations(args *structs.JobSpecificRequest, reply *structs.JobEvaluationsResponse) error

	// Allocations returns the set of allocations that are part of the
	// deployment.
	Allocations(args *structs.DeploymentSpecificRequest, reply *structs.AllocListResponse) error

	// List is used to list all the deployments in the system
	List(args *structs.DeploymentListRequest, reply *structs.DeploymentListResponse) error

	// GetDeployment is used to lookup a particular deployment.
	GetDeployment(args *structs.DeploymentSpecificRequest, reply *structs.SingleDeploymentResponse) error

	// GetJobVersions is used to lookup the versions of a job. This is used when
	// rolling back to find the latest stable job
	GetJobVersions(args *structs.JobVersionsRequest, reply *structs.JobVersionsResponse) error

	// GetJob is used to lookup a particular job.
	GetJob(args *structs.JobSpecificRequest, reply *structs.SingleJobResponse) error
}

// Watcher is used to watch deployments and their allocations created
// by the scheduler and trigger the scheduler when allocation health
// transistions.
type Watcher struct {
	enabled bool
	logger  *log.Logger

	// queryLimiter is used to limit the rate of blocking queries
	queryLimiter *rate.Limiter

	// evalBatchDuration is the duration to batch eval creation across all
	// deployment watchers
	evalBatchDuration time.Duration

	// raft contains the set of Raft endpoints that can be used by the
	// deployments watcher
	raft DeploymentRaftEndpoints

	// stateWatchers is the set of functions required to watch a deployment for
	// state changes
	stateWatchers DeploymentStateWatchers

	// watchers is the set of active watchers, one per deployment
	watchers map[string]*deploymentWatcher

	// evalBatcher is used to batch the creation of evaluations
	evalBatcher *EvalBatcher

	// ctx and exitFn are used to cancel the watcher
	ctx    context.Context
	exitFn context.CancelFunc

	l sync.RWMutex
}

// NewDeploymentsWatcher returns a deployments watcher that is used to watch
// deployments and trigger the scheduler as needed.
func NewDeploymentsWatcher(logger *log.Logger, watchers DeploymentStateWatchers,
	raft DeploymentRaftEndpoints, stateQueriesPerSecond float64,
	evalBatchDuration time.Duration) *Watcher {

	return &Watcher{
		stateWatchers:     watchers,
		raft:              raft,
		queryLimiter:      rate.NewLimiter(rate.Limit(stateQueriesPerSecond), 100),
		evalBatchDuration: evalBatchDuration,
		logger:            logger,
	}
}

// SetEnabled is used to control if the watcher is enabled. The watcher
// should only be enabled on the active leader.
func (w *Watcher) SetEnabled(enabled bool) error {
	w.l.Lock()
	defer w.l.Unlock()

	wasEnabled := w.enabled
	w.enabled = enabled

	// Flush the state to create the necessary objects
	w.flush()

	// If we are starting now, launch the watch daemon
	if enabled && !wasEnabled {
		go w.watchDeployments(w.ctx)
	}

	return nil
}

// flush is used to clear the state of the watcher
func (w *Watcher) flush() {
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
	w.evalBatcher = NewEvalBatcher(w.evalBatchDuration, w.raft, w.ctx)
}

// watchDeployments is the long lived go-routine that watches for deployments to
// add and remove watchers on.
func (w *Watcher) watchDeployments(ctx context.Context) {
	dindex := uint64(1)
	for {
		// Block getting all deployments using the last deployment index.
		resp, err := w.getDeploys(ctx, dindex)
		if err != nil {
			if err == context.Canceled || ctx.Err() == context.Canceled {
				return
			}

			w.logger.Printf("[ERR] nomad.deployments_watcher: failed to retrieve deploylements: %v", err)
		}

		// Guard against npe
		if resp == nil {
			continue
		}

		// Ensure we are tracking the things we should and not tracking what we
		// shouldn't be
		for _, d := range resp.Deployments {
			if d.Active() {
				if err := w.add(d); err != nil {
					w.logger.Printf("[ERR] nomad.deployments_watcher: failed to track deployment %q: %v", d.ID, err)
				}
			} else {
				w.remove(d)
			}
		}

		// Update the latest index
		dindex = resp.Index
	}
}

// getDeploys retrieves all deployments blocking at the given index.
func (w *Watcher) getDeploys(ctx context.Context, index uint64) (*structs.DeploymentListResponse, error) {
	// Build the request
	args := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			MinQueryIndex: index,
		},
	}
	var resp structs.DeploymentListResponse

	for resp.Index <= index {
		if err := w.queryLimiter.Wait(ctx); err != nil {
			return nil, err
		}

		if err := w.stateWatchers.List(args, &resp); err != nil {
			return nil, err
		}
	}

	return &resp, nil
}

// add adds a deployment to the watch list
func (w *Watcher) add(d *structs.Deployment) error {
	w.l.Lock()
	defer w.l.Unlock()
	_, err := w.addLocked(d)
	return err
}

// addLocked adds a deployment to the watch list and should only be called when
// locked.
func (w *Watcher) addLocked(d *structs.Deployment) (*deploymentWatcher, error) {
	// Not enabled so no-op
	if !w.enabled {
		return nil, nil
	}

	if !d.Active() {
		return nil, fmt.Errorf("deployment %q is terminal", d.ID)
	}

	// Already watched so no-op
	if _, ok := w.watchers[d.ID]; ok {
		return nil, nil
	}

	// Get the job the deployment is referencing
	args := &structs.JobSpecificRequest{
		JobID: d.JobID,
	}
	var resp structs.SingleJobResponse
	if err := w.stateWatchers.GetJob(args, &resp); err != nil {
		return nil, err
	}
	if resp.Job == nil {
		return nil, fmt.Errorf("deployment %q references unknown job %q", d.ID, d.JobID)
	}

	watcher := newDeploymentWatcher(w.ctx, w.queryLimiter, w.logger, w.stateWatchers, d, resp.Job, w)
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
	// Build the request
	args := &structs.DeploymentSpecificRequest{DeploymentID: dID}
	var resp structs.SingleDeploymentResponse
	if err := w.stateWatchers.GetDeployment(args, &resp); err != nil {
		return nil, err
	}

	if resp.Deployment == nil {
		return nil, fmt.Errorf("unknown deployment %q", dID)
	}

	return w.addLocked(resp.Deployment)
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

// createEvaluation commits the given evaluation to Raft but batches the commit
// with other calls.
func (w *Watcher) createEvaluation(eval *structs.Evaluation) (uint64, error) {
	return w.evalBatcher.CreateEval(eval).Results()
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
