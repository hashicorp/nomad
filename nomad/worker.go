// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
)

const (
	// backoffBaselineFast is the baseline time for exponential backoff
	backoffBaselineFast = 20 * time.Millisecond

	// backoffBaselineSlow is the baseline time for exponential backoff
	// but that is much slower than backoffBaselineFast
	backoffBaselineSlow = 500 * time.Millisecond

	// backoffLimitSlow is the limit of the exponential backoff for
	// the slower backoff
	backoffLimitSlow = 10 * time.Second

	// backoffSchedulerVersionMismatch is the backoff between retries when the
	// scheduler version mismatches that of the leader.
	backoffSchedulerVersionMismatch = 30 * time.Second

	// dequeueTimeout is used to timeout an evaluation dequeue so that
	// we can check if there is a shutdown event
	dequeueTimeout = 500 * time.Millisecond

	// raftSyncLimit is the limit of time we will wait for Raft replication
	// to catch up to the evaluation. This is used to fast Nack and
	// allow another scheduler to pick it up.
	raftSyncLimit = 5 * time.Second

	// dequeueErrGrace is the grace period where we don't log about
	// dequeue errors after start. This is to improve the user experience
	// in dev mode where the leader isn't elected for a few seconds.
	dequeueErrGrace = 10 * time.Second
)

type WorkerStatus int

//go:generate stringer -trimprefix=Worker -output worker_string_workerstatus.go -linecomment -type=WorkerStatus
const (
	WorkerUnknownStatus WorkerStatus = iota // Unknown
	WorkerStarting
	WorkerStarted
	WorkerPausing
	WorkerPaused
	WorkerResuming
	WorkerStopping
	WorkerStopped
)

type SchedulerWorkerStatus int

//go:generate stringer -trimprefix=Workload -output worker_string_schedulerworkerstatus.go -linecomment -type=SchedulerWorkerStatus
const (
	WorkloadUnknownStatus SchedulerWorkerStatus = iota
	WorkloadRunning
	WorkloadWaitingToDequeue
	WorkloadWaitingForRaft
	WorkloadScheduling
	WorkloadSubmitting
	WorkloadBackoff
	WorkloadStopped
	WorkloadPaused
)

// Worker is a single threaded scheduling worker. There may be multiple
// running per server (leader or follower). They are responsible for dequeuing
// pending evaluations, invoking schedulers, plan submission and the
// lifecycle around making task allocations. They bridge the business logic
// of the scheduler with the plumbing required to make it all work.
type Worker struct {
	srv    *Server
	logger log.Logger
	start  time.Time
	id     string

	status         WorkerStatus
	workloadStatus SchedulerWorkerStatus
	statusLock     sync.RWMutex

	pauseFlag bool
	pauseLock sync.Mutex
	pauseCond *sync.Cond
	ctx       context.Context
	cancelFn  context.CancelFunc

	// the Server.Config.EnabledSchedulers value is not safe for concurrent access, so
	// the worker needs a cached copy of it. Workers are stopped if this value changes.
	enabledSchedulers []string

	// failures is the count of errors encountered while dequeueing evaluations
	// and is used to calculate backoff.
	failures       uint64
	failureBackoff time.Duration
	evalToken      string

	// snapshotIndex is the index of the snapshot in which the scheduler was
	// first invoked. It is used to mark the SnapshotIndex of evaluations
	// Created, Updated or Reblocked.
	snapshotIndex uint64
}

// NewWorker starts a new scheduler worker associated with the given server
func NewWorker(ctx context.Context, srv *Server, args SchedulerWorkerPoolArgs) (*Worker, error) {
	w := newWorker(ctx, srv, args)
	w.Start()
	return w, nil
}

// _newWorker creates a worker without calling its Start func. This is useful for testing.
func newWorker(ctx context.Context, srv *Server, args SchedulerWorkerPoolArgs) *Worker {
	w := &Worker{
		id:                uuid.Generate(),
		srv:               srv,
		start:             time.Now(),
		status:            WorkerStarting,
		enabledSchedulers: make([]string, len(args.EnabledSchedulers)),
		failureBackoff:    time.Duration(0),
	}
	copy(w.enabledSchedulers, args.EnabledSchedulers)

	w.logger = srv.logger.ResetNamed("worker").With("worker_id", w.id)
	w.pauseCond = sync.NewCond(&w.pauseLock)
	w.ctx, w.cancelFn = context.WithCancel(ctx)

	return w
}

