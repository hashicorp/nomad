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

func TestCoreScheduler_EvalGC_Batch_Allocs(t *testing.T) {
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
	gc := s1.forceCoreJobEval(structs.CoreJobEvalGC)
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
	gc := s1.forceCoreJobEval(structs.CoreJobNodeGC)
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
		test, evalStatus, desiredAllocStatus, clientAllocStatus string
		shouldExist                                             bool
	}{
		{
			test:               "Terminal",
			evalStatus:         structs.EvalStatusFailed,
			desiredAllocStatus: structs.AllocDesiredStatusRun,
			clientAllocStatus:  structs.AllocDesiredStatusFailed,
			shouldExist:        false,
		},
		{
			test:               "Has Failed Alloc",
			evalStatus:         structs.EvalStatusFailed,
			desiredAllocStatus: structs.AllocDesiredStatusRun,
			clientAllocStatus:  structs.AllocDesiredStatusFailed,
			shouldExist:        false,
		},
		{
			test:               "Has Running Alloc",
			evalStatus:         structs.EvalStatusFailed,
			desiredAllocStatus: structs.AllocDesiredStatusRun,
			clientAllocStatus:  structs.AllocDesiredStatusRun,
			shouldExist:        true,
		},
		{
			test:               "Has Eval",
			evalStatus:         structs.EvalStatusPending,
			desiredAllocStatus: structs.AllocDesiredStatusRun,
			clientAllocStatus:  structs.AllocDesiredStatusFailed,
			shouldExist:        true,
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
		alloc.DesiredStatus = test.desiredAllocStatus
		alloc.ClientStatus = test.clientAllocStatus
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
		test, evalStatus, desiredAllocStatus, clientAllocStatus string
		shouldExist                                             bool
	}{
		{
			test:               "Terminal",
			evalStatus:         structs.EvalStatusFailed,
			desiredAllocStatus: structs.AllocDesiredStatusRun,
			clientAllocStatus:  structs.AllocDesiredStatusFailed,
			shouldExist:        false,
		},
		{
			test:               "Has Failed Alloc",
			evalStatus:         structs.EvalStatusFailed,
			desiredAllocStatus: structs.AllocDesiredStatusRun,
			clientAllocStatus:  structs.AllocDesiredStatusFailed,
			shouldExist:        false,
		},
		{
			test:               "Has Running Alloc",
			evalStatus:         structs.EvalStatusFailed,
			desiredAllocStatus: structs.AllocDesiredStatusRun,
			clientAllocStatus:  structs.AllocDesiredStatusRun,
			shouldExist:        true,
		},
		{
			test:               "Has Eval",
			evalStatus:         structs.EvalStatusPending,
			desiredAllocStatus: structs.AllocDesiredStatusRun,
			clientAllocStatus:  structs.AllocDesiredStatusFailed,
			shouldExist:        true,
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
		alloc.DesiredStatus = test.desiredAllocStatus
		alloc.ClientStatus = test.clientAllocStatus
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
		gc := s1.forceCoreJobEval(structs.CoreJobJobGC)
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
