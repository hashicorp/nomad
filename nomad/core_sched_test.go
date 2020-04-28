package nomad

import (
	"fmt"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoreScheduler_EvalGC(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	require := require.New(t)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	state.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	require.Nil(err)

	// Insert mock job with rescheduling disabled
	job := mock.Job()
	job.ID = eval.JobID
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err = state.UpsertJob(1001, job)
	require.Nil(err)

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
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err := state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 != nil {
		t.Fatalf("bad: %v", outA2)
	}
}

// Tests GC behavior on allocations being rescheduled
func TestCoreScheduler_EvalGC_ReschedulingAllocs(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	require := require.New(t)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	state.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	require.Nil(err)

	// Insert "pending" eval for same job
	eval2 := mock.Eval()
	eval2.JobID = eval.JobID
	state.UpsertJobSummary(999, mock.JobSummary(eval2.JobID))
	err = state.UpsertEvals(1003, []*structs.Evaluation{eval2})
	require.Nil(err)

	// Insert mock job with default reschedule policy of 2 in 10 minutes
	job := mock.Job()
	job.ID = eval.JobID

	err = state.UpsertJob(1001, job)
	require.Nil(err)

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
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc, alloc2})
	require.Nil(err)

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC, job has all terminal allocs and one pending eval
	gc := s1.coreJobEval(structs.CoreJobEvalGC, 2000)
	err = core.Process(gc)
	require.Nil(err)

	// Eval should still exist
	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	require.Nil(err)
	require.NotNil(out)
	require.Equal(eval.ID, out.ID)

	outA, err := state.AllocByID(ws, alloc.ID)
	require.Nil(err)
	require.Nil(outA)

	outA2, err := state.AllocByID(ws, alloc2.ID)
	require.Nil(err)
	require.Equal(alloc2.ID, outA2.ID)

}

// Tests GC behavior on stopped job with reschedulable allocs
func TestCoreScheduler_EvalGC_StoppedJob_Reschedulable(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	require := require.New(t)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	state.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	require.Nil(err)

	// Insert mock stopped job with default reschedule policy of 2 in 10 minutes
	job := mock.Job()
	job.ID = eval.JobID
	job.Stop = true

	err = state.UpsertJob(1001, job)
	require.Nil(err)

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
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc})
	require.Nil(err)

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobEvalGC, 2000)
	err = core.Process(gc)
	require.Nil(err)

	// Eval should not exist
	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	require.Nil(err)
	require.Nil(out)

	// Alloc should not exist
	outA, err := state.AllocByID(ws, alloc.ID)
	require.Nil(err)
	require.Nil(outA)

}

// An EvalGC should never reap a batch job that has not been stopped
func TestCoreScheduler_EvalGC_Batch(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert a "dead" job
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusDead
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "complete" eval
	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.Type = structs.JobTypeBatch
	eval.JobID = job.ID
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "failed" alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
	alloc2.Job = job
	alloc2.JobID = job.ID
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusLost

	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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

	// Nothing should be gone
	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err := state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 == nil {
		t.Fatalf("bad: %v", outA2)
	}

	outB, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outB == nil {
		t.Fatalf("bad: %v", outB)
	}
}

// An EvalGC should reap allocations from jobs with an older modify index
func TestCoreScheduler_EvalGC_Batch_OldVersion(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert a "dead" job
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusDead
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "complete" eval
	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.Type = structs.JobTypeBatch
	eval.JobID = job.ID
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "failed" alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
	alloc2.Job = job
	alloc2.JobID = job.ID
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusLost

	// Insert alloc with older job modifyindex
	alloc3 := mock.Alloc()
	job2 := job.Copy()

	alloc3.Job = job2
	alloc3.JobID = job2.ID
	alloc3.EvalID = eval.ID
	job2.CreateIndex = 500
	alloc3.DesiredStatus = structs.AllocDesiredStatusRun
	alloc3.ClientStatus = structs.AllocClientStatusLost

	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc, alloc2, alloc3})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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

	// Alloc1 and 2 should be there, and alloc3 should be gone
	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err := state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 == nil {
		t.Fatalf("bad: %v", outA2)
	}

	outA3, err := state.AllocByID(ws, alloc3.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA3 != nil {
		t.Fatalf("expected alloc to be nil:%v", outA2)
	}

	outB, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outB == nil {
		t.Fatalf("bad: %v", outB)
	}
}

