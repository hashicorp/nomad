package state

import (
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/watch"
)

func testStateStore(t *testing.T) *StateStore {
	state, err := NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if state == nil {
		t.Fatalf("missing state")
	}
	return state
}

func TestStateStore_UpsertNode_Node(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "nodes"},
		watch.Item{Node: node.ID})

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(node, out) {
		t.Fatalf("bad: %#v %#v", node, out)
	}

	index, err := state.Index("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_DeleteNode_Node(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "nodes"},
		watch.Item{Node: node.ID})

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeleteNode(1001, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", node, out)
	}

	index, err := state.Index("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_UpdateNodeStatus_Node(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "nodes"},
		watch.Item{Node: node.ID})

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpdateNodeStatus(1001, node.ID, structs.NodeStatusReady)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.Status != structs.NodeStatusReady {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_UpdateNodeDrain_Node(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "nodes"},
		watch.Item{Node: node.ID})

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpdateNodeDrain(1001, node.ID, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !out.Drain {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_Nodes(t *testing.T) {
	state := testStateStore(t)
	var nodes []*structs.Node

	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)

		err := state.UpsertNode(1000+uint64(i), node)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.Nodes()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Node
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Node))
	}

	sort.Sort(NodeIDSort(nodes))
	sort.Sort(NodeIDSort(out))

	if !reflect.DeepEqual(nodes, out) {
		t.Fatalf("bad: %#v %#v", nodes, out)
	}
}

func TestStateStore_RestoreNode(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "nodes"},
		watch.Item{Node: node.ID})

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.NodeRestore(node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, node) {
		t.Fatalf("Bad: %#v %#v", out, node)
	}

	notify.verify(t)
}

