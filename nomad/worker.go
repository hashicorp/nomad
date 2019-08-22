package nomad

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
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

// Worker is a single threaded scheduling worker. There may be multiple
// running per server (leader or follower). They are responsible for dequeuing
// pending evaluations, invoking schedulers, plan submission and the
// lifecycle around making task allocations. They bridge the business logic
// of the scheduler with the plumbing required to make it all work.
type Worker struct {
	srv    *Server
	logger log.Logger
	start  time.Time

	paused    bool
	pauseLock sync.Mutex
	pauseCond *sync.Cond

	failures uint

	evalToken string

	// snapshotIndex is the index of the snapshot in which the scheduler was
	// first invoked. It is used to mark the SnapshotIndex of evaluations
	// Created, Updated or Reblocked.
	snapshotIndex uint64
}

// NewWorker starts a new worker associated with the given server
func NewWorker(srv *Server) (*Worker, error) {
	w := &Worker{
		srv:    srv,
		logger: srv.logger.ResetNamed("worker"),
		start:  time.Now(),
	}
	w.pauseCond = sync.NewCond(&w.pauseLock)
	go w.run()
	return w, nil
}

// SetPause is used to pause or unpause a worker
func (w *Worker) SetPause(p bool) {
	w.pauseLock.Lock()
	w.paused = p
	w.pauseLock.Unlock()
	if !p {
		w.pauseCond.Broadcast()
	}
}

// checkPaused is used to park the worker when paused
func (w *Worker) checkPaused() {
	w.pauseLock.Lock()
	for w.paused {
		w.pauseCond.Wait()
	}
	w.pauseLock.Unlock()
}

// run is the long-lived goroutine which is used to run the worker
func (w *Worker) run() {
	for {
		// Dequeue a pending evaluation
		eval, token, waitIndex, shutdown := w.dequeueEvaluation(dequeueTimeout)
		if shutdown {
			return
		}

		// Check for a shutdown
		if w.srv.IsShutdown() {
			w.logger.Error("nacking eval because the server is shutting down", "eval", log.Fmt("%#v", eval))
			w.sendAck(eval.ID, token, false)
			return
		}

		// Wait for the raft log to catchup to the evaluation
		snap, err := w.snapshotMinIndex(waitIndex, raftSyncLimit)
		if err != nil {
			w.logger.Error("error waiting for Raft index", "error", err, "index", waitIndex)
			w.sendAck(eval.ID, token, false)
			continue
		}

		// Invoke the scheduler to determine placements
		if err := w.invokeScheduler(snap, eval, token); err != nil {
			w.logger.Error("error invoking scheduler", "error", err)
			w.sendAck(eval.ID, token, false)
			continue
		}

		// Complete the evaluation
		w.sendAck(eval.ID, token, true)
	}
}

// dequeueEvaluation is used to fetch the next ready evaluation.
// This blocks until an evaluation is available or a timeout is reached.
func (w *Worker) dequeueEvaluation(timeout time.Duration) (
	eval *structs.Evaluation, token string, waitIndex uint64, shutdown bool) {
	// Setup the request
	req := structs.EvalDequeueRequest{
		Schedulers:       w.srv.config.EnabledSchedulers,
		Timeout:          timeout,
		SchedulerVersion: scheduler.SchedulerVersion,
		WriteRequest: structs.WriteRequest{
			Region: w.srv.config.Region,
		},
	}
	var resp structs.EvalDequeueResponse

REQ:
	// Check if we are paused
	w.checkPaused()

	// Make a blocking RPC
	start := time.Now()
	err := w.srv.RPC("Eval.Dequeue", &req, &resp)
	metrics.MeasureSince([]string{"nomad", "worker", "dequeue_eval"}, start)
	if err != nil {
		if time.Since(w.start) > dequeueErrGrace && !w.srv.IsShutdown() {
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
		w.logger.Debug("dequeued evaluation", "eval_id", resp.Eval.ID)
		return resp.Eval, resp.Token, resp.GetWaitIndex(), false
	}

	// Check for potential shutdown
	if w.srv.IsShutdown() {
		return nil, "", 0, true
	}
	goto REQ
}

// sendAck makes a best effort to ack or nack the evaluation.
// Any errors are logged but swallowed.
func (w *Worker) sendAck(evalID, token string, ack bool) {
	defer metrics.MeasureSince([]string{"nomad", "worker", "send_ack"}, time.Now())
	// Setup the request
	req := structs.EvalAckRequest{
		EvalID: evalID,
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
		w.logger.Error(fmt.Sprintf("failed to %s evaluation", verb), "eval_id", evalID, "error", err)
	} else {
		w.logger.Debug(fmt.Sprintf("%s evaluation", verb), "eval_id", evalID)
	}
}

// snapshotMinIndex times calls to StateStore.SnapshotAfter which may block.
func (w *Worker) snapshotMinIndex(waitIndex uint64, timeout time.Duration) (*state.StateSnapshot, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(w.srv.shutdownCtx, timeout)
	snap, err := w.srv.fsm.State().SnapshotMinIndex(ctx, waitIndex)
	cancel()
	metrics.MeasureSince([]string{"nomad", "worker", "wait_for_index"}, start)

	// Wrap error to ensure callers don't disregard timeouts.
	if err == context.DeadlineExceeded {
		err = fmt.Errorf("timed out after %s waiting for index=%d", timeout, waitIndex)
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
		sched, err = scheduler.NewScheduler(eval.Type, w.logger, snap, w)
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

// SubmitPlan is used to submit a plan for consideration. This allows
// the worker to act as the planner for the scheduler.
func (w *Worker) SubmitPlan(plan *structs.Plan) (*structs.PlanResult, scheduler.State, error) {
	// Check for a shutdown before plan submission
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
	normalizePlan := ServersMeetMinimumVersion(w.srv.Members(), MinVersionPlanNormalization, true)
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
	// Check for a shutdown before plan submission
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
	// Check for a shutdown before plan submission
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
	// Check for a shutdown before plan submission
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
func (w *Worker) backoffErr(base, limit time.Duration) bool {
	backoff := (1 << (2 * w.failures)) * base
	if backoff > limit {
		backoff = limit
	} else {
		w.failures++
	}
	select {
	case <-time.After(backoff):
		return false
	case <-w.srv.shutdownCh:
		return true
	}
}

// backoffReset is used to reset the failure count for
// exponential backoff
func (w *Worker) backoffReset() {
	w.failures = 0
}
