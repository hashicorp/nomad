// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoreScheduler_EvalGC(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	store := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	store.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := store.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	require.Nil(t, err)

	// Insert mock job with rescheduling disabled
	job := mock.Job()
	job.ID = eval.JobID
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err = store.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job)
	require.Nil(t, err)

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	alloc.JobID = eval.JobID
	alloc.TaskGroup = job.TaskGroups[0].Name

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusLost
	alloc2.JobID = eval.JobID
	alloc2.TaskGroup = job.TaskGroups[0].Name
	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobEvalGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone
	ws := memdb.NewWatchSet()
	out, err := store.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := store.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err := store.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 != nil {
		t.Fatalf("bad: %v", outA2)
	}
}

// Tests GC behavior on allocations being rescheduled
func TestCoreScheduler_EvalGC_ReschedulingAllocs(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	store := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	store.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := store.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	require.Nil(t, err)

	// Insert "pending" eval for same job
	eval2 := mock.Eval()
	eval2.JobID = eval.JobID
	store.UpsertJobSummary(999, mock.JobSummary(eval2.JobID))
	err = store.UpsertEvals(structs.MsgTypeTestSetup, 1003, []*structs.Evaluation{eval2})
	require.Nil(t, err)

	// Insert mock job with default reschedule policy of 2 in 10 minutes
	job := mock.Job()
	job.ID = eval.JobID

	err = store.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job)
	require.Nil(t, err)

	// Insert failed alloc with an old reschedule attempt, can be GCed
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusFailed
	alloc.JobID = eval.JobID
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.NextAllocation = uuid.Generate()
	alloc.RescheduleTracker = &structs.RescheduleTracker{
		Events: []*structs.RescheduleEvent{
			{
				RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
				PrevNodeID:     uuid.Generate(),
				PrevAllocID:    uuid.Generate(),
			},
		},
	}

	alloc2 := mock.Alloc()
	alloc2.Job = job
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusFailed
	alloc2.JobID = eval.JobID
	alloc2.TaskGroup = job.TaskGroups[0].Name
	alloc2.RescheduleTracker = &structs.RescheduleTracker{
		Events: []*structs.RescheduleEvent{
			{
				RescheduleTime: time.Now().Add(-3 * time.Minute).UTC().UnixNano(),
				PrevNodeID:     uuid.Generate(),
				PrevAllocID:    uuid.Generate(),
			},
		},
	}
	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc, alloc2})
	require.Nil(t, err)

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC, job has all terminal allocs and one pending eval
	gc := s1.coreJobEval(structs.CoreJobEvalGC, 2000)
	err = core.Process(gc)
	require.Nil(t, err)

	// Eval should still exist
	ws := memdb.NewWatchSet()
	out, err := store.EvalByID(ws, eval.ID)
	require.Nil(t, err)
	require.NotNil(t, out)
	require.Equal(t, eval.ID, out.ID)

	outA, err := store.AllocByID(ws, alloc.ID)
	require.Nil(t, err)
	require.Nil(t, outA)

	outA2, err := store.AllocByID(ws, alloc2.ID)
	require.Nil(t, err)
	require.Equal(t, alloc2.ID, outA2.ID)

}

// Tests GC behavior on stopped job with reschedulable allocs
func TestCoreScheduler_EvalGC_StoppedJob_Reschedulable(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	store := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	store.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := store.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	require.Nil(t, err)

	// Insert mock stopped job with default reschedule policy of 2 in 10 minutes
	job := mock.Job()
	job.ID = eval.JobID
	job.Stop = true

	err = store.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job)
	require.Nil(t, err)

	// Insert failed alloc with a recent reschedule attempt
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusLost
	alloc.JobID = eval.JobID
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.RescheduleTracker = &structs.RescheduleTracker{
		Events: []*structs.RescheduleEvent{
			{
				RescheduleTime: time.Now().Add(-3 * time.Minute).UTC().UnixNano(),
				PrevNodeID:     uuid.Generate(),
				PrevAllocID:    uuid.Generate(),
			},
		},
	}
	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	require.Nil(t, err)

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobEvalGC, 2000)
	err = core.Process(gc)
	require.Nil(t, err)

	// Eval should not exist
	ws := memdb.NewWatchSet()
	out, err := store.EvalByID(ws, eval.ID)
	require.Nil(t, err)
	require.Nil(t, out)

	// Alloc should not exist
	outA, err := store.AllocByID(ws, alloc.ID)
	require.Nil(t, err)
	require.Nil(t, outA)

}