func TestStateStore_UpsertJob_Job(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "jobs"},
		watch.Item{Job: job.ID})

	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(job, out) {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_UpdateUpsertJob_Job(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "jobs"},
		watch.Item{Job: job.ID})

	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	job2 := mock.Job()
	job2.ID = job.ID
	err = state.UpsertJob(1001, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(job2, out) {
		t.Fatalf("bad: %#v %#v", job2, out)
	}

	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_DeleteJob_Job(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "jobs"},
		watch.Item{Job: job.ID})

	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeleteJob(1001, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_Jobs(t *testing.T) {
	state := testStateStore(t)
	var jobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		jobs = append(jobs, job)

		err := state.UpsertJob(1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.Jobs()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(jobs))
	sort.Sort(JobIDSort(out))

	if !reflect.DeepEqual(jobs, out) {
		t.Fatalf("bad: %#v %#v", jobs, out)
	}
}

func TestStateStore_ChildJobs(t *testing.T) {
	state := testStateStore(t)
	parent := mock.Job()
	var childJobs []*structs.Job

	if err := state.UpsertJob(999, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	for i := 0; i < 10; i++ {
		job := mock.Job()
		job.ParentID = parent.ID
		childJobs = append(childJobs, job)

		err := state.UpsertJob(1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.ChildJobs(parent.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(childJobs))
	sort.Sort(JobIDSort(out))

	if !reflect.DeepEqual(childJobs, out) {
		t.Fatalf("bad: %#v %#v", childJobs, out)
	}
}

func TestStateStore_JobsByPeriodic(t *testing.T) {
	state := testStateStore(t)
	var periodic, nonPeriodic []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		nonPeriodic = append(nonPeriodic, job)

		err := state.UpsertJob(1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		job := mock.PeriodicJob()
		periodic = append(periodic, job)

		err := state.UpsertJob(2000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.JobsByPeriodic(true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outPeriodic []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outPeriodic = append(outPeriodic, raw.(*structs.Job))
	}

	iter, err = state.JobsByPeriodic(false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outNonPeriodic []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outNonPeriodic = append(outNonPeriodic, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(periodic))
	sort.Sort(JobIDSort(nonPeriodic))
	sort.Sort(JobIDSort(outPeriodic))
	sort.Sort(JobIDSort(outNonPeriodic))

	if !reflect.DeepEqual(periodic, outPeriodic) {
		t.Fatalf("bad: %#v %#v", periodic, outPeriodic)
	}

	if !reflect.DeepEqual(nonPeriodic, outNonPeriodic) {
		t.Fatalf("bad: %#v %#v", nonPeriodic, outNonPeriodic)
	}
}

func TestStateStore_JobsByScheduler(t *testing.T) {
	state := testStateStore(t)
	var serviceJobs []*structs.Job
	var sysJobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		serviceJobs = append(serviceJobs, job)

		err := state.UpsertJob(1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		job := mock.SystemJob()
		sysJobs = append(sysJobs, job)

		err := state.UpsertJob(2000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.JobsByScheduler("service")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outService []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outService = append(outService, raw.(*structs.Job))
	}

	iter, err = state.JobsByScheduler("system")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outSystem []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outSystem = append(outSystem, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(serviceJobs))
	sort.Sort(JobIDSort(sysJobs))
	sort.Sort(JobIDSort(outService))
	sort.Sort(JobIDSort(outSystem))

	if !reflect.DeepEqual(serviceJobs, outService) {
		t.Fatalf("bad: %#v %#v", serviceJobs, outService)
	}

	if !reflect.DeepEqual(sysJobs, outSystem) {
		t.Fatalf("bad: %#v %#v", sysJobs, outSystem)
	}
}

func TestStateStore_JobsByGC(t *testing.T) {
	state := testStateStore(t)
	var gc, nonGc []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		nonGc = append(nonGc, job)

		if err := state.UpsertJob(1000+uint64(i), job); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		job := mock.Job()
		job.GC = true
		gc = append(gc, job)

		if err := state.UpsertJob(2000+uint64(i), job); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.JobsByGC(true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outGc []*structs.Job
	for i := iter.Next(); i != nil; i = iter.Next() {
		outGc = append(outGc, i.(*structs.Job))
	}

	iter, err = state.JobsByGC(false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outNonGc []*structs.Job
	for i := iter.Next(); i != nil; i = iter.Next() {
		outNonGc = append(outNonGc, i.(*structs.Job))
	}

	sort.Sort(JobIDSort(gc))
	sort.Sort(JobIDSort(nonGc))
	sort.Sort(JobIDSort(outGc))
	sort.Sort(JobIDSort(outNonGc))

	if !reflect.DeepEqual(gc, outGc) {
		t.Fatalf("bad: %#v %#v", gc, outGc)
	}

	if !reflect.DeepEqual(nonGc, outNonGc) {
		t.Fatalf("bad: %#v %#v", nonGc, outNonGc)
	}
}

func TestStateStore_RestoreJob(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "jobs"},
		watch.Item{Job: job.ID})

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.JobRestore(job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, job) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}

	notify.verify(t)
}

func TestStateStore_Indexes(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	iter, err := state.Indexes()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*IndexEntry
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*IndexEntry))
	}

	expect := []*IndexEntry{
		&IndexEntry{"nodes", 1000},
	}

	if !reflect.DeepEqual(expect, out) {
		t.Fatalf("bad: %#v %#v", expect, out)
	}
}

func TestStateStore_RestoreIndex(t *testing.T) {
	state := testStateStore(t)

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	index := &IndexEntry{"jobs", 1000}
	err = restore.IndexRestore(index)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != 1000 {
		t.Fatalf("Bad: %#v %#v", out, 1000)
	}
}

func TestStateStore_UpsertEvals_Eval(t *testing.T) {
	state := testStateStore(t)
	eval := mock.Eval()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "evals"},
		watch.Item{Eval: eval.ID})

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval, out) {
		t.Fatalf("bad: %#v %#v", eval, out)
	}

	index, err := state.Index("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_Update_UpsertEvals_Eval(t *testing.T) {
	state := testStateStore(t)
	eval := mock.Eval()

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "evals"},
		watch.Item{Eval: eval.ID})

	eval2 := mock.Eval()
	eval2.ID = eval.ID
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval2, out) {
		t.Fatalf("bad: %#v %#v", eval2, out)
	}

	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_DeleteEval_Eval(t *testing.T) {
	state := testStateStore(t)
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "evals"},
		watch.Item{Table: "allocs"},
		watch.Item{Eval: eval1.ID},
		watch.Item{Eval: eval2.ID},
		watch.Item{Alloc: alloc1.ID},
		watch.Item{Alloc: alloc2.ID},
		watch.Item{AllocEval: alloc1.EvalID},
		watch.Item{AllocEval: alloc2.EvalID},
		watch.Item{AllocJob: alloc1.JobID},
		watch.Item{AllocJob: alloc2.JobID},
		watch.Item{AllocNode: alloc1.NodeID},
		watch.Item{AllocNode: alloc2.NodeID})

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval1, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc1, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeleteEval(1002, []string{eval1.ID, eval2.ID}, []string{alloc1.ID, alloc2.ID})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.EvalByID(eval1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", eval1, out)
	}

	out, err = state.EvalByID(eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", eval1, out)
	}

	outA, err := state.AllocByID(alloc1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", alloc1, outA)
	}

	outA, err = state.AllocByID(alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", alloc1, outA)
	}

	index, err := state.Index("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1002 {
		t.Fatalf("bad: %d", index)
	}

	index, err = state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1002 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_EvalsByJob(t *testing.T) {
	state := testStateStore(t)

	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	eval3 := mock.Eval()
	evals := []*structs.Evaluation{eval1, eval2}

	err := state.UpsertEvals(1000, evals)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval3})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.EvalsByJob(eval1.JobID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sort.Sort(EvalIDSort(evals))
	sort.Sort(EvalIDSort(out))

	if !reflect.DeepEqual(evals, out) {
		t.Fatalf("bad: %#v %#v", evals, out)
	}
}