// ID returns a string ID for the worker.
func (w *Worker) ID() string {
	return w.id
}

// Start transitions a worker to the starting state. Check
// to see if it paused using IsStarted()
func (w *Worker) Start() {
	w.setStatus(WorkerStarting)
	go w.run(raftSyncLimit)
}

// Pause transitions a worker to the pausing state. Check
// to see if it paused using IsPaused()
func (w *Worker) Pause() {
	if w.isPausable() {
		w.setStatus(WorkerPausing)
		w.setPauseFlag(true)
	}
}

// Resume transitions a worker to the resuming state. Check
// to see if the worker restarted by calling IsStarted()
func (w *Worker) Resume() {
	if w.IsPaused() {
		w.setStatus(WorkerResuming)
		w.setPauseFlag(false)
		w.pauseCond.Broadcast()
	}
}

// Resume transitions a worker to the stopping state. Check
// to see if the worker stopped by calling IsStopped()
func (w *Worker) Stop() {
	w.setStatus(WorkerStopping)
	w.shutdown()
}

// IsStarted returns a boolean indicating if this worker has been started.
func (w *Worker) IsStarted() bool {
	return w.GetStatus() == WorkerStarted
}

// IsPaused returns a boolean indicating if this worker has been paused.
func (w *Worker) IsPaused() bool {
	return w.GetStatus() == WorkerPaused
}

// IsStopped returns a boolean indicating if this worker has been stopped.
func (w *Worker) IsStopped() bool {
	return w.GetStatus() == WorkerStopped
}

func (w *Worker) isPausable() bool {
	w.statusLock.RLock()
	defer w.statusLock.RUnlock()
	switch w.status {
	case WorkerPausing, WorkerPaused, WorkerStopping, WorkerStopped:
		return false
	default:
		return true
	}
}

// GetStatus returns the status of the Worker
func (w *Worker) GetStatus() WorkerStatus {
	w.statusLock.RLock()
	defer w.statusLock.RUnlock()
	return w.status
}

// setStatuses is used internally to the worker to update the
// status of the worker and workload at one time, since some
// transitions need to update both values using the same lock.
func (w *Worker) setStatuses(newWorkerStatus WorkerStatus, newWorkloadStatus SchedulerWorkerStatus) {
	w.statusLock.Lock()
	defer w.statusLock.Unlock()
	w.setWorkerStatusLocked(newWorkerStatus)
	w.setWorkloadStatusLocked(newWorkloadStatus)
}

// setStatus is used internally to the worker to update the
// status of the worker based on calls to the Worker API. For
// atomically updating the scheduler status and the workload
// status, use `setStatuses`.
func (w *Worker) setStatus(newStatus WorkerStatus) {
	w.statusLock.Lock()
	defer w.statusLock.Unlock()
	w.setWorkerStatusLocked(newStatus)
}

func (w *Worker) setWorkerStatusLocked(newStatus WorkerStatus) {
	if newStatus == w.status {
		return
	}
	w.logger.Trace("changed worker status", "from", w.status, "to", newStatus)
	w.status = newStatus
}

// GetStatus returns the status of the Worker's Workload.
func (w *Worker) GetWorkloadStatus() SchedulerWorkerStatus {
	w.statusLock.RLock()
	defer w.statusLock.RUnlock()
	return w.workloadStatus
}

// setWorkloadStatus is used internally to the worker to update the
// status of the worker based updates from the workload.
func (w *Worker) setWorkloadStatus(newStatus SchedulerWorkerStatus) {
	w.statusLock.Lock()
	defer w.statusLock.Unlock()
	w.setWorkloadStatusLocked(newStatus)
}

func (w *Worker) setWorkloadStatusLocked(newStatus SchedulerWorkerStatus) {
	if newStatus == w.workloadStatus {
		return
	}
	w.logger.Trace("changed workload status", "from", w.workloadStatus, "to", newStatus)
	w.workloadStatus = newStatus
}

type WorkerInfo struct {
	ID                string    `json:"id"`
	EnabledSchedulers []string  `json:"enabled_schedulers"`
	Started           time.Time `json:"started"`
	Status            string    `json:"status"`
	WorkloadStatus    string    `json:"workload_status"`
}

