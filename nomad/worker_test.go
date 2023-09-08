// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

type NoopScheduler struct {
	state    scheduler.State
	planner  scheduler.Planner
	eval     *structs.Evaluation
	eventsCh chan<- interface{}
	err      error
}

func (n *NoopScheduler) Process(eval *structs.Evaluation) error {
	if n.state == nil {
		panic("missing state")
	}
	if n.planner == nil {
		panic("missing planner")
	}
	n.eval = eval
	return n.err
}

func init() {
	scheduler.BuiltinSchedulers["noop"] = func(logger log.Logger, eventsCh chan<- interface{}, s scheduler.State, p scheduler.Planner) scheduler.Scheduler {
		n := &NoopScheduler{
			state:   s,
			planner: p,
		}
		return n
	}
}

// NewTestWorker returns the worker without calling it's run method.
func NewTestWorker(shutdownCtx context.Context, srv *Server) *Worker {
	w := &Worker{
		srv:               srv,
		start:             time.Now(),
		id:                uuid.Generate(),
		enabledSchedulers: srv.config.EnabledSchedulers,
	}
	w.logger = srv.logger.ResetNamed("worker").With("worker_id", w.id)
	w.pauseCond = sync.NewCond(&w.pauseLock)
	w.ctx, w.cancelFn = context.WithCancel(shutdownCtx)
	return w
}

func TestWorker_dequeueEvaluation(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the evaluation
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	// Create a worker
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w, _ := NewWorker(s1.shutdownCtx, s1, poolArgs)

	// Attempt dequeue
	eval, token, waitIndex, shutdown := w.dequeueEvaluation(10 * time.Millisecond)
	if shutdown {
		t.Fatalf("should not shutdown")
	}
	if token == "" {
		t.Fatalf("should get token")
	}
	if waitIndex != eval1.ModifyIndex {
		t.Fatalf("bad wait index; got %d; want %d", waitIndex, eval1.ModifyIndex)
	}

	// Ensure we get a sane eval
	if !reflect.DeepEqual(eval, eval1) {
		t.Fatalf("bad: %#v %#v", eval, eval1)
	}
}

// Test that the worker picks up the correct wait index when there are multiple
// evals for the same job.
func TestWorker_dequeueEvaluation_SerialJobs(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the evaluation
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID

	// Insert the evals into the state store
	must.NoError(t, s1.fsm.State().UpsertEvals(
		structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1}))
	must.NoError(t, s1.fsm.State().UpsertEvals(
		structs.MsgTypeTestSetup, 2000, []*structs.Evaluation{eval2}))

	s1.evalBroker.Enqueue(eval1)
	s1.evalBroker.Enqueue(eval2)

	// Create a worker
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)

	// Attempt dequeue
	eval, token, waitIndex, shutdown := w.dequeueEvaluation(10 * time.Millisecond)
	must.False(t, shutdown, must.Sprint("should not be shutdown"))
	must.NotEq(t, token, "", must.Sprint("should get a token"))
	must.NotEq(t, eval1.ModifyIndex, waitIndex, must.Sprintf("bad wait index"))
	must.Eq(t, eval, eval1)

	// Update the modify index of the first eval
	must.NoError(t, s1.fsm.State().UpsertEvals(
		structs.MsgTypeTestSetup, 1500, []*structs.Evaluation{eval1}))

	// Send the Ack
	w.sendAck(eval1, token)

	// Attempt second dequeue; it should succeed because the 2nd eval has a
	// lower modify index than the snapshot used to schedule the 1st
	// eval. Normally this can only happen if the worker is on a follower that's
	// trailing behind in raft logs
	eval, token, waitIndex, shutdown = w.dequeueEvaluation(10 * time.Millisecond)

	must.False(t, shutdown, must.Sprint("should not be shutdown"))
	must.NotEq(t, token, "", must.Sprint("should get a token"))
	must.Eq(t, waitIndex, 2000, must.Sprintf("bad wait index"))
	must.Eq(t, eval, eval2)

}