// An EvalGC should never reap a batch job that has not been stopped
func TestCoreScheduler_EvalGC_Batch(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		// Set EvalGCThreshold past BatchEvalThreshold to make sure that only
		// BatchEvalThreshold affects the results.
		c.BatchEvalGCThreshold = time.Hour
		c.EvalGCThreshold = 2 * time.Hour
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 2, 10)

	var jobModifyIdx uint64 = 1000

	// A "stopped" job containing one "complete" eval with one terminal allocation.
	store := s1.fsm.State()
	stoppedJob := mock.Job()
	stoppedJob.Type = structs.JobTypeBatch
	stoppedJob.Status = structs.JobStatusDead
	stoppedJob.Stop = true
	stoppedJob.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err := store.UpsertJob(structs.MsgTypeTestSetup, jobModifyIdx+1, nil, stoppedJob)
	must.NoError(t, err)

	stoppedJobEval := mock.Eval()
	stoppedJobEval.Status = structs.EvalStatusComplete
	stoppedJobEval.Type = structs.JobTypeBatch
	stoppedJobEval.JobID = stoppedJob.ID
	err = store.UpsertEvals(structs.MsgTypeTestSetup, jobModifyIdx+2, []*structs.Evaluation{stoppedJobEval})
	must.NoError(t, err)

	stoppedJobStoppedAlloc := mock.Alloc()
	stoppedJobStoppedAlloc.Job = stoppedJob
	stoppedJobStoppedAlloc.JobID = stoppedJob.ID
	stoppedJobStoppedAlloc.EvalID = stoppedJobEval.ID
	stoppedJobStoppedAlloc.DesiredStatus = structs.AllocDesiredStatusStop
	stoppedJobStoppedAlloc.ClientStatus = structs.AllocClientStatusFailed

	stoppedJobLostAlloc := mock.Alloc()
	stoppedJobLostAlloc.Job = stoppedJob
	stoppedJobLostAlloc.JobID = stoppedJob.ID
	stoppedJobLostAlloc.EvalID = stoppedJobEval.ID
	stoppedJobLostAlloc.DesiredStatus = structs.AllocDesiredStatusRun
	stoppedJobLostAlloc.ClientStatus = structs.AllocClientStatusLost

	err = store.UpsertAllocs(structs.MsgTypeTestSetup, jobModifyIdx+3, []*structs.Allocation{stoppedJobStoppedAlloc, stoppedJobLostAlloc})
	must.NoError(t, err)

	// A "dead" job containing one "complete" eval with:
	//	1. A "stopped" alloc
	//	2. A "lost" alloc
	// Both allocs upserted at 1002.
	deadJob := mock.Job()
	deadJob.Type = structs.JobTypeBatch
	deadJob.Status = structs.JobStatusDead
	err = store.UpsertJob(structs.MsgTypeTestSetup, jobModifyIdx, nil, deadJob)
	must.NoError(t, err)

	deadJobEval := mock.Eval()
	deadJobEval.Status = structs.EvalStatusComplete
	deadJobEval.Type = structs.JobTypeBatch
	deadJobEval.JobID = deadJob.ID
	err = store.UpsertEvals(structs.MsgTypeTestSetup, jobModifyIdx+1, []*structs.Evaluation{deadJobEval})
	must.NoError(t, err)

	stoppedAlloc := mock.Alloc()
	stoppedAlloc.Job = deadJob
	stoppedAlloc.JobID = deadJob.ID
	stoppedAlloc.EvalID = deadJobEval.ID
	stoppedAlloc.DesiredStatus = structs.AllocDesiredStatusStop
	stoppedAlloc.ClientStatus = structs.AllocClientStatusFailed

	lostAlloc := mock.Alloc()
	lostAlloc.Job = deadJob
	lostAlloc.JobID = deadJob.ID
	lostAlloc.EvalID = deadJobEval.ID
	lostAlloc.DesiredStatus = structs.AllocDesiredStatusRun
	lostAlloc.ClientStatus = structs.AllocClientStatusLost

	err = store.UpsertAllocs(structs.MsgTypeTestSetup, jobModifyIdx+2, []*structs.Allocation{stoppedAlloc, lostAlloc})
	must.NoError(t, err)

	// An "alive" job #2 containing two complete evals. The first with:
	//	1. A "lost" alloc
	//	2. A "running" alloc
	// Both allocs upserted at 999
	//
	// The second with just terminal allocs:
	//	1. A "completed" alloc
	// All allocs upserted at 999. The eval upserted at 999 as well.
	activeJob := mock.Job()
	activeJob.Type = structs.JobTypeBatch
	activeJob.Status = structs.JobStatusDead
	err = store.UpsertJob(structs.MsgTypeTestSetup, jobModifyIdx, nil, activeJob)
	must.NoError(t, err)

	activeJobEval := mock.Eval()
	activeJobEval.Status = structs.EvalStatusComplete
	activeJobEval.Type = structs.JobTypeBatch
	activeJobEval.JobID = activeJob.ID
	err = store.UpsertEvals(structs.MsgTypeTestSetup, jobModifyIdx+1, []*structs.Evaluation{activeJobEval})
	must.NoError(t, err)

	activeJobRunningAlloc := mock.Alloc()
	activeJobRunningAlloc.Job = activeJob
	activeJobRunningAlloc.JobID = activeJob.ID
	activeJobRunningAlloc.EvalID = activeJobEval.ID
	activeJobRunningAlloc.DesiredStatus = structs.AllocDesiredStatusRun
	activeJobRunningAlloc.ClientStatus = structs.AllocClientStatusRunning

	activeJobLostAlloc := mock.Alloc()
	activeJobLostAlloc.Job = activeJob
	activeJobLostAlloc.JobID = activeJob.ID
	activeJobLostAlloc.EvalID = activeJobEval.ID
	activeJobLostAlloc.DesiredStatus = structs.AllocDesiredStatusRun
	activeJobLostAlloc.ClientStatus = structs.AllocClientStatusLost

	err = store.UpsertAllocs(structs.MsgTypeTestSetup, jobModifyIdx-1, []*structs.Allocation{activeJobRunningAlloc, activeJobLostAlloc})
	must.NoError(t, err)

	activeJobCompleteEval := mock.Eval()
	activeJobCompleteEval.Status = structs.EvalStatusComplete
	activeJobCompleteEval.Type = structs.JobTypeBatch
	activeJobCompleteEval.JobID = activeJob.ID
	err = store.UpsertEvals(structs.MsgTypeTestSetup, jobModifyIdx-1, []*structs.Evaluation{activeJobCompleteEval})
	must.NoError(t, err)

	activeJobCompletedEvalCompletedAlloc := mock.Alloc()
	activeJobCompletedEvalCompletedAlloc.Job = activeJob
	activeJobCompletedEvalCompletedAlloc.JobID = activeJob.ID
	activeJobCompletedEvalCompletedAlloc.EvalID = activeJobCompleteEval.ID
	activeJobCompletedEvalCompletedAlloc.DesiredStatus = structs.AllocDesiredStatusStop
	activeJobCompletedEvalCompletedAlloc.ClientStatus = structs.AllocClientStatusComplete

	err = store.UpsertAllocs(structs.MsgTypeTestSetup, jobModifyIdx-1, []*structs.Allocation{activeJobCompletedEvalCompletedAlloc})
	must.NoError(t, err)

	// A job that ran once and was then purged.
	purgedJob := mock.Job()
	purgedJob.Type = structs.JobTypeBatch
	purgedJob.Status = structs.JobStatusDead
	err = store.UpsertJob(structs.MsgTypeTestSetup, jobModifyIdx, nil, purgedJob)
	must.NoError(t, err)

	purgedJobEval := mock.Eval()
	purgedJobEval.Status = structs.EvalStatusComplete
	purgedJobEval.Type = structs.JobTypeBatch
	purgedJobEval.JobID = purgedJob.ID
	err = store.UpsertEvals(structs.MsgTypeTestSetup, jobModifyIdx+1, []*structs.Evaluation{purgedJobEval})
	must.NoError(t, err)

	purgedJobCompleteAlloc := mock.Alloc()
	purgedJobCompleteAlloc.Job = purgedJob
	purgedJobCompleteAlloc.JobID = purgedJob.ID
	purgedJobCompleteAlloc.EvalID = purgedJobEval.ID
	purgedJobCompleteAlloc.DesiredStatus = structs.AllocDesiredStatusRun
	purgedJobCompleteAlloc.ClientStatus = structs.AllocClientStatusLost

	err = store.UpsertAllocs(structs.MsgTypeTestSetup, jobModifyIdx-1, []*structs.Allocation{purgedJobCompleteAlloc})
	must.NoError(t, err)

	purgedJobCompleteEval := mock.Eval()
	purgedJobCompleteEval.Status = structs.EvalStatusComplete
	purgedJobCompleteEval.Type = structs.JobTypeBatch
	purgedJobCompleteEval.JobID = purgedJob.ID
	err = store.UpsertEvals(structs.MsgTypeTestSetup, jobModifyIdx-1, []*structs.Evaluation{purgedJobCompleteEval})
	must.NoError(t, err)

	// Purge job.
	err = store.DeleteJob(jobModifyIdx, purgedJob.Namespace, purgedJob.ID)
	must.NoError(t, err)

	// A little helper for assertions
	assertCorrectJobEvalAlloc := func(
		ws memdb.WatchSet,
		jobsShouldExist []*structs.Job,
		jobsShouldNotExist []*structs.Job,
		evalsShouldExist []*structs.Evaluation,
		evalsShouldNotExist []*structs.Evaluation,
		allocsShouldExist []*structs.Allocation,
		allocsShouldNotExist []*structs.Allocation,
	) {
		t.Helper()
		for _, job := range jobsShouldExist {
			out, err := store.JobByID(ws, job.Namespace, job.ID)
			must.NoError(t, err)
			must.NotNil(t, out)
		}

		for _, job := range jobsShouldNotExist {
			out, err := store.JobByID(ws, job.Namespace, job.ID)
			must.NoError(t, err)
			must.Nil(t, out)
		}

		for _, eval := range evalsShouldExist {
			out, err := store.EvalByID(ws, eval.ID)
			must.NoError(t, err)
			must.NotNil(t, out)
		}

		for _, eval := range evalsShouldNotExist {
			out, err := store.EvalByID(ws, eval.ID)
			must.NoError(t, err)
			must.Nil(t, out)
		}

		for _, alloc := range allocsShouldExist {
			outA, err := store.AllocByID(ws, alloc.ID)
			must.NoError(t, err)
			must.NotNil(t, outA)
		}

		for _, alloc := range allocsShouldNotExist {
			outA, err := store.AllocByID(ws, alloc.ID)
			must.NoError(t, err)
			must.Nil(t, outA)
		}
	}

	// Create a core scheduler
	snap, err := store.Snapshot()
	must.NoError(t, err)
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC without moving the time at all
	gc := s1.coreJobEval(structs.CoreJobEvalGC, jobModifyIdx)
	err = core.Process(gc)
	must.NoError(t, err)

	// Nothing is gone
	assertCorrectJobEvalAlloc(
		memdb.NewWatchSet(),
		[]*structs.Job{deadJob, activeJob, stoppedJob},
		[]*structs.Job{},
		[]*structs.Evaluation{
			deadJobEval,
			activeJobEval, activeJobCompleteEval,
			stoppedJobEval,
			purgedJobEval,
		},
		[]*structs.Evaluation{},
		[]*structs.Allocation{
			stoppedAlloc, lostAlloc,
			activeJobRunningAlloc, activeJobLostAlloc, activeJobCompletedEvalCompletedAlloc,
			stoppedJobStoppedAlloc, stoppedJobLostAlloc,
			purgedJobCompleteAlloc,
		},
		[]*structs.Allocation{},
	)

	// Update the time tables by half of the BatchEvalGCThreshold which is too
	// small to GC anything.
	tt := s1.fsm.TimeTable()
	tt.Witness(2*jobModifyIdx, time.Now().UTC().Add((-1)*s1.config.BatchEvalGCThreshold/2))

	gc = s1.coreJobEval(structs.CoreJobEvalGC, jobModifyIdx*2)
	err = core.Process(gc)
	must.NoError(t, err)

	// Nothing is gone.
	assertCorrectJobEvalAlloc(
		memdb.NewWatchSet(),
		[]*structs.Job{deadJob, activeJob, stoppedJob},
		[]*structs.Job{},
		[]*structs.Evaluation{
			deadJobEval,
			activeJobEval, activeJobCompleteEval,
			stoppedJobEval,
			purgedJobEval,
		},
		[]*structs.Evaluation{},
		[]*structs.Allocation{
			stoppedAlloc, lostAlloc,
			activeJobRunningAlloc, activeJobLostAlloc, activeJobCompletedEvalCompletedAlloc,
			stoppedJobStoppedAlloc, stoppedJobLostAlloc,
			purgedJobCompleteAlloc,
		},
		[]*structs.Allocation{},
	)

	// Update the time tables so that BatchEvalGCThreshold has elapsed.
	s1.fsm.timetable.table = make([]TimeTableEntry, 2, 10)
	tt = s1.fsm.TimeTable()
	tt.Witness(2*jobModifyIdx, time.Now().UTC().Add(-1*s1.config.BatchEvalGCThreshold))

	gc = s1.coreJobEval(structs.CoreJobEvalGC, jobModifyIdx*2)
	err = core.Process(gc)
	must.NoError(t, err)

	// We expect the following:
	//
	//	1. The stopped job remains, but its evaluation and allocations are both removed.
	//	2. The dead job remains with its evaluation and allocations intact. This is because
	//    for them the BatchEvalGCThreshold has not yet elapsed (their modification idx are larger
	//    than that of the job).
	//	3. The active job remains since it is active, even though the allocations are otherwise
	//      eligible for GC. However, the inactive allocation is GCed for it.
	//	4. The eval and allocation for the purged job are GCed.
	assertCorrectJobEvalAlloc(
		memdb.NewWatchSet(),
		[]*structs.Job{deadJob, activeJob, stoppedJob},
		[]*structs.Job{},
		[]*structs.Evaluation{deadJobEval, activeJobEval},
		[]*structs.Evaluation{activeJobCompleteEval, stoppedJobEval, purgedJobEval},
		[]*structs.Allocation{stoppedAlloc, lostAlloc, activeJobRunningAlloc},
		[]*structs.Allocation{
			activeJobLostAlloc, activeJobCompletedEvalCompletedAlloc,
			stoppedJobLostAlloc, stoppedJobLostAlloc,
			purgedJobCompleteAlloc,
		})
}