func (w WorkerInfo) Copy() WorkerInfo {
	out := WorkerInfo{
		ID:                w.ID,
		EnabledSchedulers: make([]string, len(w.EnabledSchedulers)),
		Started:           w.Started,
		Status:            w.Status,
		WorkloadStatus:    w.WorkloadStatus,
	}
	copy(out.EnabledSchedulers, w.EnabledSchedulers)
	return out
}

func (w WorkerInfo) String() string {
	// lazy implementation of WorkerInfo to string
	out, _ := json.Marshal(w)
	return string(out)
}

func (w *Worker) Info() WorkerInfo {
	w.pauseLock.Lock()
	defer w.pauseLock.Unlock()
	out := WorkerInfo{
		ID:                w.id,
		Status:            w.status.String(),
		WorkloadStatus:    w.workloadStatus.String(),
		EnabledSchedulers: make([]string, len(w.enabledSchedulers)),
	}
	out.Started = w.start
	copy(out.EnabledSchedulers, w.enabledSchedulers)
	return out
}

// ----------------------------------
//  Pause Implementation
//    These functions are used to support the worker's pause behaviors.
// ----------------------------------

func (w *Worker) setPauseFlag(pause bool) {
	w.pauseLock.Lock()
	defer w.pauseLock.Unlock()
	w.pauseFlag = pause
}

// maybeWait is responsible for making the transition from `pausing`
// to `paused`, waiting, and then transitioning back to the running
// values.
func (w *Worker) maybeWait() {
	w.pauseLock.Lock()
	defer w.pauseLock.Unlock()

	if !w.pauseFlag {
		return
	}

	w.statusLock.Lock()
	w.status = WorkerPaused
	originalWorkloadStatus := w.workloadStatus
	w.workloadStatus = WorkloadPaused
	w.logger.Trace("changed workload status", "from", originalWorkloadStatus, "to", w.workloadStatus)

	w.statusLock.Unlock()

	for w.pauseFlag {
		w.pauseCond.Wait()
	}

	w.statusLock.Lock()

	w.logger.Trace("changed workload status", "from", w.workloadStatus, "to", originalWorkloadStatus)
	w.workloadStatus = originalWorkloadStatus

	// only reset the worker status if the worker is not resuming to stop the paused workload.
	if w.status != WorkerStopping {
		w.logger.Trace("changed worker status", "from", w.status, "to", WorkerStarted)
		w.status = WorkerStarted
	}
	w.statusLock.Unlock()
}

// Shutdown is used to signal that the worker should shutdown.
func (w *Worker) shutdown() {
	w.pauseLock.Lock()
	wasPaused := w.pauseFlag
	w.pauseFlag = false
	w.pauseLock.Unlock()

	w.logger.Trace("shutdown request received")
	w.cancelFn()
	if wasPaused {
		w.pauseCond.Broadcast()
	}
}

// markStopped is used to mark the worker  and workload as stopped. It should be called in a
// defer immediately upon entering the run() function.
func (w *Worker) markStopped() {
	w.setStatuses(WorkerStopped, WorkloadStopped)
	w.logger.Debug("stopped")
}

func (w *Worker) workerShuttingDown() bool {
	select {
	case <-w.ctx.Done():
		return true
	default:
		return false
	}
}

// ----------------------------------
//  Workload behavior code
// ----------------------------------