func TestWorker_dequeueEvaluation_paused(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the evaluation
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	// Create a worker
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	w.pauseCond = sync.NewCond(&w.pauseLock)

	// PAUSE the worker
	w.Pause()

	go func() {
		time.Sleep(100 * time.Millisecond)
		w.Resume()
	}()

	// Attempt dequeue
	start := time.Now()
	eval, token, waitIndex, shutdown := w.dequeueEvaluation(10 * time.Millisecond)
	if diff := time.Since(start); diff < 100*time.Millisecond {
		t.Fatalf("should have paused: %v", diff)
	}
	if shutdown {
		t.Fatalf("should not shutdown")
	}
	if token == "" {
		t.Fatalf("should get token")
	}
	if waitIndex != eval1.ModifyIndex {
		t.Fatalf("bad wait index; got %d; want %d", waitIndex, eval1.ModifyIndex)
	}

	// Ensure we get a sane eval
	if !reflect.DeepEqual(eval, eval1) {
		t.Fatalf("bad: %#v %#v", eval, eval1)
	}
}

func TestWorker_dequeueEvaluation_shutdown(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a worker
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)

	go func() {
		time.Sleep(10 * time.Millisecond)
		s1.Shutdown()
	}()

	// Attempt dequeue
	eval, _, _, shutdown := w.dequeueEvaluation(10 * time.Millisecond)
	if !shutdown {
		t.Fatalf("should not shutdown")
	}

	// Ensure we get a sane eval
	if eval != nil {
		t.Fatalf("bad: %#v", eval)
	}
}

func TestWorker_Shutdown(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)

	go func() {
		time.Sleep(10 * time.Millisecond)
		w.Stop()
	}()

	// Attempt dequeue
	eval, _, _, shutdown := w.dequeueEvaluation(10 * time.Millisecond)
	require.True(t, shutdown)
	require.Nil(t, eval)
}

func TestWorker_Shutdown_paused(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w, _ := NewWorker(s1.shutdownCtx, s1, poolArgs)

	w.Pause()

	// pausing can take up to 500ms because of the blocking query timeout in dequeueEvaluation.
	require.Eventually(t, w.IsPaused, 550*time.Millisecond, 10*time.Millisecond, "should pause")

	go func() {
		w.Stop()
	}()

	// transitioning to stopped from paused should be very quick,
	// but might not be immediate.
	require.Eventually(t, w.IsStopped, 100*time.Millisecond, 10*time.Millisecond, "should stop when paused")
}

func TestWorker_sendAck(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the evaluation
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	// Create a worker
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)

	// Attempt dequeue
	eval, token, _, _ := w.dequeueEvaluation(10 * time.Millisecond)

	// Check the depth is 0, 1 unacked
	stats := s1.evalBroker.Stats()
	if stats.TotalReady != 0 && stats.TotalUnacked != 1 {
		t.Fatalf("bad: %#v", stats)
	}

	// Send the Nack
	w.sendNack(eval, token)

	// Check the depth is 1, nothing unacked
	stats = s1.evalBroker.Stats()
	if stats.TotalReady != 1 && stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}

	// Attempt dequeue
	eval, token, _, _ = w.dequeueEvaluation(10 * time.Millisecond)

	// Send the Ack
	w.sendAck(eval, token)

	// Check the depth is 0
	stats = s1.evalBroker.Stats()
	if stats.TotalReady != 0 && stats.TotalUnacked != 0 {
		t.Fatalf("bad: %#v", stats)
	}
}

func TestWorker_runBackoff(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupSrv()
	testutil.WaitForLeader(t, srv.RPC)

	eval1 := mock.Eval()
	eval1.ModifyIndex = 1000
	srv.evalBroker.Enqueue(eval1)
	must.Eq(t, 1, srv.evalBroker.Stats().TotalReady)

	// make a new context here so we can still check the broker's state after
	// we've shut down the worker
	workerCtx, workerCancel := context.WithCancel(srv.shutdownCtx)
	defer workerCancel()

	w := NewTestWorker(workerCtx, srv)
	doneCh := make(chan struct{})

	go func() {
		w.run(time.Millisecond)
		doneCh <- struct{}{}
	}()

	// We expect to be paused for 10ms + 1ms but otherwise can't be all that
	// precise here because of concurrency. But checking coverage for this test
	// shows we've covered the logic
	t1, cancelT1 := helper.NewSafeTimer(100 * time.Millisecond)
	defer cancelT1()
	select {
	case <-doneCh:
		t.Fatal("returned early")
	case <-t1.C:
	}

	workerCancel()
	<-doneCh

	must.Eq(t, 1, srv.evalBroker.Stats().TotalWaiting)
	must.Eq(t, 0, srv.evalBroker.Stats().TotalReady)
	must.Eq(t, 0, srv.evalBroker.Stats().TotalPending)
	must.Eq(t, 0, srv.evalBroker.Stats().TotalUnacked)
}