func TestCoreScheduler_EvalGC_Partial(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	store := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	store.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := store.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create mock job with id same as eval
	job := mock.Job()
	job.ID = eval.JobID

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	alloc.TaskGroup = job.TaskGroups[0].Name
	store.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
	alloc2.JobID = job.ID
	alloc2.EvalID = eval.ID
	alloc2.TaskGroup = job.TaskGroups[0].Name
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusLost

	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "running" alloc
	alloc3 := mock.Alloc()
	alloc3.EvalID = eval.ID
	alloc3.JobID = job.ID
	store.UpsertJobSummary(1003, mock.JobSummary(alloc3.JobID))
	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1004, []*structs.Allocation{alloc3})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert mock job with rescheduling disabled
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err = store.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job)
	require.Nil(t, err)

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobEvalGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should not be gone
	ws := memdb.NewWatchSet()
	out, err := store.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := store.AllocByID(ws, alloc3.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}

	// Should be gone
	outB, err := store.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outB != nil {
		t.Fatalf("bad: %v", outB)
	}

	outC, err := store.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outC != nil {
		t.Fatalf("bad: %v", outC)
	}
}

func TestCoreScheduler_EvalGC_Force(t *testing.T) {
	ci.Parallel(t)
	for _, withAcl := range []bool{false, true} {
		t.Run(fmt.Sprintf("with acl %v", withAcl), func(t *testing.T) {
			var server *Server
			var cleanup func()
			if withAcl {
				server, _, cleanup = TestACLServer(t, nil)
			} else {
				server, cleanup = TestServer(t, nil)
			}
			defer cleanup()
			testutil.WaitForLeader(t, server.RPC)

			// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
			server.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

			// Insert "dead" eval
			store := server.fsm.State()
			eval := mock.Eval()
			eval.Status = structs.EvalStatusFailed
			store.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
			err := store.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Insert mock job with rescheduling disabled
			job := mock.Job()
			job.ID = eval.JobID
			job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
				Attempts: 0,
				Interval: 0 * time.Second,
			}
			err = store.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job)
			require.Nil(t, err)

			// Insert "dead" alloc
			alloc := mock.Alloc()
			alloc.EvalID = eval.ID
			alloc.DesiredStatus = structs.AllocDesiredStatusStop
			alloc.TaskGroup = job.TaskGroups[0].Name
			store.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))
			err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc})
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Create a core scheduler
			snap, err := store.Snapshot()
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			core := NewCoreScheduler(server, snap)

			// Attempt the GC
			gc := server.coreJobEval(structs.CoreJobForceGC, 1002)
			err = core.Process(gc)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Should be gone
			ws := memdb.NewWatchSet()
			out, err := store.EvalByID(ws, eval.ID)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if out != nil {
				t.Fatalf("bad: %v", out)
			}

			outA, err := store.AllocByID(ws, alloc.ID)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if outA != nil {
				t.Fatalf("bad: %v", outA)
			}
		})
	}
}

func TestCoreScheduler_NodeGC(t *testing.T) {
	ci.Parallel(t)
	for _, withAcl := range []bool{false, true} {
		t.Run(fmt.Sprintf("with acl %v", withAcl), func(t *testing.T) {
			var server *Server
			var cleanup func()
			if withAcl {
				server, _, cleanup = TestACLServer(t, nil)
			} else {
				server, cleanup = TestServer(t, nil)
			}
			defer cleanup()
			testutil.WaitForLeader(t, server.RPC)

			// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
			server.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

			// Insert "dead" node
			store := server.fsm.State()
			node := mock.Node()
			node.Status = structs.NodeStatusDown
			err := store.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Update the time tables to make this work
			tt := server.fsm.TimeTable()
			tt.Witness(2000, time.Now().UTC().Add(-1*server.config.NodeGCThreshold))

			// Create a core scheduler
			snap, err := store.Snapshot()
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			core := NewCoreScheduler(server, snap)

			// Attempt the GC
			gc := server.coreJobEval(structs.CoreJobNodeGC, 2000)
			err = core.Process(gc)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Should be gone
			ws := memdb.NewWatchSet()
			out, err := store.NodeByID(ws, node.ID)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if out != nil {
				t.Fatalf("bad: %v", out)
			}
		})
	}
}