// run is the long-lived goroutine which is used to run the worker
func (w *Worker) run(raftSyncLimit time.Duration) {
	defer func() {
		w.markStopped()
	}()
	w.setStatuses(WorkerStarted, WorkloadRunning)
	w.logger.Debug("running")
	for {
		// Check to see if the context has been cancelled. Server shutdown and Shutdown()
		// should do this.
		if w.workerShuttingDown() {
			return
		}
		// Dequeue a pending evaluation
		eval, token, waitIndex, shutdown := w.dequeueEvaluation(dequeueTimeout)
		if shutdown {
			return
		}

		// since dequeue takes time, we could have shutdown the server after
		// getting an eval that needs to be nacked before we exit. Explicitly
		// check the server whether to allow this eval to be processed.
		if w.srv.IsShutdown() {
			w.logger.Warn("nacking eval because the server is shutting down",
				"eval", log.Fmt("%#v", eval))
			w.sendNack(eval, token)
			return
		}

		// Wait for the raft log to catchup to the evaluation
		w.setWorkloadStatus(WorkloadWaitingForRaft)
		snap, err := w.snapshotMinIndex(waitIndex, raftSyncLimit)
		if err != nil {
			var timeoutErr ErrMinIndexDeadlineExceeded
			if errors.As(err, &timeoutErr) {
				w.logger.Warn("timeout waiting for Raft index required by eval",
					"eval", eval.ID, "index", waitIndex, "timeout", raftSyncLimit)
				w.sendNack(eval, token)

				// Timing out above means this server is woefully behind the
				// leader's index. This can happen when a new server is added to
				// a cluster and must initially sync the cluster state.
				// Backoff dequeuing another eval until there's some indication
				// this server would be up to date enough to process it.
				slowServerSyncLimit := 10 * raftSyncLimit
				if _, err := w.snapshotMinIndex(waitIndex, slowServerSyncLimit); err != nil {
					w.logger.Warn("server is unable to catch up to last eval's index", "error", err)
				}

			} else if errors.Is(err, context.Canceled) {
				// If the server has shutdown while we're waiting, we'll get the
				// Canceled error from the worker's context. We need to nack any
				// dequeued evals before we exit.
				w.logger.Warn("nacking eval because the server is shutting down", "eval", eval.ID)
				w.sendNack(eval, token)
				return
			} else {
				w.logger.Error("error waiting for Raft index", "error", err, "index", waitIndex)
				w.sendNack(eval, token)
			}

			continue
		}

		// Invoke the scheduler to determine placements
		w.setWorkloadStatus(WorkloadScheduling)
		if err := w.invokeScheduler(snap, eval, token); err != nil {
			w.logger.Error("error invoking scheduler", "error", err)
			w.sendNack(eval, token)
			continue
		}

		// Complete the evaluation
		w.sendAck(eval, token)
	}
}

// dequeueEvaluation is used to fetch the next ready evaluation.
// This blocks until an evaluation is available or a timeout is reached.
func (w *Worker) dequeueEvaluation(timeout time.Duration) (
	eval *structs.Evaluation, token string, waitIndex uint64, shutdown bool) {
	// Setup the request
	req := structs.EvalDequeueRequest{
		Schedulers:       w.enabledSchedulers,
		Timeout:          timeout,
		SchedulerVersion: scheduler.SchedulerVersion,
		WriteRequest: structs.WriteRequest{
			Region: w.srv.config.Region,
		},
	}
	var resp structs.EvalDequeueResponse

REQ:
	// Wait inside this function if the worker is paused.
	w.maybeWait()
	// Immediately check to see if the worker has been shutdown.
	if w.workerShuttingDown() {
		return nil, "", 0, true
	}

	// Make a blocking RPC
	start := time.Now()
	w.setWorkloadStatus(WorkloadWaitingToDequeue)
	err := w.srv.RPC("Eval.Dequeue", &req, &resp)
	metrics.MeasureSince([]string{"nomad", "worker", "dequeue_eval"}, start)
	if err != nil {
		if time.Since(w.start) > dequeueErrGrace && !w.workerShuttingDown() {
			w.logger.Error("failed to dequeue evaluation", "error", err)
		}

		// Adjust the backoff based on the error. If it is a scheduler version
		// mismatch we increase the baseline.
		base, limit := backoffBaselineFast, backoffLimitSlow
		if strings.Contains(err.Error(), "calling scheduler version") {
			base = backoffSchedulerVersionMismatch
			limit = backoffSchedulerVersionMismatch
		}

		if w.backoffErr(base, limit) {
			return nil, "", 0, true
		}
		goto REQ
	}
	w.backoffReset()

	// Check if we got a response
	if resp.Eval != nil {
		w.logger.Debug("dequeued evaluation", "eval_id", resp.Eval.ID, "type", resp.Eval.Type, "namespace", resp.Eval.Namespace, "job_id", resp.Eval.JobID, "node_id", resp.Eval.NodeID, "triggered_by", resp.Eval.TriggeredBy)
		return resp.Eval, resp.Token, resp.GetWaitIndex(), false
	}

	goto REQ
}