func TestWorker_waitForIndex(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Get the current index
	index := s1.raft.AppliedIndex()

	// Cause an increment
	errCh := make(chan error, 1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		n := mock.Node()
		errCh <- s1.fsm.state.UpsertNode(structs.MsgTypeTestSetup, index+1, n)
	}()

	// Wait for a future index
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	snap, err := w.snapshotMinIndex(index+1, time.Second)
	require.NoError(t, err)
	require.NotNil(t, snap)

	// No error from upserting
	require.NoError(t, <-errCh)

	// Cause a timeout
	waitIndex := index + 100
	timeout := 10 * time.Millisecond
	snap, err = w.snapshotMinIndex(index+100, timeout)
	require.Nil(t, snap)
	require.EqualError(t, err,
		fmt.Sprintf("timed out after %s waiting for index=%d", timeout, waitIndex))
	require.True(t, errors.Is(err, context.DeadlineExceeded), "expect error to wrap DeadlineExceeded")
}

func TestWorker_invokeScheduler(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()

	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	eval := mock.Eval()
	eval.Type = "noop"

	snap, err := s1.fsm.state.Snapshot()
	require.NoError(t, err)

	err = w.invokeScheduler(snap, eval, uuid.Generate())
	require.NoError(t, err)
}

func TestWorker_SubmitPlan(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	job := mock.Job()
	eval1 := mock.Eval()
	eval1.JobID = job.ID
	s1.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})

	// Create the register request
	s1.evalBroker.Enqueue(eval1)

	evalOut, token, err := s1.evalBroker.Dequeue([]string{eval1.Type}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if evalOut != eval1 {
		t.Fatalf("Bad eval")
	}

	// Create an allocation plan
	alloc := mock.Alloc()
	plan := &structs.Plan{
		Job:    job,
		EvalID: eval1.ID,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	// Attempt to submit a plan
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	w.evalToken = token

	result, state, err := w.SubmitPlan(plan)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have no update
	if state != nil {
		t.Fatalf("unexpected state update")
	}

	// Result should have allocated
	if result == nil {
		t.Fatalf("missing result")
	}

	if result.AllocIndex == 0 {
		t.Fatalf("Bad: %#v", result)
	}
	if len(result.NodeAllocation) != 1 {
		t.Fatalf("Bad: %#v", result)
	}
}

func TestWorker_SubmitPlanNormalizedAllocations(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
		c.Build = "1.4.0"
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	job := mock.Job()
	eval1 := mock.Eval()
	eval1.JobID = job.ID
	s1.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 0, nil, job)
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 0, []*structs.Evaluation{eval1})

	stoppedAlloc := mock.Alloc()
	preemptedAlloc := mock.Alloc()
	s1.fsm.State().UpsertAllocs(structs.MsgTypeTestSetup, 5, []*structs.Allocation{stoppedAlloc, preemptedAlloc})

	// Create an allocation plan
	plan := &structs.Plan{
		Job:             job,
		EvalID:          eval1.ID,
		NodeUpdate:      make(map[string][]*structs.Allocation),
		NodePreemptions: make(map[string][]*structs.Allocation),
	}
	desiredDescription := "desired desc"
	plan.AppendStoppedAlloc(stoppedAlloc, desiredDescription, structs.AllocClientStatusLost, "")
	preemptingAllocID := uuid.Generate()
	plan.AppendPreemptedAlloc(preemptedAlloc, preemptingAllocID)

	// Attempt to submit a plan
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	w.SubmitPlan(plan)

	assert.Equal(t, &structs.Allocation{
		ID:                    preemptedAlloc.ID,
		PreemptedByAllocation: preemptingAllocID,
	}, plan.NodePreemptions[preemptedAlloc.NodeID][0])
	assert.Equal(t, &structs.Allocation{
		ID:                 stoppedAlloc.ID,
		DesiredDescription: desiredDescription,
		ClientStatus:       structs.AllocClientStatusLost,
	}, plan.NodeUpdate[stoppedAlloc.NodeID][0])
}