func TestCoreScheduler_NodeGC_TerminalAllocs(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" node
	store := s1.fsm.State()
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	err := store.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert a terminal alloc on that node
	alloc := mock.Alloc()
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	store.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))
	if err := store.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.NodeGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobNodeGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone
	ws := memdb.NewWatchSet()
	out, err := store.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_NodeGC_RunningAllocs(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" node
	store := s1.fsm.State()
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	err := store.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert a running alloc on that node
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusRunning
	store.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))
	if err := store.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.NodeGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobNodeGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should still be here
	ws := memdb.NewWatchSet()
	out, err := store.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_NodeGC_Force(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" node
	store := s1.fsm.State()
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	err := store.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobForceGC, 1000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone
	ws := memdb.NewWatchSet()
	out, err := store.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_JobGC_OutstandingEvals(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	store := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusDead
	err := store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert two evals, one terminal and one not
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusComplete

	eval2 := mock.Eval()
	eval2.JobID = job.ID
	eval2.Status = structs.EvalStatusPending
	err = store.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobJobGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should still exist
	ws := memdb.NewWatchSet()
	out, err := store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outE, err := store.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE == nil {
		t.Fatalf("bad: %v", outE)
	}

	outE2, err := store.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE2 == nil {
		t.Fatalf("bad: %v", outE2)
	}

	// Update the second eval to be terminal
	eval2.Status = structs.EvalStatusComplete
	err = store.UpsertEvals(structs.MsgTypeTestSetup, 1003, []*structs.Evaluation{eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err = store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core = NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc = s1.coreJobEval(structs.CoreJobJobGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should not still exist
	out, err = store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outE, err = store.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE != nil {
		t.Fatalf("bad: %v", outE)
	}

	outE2, err = store.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE2 != nil {
		t.Fatalf("bad: %v", outE2)
	}
}

func TestCoreScheduler_JobGC_OutstandingAllocs(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	store := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusDead
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err := store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert an eval
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusComplete
	err = store.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert two allocs, one terminal and one not
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusComplete
	alloc.TaskGroup = job.TaskGroups[0].Name

	alloc2 := mock.Alloc()
	alloc2.JobID = job.ID
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	alloc2.TaskGroup = job.TaskGroups[0].Name

	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobJobGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should still exist
	ws := memdb.NewWatchSet()
	out, err := store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := store.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err := store.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 == nil {
		t.Fatalf("bad: %v", outA2)
	}

	// Update the second alloc to be terminal
	alloc2.ClientStatus = structs.AllocClientStatusComplete
	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err = store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core = NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc = s1.coreJobEval(structs.CoreJobJobGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should not still exist
	out, err = store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err = store.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err = store.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 != nil {
		t.Fatalf("bad: %v", outA2)
	}
}

// This test ensures that batch jobs are GC'd in one shot, meaning it all
// allocs/evals and job or nothing
func TestCoreScheduler_JobGC_OneShot(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	store := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	err := store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert two complete evals
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusComplete

	eval2 := mock.Eval()
	eval2.JobID = job.ID
	eval2.Status = structs.EvalStatusComplete

	err = store.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert one complete alloc and one running on distinct evals
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop

	alloc2 := mock.Alloc()
	alloc2.JobID = job.ID
	alloc2.EvalID = eval2.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun

	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force the jobs state to dead
	job.Status = structs.JobStatusDead

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobJobGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should still exist
	ws := memdb.NewWatchSet()
	out, err := store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outE, err := store.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE == nil {
		t.Fatalf("bad: %v", outE)
	}

	outE2, err := store.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE2 == nil {
		t.Fatalf("bad: %v", outE2)
	}

	outA, err := store.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}
	outA2, err := store.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 == nil {
		t.Fatalf("bad: %v", outA2)
	}
}

// This test ensures that stopped jobs are GCd
func TestCoreScheduler_JobGC_Stopped(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	store := s1.fsm.State()
	job := mock.Job()
	job.Stop = true
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err := store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert two complete evals
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusComplete

	eval2 := mock.Eval()
	eval2.JobID = job.ID
	eval2.Status = structs.EvalStatusComplete

	err = store.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert one complete alloc
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	alloc.TaskGroup = job.TaskGroups[0].Name
	err = store.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobJobGC, 2000)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Shouldn't still exist
	ws := memdb.NewWatchSet()
	out, err := store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outE, err := store.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE != nil {
		t.Fatalf("bad: %v", outE)
	}

	outE2, err := store.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE2 != nil {
		t.Fatalf("bad: %v", outE2)
	}

	outA, err := store.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}
}

func TestCoreScheduler_JobGC_Force(t *testing.T) {
	ci.Parallel(t)
	for _, withAcl := range []bool{false, true} {
		t.Run(fmt.Sprintf("with acl %v", withAcl), func(t *testing.T) {
			var server *Server
			var cleanup func()
			if withAcl {
				server, _, cleanup = TestACLServer(t, nil)
			} else {
				server, cleanup = TestServer(t, nil)
			}
			defer cleanup()
			testutil.WaitForLeader(t, server.RPC)

			// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
			server.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

			// Insert job.
			store := server.fsm.State()
			job := mock.Job()
			job.Type = structs.JobTypeBatch
			job.Status = structs.JobStatusDead
			err := store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Insert a terminal eval
			eval := mock.Eval()
			eval.JobID = job.ID
			eval.Status = structs.EvalStatusComplete
			err = store.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval})
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Create a core scheduler
			snap, err := store.Snapshot()
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			core := NewCoreScheduler(server, snap)

			// Attempt the GC
			gc := server.coreJobEval(structs.CoreJobForceGC, 1002)
			err = core.Process(gc)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Shouldn't still exist
			ws := memdb.NewWatchSet()
			out, err := store.JobByID(ws, job.Namespace, job.ID)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if out != nil {
				t.Fatalf("bad: %v", out)
			}

			outE, err := store.EvalByID(ws, eval.ID)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if outE != nil {
				t.Fatalf("bad: %v", outE)
			}
		})
	}
}

// This test ensures parameterized jobs only get gc'd when stopped
func TestCoreScheduler_JobGC_Parameterized(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert a parameterized job.
	store := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusRunning
	job.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadRequired,
	}
	err := store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobForceGC, 1002)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should still exist
	ws := memdb.NewWatchSet()
	out, err := store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	// Mark the job as stopped and try again
	job2 := job.Copy()
	job2.Stop = true
	err = store.UpsertJob(structs.MsgTypeTestSetup, 2000, nil, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err = store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core = NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc = s1.coreJobEval(structs.CoreJobForceGC, 2002)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should not exist
	out, err = store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %+v", out)
	}
}

// This test ensures periodic jobs don't get GCd until they are stopped
func TestCoreScheduler_JobGC_Periodic(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert a parameterized job.
	store := s1.fsm.State()
	job := mock.PeriodicJob()
	err := store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobForceGC, 1002)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should still exist
	ws := memdb.NewWatchSet()
	out, err := store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	// Mark the job as stopped and try again
	job2 := job.Copy()
	job2.Stop = true
	err = store.UpsertJob(structs.MsgTypeTestSetup, 2000, nil, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err = store.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core = NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc = s1.coreJobEval(structs.CoreJobForceGC, 2002)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should not exist
	out, err = store.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %+v", out)
	}
}

func TestCoreScheduler_DeploymentGC(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert an active, terminal, and terminal with allocations deployment
	store := s1.fsm.State()
	d1, d2, d3 := mock.Deployment(), mock.Deployment(), mock.Deployment()
	d1.Status = structs.DeploymentStatusFailed
	d3.Status = structs.DeploymentStatusSuccessful
	assert.Nil(store.UpsertDeployment(1000, d1), "UpsertDeployment")
	assert.Nil(store.UpsertDeployment(1001, d2), "UpsertDeployment")
	assert.Nil(store.UpsertDeployment(1002, d3), "UpsertDeployment")

	a := mock.Alloc()
	a.JobID = d3.JobID
	a.DeploymentID = d3.ID
	assert.Nil(store.UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{a}), "UpsertAllocs")

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.DeploymentGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	assert.Nil(err, "Snapshot")
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobDeploymentGC, 2000)
	assert.Nil(core.Process(gc), "Process GC")

	// Should be gone
	ws := memdb.NewWatchSet()
	out, err := store.DeploymentByID(ws, d1.ID)
	assert.Nil(err, "DeploymentByID")
	assert.Nil(out, "Terminal Deployment")
	out2, err := store.DeploymentByID(ws, d2.ID)
	assert.Nil(err, "DeploymentByID")
	assert.NotNil(out2, "Active Deployment")
	out3, err := store.DeploymentByID(ws, d3.ID)
	assert.Nil(err, "DeploymentByID")
	assert.NotNil(out3, "Terminal Deployment With Allocs")
}

func TestCoreScheduler_DeploymentGC_Force(t *testing.T) {
	ci.Parallel(t)
	for _, withAcl := range []bool{false, true} {
		t.Run(fmt.Sprintf("with acl %v", withAcl), func(t *testing.T) {
			var server *Server
			var cleanup func()
			if withAcl {
				server, _, cleanup = TestACLServer(t, nil)
			} else {
				server, cleanup = TestServer(t, nil)
			}
			defer cleanup()
			testutil.WaitForLeader(t, server.RPC)
			assert := assert.New(t)

			// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
			server.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

			// Insert terminal and active deployment
			store := server.fsm.State()
			d1, d2 := mock.Deployment(), mock.Deployment()
			d1.Status = structs.DeploymentStatusFailed
			assert.Nil(store.UpsertDeployment(1000, d1), "UpsertDeployment")
			assert.Nil(store.UpsertDeployment(1001, d2), "UpsertDeployment")

			// Create a core scheduler
			snap, err := store.Snapshot()
			assert.Nil(err, "Snapshot")
			core := NewCoreScheduler(server, snap)

			// Attempt the GC
			gc := server.coreJobEval(structs.CoreJobForceGC, 1000)
			assert.Nil(core.Process(gc), "Process Force GC")

			// Should be gone
			ws := memdb.NewWatchSet()
			out, err := store.DeploymentByID(ws, d1.ID)
			assert.Nil(err, "DeploymentByID")
			assert.Nil(out, "Terminal Deployment")
			out2, err := store.DeploymentByID(ws, d2.ID)
			assert.Nil(err, "DeploymentByID")
			assert.NotNil(out2, "Active Deployment")
		})
	}
}