// sendAcknowledgement should not be called directly. Call `sendAck` or `sendNack` instead.
// This function implements `ack`ing or `nack`ing the evaluation generally.
// Any errors are logged but swallowed.
func (w *Worker) sendAcknowledgement(eval *structs.Evaluation, token string, ack bool) {
	defer metrics.MeasureSince([]string{"nomad", "worker", "send_ack"}, time.Now())
	// Setup the request
	req := structs.EvalAckRequest{
		EvalID: eval.ID,
		Token:  token,
		WriteRequest: structs.WriteRequest{
			Region: w.srv.config.Region,
		},
	}
	var resp structs.GenericResponse

	// Determine if this is an Ack or Nack
	verb := "ack"
	endpoint := "Eval.Ack"
	if !ack {
		verb = "nack"
		endpoint = "Eval.Nack"
	}

	// Make the RPC call
	err := w.srv.RPC(endpoint, &req, &resp)
	if err != nil {
		w.logger.Error(fmt.Sprintf("failed to %s evaluation", verb), "eval_id", eval.ID, "error", err)
	} else {
		w.logger.Debug(fmt.Sprintf("%s evaluation", verb), "eval_id", eval.ID, "type", eval.Type, "namespace", eval.Namespace, "job_id", eval.JobID, "node_id", eval.NodeID, "triggered_by", eval.TriggeredBy)
	}
}

// sendNack makes a best effort to nack the evaluation.
// Any errors are logged but swallowed.
func (w *Worker) sendNack(eval *structs.Evaluation, token string) {
	w.sendAcknowledgement(eval, token, false)
}

// sendAck makes a best effort to ack the evaluation.
// Any errors are logged but swallowed.
func (w *Worker) sendAck(eval *structs.Evaluation, token string) {
	w.sendAcknowledgement(eval, token, true)
}

type ErrMinIndexDeadlineExceeded struct {
	waitIndex uint64
	timeout   time.Duration
}

// Unwrapping an ErrMinIndexDeadlineExceeded always return
// context.DeadlineExceeded
func (ErrMinIndexDeadlineExceeded) Unwrap() error {
	return context.DeadlineExceeded
}

func (e ErrMinIndexDeadlineExceeded) Error() string {
	return fmt.Sprintf("timed out after %s waiting for index=%d", e.timeout, e.waitIndex)
}

// snapshotMinIndex times calls to StateStore.SnapshotAfter which may block.
func (w *Worker) snapshotMinIndex(waitIndex uint64, timeout time.Duration) (*state.StateSnapshot, error) {
	defer metrics.MeasureSince([]string{"nomad", "worker", "wait_for_index"}, time.Now())

	ctx, cancel := context.WithTimeout(w.ctx, timeout)
	snap, err := w.srv.fsm.State().SnapshotMinIndex(ctx, waitIndex)
	cancel()

	// Wrap error to ensure callers can detect timeouts.
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, ErrMinIndexDeadlineExceeded{
			waitIndex: waitIndex,
			timeout:   timeout,
		}
	}

	return snap, err
}

// invokeScheduler is used to invoke the business logic of the scheduler
func (w *Worker) invokeScheduler(snap *state.StateSnapshot, eval *structs.Evaluation, token string) error {
	defer metrics.MeasureSince([]string{"nomad", "worker", "invoke_scheduler", eval.Type}, time.Now())
	// Store the evaluation token
	w.evalToken = token

	// Store the snapshot's index
	var err error
	w.snapshotIndex, err = snap.LatestIndex()
	if err != nil {
		return fmt.Errorf("failed to determine snapshot's index: %v", err)
	}

	// Create the scheduler, or use the special core scheduler
	var sched scheduler.Scheduler
	if eval.Type == structs.JobTypeCore {
		sched = NewCoreScheduler(w.srv, snap)
	} else {
		sched, err = scheduler.NewScheduler(eval.Type, w.logger, w.srv.workersEventCh, snap, w)
		if err != nil {
			return fmt.Errorf("failed to instantiate scheduler: %v", err)
		}
	}

	// Process the evaluation
	err = sched.Process(eval)
	if err != nil {
		return fmt.Errorf("failed to process evaluation: %v", err)
	}
	return nil
}

// ServersMeetMinimumVersion allows implementations of the Scheduler interface in
// other packages to perform server version checks without direct references to
// the Nomad server.
func (w *Worker) ServersMeetMinimumVersion(minVersion *version.Version, checkFailedServers bool) bool {
	return ServersMeetMinimumVersion(w.srv.Members(), w.srv.Region(), minVersion, checkFailedServers)
}