// An EvalGC should  reap a batch job that has been stopped
func TestCoreScheduler_EvalGC_BatchStopped(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	require := require.New(t)
	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Create a "dead" job
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusDead
	job.Stop = true
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err := state.UpsertJob(1001, job)
	require.Nil(err)

	// Insert "complete" eval
	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.Type = structs.JobTypeBatch
	eval.JobID = job.ID
	err = state.UpsertEvals(1002, []*structs.Evaluation{eval})
	require.Nil(err)

	// Insert "failed" alloc
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.DesiredStatus = structs.AllocDesiredStatusStop

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
	alloc2.JobID = job.ID
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusLost
	alloc2.TaskGroup = job.TaskGroups[0].Name

	err = state.UpsertAllocs(1003, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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

	// Everything should be gone
	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err := state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 != nil {
		t.Fatalf("bad: %v", outA2)
	}
}

func TestCoreScheduler_EvalGC_Partial(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	require := require.New(t)
	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	state.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
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
	state.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
	alloc2.JobID = job.ID
	alloc2.EvalID = eval.ID
	alloc2.TaskGroup = job.TaskGroups[0].Name
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusLost

	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "running" alloc
	alloc3 := mock.Alloc()
	alloc3.EvalID = eval.ID
	alloc3.JobID = job.ID
	state.UpsertJobSummary(1003, mock.JobSummary(alloc3.JobID))
	err = state.UpsertAllocs(1004, []*structs.Allocation{alloc3})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert mock job with rescheduling disabled
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err = state.UpsertJob(1001, job)
	require.Nil(err)

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.EvalGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(ws, alloc3.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}

	// Should be gone
	outB, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outB != nil {
		t.Fatalf("bad: %v", outB)
	}

	outC, err := state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outC != nil {
		t.Fatalf("bad: %v", outC)
	}
}

func TestCoreScheduler_EvalGC_Force(t *testing.T) {
	t.Parallel()
	for _, withAcl := range []bool{false, true} {
		t.Run(fmt.Sprintf("with acl %v", withAcl), func(t *testing.T) {
			require := require.New(t)
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
			state := server.fsm.State()
			eval := mock.Eval()
			eval.Status = structs.EvalStatusFailed
			state.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
			err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
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
			err = state.UpsertJob(1001, job)
			require.Nil(err)

			// Insert "dead" alloc
			alloc := mock.Alloc()
			alloc.EvalID = eval.ID
			alloc.DesiredStatus = structs.AllocDesiredStatusStop
			alloc.TaskGroup = job.TaskGroups[0].Name
			state.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))
			err = state.UpsertAllocs(1002, []*structs.Allocation{alloc})
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Create a core scheduler
			snap, err := state.Snapshot()
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
			out, err := state.EvalByID(ws, eval.ID)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if out != nil {
				t.Fatalf("bad: %v", out)
			}

			outA, err := state.AllocByID(ws, alloc.ID)
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
	t.Parallel()
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
			state := server.fsm.State()
			node := mock.Node()
			node.Status = structs.NodeStatusDown
			err := state.UpsertNode(1000, node)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Update the time tables to make this work
			tt := server.fsm.TimeTable()
			tt.Witness(2000, time.Now().UTC().Add(-1*server.config.NodeGCThreshold))

			// Create a core scheduler
			snap, err := state.Snapshot()
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
			out, err := state.NodeByID(ws, node.ID)
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
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" node
	state := s1.fsm.State()
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert a terminal alloc on that node
	alloc := mock.Alloc()
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	state.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))
	if err := state.UpsertAllocs(1002, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.NodeGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_NodeGC_RunningAllocs(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" node
	state := s1.fsm.State()
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert a running alloc on that node
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusRunning
	state.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))
	if err := state.UpsertAllocs(1002, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.NodeGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_NodeGC_Force(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" node
	state := s1.fsm.State()
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_JobGC_OutstandingEvals(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusDead
	err := state.UpsertJob(1000, job)
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
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outE, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE == nil {
		t.Fatalf("bad: %v", outE)
	}

	outE2, err := state.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE2 == nil {
		t.Fatalf("bad: %v", outE2)
	}

	// Update the second eval to be terminal
	eval2.Status = structs.EvalStatusComplete
	err = state.UpsertEvals(1003, []*structs.Evaluation{eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err = state.Snapshot()
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
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outE, err = state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE != nil {
		t.Fatalf("bad: %v", outE)
	}

	outE2, err = state.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE2 != nil {
		t.Fatalf("bad: %v", outE2)
	}
}

func TestCoreScheduler_JobGC_OutstandingAllocs(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusDead
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert an eval
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusComplete
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval})
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

	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err := state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 == nil {
		t.Fatalf("bad: %v", outA2)
	}

	// Update the second alloc to be terminal
	alloc2.ClientStatus = structs.AllocClientStatusComplete
	err = state.UpsertAllocs(1003, []*structs.Allocation{alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err = state.Snapshot()
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
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err = state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}

	outA2, err = state.AllocByID(ws, alloc2.ID)
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
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	err := state.UpsertJob(1000, job)
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

	err = state.UpsertEvals(1001, []*structs.Evaluation{eval, eval2})
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

	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force the jobs state to dead
	job.Status = structs.JobStatusDead

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outE, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE == nil {
		t.Fatalf("bad: %v", outE)
	}

	outE2, err := state.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE2 == nil {
		t.Fatalf("bad: %v", outE2)
	}

	outA, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}
	outA2, err := state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA2 == nil {
		t.Fatalf("bad: %v", outA2)
	}
}

// This test ensures that stopped jobs are GCd
func TestCoreScheduler_JobGC_Stopped(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	state := s1.fsm.State()
	job := mock.Job()
	job.Stop = true
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 0,
		Interval: 0 * time.Second,
	}
	err := state.UpsertJob(1000, job)
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

	err = state.UpsertEvals(1001, []*structs.Evaluation{eval, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert one complete alloc
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	alloc.TaskGroup = job.TaskGroups[0].Name
	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outE, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE != nil {
		t.Fatalf("bad: %v", outE)
	}

	outE2, err := state.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE2 != nil {
		t.Fatalf("bad: %v", outE2)
	}

	outA, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}
}

func TestCoreScheduler_JobGC_Force(t *testing.T) {
	t.Parallel()
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
			state := server.fsm.State()
			job := mock.Job()
			job.Type = structs.JobTypeBatch
			job.Status = structs.JobStatusDead
			err := state.UpsertJob(1000, job)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Insert a terminal eval
			eval := mock.Eval()
			eval.JobID = job.ID
			eval.Status = structs.EvalStatusComplete
			err = state.UpsertEvals(1001, []*structs.Evaluation{eval})
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Create a core scheduler
			snap, err := state.Snapshot()
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
			out, err := state.JobByID(ws, job.Namespace, job.ID)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if out != nil {
				t.Fatalf("bad: %v", out)
			}

			outE, err := state.EvalByID(ws, eval.ID)
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
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert a parameterized job.
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusRunning
	job.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadRequired,
	}
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	// Mark the job as stopped and try again
	job2 := job.Copy()
	job2.Stop = true
	err = state.UpsertJob(2000, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err = state.Snapshot()
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
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %+v", out)
	}
}

// This test ensures periodic jobs don't get GCd until they are stopped
func TestCoreScheduler_JobGC_Periodic(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert a parameterized job.
	state := s1.fsm.State()
	job := mock.PeriodicJob()
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err := state.Snapshot()
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
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	// Mark the job as stopped and try again
	job2 := job.Copy()
	job2.Stop = true
	err = state.UpsertJob(2000, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a core scheduler
	snap, err = state.Snapshot()
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
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %+v", out)
	}
}

func TestCoreScheduler_DeploymentGC(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert an active, terminal, and terminal with allocations deployment
	state := s1.fsm.State()
	d1, d2, d3 := mock.Deployment(), mock.Deployment(), mock.Deployment()
	d1.Status = structs.DeploymentStatusFailed
	d3.Status = structs.DeploymentStatusSuccessful
	assert.Nil(state.UpsertDeployment(1000, d1), "UpsertDeployment")
	assert.Nil(state.UpsertDeployment(1001, d2), "UpsertDeployment")
	assert.Nil(state.UpsertDeployment(1002, d3), "UpsertDeployment")

	a := mock.Alloc()
	a.JobID = d3.JobID
	a.DeploymentID = d3.ID
	assert.Nil(state.UpsertAllocs(1003, []*structs.Allocation{a}), "UpsertAllocs")

	// Update the time tables to make this work
	tt := s1.fsm.TimeTable()
	tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.DeploymentGCThreshold))

	// Create a core scheduler
	snap, err := state.Snapshot()
	assert.Nil(err, "Snapshot")
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobDeploymentGC, 2000)
	assert.Nil(core.Process(gc), "Process GC")

	// Should be gone
	ws := memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, d1.ID)
	assert.Nil(err, "DeploymentByID")
	assert.Nil(out, "Terminal Deployment")
	out2, err := state.DeploymentByID(ws, d2.ID)
	assert.Nil(err, "DeploymentByID")
	assert.NotNil(out2, "Active Deployment")
	out3, err := state.DeploymentByID(ws, d3.ID)
	assert.Nil(err, "DeploymentByID")
	assert.NotNil(out3, "Terminal Deployment With Allocs")
}