func TestCoreScheduler_PartitionEvalReap(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Create a core scheduler
	snap, err := s1.fsm.State().Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	evals := []string{"a", "b", "c"}
	allocs := []string{"1", "2", "3"}

	// Set the max ids per reap to something lower.
	requests := core.(*CoreScheduler).partitionEvalReap(evals, allocs, 2)
	if len(requests) != 3 {
		t.Fatalf("Expected 3 requests got: %v", requests)
	}

	first := requests[0]
	if len(first.Allocs) != 2 && len(first.Evals) != 0 {
		t.Fatalf("Unexpected first request: %v", first)
	}

	second := requests[1]
	if len(second.Allocs) != 1 && len(second.Evals) != 1 {
		t.Fatalf("Unexpected second request: %v", second)
	}

	third := requests[2]
	if len(third.Allocs) != 0 && len(third.Evals) != 2 {
		t.Fatalf("Unexpected third request: %v", third)
	}
}

func TestCoreScheduler_PartitionDeploymentReap(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Create a core scheduler
	snap, err := s1.fsm.State().Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	deployments := []string{"a", "b", "c"}
	// Set the max ids per reap to something lower.
	requests := core.(*CoreScheduler).partitionDeploymentReap(deployments, 2)
	if len(requests) != 2 {
		t.Fatalf("Expected 2 requests got: %v", requests)
	}

	first := requests[0]
	if len(first.Deployments) != 2 {
		t.Fatalf("Unexpected first request: %v", first)
	}

	second := requests[1]
	if len(second.Deployments) != 1 {
		t.Fatalf("Unexpected second request: %v", second)
	}
}

func TestCoreScheduler_PartitionJobReap(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a core scheduler
	snap, err := s1.fsm.State().Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)
	jobs := []*structs.Job{mock.Job(), mock.Job(), mock.Job()}

	// Set the max ids per reap to something lower.
	requests := core.(*CoreScheduler).partitionJobReap(jobs, "", 2)
	require.Len(t, requests, 2)

	first := requests[0]
	second := requests[1]
	require.Len(t, first.Jobs, 2)
	require.Len(t, second.Jobs, 1)
}