// SubmitPlan is used to submit a plan for consideration. This allows
// the worker to act as the planner for the scheduler.
func (w *Worker) SubmitPlan(plan *structs.Plan) (*structs.PlanResult, scheduler.State, error) {
	// Check for a shutdown before plan submission. Checking server state rather than
	// worker state to allow work in flight to complete before stopping.
	if w.srv.IsShutdown() {
		return nil, nil, fmt.Errorf("shutdown while planning")
	}
	defer metrics.MeasureSince([]string{"nomad", "worker", "submit_plan"}, time.Now())

	// Add the evaluation token to the plan
	plan.EvalToken = w.evalToken

	// Add SnapshotIndex to ensure leader's StateStore processes the Plan
	// at or after the index it was created.
	plan.SnapshotIndex = w.snapshotIndex

	// Normalize stopped and preempted allocs before RPC
	normalizePlan := ServersMeetMinimumVersion(w.srv.Members(), w.srv.Region(), MinVersionPlanNormalization, true)
	if normalizePlan {
		plan.NormalizeAllocations()
	}

	// Setup the request
	req := structs.PlanRequest{
		Plan: plan,
		WriteRequest: structs.WriteRequest{
			Region: w.srv.config.Region,
		},
	}
	var resp structs.PlanResponse

SUBMIT:
	// Make the RPC call
	if err := w.srv.RPC("Plan.Submit", &req, &resp); err != nil {
		w.logger.Error("failed to submit plan for evaluation", "eval_id", plan.EvalID, "error", err)
		if w.shouldResubmit(err) && !w.backoffErr(backoffBaselineSlow, backoffLimitSlow) {
			goto SUBMIT
		}
		return nil, nil, err
	} else {
		w.logger.Debug("submitted plan for evaluation", "eval_id", plan.EvalID)
		w.backoffReset()
	}

	// Look for a result
	result := resp.Result
	if result == nil {
		return nil, nil, fmt.Errorf("missing result")
	}

	// Check if a state update is required. This could be required if we
	// planned based on stale data, which is causing issues. For example, a
	// node failure since the time we've started planning or conflicting task
	// allocations.
	var state scheduler.State
	if result.RefreshIndex != 0 {
		// Wait for the raft log to catchup to the evaluation
		w.logger.Debug("refreshing state", "refresh_index", result.RefreshIndex, "eval_id", plan.EvalID)

		var err error
		state, err = w.snapshotMinIndex(result.RefreshIndex, raftSyncLimit)
		if err != nil {
			return nil, nil, err
		}
	}

	// Return the result and potential state update
	return result, state, nil
}

// UpdateEval is used to submit an updated evaluation. This allows
// the worker to act as the planner for the scheduler.
func (w *Worker) UpdateEval(eval *structs.Evaluation) error {
	// Check for a shutdown before plan submission. Checking server state rather than
	// worker state to allow a workers work in flight to complete before stopping.
	if w.srv.IsShutdown() {
		return fmt.Errorf("shutdown while planning")
	}
	defer metrics.MeasureSince([]string{"nomad", "worker", "update_eval"}, time.Now())

	// Store the snapshot index in the eval
	eval.SnapshotIndex = w.snapshotIndex
	eval.UpdateModifyTime()

	// Setup the request
	req := structs.EvalUpdateRequest{
		Evals:     []*structs.Evaluation{eval},
		EvalToken: w.evalToken,
		WriteRequest: structs.WriteRequest{
			Region: w.srv.config.Region,
		},
	}
	var resp structs.GenericResponse

SUBMIT:
	// Make the RPC call
	if err := w.srv.RPC("Eval.Update", &req, &resp); err != nil {
		w.logger.Error("failed to update evaluation", "eval", log.Fmt("%#v", eval), "error", err)
		if w.shouldResubmit(err) && !w.backoffErr(backoffBaselineSlow, backoffLimitSlow) {
			goto SUBMIT
		}
		return err
	} else {
		w.logger.Debug("updated evaluation", "eval", log.Fmt("%#v", eval))
		w.backoffReset()
	}
	return nil
}

