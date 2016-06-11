package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestCoreScheduler_EvalGC(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusFailed
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc})
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
	gc := s1.coreJobEval(structs.CoreJobEvalGC)
	gc.ModifyIndex = 2000
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone
	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}
}

func TestCoreScheduler_EvalGC_Partial(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusFailed
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "running" alloc
	alloc2 := mock.Alloc()
	alloc2.EvalID = eval.ID
	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc2})
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
	gc := s1.coreJobEval(structs.CoreJobEvalGC)
	gc.ModifyIndex = 2000
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should not be gone
	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}

	// Should be gone
	outB, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outB != nil {
		t.Fatalf("bad: %v", outB)
	}
}

func TestCoreScheduler_EvalGC_Batch_NoAllocs(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Type = structs.JobTypeBatch
	eval.Status = structs.EvalStatusFailed
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
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
	gc := s1.coreJobEval(structs.CoreJobEvalGC)
	gc.ModifyIndex = 2000
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone because there is no alloc associated
	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_EvalGC_Batch_Allocs_WithJob(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert job.
	state := s1.fsm.State()
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "dead" eval
	eval := mock.Eval()
	eval.Type = structs.JobTypeBatch
	eval.Status = structs.EvalStatusFailed
	eval.JobID = job.ID
	if err := state.UpsertEvals(1001, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.JobID = job.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusFailed
	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc})
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
	gc := s1.coreJobEval(structs.CoreJobEvalGC)
	gc.ModifyIndex = 2000
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Shouldn't be gone because there are associated allocs.
	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA == nil {
		t.Fatalf("bad: %v", outA)
	}
}

func TestCoreScheduler_EvalGC_Batch_Allocs_NoJob(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Type = structs.JobTypeBatch
	eval.Status = structs.EvalStatusFailed
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusFailed
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc})
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
	gc := s1.coreJobEval(structs.CoreJobEvalGC)
	gc.ModifyIndex = 2000
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone because the job is deregistered.
	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_EvalGC_Force(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Insert "dead" eval
	state := s1.fsm.State()
	eval := mock.Eval()
	eval.Status = structs.EvalStatusFailed
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Insert "dead" alloc
	alloc := mock.Alloc()
	alloc.EvalID = eval.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusFailed
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc})
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
	gc := s1.coreJobEval(structs.CoreJobForceGC)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone
	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}

	outA, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outA != nil {
		t.Fatalf("bad: %v", outA)
	}
}

func TestCoreScheduler_NodeGC(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

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
	gc := s1.coreJobEval(structs.CoreJobNodeGC)
	gc.ModifyIndex = 2000
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone
	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_NodeGC_TerminalAllocs(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

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
	if err := state.UpsertAllocs(1001, []*structs.Allocation{alloc}); err != nil {
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
	gc := s1.coreJobEval(structs.CoreJobNodeGC)
	gc.ModifyIndex = 2000
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone
	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_NodeGC_RunningAllocs(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

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
	if err := state.UpsertAllocs(1001, []*structs.Allocation{alloc}); err != nil {
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
	gc := s1.coreJobEval(structs.CoreJobNodeGC)
	gc.ModifyIndex = 2000
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should still be here
	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_NodeGC_Force(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

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
	gc := s1.coreJobEval(structs.CoreJobForceGC)
	err = core.Process(gc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be gone
	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %v", out)
	}
}

func TestCoreScheduler_JobGC(t *testing.T) {
	tests := []struct {
		test, evalStatus, allocStatus string
		shouldExist                   bool
	}{
		{
			test:        "Terminal",
			evalStatus:  structs.EvalStatusFailed,
			allocStatus: structs.AllocDesiredStatusFailed,
			shouldExist: false,
		},
		{
			test:        "Has Alloc",
			evalStatus:  structs.EvalStatusFailed,
			allocStatus: structs.AllocDesiredStatusRun,
			shouldExist: true,
		},
		{
			test:        "Has Eval",
			evalStatus:  structs.EvalStatusPending,
			allocStatus: structs.AllocDesiredStatusFailed,
			shouldExist: true,
		},
	}

	for _, test := range tests {
		s1 := testServer(t, nil)
		defer s1.Shutdown()
		testutil.WaitForLeader(t, s1.RPC)

		// Insert job.
		state := s1.fsm.State()
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		err := state.UpsertJob(1000, job)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}

		// Insert eval
		eval := mock.Eval()
		eval.JobID = job.ID
		eval.Status = test.evalStatus
		err = state.UpsertEvals(1001, []*structs.Evaluation{eval})
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}

		// Insert alloc
		alloc := mock.Alloc()
		alloc.JobID = job.ID
		alloc.EvalID = eval.ID
		alloc.DesiredStatus = test.allocStatus
		err = state.UpsertAllocs(1002, []*structs.Allocation{alloc})
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}

		// Update the time tables to make this work
		tt := s1.fsm.TimeTable()
		tt.Witness(2000, time.Now().UTC().Add(-1*s1.config.JobGCThreshold))

		// Create a core scheduler
		snap, err := state.Snapshot()
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}
		core := NewCoreScheduler(s1, snap)

		// Attempt the GC
		gc := s1.coreJobEval(structs.CoreJobJobGC)
		gc.ModifyIndex = 2000
		err = core.Process(gc)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}

		// Should still exist
		out, err := state.JobByID(job.ID)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}
		if (test.shouldExist && out == nil) || (!test.shouldExist && out != nil) {
			t.Fatalf("test(%s) bad: %v", test.test, out)
		}

		outE, err := state.EvalByID(eval.ID)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}
		if (test.shouldExist && outE == nil) || (!test.shouldExist && outE != nil) {
			t.Fatalf("test(%s) bad: %v", test.test, out)
		}

		outA, err := state.AllocByID(alloc.ID)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}
		if (test.shouldExist && outA == nil) || (!test.shouldExist && outA != nil) {
			t.Fatalf("test(%s) bad: %v", test.test, outA)
		}
	}
}