func TestWorker_SubmitPlan_MissingNodeRefresh(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	// Create the job
	job := mock.Job()
	s1.fsm.State().UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)

	// Create the register request
	eval1 := mock.Eval()
	eval1.JobID = job.ID
	s1.evalBroker.Enqueue(eval1)

	evalOut, token, err := s1.evalBroker.Dequeue([]string{eval1.Type}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if evalOut != eval1 {
		t.Fatalf("Bad eval")
	}

	// Create an allocation plan, with unregistered node
	node2 := mock.Node()
	alloc := mock.Alloc()
	plan := &structs.Plan{
		Job:    job,
		EvalID: eval1.ID,
		NodeAllocation: map[string][]*structs.Allocation{
			node2.ID: {alloc},
		},
	}

	// Attempt to submit a plan
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	w.evalToken = token

	result, state, err := w.SubmitPlan(plan)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Result should have allocated
	if result == nil {
		t.Fatalf("missing result")
	}

	// Expect no allocation and forced refresh
	if result.AllocIndex != 0 {
		t.Fatalf("Bad: %#v", result)
	}
	if result.RefreshIndex == 0 {
		t.Fatalf("Bad: %#v", result)
	}
	if len(result.NodeAllocation) != 0 {
		t.Fatalf("Bad: %#v", result)
	}

	// Should have an update
	if state == nil {
		t.Fatalf("expected state update")
	}
}