func TestStateStore_Evals(t *testing.T) {
	state := testStateStore(t)
	var evals []*structs.Evaluation

	for i := 0; i < 10; i++ {
		eval := mock.Eval()
		evals = append(evals, eval)

		err := state.UpsertEvals(1000+uint64(i), []*structs.Evaluation{eval})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.Evals()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Evaluation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Evaluation))
	}

	sort.Sort(EvalIDSort(evals))
	sort.Sort(EvalIDSort(out))

	if !reflect.DeepEqual(evals, out) {
		t.Fatalf("bad: %#v %#v", evals, out)
	}
}

func TestStateStore_RestoreEval(t *testing.T) {
	state := testStateStore(t)
	eval := mock.Eval()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "evals"},
		watch.Item{Eval: eval.ID})

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.EvalRestore(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	out, err := state.EvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, eval) {
		t.Fatalf("Bad: %#v %#v", out, eval)
	}

	notify.verify(t)
}

func TestStateStore_UpdateAllocFromClient(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "allocs"},
		watch.Item{Alloc: alloc.ID},
		watch.Item{AllocEval: alloc.EvalID},
		watch.Item{AllocJob: alloc.JobID},
		watch.Item{AllocNode: alloc.NodeID})

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	update := new(structs.Allocation)
	*update = *alloc
	update.ClientStatus = structs.AllocClientStatusFailed

	err = state.UpdateAllocFromClient(1001, update)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	update.ModifyIndex = 1001
	if !reflect.DeepEqual(update, out) {
		t.Fatalf("bad: %#v %#v", update, out)
	}

	index, err := state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_UpsertAlloc_Alloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "allocs"},
		watch.Item{Alloc: alloc.ID},
		watch.Item{AllocEval: alloc.EvalID},
		watch.Item{AllocJob: alloc.JobID},
		watch.Item{AllocNode: alloc.NodeID})

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	index, err := state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_UpdateAlloc_Alloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2 := mock.Alloc()
	alloc2.ID = alloc.ID
	alloc2.NodeID = alloc.NodeID + ".new"

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "allocs"},
		watch.Item{Alloc: alloc2.ID},
		watch.Item{AllocEval: alloc2.EvalID},
		watch.Item{AllocJob: alloc2.JobID},
		watch.Item{AllocNode: alloc2.NodeID})

	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(alloc2, out) {
		t.Fatalf("bad: %#v %#v", alloc2, out)
	}

	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	notify.verify(t)
}

func TestStateStore_EvictAlloc_Alloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2 := new(structs.Allocation)
	*alloc2 = *alloc
	alloc2.DesiredStatus = structs.AllocDesiredStatusEvict
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.DesiredStatus != structs.AllocDesiredStatusEvict {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	index, err := state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_AllocsByNode(t *testing.T) {
	state := testStateStore(t)
	var allocs []*structs.Allocation

	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.NodeID = "foo"
		allocs = append(allocs, alloc)
	}

	err := state.UpsertAllocs(1000, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.AllocsByNode("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	if !reflect.DeepEqual(allocs, out) {
		t.Fatalf("bad: %#v %#v", allocs, out)
	}
}

func TestStateStore_AllocsByJob(t *testing.T) {
	state := testStateStore(t)
	var allocs []*structs.Allocation

	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.JobID = "foo"
		allocs = append(allocs, alloc)
	}

	err := state.UpsertAllocs(1000, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.AllocsByJob("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	if !reflect.DeepEqual(allocs, out) {
		t.Fatalf("bad: %#v %#v", allocs, out)
	}
}

func TestStateStore_Allocs(t *testing.T) {
	state := testStateStore(t)
	var allocs []*structs.Allocation

	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		allocs = append(allocs, alloc)
	}

	err := state.UpsertAllocs(1000, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	iter, err := state.Allocs()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Allocation))
	}

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	if !reflect.DeepEqual(allocs, out) {
		t.Fatalf("bad: %#v %#v", allocs, out)
	}
}