func TestCoreScheduler_DeploymentGC_Force(t *testing.T) {
	t.Parallel()
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
			state := server.fsm.State()
			d1, d2 := mock.Deployment(), mock.Deployment()
			d1.Status = structs.DeploymentStatusFailed
			assert.Nil(state.UpsertDeployment(1000, d1), "UpsertDeployment")
			assert.Nil(state.UpsertDeployment(1001, d2), "UpsertDeployment")

			// Create a core scheduler
			snap, err := state.Snapshot()
			assert.Nil(err, "Snapshot")
			core := NewCoreScheduler(server, snap)

			// Attempt the GC
			gc := server.coreJobEval(structs.CoreJobForceGC, 1000)
			assert.Nil(core.Process(gc), "Process Force GC")

			// Should be gone
			ws := memdb.NewWatchSet()
			out, err := state.DeploymentByID(ws, d1.ID)
			assert.Nil(err, "DeploymentByID")
			assert.Nil(out, "Terminal Deployment")
			out2, err := state.DeploymentByID(ws, d2.ID)
			assert.Nil(err, "DeploymentByID")
			assert.NotNil(out2, "Active Deployment")
		})
	}
}

func TestCoreScheduler_PartitionEvalReap(t *testing.T) {
	t.Parallel()

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

	// Set the max ids per reap to something lower.
	maxIdsPerReap = 2

	evals := []string{"a", "b", "c"}
	allocs := []string{"1", "2", "3"}
	requests := core.(*CoreScheduler).partitionEvalReap(evals, allocs)
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
	t.Parallel()

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

	// Set the max ids per reap to something lower.
	maxIdsPerReap = 2

	deployments := []string{"a", "b", "c"}
	requests := core.(*CoreScheduler).partitionDeploymentReap(deployments)
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
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Create a core scheduler
	snap, err := s1.fsm.State().Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	core := NewCoreScheduler(s1, snap)

	// Set the max ids per reap to something lower.
	maxIdsPerReap = 2

	jobs := []*structs.Job{mock.Job(), mock.Job(), mock.Job()}
	requests := core.(*CoreScheduler).partitionJobReap(jobs, "")
	require.Len(requests, 2)

	first := requests[0]
	second := requests[1]
	require.Len(first.Jobs, 2)
	require.Len(second.Jobs, 1)
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
	require := require.New(t)
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusComplete
	require.True(allocGCEligible(alloc, nil, time.Now(), 1000))
}