// Tests various scenarios when allocations are eligible to be GCed
func TestAllocation_GCEligible(t *testing.T) {
	type testCase struct {
		Desc                string
		GCTime              time.Time
		ClientStatus        string
		DesiredStatus       string
		JobStatus           string
		JobStop             bool
		AllocJobModifyIndex uint64
		JobModifyIndex      uint64
		ModifyIndex         uint64
		NextAllocID         string
		ReschedulePolicy    *structs.ReschedulePolicy
		RescheduleTrackers  []*structs.RescheduleEvent
		ThresholdIndex      uint64
		ShouldGC            bool
	}

	fail := time.Now()

	harness := []testCase{
		{
			Desc:           "Don't GC when non terminal",
			ClientStatus:   structs.AllocClientStatusPending,
			DesiredStatus:  structs.AllocDesiredStatusRun,
			GCTime:         fail,
			ModifyIndex:    90,
			ThresholdIndex: 90,
			ShouldGC:       false,
		},
		{
			Desc:           "Don't GC when non terminal and job stopped",
			ClientStatus:   structs.AllocClientStatusPending,
			DesiredStatus:  structs.AllocDesiredStatusRun,
			JobStop:        true,
			GCTime:         fail,
			ModifyIndex:    90,
			ThresholdIndex: 90,
			ShouldGC:       false,
		},
		{
			Desc:           "Don't GC when non terminal and job dead",
			ClientStatus:   structs.AllocClientStatusPending,
			DesiredStatus:  structs.AllocDesiredStatusRun,
			JobStatus:      structs.JobStatusDead,
			GCTime:         fail,
			ModifyIndex:    90,
			ThresholdIndex: 90,
			ShouldGC:       false,
		},
		{
			Desc:           "Don't GC when non terminal on client and job dead",
			ClientStatus:   structs.AllocClientStatusRunning,
			DesiredStatus:  structs.AllocDesiredStatusStop,
			JobStatus:      structs.JobStatusDead,
			GCTime:         fail,
			ModifyIndex:    90,
			ThresholdIndex: 90,
			ShouldGC:       false,
		},
		{
			Desc:             "GC when terminal but not failed ",
			ClientStatus:     structs.AllocClientStatusComplete,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ModifyIndex:      90,
			ThresholdIndex:   90,
			ReschedulePolicy: nil,
			ShouldGC:         true,
		},
		{
			Desc:             "Don't GC when threshold not met",
			ClientStatus:     structs.AllocClientStatusComplete,
			DesiredStatus:    structs.AllocDesiredStatusStop,
			GCTime:           fail,
			ModifyIndex:      100,
			ThresholdIndex:   90,
			ReschedulePolicy: nil,
			ShouldGC:         false,
		},
		{
			Desc:             "GC when no reschedule policy",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ReschedulePolicy: nil,
			ModifyIndex:      90,
			ThresholdIndex:   90,
			ShouldGC:         true,
		},
		{
			Desc:             "GC when empty policy",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ReschedulePolicy: &structs.ReschedulePolicy{Attempts: 0, Interval: 0 * time.Minute},
			ModifyIndex:      90,
			ThresholdIndex:   90,
			ShouldGC:         true,
		},
		{
			Desc:             "Don't GC when no previous reschedule attempts",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ModifyIndex:      90,
			ThresholdIndex:   90,
			ReschedulePolicy: &structs.ReschedulePolicy{Attempts: 1, Interval: 1 * time.Minute},
			ShouldGC:         false,
		},
		{
			Desc:             "Don't GC when prev reschedule attempt within interval",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			ReschedulePolicy: &structs.ReschedulePolicy{Attempts: 2, Interval: 30 * time.Minute},
			GCTime:           fail,
			ModifyIndex:      90,
			ThresholdIndex:   90,
			RescheduleTrackers: []*structs.RescheduleEvent{
				{
					RescheduleTime: fail.Add(-5 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldGC: false,
		},
		{
			Desc:             "GC with prev reschedule attempt outside interval",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ReschedulePolicy: &structs.ReschedulePolicy{Attempts: 5, Interval: 30 * time.Minute},
			RescheduleTrackers: []*structs.RescheduleEvent{
				{
					RescheduleTime: fail.Add(-45 * time.Minute).UTC().UnixNano(),
				},
				{
					RescheduleTime: fail.Add(-60 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldGC: true,
		},
		{
			Desc:             "GC when next alloc id is set",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ReschedulePolicy: &structs.ReschedulePolicy{Attempts: 5, Interval: 30 * time.Minute},
			RescheduleTrackers: []*structs.RescheduleEvent{
				{
					RescheduleTime: fail.Add(-3 * time.Minute).UTC().UnixNano(),
				},
			},
			NextAllocID: uuid.Generate(),
			ShouldGC:    true,
		},
		{
			Desc:             "Don't GC when next alloc id is not set and unlimited restarts",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ReschedulePolicy: &structs.ReschedulePolicy{Unlimited: true, Delay: 5 * time.Second, DelayFunction: "constant"},
			RescheduleTrackers: []*structs.RescheduleEvent{
				{
					RescheduleTime: fail.Add(-3 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldGC: false,
		},
		{
			Desc:             "GC when job is stopped",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ReschedulePolicy: &structs.ReschedulePolicy{Attempts: 5, Interval: 30 * time.Minute},
			RescheduleTrackers: []*structs.RescheduleEvent{
				{
					RescheduleTime: fail.Add(-3 * time.Minute).UTC().UnixNano(),
				},
			},
			JobStop:  true,
			ShouldGC: true,
		},
		{
			Desc:             "GC when job status is dead",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusRun,
			GCTime:           fail,
			ReschedulePolicy: &structs.ReschedulePolicy{Attempts: 5, Interval: 30 * time.Minute},
			RescheduleTrackers: []*structs.RescheduleEvent{
				{
					RescheduleTime: fail.Add(-3 * time.Minute).UTC().UnixNano(),
				},
			},
			JobStatus: structs.JobStatusDead,
			ShouldGC:  true,
		},
		{
			Desc:             "GC when desired status is stop, unlimited reschedule policy, no previous reschedule events",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusStop,
			GCTime:           fail,
			ReschedulePolicy: &structs.ReschedulePolicy{Unlimited: true, Delay: 5 * time.Second, DelayFunction: "constant"},
			ShouldGC:         true,
		},
		{
			Desc:             "GC when desired status is stop, limited reschedule policy, some previous reschedule events",
			ClientStatus:     structs.AllocClientStatusFailed,
			DesiredStatus:    structs.AllocDesiredStatusStop,
			GCTime:           fail,
			ReschedulePolicy: &structs.ReschedulePolicy{Attempts: 5, Interval: 30 * time.Minute},
			RescheduleTrackers: []*structs.RescheduleEvent{
				{
					RescheduleTime: fail.Add(-3 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldGC: true,
		},
	}

	for _, tc := range harness {
		alloc := &structs.Allocation{}
		alloc.ModifyIndex = tc.ModifyIndex
		alloc.DesiredStatus = tc.DesiredStatus
		alloc.ClientStatus = tc.ClientStatus
		alloc.RescheduleTracker = &structs.RescheduleTracker{Events: tc.RescheduleTrackers}
		alloc.NextAllocation = tc.NextAllocID
		job := mock.Job()
		alloc.TaskGroup = job.TaskGroups[0].Name
		job.TaskGroups[0].ReschedulePolicy = tc.ReschedulePolicy
		if tc.JobStatus != "" {
			job.Status = tc.JobStatus
		}
		job.Stop = tc.JobStop

		t.Run(tc.Desc, func(t *testing.T) {
			if got := allocGCEligible(alloc, job, tc.GCTime, tc.ThresholdIndex); got != tc.ShouldGC {
				t.Fatalf("expected %v but got %v", tc.ShouldGC, got)
			}
		})

	}

	// Verify nil job
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete
	require.True(t, allocGCEligible(alloc, nil, time.Now(), 1000))
}

func TestCoreScheduler_CSIPluginGC(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSRV := TestServer(t, nil)
	defer cleanupSRV()
	testutil.WaitForLeader(t, srv.RPC)

	srv.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	deleteNodes := state.CreateTestCSIPlugin(srv.fsm.State(), "foo")
	defer deleteNodes()
	store := srv.fsm.State()

	// Update the time tables to make this work
	tt := srv.fsm.TimeTable()
	index := uint64(2000)
	tt.Witness(index, time.Now().UTC().Add(-1*srv.config.CSIPluginGCThreshold))

	// Create a core scheduler
	snap, err := store.Snapshot()
	require.NoError(t, err)
	core := NewCoreScheduler(srv, snap)

	// Attempt the GC
	index++
	gc := srv.coreJobEval(structs.CoreJobCSIPluginGC, index)
	require.NoError(t, core.Process(gc))

	// Should not be gone (plugin in use)
	ws := memdb.NewWatchSet()
	plug, err := store.CSIPluginByID(ws, "foo")
	require.NotNil(t, plug)
	require.NoError(t, err)

	// Empty the plugin
	plug = plug.Copy()
	plug.Controllers = map[string]*structs.CSIInfo{}
	plug.Nodes = map[string]*structs.CSIInfo{}

	index++
	err = store.UpsertCSIPlugin(index, plug)
	require.NoError(t, err)

	// Retry
	index++
	gc = srv.coreJobEval(structs.CoreJobCSIPluginGC, index)
	require.NoError(t, core.Process(gc))

	// Should be gone
	plug, err = store.CSIPluginByID(ws, "foo")
	require.Nil(t, plug)
	require.NoError(t, err)
}

func TestCoreScheduler_CSIVolumeClaimGC(t *testing.T) {
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	index := uint64(1)
	volID := uuid.Generate()
	ns := structs.DefaultNamespace
	pluginID := "foo"

	store := srv.fsm.State()
	ws := memdb.NewWatchSet()

	index, _ = store.LatestIndex()

	// Create client node and plugin
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.11.0" // needs client RPCs
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		pluginID: {
			PluginID: pluginID,
			Healthy:  true,
			NodeInfo: &structs.CSINodeInfo{},
		},
	}
	index++
	err := store.UpsertNode(structs.MsgTypeTestSetup, index, node)
	require.NoError(t, err)

	// *Important*: for volume writes in this test we must use RPCs
	// rather than StateStore methods directly, or the blocking query
	// in volumewatcher won't get the final update for GC because it's
	// watching on a different store at that point

	// Register a volume
	vols := []*structs.CSIVolume{{
		ID:         volID,
		Namespace:  ns,
		PluginID:   pluginID,
		Topologies: []*structs.CSITopology{},
		RequestedCapabilities: []*structs.CSIVolumeCapability{{
			AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		}},
	}}
	volReq := &structs.CSIVolumeRegisterRequest{Volumes: vols}
	volReq.Namespace = ns
	volReq.Region = srv.config.Region

	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Register",
		volReq, &structs.CSIVolumeRegisterResponse{})
	require.NoError(t, err)

	// Create a job with two allocs that claim the volume.
	// We use two allocs here, one of which is not running, so
	// that we can assert the volumewatcher has made one
	// complete pass (and removed the 2nd alloc) before we
	// run the GC
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	index++
	store.UpsertJobSummary(index, mock.JobSummary(eval.JobID))
	index++
	err = store.UpsertEvals(structs.MsgTypeTestSetup, index, []*structs.Evaluation{eval})
	require.Nil(t, err)

	job := mock.Job()
	job.ID = eval.JobID
	job.Status = structs.JobStatusRunning
	index++
	err = store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job)
	require.NoError(t, err)

	alloc1, alloc2 := mock.Alloc(), mock.Alloc()
	alloc1.NodeID = node.ID
	alloc1.ClientStatus = structs.AllocClientStatusRunning
	alloc1.Job = job
	alloc1.JobID = job.ID
	alloc1.EvalID = eval.ID

	alloc2.NodeID = node.ID
	alloc2.ClientStatus = structs.AllocClientStatusComplete
	alloc2.DesiredStatus = structs.AllocDesiredStatusStop
	alloc2.Job = job
	alloc2.JobID = job.ID
	alloc2.EvalID = eval.ID

	summary := mock.JobSummary(alloc1.JobID)
	index++
	require.NoError(t, store.UpsertJobSummary(index, summary))
	summary = mock.JobSummary(alloc2.JobID)
	index++
	require.NoError(t, store.UpsertJobSummary(index, summary))
	index++
	require.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc1, alloc2}))

	req := &structs.CSIVolumeClaimRequest{
		VolumeID:       volID,
		AllocationID:   alloc1.ID,
		NodeID:         uuid.Generate(), // doesn't exist so we don't get errors trying to unmount volumes from it
		Claim:          structs.CSIVolumeClaimWrite,
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeMultiWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		State:          structs.CSIVolumeClaimStateTaken,
		WriteRequest: structs.WriteRequest{
			Namespace: ns,
			Region:    srv.config.Region,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim",
		req, &structs.CSIVolumeClaimResponse{})
	require.NoError(t, err, "write claim should succeed")

	req.AllocationID = alloc2.ID
	req.State = structs.CSIVolumeClaimStateUnpublishing

	err = msgpackrpc.CallWithCodec(codec, "CSIVolume.Claim",
		req, &structs.CSIVolumeClaimResponse{})
	require.NoError(t, err, "unpublishing claim should succeed")

	require.Eventually(t, func() bool {
		vol, err := store.CSIVolumeByID(ws, ns, volID)
		require.NoError(t, err)
		return len(vol.WriteClaims) == 1 &&
			len(vol.WriteAllocs) == 1 &&
			len(vol.PastClaims) == 0
	}, time.Second*1, 100*time.Millisecond,
		"volumewatcher should have released unpublishing claim without GC")

	// At this point we can guarantee that volumewatcher is waiting
	// for new work. Delete allocation and job so that the next pass
	// thru volumewatcher has more work to do
	index, _ = store.LatestIndex()
	index++
	err = store.DeleteJob(index, ns, job.ID)
	require.NoError(t, err)
	index, _ = store.LatestIndex()
	index++
	err = store.DeleteEval(index, []string{eval.ID}, []string{alloc1.ID}, false)
	require.NoError(t, err)

	// Create a core scheduler and attempt the volume claim GC
	snap, err := store.Snapshot()
	require.NoError(t, err)

	core := NewCoreScheduler(srv, snap)

	index, _ = snap.LatestIndex()
	index++
	gc := srv.coreJobEval(structs.CoreJobForceGC, index)
	c := core.(*CoreScheduler)
	require.NoError(t, c.csiVolumeClaimGC(gc))

	// the only remaining claim is for a deleted alloc with no path to
	// the non-existent node, so volumewatcher will release the
	// remaining claim
	require.Eventually(t, func() bool {
		vol, _ := store.CSIVolumeByID(ws, ns, volID)
		return len(vol.WriteClaims) == 0 &&
			len(vol.WriteAllocs) == 0 &&
			len(vol.PastClaims) == 0
	}, time.Second*2, 10*time.Millisecond, "claims were not released")

}

func TestCoreScheduler_CSIBadState_ClaimGC(t *testing.T) {
	ci.Parallel(t)

	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	err := state.TestBadCSIState(t, srv.State())
	require.NoError(t, err)

	snap, err := srv.State().Snapshot()
	require.NoError(t, err)
	core := NewCoreScheduler(srv, snap)

	index, _ := srv.State().LatestIndex()
	index++
	gc := srv.coreJobEval(structs.CoreJobForceGC, index)
	c := core.(*CoreScheduler)
	require.NoError(t, c.csiVolumeClaimGC(gc))

	require.Eventually(t, func() bool {
		vol, _ := srv.State().CSIVolumeByID(nil,
			structs.DefaultNamespace, "csi-volume-nfs0")
		if len(vol.PastClaims) != 2 {
			return false
		}
		for _, claim := range vol.PastClaims {
			if claim.State != structs.CSIVolumeClaimStateUnpublishing {
				return false
			}
		}
		return true
	}, time.Second*5, 10*time.Millisecond, "invalid claims should be marked for GC")

}

// TestCoreScheduler_RootKeyGC exercises root key GC
func TestCoreScheduler_RootKeyGC(t *testing.T) {
	ci.Parallel(t)

	srv, cleanup := TestServer(t, nil)
	defer cleanup()
	testutil.WaitForLeader(t, srv.RPC)

	// reset the time table
	srv.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// active key, will never be GC'd
	store := srv.fsm.State()
	key0, err := store.GetActiveRootKeyMeta(nil)
	require.NotNil(t, key0, "expected keyring to be bootstapped")
	require.NoError(t, err)

	// insert an "old" inactive key
	key1 := structs.NewRootKeyMeta()
	key1.SetInactive()
	require.NoError(t, store.UpsertRootKeyMeta(600, key1, false))

	// insert an "old" and inactive key with a variable that's using it
	key2 := structs.NewRootKeyMeta()
	key2.SetInactive()
	require.NoError(t, store.UpsertRootKeyMeta(700, key2, false))

	variable := mock.VariableEncrypted()
	variable.KeyID = key2.KeyID

	setResp := store.VarSet(601, &structs.VarApplyStateRequest{
		Op:  structs.VarOpSet,
		Var: variable,
	})
	require.NoError(t, setResp.Error)

	// insert an "old" key that's inactive but being used by an alloc
	key3 := structs.NewRootKeyMeta()
	key3.SetInactive()
	require.NoError(t, store.UpsertRootKeyMeta(800, key3, false))

	// insert the allocation using key3
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.SigningKeyID = key3.KeyID
	require.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 850, []*structs.Allocation{alloc}))

	// insert an "old" key that's inactive but being used by an alloc
	key4 := structs.NewRootKeyMeta()
	key4.SetInactive()
	require.NoError(t, store.UpsertRootKeyMeta(900, key4, false))

	// insert the dead allocation using key4
	alloc2 := mock.Alloc()
	alloc2.ClientStatus = structs.AllocClientStatusFailed
	alloc2.DesiredStatus = structs.AllocDesiredStatusStop
	alloc2.SigningKeyID = key4.KeyID
	require.NoError(t, store.UpsertAllocs(
		structs.MsgTypeTestSetup, 950, []*structs.Allocation{alloc2}))

	// insert a time table index before the last key
	tt := srv.fsm.TimeTable()
	tt.Witness(1000, time.Now().UTC().Add(-1*srv.config.RootKeyGCThreshold))

	// insert a "new" but inactive key
	key5 := structs.NewRootKeyMeta()
	key5.SetInactive()
	require.NoError(t, store.UpsertRootKeyMeta(1500, key5, false))

	// run the core job
	snap, err := store.Snapshot()
	require.NoError(t, err)
	core := NewCoreScheduler(srv, snap)
	eval := srv.coreJobEval(structs.CoreJobRootKeyRotateOrGC, 2000)
	c := core.(*CoreScheduler)
	require.NoError(t, c.rootKeyRotateOrGC(eval))

	ws := memdb.NewWatchSet()
	key, err := store.RootKeyMetaByID(ws, key0.KeyID)
	require.NoError(t, err)
	require.NotNil(t, key, "active key should not have been GCd")

	key, err = store.RootKeyMetaByID(ws, key1.KeyID)
	require.NoError(t, err)
	require.Nil(t, key, "old and unused inactive key should have been GCd")

	key, err = store.RootKeyMetaByID(ws, key2.KeyID)
	require.NoError(t, err)
	require.NotNil(t, key, "old key should not have been GCd if still in use")

	key, err = store.RootKeyMetaByID(ws, key3.KeyID)
	require.NoError(t, err)
	require.NotNil(t, key, "old key used to sign a live alloc should not have been GCd")

	key, err = store.RootKeyMetaByID(ws, key4.KeyID)
	require.NoError(t, err)
	require.Nil(t, key, "old key used to sign a terminal alloc should have been GCd")

	key, err = store.RootKeyMetaByID(ws, key5.KeyID)
	require.NoError(t, err)
	require.NotNil(t, key, "new key should not have been GCd")

}

// TestCoreScheduler_VariablesRekey exercises variables rekeying
func TestCoreScheduler_VariablesRekey(t *testing.T) {
	ci.Parallel(t)

	srv, cleanup := TestServer(t, nil)
	defer cleanup()
	testutil.WaitForLeader(t, srv.RPC)

	store := srv.fsm.State()
	key0, err := store.GetActiveRootKeyMeta(nil)
	require.NotNil(t, key0, "expected keyring to be bootstapped")
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		req := &structs.VariablesApplyRequest{
			Op:           structs.VarOpSet,
			Var:          mock.Variable(),
			WriteRequest: structs.WriteRequest{Region: srv.config.Region},
		}
		resp := &structs.VariablesApplyResponse{}
		require.NoError(t, srv.RPC("Variables.Apply", req, resp))
	}

	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region: srv.config.Region,
		},
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	require.NoError(t, srv.RPC("Keyring.Rotate", rotateReq, &rotateResp))

	for i := 0; i < 3; i++ {
		req := &structs.VariablesApplyRequest{
			Op:           structs.VarOpSet,
			Var:          mock.Variable(),
			WriteRequest: structs.WriteRequest{Region: srv.config.Region},
		}
		resp := &structs.VariablesApplyResponse{}
		require.NoError(t, srv.RPC("Variables.Apply", req, resp))
	}

	rotateReq.Full = true
	require.NoError(t, srv.RPC("Keyring.Rotate", rotateReq, &rotateResp))
	newKeyID := rotateResp.Key.KeyID

	require.Eventually(t, func() bool {
		ws := memdb.NewWatchSet()
		iter, err := store.Variables(ws)
		require.NoError(t, err)
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			variable := raw.(*structs.VariableEncrypted)
			if variable.KeyID != newKeyID {
				return false
			}
		}
		return true
	}, time.Second*5, 100*time.Millisecond,
		"variable rekey should be complete")

}

func TestCoreScheduler_FailLoop(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
		c.EvalDeliveryLimit = 2
		c.EvalFailedFollowupBaselineDelay = time.Duration(50 * time.Millisecond)
		c.EvalFailedFollowupDelayRange = time.Duration(1 * time.Millisecond)
	})
	defer cleanupSrv()
	codec := rpcClient(t, srv)
	sched := []string{structs.JobTypeCore}

	testutil.WaitForResult(func() (bool, error) {
		return srv.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Enqueue a core job eval that can never succeed because it was enqueued
	// by another leader that's now gone
	expected := srv.coreJobEval(structs.CoreJobCSIPluginGC, 100)
	expected.LeaderACL = "nonsense"
	srv.evalBroker.Enqueue(expected)

	nack := func(evalID, token string) error {
		req := &structs.EvalAckRequest{
			EvalID:       evalID,
			Token:        token,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.GenericResponse
		return msgpackrpc.CallWithCodec(codec, "Eval.Nack", req, &resp)
	}

	out, token, err := srv.evalBroker.Dequeue(sched, time.Second*5)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, expected, out)

	// first fail
	require.NoError(t, nack(out.ID, token))

	out, token, err = srv.evalBroker.Dequeue(sched, time.Second*5)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, expected, out)

	// second fail, should not result in failed-follow-up
	require.NoError(t, nack(out.ID, token))

	out, token, err = srv.evalBroker.Dequeue(sched, time.Second*5)
	require.NoError(t, err)
	if out != nil {
		t.Fatalf(
			"failed core jobs should not result in follow-up. TriggeredBy: %v",
			out.TriggeredBy)
	}
}

func TestCoreScheduler_ExpiredACLTokenGC(t *testing.T) {
	ci.Parallel(t)

	testServer, rootACLToken, testServerShutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer testServerShutdown()
	testutil.WaitForLeader(t, testServer.RPC)

	now := time.Now().UTC()

	// Craft some specific local and global tokens. For each type, one is
	// expired, one is not.
	expiredGlobal := mock.ACLToken()
	expiredGlobal.Global = true
	expiredGlobal.ExpirationTime = pointer.Of(now.Add(-2 * time.Hour))

	unexpiredGlobal := mock.ACLToken()
	unexpiredGlobal.Global = true
	unexpiredGlobal.ExpirationTime = pointer.Of(now.Add(2 * time.Hour))

	expiredLocal := mock.ACLToken()
	expiredLocal.ExpirationTime = pointer.Of(now.Add(-2 * time.Hour))

	unexpiredLocal := mock.ACLToken()
	unexpiredLocal.ExpirationTime = pointer.Of(now.Add(2 * time.Hour))

	// Upsert these into state.
	err := testServer.State().UpsertACLTokens(structs.MsgTypeTestSetup, 10, []*structs.ACLToken{
		expiredGlobal, unexpiredGlobal, expiredLocal, unexpiredLocal,
	})
	require.NoError(t, err)

	// Overwrite the timetable. The existing timetable has an entry due to the
	// ACL bootstrapping which makes witnessing a new index at a timestamp in
	// the past impossible.
	tt := NewTimeTable(timeTableGranularity, timeTableLimit)
	tt.Witness(20, time.Now().UTC().Add(-1*testServer.config.ACLTokenExpirationGCThreshold))
	testServer.fsm.timetable = tt

	// Generate the core scheduler.
	snap, err := testServer.State().Snapshot()
	require.NoError(t, err)
	coreScheduler := NewCoreScheduler(testServer, snap)

	// Trigger global and local periodic garbage collection runs.
	index, err := testServer.State().LatestIndex()
	require.NoError(t, err)
	index++

	globalGCEval := testServer.coreJobEval(structs.CoreJobGlobalTokenExpiredGC, index)
	require.NoError(t, coreScheduler.Process(globalGCEval))

	localGCEval := testServer.coreJobEval(structs.CoreJobLocalTokenExpiredGC, index)
	require.NoError(t, coreScheduler.Process(localGCEval))

	// Ensure the ACL tokens stored within state are as expected.
	iter, err := testServer.State().ACLTokens(nil, state.SortDefault)
	require.NoError(t, err)

	var tokens []*structs.ACLToken
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		tokens = append(tokens, raw.(*structs.ACLToken))
	}
	require.ElementsMatch(t, []*structs.ACLToken{rootACLToken, unexpiredGlobal, unexpiredLocal}, tokens)
}