func TestStateStore_RestoreAlloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	notify := setupNotifyTest(
		state,
		watch.Item{Table: "allocs"},
		watch.Item{Alloc: alloc.ID},
		watch.Item{AllocEval: alloc.EvalID},
		watch.Item{AllocJob: alloc.JobID},
		watch.Item{AllocNode: alloc.NodeID})

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.AllocRestore(alloc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, alloc) {
		t.Fatalf("Bad: %#v %#v", out, alloc)
	}

	notify.verify(t)
}

func TestStateWatch_watch(t *testing.T) {
	sw := newStateWatch()
	notify1 := make(chan struct{}, 1)
	notify2 := make(chan struct{}, 1)
	notify3 := make(chan struct{}, 1)

	// Notifications trigger subscribed channels
	sw.watch(watch.NewItems(watch.Item{Table: "foo"}), notify1)
	sw.watch(watch.NewItems(watch.Item{Table: "bar"}), notify2)
	sw.watch(watch.NewItems(watch.Item{Table: "baz"}), notify3)

	items := watch.NewItems()
	items.Add(watch.Item{Table: "foo"})
	items.Add(watch.Item{Table: "bar"})

	sw.notify(items)
	if len(notify1) != 1 {
		t.Fatalf("should notify")
	}
	if len(notify2) != 1 {
		t.Fatalf("should notify")
	}
	if len(notify3) != 0 {
		t.Fatalf("should not notify")
	}
}

func TestStateWatch_stopWatch(t *testing.T) {
	sw := newStateWatch()
	notify := make(chan struct{})

	// First subscribe
	sw.watch(watch.NewItems(watch.Item{Table: "foo"}), notify)

	// Unsubscribe stop notifications
	sw.stopWatch(watch.NewItems(watch.Item{Table: "foo"}), notify)

	// Check that the group was removed
	if _, ok := sw.items[watch.Item{Table: "foo"}]; ok {
		t.Fatalf("should remove group")
	}

	// Check that we are not notified
	sw.notify(watch.NewItems(watch.Item{Table: "foo"}))
	if len(notify) != 0 {
		t.Fatalf("should not notify")
	}
}

// setupNotifyTest takes a state store and a set of watch items, then creates
// and subscribes a notification channel for each item.
func setupNotifyTest(state *StateStore, items ...watch.Item) notifyTest {
	var n notifyTest
	for _, item := range items {
		ch := make(chan struct{}, 1)
		state.Watch(watch.NewItems(item), ch)
		n = append(n, &notifyTestCase{item, ch})
	}
	return n
}

// notifyTestCase is used to set up and verify watch triggers.
type notifyTestCase struct {
	item watch.Item
	ch   chan struct{}
}

// notifyTest is a suite of notifyTestCases.
type notifyTest []*notifyTestCase

// verify ensures that each channel received a notification.
func (n notifyTest) verify(t *testing.T) {
	for _, tcase := range n {
		if len(tcase.ch) != 1 {
			t.Fatalf("should notify %#v", tcase.item)
		}
	}
}

// NodeIDSort is used to sort nodes by ID
type NodeIDSort []*structs.Node

func (n NodeIDSort) Len() int {
	return len(n)
}

func (n NodeIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n NodeIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// JobIDis used to sort jobs by id
type JobIDSort []*structs.Job

func (n JobIDSort) Len() int {
	return len(n)
}

func (n JobIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n JobIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// EvalIDis used to sort evals by id
type EvalIDSort []*structs.Evaluation

func (n EvalIDSort) Len() int {
	return len(n)
}

func (n EvalIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n EvalIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// AllocIDsort used to sort allocations by id
type AllocIDSort []*structs.Allocation

func (n AllocIDSort) Len() int {
	return len(n)
}

func (n AllocIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n AllocIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
