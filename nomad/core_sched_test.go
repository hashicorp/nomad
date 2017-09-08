package nomad

import (
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestCoreScheduler_EvalGC(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	state.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	alloc.JobID = eval.JobID

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusLost
	alloc2.JobID = eval.JobID
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

// An EvalGC should never reap a batch job that has not been stopped
func TestCoreScheduler_EvalGC_Batch(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
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

// An EvalGC should  reap a batch job that has been stopped
func TestCoreScheduler_EvalGC_BatchStopped(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Create a "dead" job
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Status = structs.JobStatusDead

	// Insert "complete" eval
	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.Type = structs.JobTypeBatch
	eval.JobID = job.ID
	err := state.UpsertEvals(1001, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "failed" alloc
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

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

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	state.UpsertJobSummary(1001, mock.JobSummary(alloc.JobID))

	// Insert "lost" alloc
	alloc2 := mock.Alloc()
	alloc2.JobID = alloc.JobID
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusLost

	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "running" alloc
	alloc3 := mock.Alloc()
	alloc3.EvalID = eval.ID
	state.UpsertJobSummary(1003, mock.JobSummary(alloc3.JobID))
	err = state.UpsertAllocs(1004, []*structs.Allocation{alloc3})
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	state.UpsertJobSummary(999, mock.JobSummary(eval.JobID))
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
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
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobForceGC, 1002)
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
}

func TestCoreScheduler_NodeGC(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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

func TestCoreScheduler_NodeGC_TerminalAllocs(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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

	alloc2 := mock.Alloc()
	alloc2.JobID = job.ID
	alloc2.EvalID = eval.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusRun
	alloc2.ClientStatus = structs.AllocClientStatusRunning

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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert job.
	state := s1.fsm.State()
	job := mock.Job()
	//job.Status = structs.JobStatusDead
	job.Stop = true
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobForceGC, 1002)
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
}

// This test ensures parameterized jobs only get gc'd when stopped
func TestCoreScheduler_JobGC_Parameterized(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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

// This test ensures periodic jobs don't get GCd til they are stopped
func TestCoreScheduler_JobGC_Periodic(t *testing.T) {
	t.Parallel()

	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert an active, terminal, and terminal with allocations edeployment
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// COMPAT Remove in 0.6: Reset the FSM time table since we reconcile which sets index 0
	s1.fsm.timetable.table = make([]TimeTableEntry, 1, 10)

	// Insert terminal and active deployment
	state := s1.fsm.State()
	d1, d2 := mock.Deployment(), mock.Deployment()
	d1.Status = structs.DeploymentStatusFailed
	assert.Nil(state.UpsertDeployment(1000, d1), "UpsertDeployment")
	assert.Nil(state.UpsertDeployment(1001, d2), "UpsertDeployment")

	// Create a core scheduler
	snap, err := state.Snapshot()
	assert.Nil(err, "Snapshot")
	core := NewCoreScheduler(s1, snap)

	// Attempt the GC
	gc := s1.coreJobEval(structs.CoreJobForceGC, 1000)
	assert.Nil(core.Process(gc), "Process Force GC")

	// Should be gone
	ws := memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, d1.ID)
	assert.Nil(err, "DeploymentByID")
	assert.Nil(out, "Terminal Deployment")
	out2, err := state.DeploymentByID(ws, d2.ID)
	assert.Nil(err, "DeploymentByID")
	assert.NotNil(out2, "Active Deployment")
}

func TestCoreScheduler_PartitionEvalReap(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
