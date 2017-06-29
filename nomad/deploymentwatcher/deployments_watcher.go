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

// DeploymentRaftEndpoints exposes the deployment watcher to a set of functions
// to apply data transforms via Raft.
type DeploymentRaftEndpoints interface {
	// UpsertEvals is used to upsert a set of evaluations
	UpsertEvals([]*structs.Evaluation) (uint64, error)

	// UpsertJob is used to upsert a job
	UpsertJob(job *structs.Job) (uint64, error)

	// UpsertDeploymentStatusUpdate is used to upsert a deployment status update
	// and potentially create an evaluation.
	UpsertDeploymentStatusUpdate(u *structs.DeploymentStatusUpdateRequest) (uint64, error)

	// UpsertDeploymentPromotion is used to promote canaries in a deployment
	UpsertDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error)

	// UpsertDeploymentAllocHealth is used to set the health of allocations in a
	// deployment
	UpsertDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error)
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
	GetJobVersions(args *structs.JobSpecificRequest, reply *structs.JobVersionsResponse) error

	// GetJob is used to lookup a particular job.
	GetJob(args *structs.JobSpecificRequest, reply *structs.SingleJobResponse) error
}

const (
	// LimitStateQueriesPerSecond is the number of state queries allowed per
	// second
	LimitStateQueriesPerSecond = 15.0

	// EvalBatchDuration is the duration in which evaluations are batched before
	// commiting to Raft.
	EvalBatchDuration = 250 * time.Millisecond
)

var (
	// notEnabled is the error returned when the deployment watcher is not
	// enabled
	notEnabled = fmt.Errorf("deployment watcher not enabled")
)

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
func NewDeploymentsWatcher(logger *log.Logger, stateQueriesPerSecond float64,
	evalBatchDuration time.Duration) *Watcher {

	return &Watcher{
		queryLimiter:      rate.NewLimiter(rate.Limit(stateQueriesPerSecond), 100),
		evalBatchDuration: evalBatchDuration,
		logger:            logger,
	}
}

// SetStateWatchers sets the interface for accessing state watchers
func (w *Watcher) SetStateWatchers(watchers DeploymentStateWatchers) {
	w.l.Lock()
	defer w.l.Unlock()
	w.stateWatchers = watchers
}

// SetRaftEndpoints sets the interface for writing to Raft
func (w *Watcher) SetRaftEndpoints(raft DeploymentRaftEndpoints) {
	w.l.Lock()
	defer w.l.Unlock()
	w.raft = raft
}

// SetEnabled is used to control if the watcher is enabled. The watcher
// should only be enabled on the active leader.
func (w *Watcher) SetEnabled(enabled bool) error {
	w.l.Lock()
	// Ensure our state is correct
	if w.stateWatchers == nil || w.raft == nil {
		return fmt.Errorf("State watchers and Raft endpoints must be set before starting")
	}

	wasEnabled := w.enabled
	w.enabled = enabled
	w.l.Unlock()

	// Flush the state to create the necessary objects
	w.Flush()

	// If we are starting now, launch the watch daemon
	if enabled && !wasEnabled {
		go w.watchDeployments()
	}

	return nil
}

// Flush is used to clear the state of the watcher
func (w *Watcher) Flush() {
	w.l.Lock()
	defer w.l.Unlock()

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
func (w *Watcher) watchDeployments() {
	dindex := uint64(0)
	for {
		// Block getting all deployments using the last deployment index.
		resp, err := w.getDeploys(dindex)
		if err != nil {
			if err == context.Canceled {
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
func (w *Watcher) getDeploys(index uint64) (*structs.DeploymentListResponse, error) {
	// Build the request
	args := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			MinQueryIndex: index,
		},
	}
	var resp structs.DeploymentListResponse

	for resp.Index <= index {
		if err := w.queryLimiter.Wait(w.ctx); err != nil {
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

// getWatcher returns the deployment watcher for the given deployment ID.
func (w *Watcher) getWatcher(dID string) (*deploymentWatcher, error) {
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
	watcher, err := w.getWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.SetAllocHealth(req, resp)
}

// PromoteDeployment is used to promote a deployment. If promote is false,
// deployment is marked as failed. Otherwise the deployment is updated and an
// evaluation is created.
func (w *Watcher) PromoteDeployment(req *structs.DeploymentPromoteRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.PromoteDeployment(req, resp)
}

// PauseDeployment is used to toggle the pause state on a deployment. If the
// deployment is being unpaused, an evaluation is created.
func (w *Watcher) PauseDeployment(req *structs.DeploymentPauseRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getWatcher(req.DeploymentID)
	if err != nil {
		return err
	}

	return watcher.PauseDeployment(req, resp)
}

// FailDeployment is used to fail the deployment.
func (w *Watcher) FailDeployment(req *structs.DeploymentFailRequest, resp *structs.DeploymentUpdateResponse) error {
	watcher, err := w.getWatcher(req.DeploymentID)
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
	return w.raft.UpsertDeploymentStatusUpdate(&structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: u,
		Eval:             e,
		Job:              j,
	})
}

// upsertDeploymentPromotion commits the given deployment promotion to Raft
func (w *Watcher) upsertDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error) {
	return w.raft.UpsertDeploymentPromotion(req)
}

// upsertDeploymentAllocHealth commits the given allocation health changes to
// Raft
func (w *Watcher) upsertDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error) {
	return w.raft.UpsertDeploymentAllocHealth(req)
}