func TestCoreScheduler_JobGC_Force(t *testing.T) {
	tests := []struct {
		test, evalStatus, allocStatus string
		shouldExist                   bool
	}{
		{
			test:        "Terminal",
			evalStatus:  structs.EvalStatusFailed,
			allocStatus: structs.AllocDesiredStatusFailed,
			shouldExist: false,
		},
		{
			test:        "Has Alloc",
			evalStatus:  structs.EvalStatusFailed,
			allocStatus: structs.AllocDesiredStatusRun,
			shouldExist: true,
		},
		{
			test:        "Has Eval",
			evalStatus:  structs.EvalStatusPending,
			allocStatus: structs.AllocDesiredStatusFailed,
			shouldExist: true,
		},
	}

	for _, test := range tests {
		s1 := testServer(t, nil)
		defer s1.Shutdown()
		testutil.WaitForLeader(t, s1.RPC)

		// Insert job.
		state := s1.fsm.State()
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		err := state.UpsertJob(1000, job)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}

		// Insert eval
		eval := mock.Eval()
		eval.JobID = job.ID
		eval.Status = test.evalStatus
		err = state.UpsertEvals(1001, []*structs.Evaluation{eval})
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}

		// Insert alloc
		alloc := mock.Alloc()
		alloc.JobID = job.ID
		alloc.EvalID = eval.ID
		alloc.DesiredStatus = test.allocStatus
		err = state.UpsertAllocs(1002, []*structs.Allocation{alloc})
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}

		// Create a core scheduler
		snap, err := state.Snapshot()
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}
		core := NewCoreScheduler(s1, snap)

		// Attempt the GC
		gc := s1.coreJobEval(structs.CoreJobForceGC)
		err = core.Process(gc)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}

		// Should still exist
		out, err := state.JobByID(job.ID)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}
		if (test.shouldExist && out == nil) || (!test.shouldExist && out != nil) {
			t.Fatalf("test(%s) bad: %v", test.test, out)
		}

		outE, err := state.EvalByID(eval.ID)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}
		if (test.shouldExist && outE == nil) || (!test.shouldExist && outE != nil) {
			t.Fatalf("test(%s) bad: %v", test.test, out)
		}

		outA, err := state.AllocByID(alloc.ID)
		if err != nil {
			t.Fatalf("test(%s) err: %v", test.test, err)
		}
		if (test.shouldExist && outA == nil) || (!test.shouldExist && outA != nil) {
			t.Fatalf("test(%s) bad: %v", test.test, outA)
		}
	}
}

func TestCoreScheduler_PartitionReap(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

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
	requests := core.(*CoreScheduler).partitionReap(evals, allocs)
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
