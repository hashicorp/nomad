package state

import (
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
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

func TestStateStore_RegisterNode_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(node, out) {
		t.Fatalf("bad: %#v %#v", node, out)
	}

	index, err := state.GetIndex("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_DeregisterNode_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeregisterNode(1001, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", node, out)
	}

	index, err := state.GetIndex("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_UpdateNodeStatus_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpdateNodeStatus(1001, node.ID, structs.NodeStatusReady)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.Status != structs.NodeStatusReady {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.GetIndex("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_UpdateNodeDrain_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpdateNodeDrain(1001, node.ID, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !out.Drain {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.GetIndex("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_Nodes(t *testing.T) {
	state := testStateStore(t)
	var nodes []*structs.Node

	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)

		err := state.RegisterNode(1000+uint64(i), node)
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

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	node := mock.Node()
	err = restore.NodeRestore(node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, node) {
		t.Fatalf("Bad: %#v %#v", out, node)
	}
}

func TestStateStore_RegisterJob_GetJob(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	err := state.RegisterJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(job, out) {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.GetIndex("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_UpdateRegisterJob_GetJob(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	err := state.RegisterJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	job2 := mock.Job()
	job2.ID = job.ID
	err = state.RegisterJob(1001, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetJobByID(job.ID)
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

	index, err := state.GetIndex("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_DeregisterJob_GetJob(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	err := state.RegisterJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeregisterJob(1001, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.GetIndex("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_Jobs(t *testing.T) {
	state := testStateStore(t)
	var jobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		jobs = append(jobs, job)

		err := state.RegisterJob(1000+uint64(i), job)
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

func TestStateStore_RestoreJob(t *testing.T) {
	state := testStateStore(t)

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	job := mock.Job()
	err = restore.JobRestore(job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, job) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}
}

func TestStateStore_Indexes(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.RegisterNode(1000, node)
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

	out, err := state.GetIndex("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != 1000 {
		t.Fatalf("Bad: %#v %#v", out, 1000)
	}
}

func TestStateStore_UpsertEvals_GetEval(t *testing.T) {
	state := testStateStore(t)
	eval := mock.Eval()

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetEvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval, out) {
		t.Fatalf("bad: %#v %#v", eval, out)
	}

	index, err := state.GetIndex("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_Update_UpsertEvals_GetEval(t *testing.T) {
	state := testStateStore(t)
	eval := mock.Eval()

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	eval2 := mock.Eval()
	eval2.ID = eval.ID
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetEvalByID(eval.ID)
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

	index, err := state.GetIndex("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_DeleteEval_GetEval(t *testing.T) {
	state := testStateStore(t)
	eval := mock.Eval()
	eval2 := mock.Eval()
	alloc := mock.Alloc()
	alloc2 := mock.Alloc()

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpdateAllocations(1001, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	notify1 := make(chan struct{}, 1)
	state.WatchAllocs(alloc.NodeID, notify1)

	err = state.DeleteEval(1002, []string{eval.ID, eval2.ID}, []string{alloc.ID, alloc2.ID})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetEvalByID(eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", eval, out)
	}

	out, err = state.GetEvalByID(eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", eval, out)
	}

	outA, err := state.GetAllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", alloc, outA)
	}

	outA, err = state.GetAllocByID(alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", alloc, outA)
	}

	index, err := state.GetIndex("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1002 {
		t.Fatalf("bad: %d", index)
	}

	index, err = state.GetIndex("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1002 {
		t.Fatalf("bad: %d", index)
	}

	select {
	case <-notify1:
	default:
		t.Fatalf("should be notified")
	}
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

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	job := mock.Eval()
	err = restore.EvalRestore(job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.GetEvalByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, job) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}
}

func TestStateStore_UpdateAllocFromClient(t *testing.T) {
	state := testStateStore(t)

	alloc := mock.Alloc()
	err := state.UpdateAllocations(1000, []*structs.Allocation{alloc})
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

	out, err := state.GetAllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	update.ModifyIndex = 1001
	if !reflect.DeepEqual(update, out) {
		t.Fatalf("bad: %#v %#v", update, out)
	}

	index, err := state.GetIndex("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_UpsertAlloc_GetAlloc(t *testing.T) {
	state := testStateStore(t)

	alloc := mock.Alloc()
	err := state.UpdateAllocations(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetAllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	index, err := state.GetIndex("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_WatchAllocs(t *testing.T) {
	state := testStateStore(t)

	notify1 := make(chan struct{}, 1)
	notify2 := make(chan struct{}, 1)
	state.WatchAllocs("foo", notify1)
	state.WatchAllocs("foo", notify2)
	state.StopWatchAllocs("foo", notify2)

	alloc := mock.Alloc()
	alloc.NodeID = "foo"
	err := state.UpdateAllocations(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case <-notify1:
	default:
		t.Fatalf("should be notified")
	}

	select {
	case <-notify2:
		t.Fatalf("should not be notified")
	default:
	}
}

func TestStateStore_UpdateAlloc_GetAlloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	err := state.UpdateAllocations(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2 := mock.Alloc()
	alloc2.ID = alloc.ID
	alloc2.NodeID = alloc.NodeID + ".new"
	err = state.UpdateAllocations(1001, []*structs.Allocation{alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetAllocByID(alloc.ID)
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

	index, err := state.GetIndex("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_EvictAlloc_GetAlloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	err := state.UpdateAllocations(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2 := new(structs.Allocation)
	*alloc2 = *alloc
	alloc2.DesiredStatus = structs.AllocDesiredStatusEvict
	err = state.UpdateAllocations(1001, []*structs.Allocation{alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetAllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.DesiredStatus != structs.AllocDesiredStatusEvict {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	index, err := state.GetIndex("allocs")
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

	err := state.UpdateAllocations(1000, allocs)
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

	err := state.UpdateAllocations(1000, allocs)
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

	err := state.UpdateAllocations(1000, allocs)
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

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc := mock.Alloc()
	err = restore.AllocRestore(alloc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.GetAllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, alloc) {
		t.Fatalf("Bad: %#v %#v", out, alloc)
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