// CreateEval is used to create a new evaluation. This allows
// the worker to act as the planner for the scheduler.
func (w *Worker) CreateEval(eval *structs.Evaluation) error {
	// Check for a shutdown before plan submission. This consults the server Shutdown state
	// instead of the worker's to prevent aborting work in flight.
	if w.srv.IsShutdown() {
		return fmt.Errorf("shutdown while planning")
	}
	defer metrics.MeasureSince([]string{"nomad", "worker", "create_eval"}, time.Now())

	// Store the snapshot index in the eval
	eval.SnapshotIndex = w.snapshotIndex

	now := time.Now().UTC().UnixNano()
	eval.CreateTime = now
	eval.ModifyTime = now

	// Setup the request
	req := structs.EvalUpdateRequest{
		Evals:     []*structs.Evaluation{eval},
		EvalToken: w.evalToken,
		WriteRequest: structs.WriteRequest{
			Region: w.srv.config.Region,
		},
	}
	var resp structs.GenericResponse

SUBMIT:
	// Make the RPC call
	if err := w.srv.RPC("Eval.Create", &req, &resp); err != nil {
		w.logger.Error("failed to create evaluation", "eval", log.Fmt("%#v", eval), "error", err)
		if w.shouldResubmit(err) && !w.backoffErr(backoffBaselineSlow, backoffLimitSlow) {
			goto SUBMIT
		}
		return err
	} else {
		w.logger.Debug("created evaluation", "eval", log.Fmt("%#v", eval))
		w.backoffReset()
	}
	return nil
}

// ReblockEval is used to reinsert a blocked evaluation into the blocked eval
// tracker. This allows the worker to act as the planner for the scheduler.
func (w *Worker) ReblockEval(eval *structs.Evaluation) error {
	// Check for a shutdown before plan submission. This checks the server state rather than
	// the worker's to prevent erroring on work in flight that would complete otherwise.
	if w.srv.IsShutdown() {
		return fmt.Errorf("shutdown while planning")
	}
	defer metrics.MeasureSince([]string{"nomad", "worker", "reblock_eval"}, time.Now())

	// Update the evaluation if the queued jobs is not same as what is
	// recorded in the job summary
	ws := memdb.NewWatchSet()
	summary, err := w.srv.fsm.state.JobSummaryByID(ws, eval.Namespace, eval.JobID)
	if err != nil {
		return fmt.Errorf("couldn't retrieve job summary: %v", err)
	}
	if summary != nil {
		var hasChanged bool
		for tg, summary := range summary.Summary {
			if queued, ok := eval.QueuedAllocations[tg]; ok {
				if queued != summary.Queued {
					hasChanged = true
					break
				}
			}
		}
		if hasChanged {
			if err := w.UpdateEval(eval); err != nil {
				return err
			}
		}
	}

	// Store the snapshot index in the eval
	eval.SnapshotIndex = w.snapshotIndex
	eval.UpdateModifyTime()

	// Setup the request
	req := structs.EvalUpdateRequest{
		Evals:     []*structs.Evaluation{eval},
		EvalToken: w.evalToken,
		WriteRequest: structs.WriteRequest{
			Region: w.srv.config.Region,
		},
	}
	var resp structs.GenericResponse

SUBMIT:
	// Make the RPC call
	if err := w.srv.RPC("Eval.Reblock", &req, &resp); err != nil {
		w.logger.Error("failed to reblock evaluation", "eval", log.Fmt("%#v", eval), "error", err)
		if w.shouldResubmit(err) && !w.backoffErr(backoffBaselineSlow, backoffLimitSlow) {
			goto SUBMIT
		}
		return err
	} else {
		w.logger.Debug("reblocked evaluation", "eval", log.Fmt("%#v", eval))
		w.backoffReset()
	}
	return nil
}

// shouldResubmit checks if a given error should be swallowed and the plan
// resubmitted after a backoff. Usually these are transient errors that
// the cluster should heal from quickly.
func (w *Worker) shouldResubmit(err error) bool {
	s := err.Error()
	switch {
	case strings.Contains(s, "No cluster leader"):
		return true
	case strings.Contains(s, "plan queue is disabled"):
		return true
	default:
		return false
	}
}

// backoffErr is used to do an exponential back off on error. This is
// maintained statefully for the worker. Returns if attempts should be
// abandoned due to shutdown.
// This uses the worker's context in order to immediately stop the
// backoff if the server or the worker is shutdown.
func (w *Worker) backoffErr(base, limit time.Duration) bool {
	w.setWorkloadStatus(WorkloadBackoff)

	backoff := helper.Backoff(base, limit, w.failures)
	w.failures++

	select {
	case <-time.After(backoff):
		return false
	case <-w.ctx.Done():
		return true
	}
}

// backoffReset is used to reset the failure count for
// exponential backoff
func (w *Worker) backoffReset() {
	w.failures = 0
}