func TestWorker_UpdateEval(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)
	evalOut, token, err := s1.evalBroker.Dequeue([]string{eval1.Type}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if evalOut != eval1 {
		t.Fatalf("Bad eval")
	}

	eval2 := evalOut.Copy()
	eval2.Status = structs.EvalStatusComplete

	// Attempt to update eval
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	w.evalToken = token

	err = w.UpdateEval(eval2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := s1.fsm.State().EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.Status != structs.EvalStatusComplete {
		t.Fatalf("bad: %v", out)
	}
	if out.SnapshotIndex != w.snapshotIndex {
		t.Fatalf("bad: %v", out)
	}
}

func TestWorker_CreateEval(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	evalOut, token, err := s1.evalBroker.Dequeue([]string{eval1.Type}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if evalOut != eval1 {
		t.Fatalf("Bad eval")
	}

	eval2 := mock.Eval()
	eval2.PreviousEval = eval1.ID

	// Attempt to create eval
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	w.evalToken = token

	err = w.CreateEval(eval2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := s1.fsm.State().EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.PreviousEval != eval1.ID {
		t.Fatalf("bad: %v", out)
	}
	if out.SnapshotIndex != w.snapshotIndex {
		t.Fatalf("bad: %v", out)
	}
}

func TestWorker_ReblockEval(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the blocked eval
	eval1 := mock.Eval()
	eval1.Status = structs.EvalStatusBlocked
	eval1.QueuedAllocations = map[string]int{"cache": 100}

	// Insert it into the state store
	if err := s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1}); err != nil {
		t.Fatal(err)
	}

	// Create the job summary
	js := mock.JobSummary(eval1.JobID)
	tg := js.Summary["web"]
	tg.Queued = 100
	js.Summary["web"] = tg
	if err := s1.fsm.State().UpsertJobSummary(1001, js); err != nil {
		t.Fatal(err)
	}

	// Enqueue the eval and then dequeue
	s1.evalBroker.Enqueue(eval1)
	evalOut, token, err := s1.evalBroker.Dequeue([]string{eval1.Type}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if evalOut != eval1 {
		t.Fatalf("Bad eval")
	}

	eval2 := evalOut.Copy()
	eval2.QueuedAllocations = map[string]int{"web": 50}

	// Attempt to reblock eval
	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()
	w := newWorker(s1.shutdownCtx, s1, poolArgs)
	w.evalToken = token

	err = w.ReblockEval(eval2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ack the eval
	w.sendAck(evalOut, token)

	// Check that it is blocked
	bStats := s1.blockedEvals.Stats()
	if bStats.TotalBlocked+bStats.TotalEscaped != 1 {
		t.Fatalf("ReblockEval didn't insert eval into the blocked eval tracker: %#v", bStats)
	}

	// Check that the eval was updated
	ws := memdb.NewWatchSet()
	eval, err := s1.fsm.State().EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(eval.QueuedAllocations, eval2.QueuedAllocations) {
		t.Fatalf("expected: %#v, actual: %#v", eval2.QueuedAllocations, eval.QueuedAllocations)
	}

	// Check that the snapshot index was set properly by unblocking the eval and
	// then dequeuing.
	s1.blockedEvals.Unblock("foobar", 1000)

	reblockedEval, _, err := s1.evalBroker.Dequeue([]string{eval1.Type}, 1*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if reblockedEval == nil {
		t.Fatalf("Nil eval")
	}
	if reblockedEval.ID != eval1.ID {
		t.Fatalf("Bad eval")
	}

	// Check that the SnapshotIndex is set
	if reblockedEval.SnapshotIndex != w.snapshotIndex {
		t.Fatalf("incorrect snapshot index; got %d; want %d",
			reblockedEval.SnapshotIndex, w.snapshotIndex)
	}
}

func TestWorker_Info(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.EnabledSchedulers = []string{structs.JobTypeService}
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	poolArgs := getSchedulerWorkerPoolArgsFromConfigLocked(s1.config).Copy()

	// Create a worker
	w := newWorker(s1.shutdownCtx, s1, poolArgs)

	require.Equal(t, WorkerStarting, w.GetStatus())
	workerInfo := w.Info()
	require.Equal(t, WorkerStarting.String(), workerInfo.Status)
}

const (
	longWait = 100 * time.Millisecond
	tinyWait = 10 * time.Millisecond
)

func TestWorker_SetPause(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)
	srv := &Server{
		logger:      logger,
		shutdownCtx: context.Background(),
	}
	args := SchedulerWorkerPoolArgs{
		EnabledSchedulers: []string{structs.JobTypeCore, structs.JobTypeBatch, structs.JobTypeSystem},
	}
	w := newWorker(context.Background(), srv, args)
	w._start(testWorkload)
	require.Eventually(t, w.IsStarted, longWait, tinyWait, "should have started")

	go func() {
		time.Sleep(tinyWait)
		w.Pause()
	}()
	require.Eventually(t, w.IsPaused, longWait, tinyWait, "should have paused")

	go func() {
		time.Sleep(tinyWait)
		w.Pause()
	}()
	require.Eventually(t, w.IsPaused, longWait, tinyWait, "pausing a paused should be okay")

	go func() {
		time.Sleep(tinyWait)
		w.Resume()
	}()
	require.Eventually(t, w.IsStarted, longWait, tinyWait, "should have restarted from pause")

	go func() {
		time.Sleep(tinyWait)
		w.Stop()
	}()
	require.Eventually(t, w.IsStopped, longWait, tinyWait, "should have shutdown")
}

func TestWorker_SetPause_OutOfOrderEvents(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)
	srv := &Server{
		logger:      logger,
		shutdownCtx: context.Background(),
	}
	args := SchedulerWorkerPoolArgs{
		EnabledSchedulers: []string{structs.JobTypeCore, structs.JobTypeBatch, structs.JobTypeSystem},
	}
	w := newWorker(context.Background(), srv, args)
	w._start(testWorkload)
	require.Eventually(t, w.IsStarted, longWait, tinyWait, "should have started")

	go func() {
		time.Sleep(tinyWait)
		w.Pause()
	}()
	require.Eventually(t, w.IsPaused, longWait, tinyWait, "should have paused")

	go func() {
		time.Sleep(tinyWait)
		w.Stop()
	}()
	require.Eventually(t, w.IsStopped, longWait, tinyWait, "stop from pause should have shutdown")

	go func() {
		time.Sleep(tinyWait)
		w.Pause()
	}()
	require.Eventually(t, w.IsStopped, longWait, tinyWait, "pausing a stopped should stay stopped")

}

// _start is a test helper function used to start a worker with an alternate workload
func (w *Worker) _start(inFunc func(w *Worker)) {
	w.setStatus(WorkerStarting)
	go inFunc(w)
}

// testWorkload is a very simple function that performs the same status updating behaviors that the
// real workload does.
func testWorkload(w *Worker) {
	defer w.markStopped()
	w.setStatuses(WorkerStarted, WorkloadRunning)
	w.logger.Debug("testWorkload running")
	for {
		// ensure state variables are happy after resuming.
		w.maybeWait()
		if w.workerShuttingDown() {
			w.logger.Debug("testWorkload stopped")
			return
		}
		// do some fake work
		time.Sleep(10 * time.Millisecond)
	}
}