func TestCoreScheduler_ExpiredACLTokenGC_Force(t *testing.T) {
	ci.Parallel(t)

	testServer, rootACLToken, testServerShutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer testServerShutdown()
	testutil.WaitForLeader(t, testServer.RPC)

	// This time is the threshold for all expiry calls to be based on. All
	// tokens with expiry can use this as their base and use Add().
	expiryTimeThreshold := time.Now().UTC()

	// Track expired and non-expired tokens for local and global tokens in
	// separate arrays, so we have a clear way to test state.
	var expiredGlobalTokens, nonExpiredGlobalTokens, expiredLocalTokens, nonExpiredLocalTokens []*structs.ACLToken

	// Add the root ACL token to the appropriate array. This will be returned
	// from state so must be accounted for and tested.
	nonExpiredGlobalTokens = append(nonExpiredGlobalTokens, rootACLToken)

	// Generate and upsert a number of mixed expired, non-expired global
	// tokens.
	for i := 0; i < 20; i++ {
		mockedToken := mock.ACLToken()
		mockedToken.Global = true
		if i%2 == 0 {
			expiredGlobalTokens = append(expiredGlobalTokens, mockedToken)
			mockedToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(-24 * time.Hour))
		} else {
			nonExpiredGlobalTokens = append(nonExpiredGlobalTokens, mockedToken)
			mockedToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(24 * time.Hour))
		}
	}

	// Generate and upsert a number of mixed expired, non-expired local
	// tokens.
	for i := 0; i < 20; i++ {
		mockedToken := mock.ACLToken()
		mockedToken.Global = false
		if i%2 == 0 {
			expiredLocalTokens = append(expiredLocalTokens, mockedToken)
			mockedToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(-24 * time.Hour))
		} else {
			nonExpiredLocalTokens = append(nonExpiredLocalTokens, mockedToken)
			mockedToken.ExpirationTime = pointer.Of(expiryTimeThreshold.Add(24 * time.Hour))
		}
	}

	allTokens := append(expiredGlobalTokens, nonExpiredGlobalTokens...)
	allTokens = append(allTokens, expiredLocalTokens...)
	allTokens = append(allTokens, nonExpiredLocalTokens...)

	// Upsert them all.
	err := testServer.State().UpsertACLTokens(structs.MsgTypeTestSetup, 10, allTokens)
	require.NoError(t, err)

	// This function provides an easy way to get all tokens out of the
	// iterator.
	fromIteratorFunc := func(iter memdb.ResultIterator) []*structs.ACLToken {
		var tokens []*structs.ACLToken
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			tokens = append(tokens, raw.(*structs.ACLToken))
		}
		return tokens
	}

	// Check all the tokens are correctly stored within state.
	iter, err := testServer.State().ACLTokens(nil, state.SortDefault)
	require.NoError(t, err)

	tokens := fromIteratorFunc(iter)
	require.ElementsMatch(t, allTokens, tokens)

	// Generate the core scheduler and trigger a forced garbage collection
	// which should delete all expired tokens.
	snap, err := testServer.State().Snapshot()
	require.NoError(t, err)
	coreScheduler := NewCoreScheduler(testServer, snap)

	index, err := testServer.State().LatestIndex()
	require.NoError(t, err)
	index++

	forceGCEval := testServer.coreJobEval(structs.CoreJobForceGC, index)
	require.NoError(t, coreScheduler.Process(forceGCEval))

	// List all the remaining ACL tokens to be sure they are as expected.
	iter, err = testServer.State().ACLTokens(nil, state.SortDefault)
	require.NoError(t, err)

	tokens = fromIteratorFunc(iter)
	require.ElementsMatch(t, append(nonExpiredGlobalTokens, nonExpiredLocalTokens...), tokens)
}
