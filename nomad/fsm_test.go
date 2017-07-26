package nomad

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/kr/pretty"
)

type MockSink struct {
	*bytes.Buffer
	cancel bool
}

func (m *MockSink) ID() string {
	return "Mock"
}

func (m *MockSink) Cancel() error {
	m.cancel = true
	return nil
}

func (m *MockSink) Close() error {
	return nil
}

func testStateStore(t *testing.T) *state.StateStore {
	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if state == nil {
		t.Fatalf("missing state")
	}
	return state
}

func testFSM(t *testing.T) *nomadFSM {
	p, _ := testPeriodicDispatcher()
	broker := testBroker(t, 0)
	blocked := NewBlockedEvals(broker)
	fsm, err := NewFSM(broker, p, blocked, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fsm == nil {
		t.Fatalf("missing fsm")
	}
	return fsm
}

func makeLog(buf []byte) *raft.Log {
	return &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  buf,
	}
}

func TestFSM_UpsertNode(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	node := mock.Node()

	// Mark an eval as blocked.
	eval := mock.Eval()
	eval.ClassEligibility = map[string]bool{node.ComputedClass: true}
	fsm.blockedEvals.Block(eval)

	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	n, err := fsm.State().NodeByID(ws, req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n == nil {
		t.Fatalf("not found!")
	}
	if n.CreateIndex != 1 {
		t.Fatalf("bad index: %d", node.CreateIndex)
	}

	tt := fsm.TimeTable()
	index := tt.NearestIndex(time.Now().UTC())
	if index != 1 {
		t.Fatalf("bad: %d", index)
	}

	// Verify the eval was unblocked.
	testutil.WaitForResult(func() (bool, error) {
		bStats := fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

}

func TestFSM_DeregisterNode(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.NodeDeregisterRequest{
		NodeID: node.ID,
	}
	buf, err = structs.Encode(structs.NodeDeregisterRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	node, err = fsm.State().NodeByID(ws, req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node != nil {
		t.Fatalf("node found!")
	}
}

func TestFSM_UpdateNodeStatus(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Mark an eval as blocked.
	eval := mock.Eval()
	eval.ClassEligibility = map[string]bool{node.ComputedClass: true}
	fsm.blockedEvals.Block(eval)

	req2 := structs.NodeUpdateStatusRequest{
		NodeID: node.ID,
		Status: structs.NodeStatusReady,
	}
	buf, err = structs.Encode(structs.NodeUpdateStatusRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify the status is ready.
	ws := memdb.NewWatchSet()
	node, err = fsm.State().NodeByID(ws, req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node.Status != structs.NodeStatusReady {
		t.Fatalf("bad node: %#v", node)
	}

	// Verify the eval was unblocked.
	testutil.WaitForResult(func() (bool, error) {
		bStats := fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestFSM_UpdateNodeDrain(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	node := mock.Node()
	req := structs.NodeRegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.NodeRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.NodeUpdateDrainRequest{
		NodeID: node.ID,
		Drain:  true,
	}
	buf, err = structs.Encode(structs.NodeUpdateDrainRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	node, err = fsm.State().NodeByID(ws, req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !node.Drain {
		t.Fatalf("bad node: %#v", node)
	}
}

func TestFSM_RegisterJob(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	job := mock.PeriodicJob()
	req := structs.JobRegisterRequest{
		Job: job,
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if jobOut == nil {
		t.Fatalf("not found!")
	}
	if jobOut.CreateIndex != 1 {
		t.Fatalf("bad index: %d", jobOut.CreateIndex)
	}

	// Verify it was added to the periodic runner.
	if _, ok := fsm.periodicDispatcher.tracked[job.ID]; !ok {
		t.Fatal("job not added to periodic runner")
	}

	// Verify the launch time was tracked.
	launchOut, err := fsm.State().PeriodicLaunchByID(ws, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if launchOut == nil {
		t.Fatalf("not found!")
	}
	if launchOut.Launch.IsZero() {
		t.Fatalf("bad launch time: %v", launchOut.Launch)
	}
}

func TestFSM_DeregisterJob_Purge(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	job := mock.PeriodicJob()
	req := structs.JobRegisterRequest{
		Job: job,
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
	}
	buf, err = structs.Encode(structs.JobDeregisterRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if jobOut != nil {
		t.Fatalf("job found!")
	}

	// Verify it was removed from the periodic runner.
	if _, ok := fsm.periodicDispatcher.tracked[job.ID]; ok {
		t.Fatal("job not removed from periodic runner")
	}

	// Verify it was removed from the periodic launch table.
	launchOut, err := fsm.State().PeriodicLaunchByID(ws, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if launchOut != nil {
		t.Fatalf("launch found!")
	}
}

func TestFSM_DeregisterJob_NoPurge(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	job := mock.PeriodicJob()
	req := structs.JobRegisterRequest{
		Job: job,
	}
	buf, err := structs.Encode(structs.JobRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: false,
	}
	buf, err = structs.Encode(structs.JobDeregisterRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	jobOut, err := fsm.State().JobByID(ws, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if jobOut == nil {
		t.Fatalf("job not found!")
	}
	if !jobOut.Stop {
		t.Fatalf("job not stopped found!")
	}

	// Verify it was removed from the periodic runner.
	if _, ok := fsm.periodicDispatcher.tracked[job.ID]; ok {
		t.Fatal("job not removed from periodic runner")
	}

	// Verify it was removed from the periodic launch table.
	launchOut, err := fsm.State().PeriodicLaunchByID(ws, req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if launchOut == nil {
		t.Fatalf("launch not found!")
	}
}

func TestFSM_UpdateEval(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)

	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{mock.Eval()},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	eval, err := fsm.State().EvalByID(ws, req.Evals[0].ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("not found!")
	}
	if eval.CreateIndex != 1 {
		t.Fatalf("bad index: %d", eval.CreateIndex)
	}

	// Verify enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v %#v", stats, eval)
	}
}

func TestFSM_UpdateEval_Blocked(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	fsm.blockedEvals.SetEnabled(true)

	// Create a blocked eval.
	eval := mock.Eval()
	eval.Status = structs.EvalStatusBlocked

	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("not found!")
	}
	if out.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out.CreateIndex)
	}

	// Verify the eval wasn't enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v %#v", stats, out)
	}

	// Verify the eval was added to the blocked tracker.
	bStats := fsm.blockedEvals.Stats()
	if bStats.TotalBlocked != 1 {
		t.Fatalf("bad: %#v %#v", bStats, out)
	}
}

func TestFSM_UpdateEval_Untrack(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	fsm.blockedEvals.SetEnabled(true)

	// Mark an eval as blocked.
	bEval := mock.Eval()
	bEval.ClassEligibility = map[string]bool{"v1:123": true}
	fsm.blockedEvals.Block(bEval)

	// Create a successful eval for the same job
	eval := mock.Eval()
	eval.JobID = bEval.JobID
	eval.Status = structs.EvalStatusComplete

	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("not found!")
	}
	if out.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out.CreateIndex)
	}

	// Verify the eval wasn't enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v %#v", stats, out)
	}

	// Verify the eval was untracked in the blocked tracker.
	bStats := fsm.blockedEvals.Stats()
	if bStats.TotalBlocked != 0 {
		t.Fatalf("bad: %#v %#v", bStats, out)
	}
}

func TestFSM_UpdateEval_NoUntrack(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	fsm.blockedEvals.SetEnabled(true)

	// Mark an eval as blocked.
	bEval := mock.Eval()
	bEval.ClassEligibility = map[string]bool{"v1:123": true}
	fsm.blockedEvals.Block(bEval)

	// Create a successful eval for the same job but with placement failures
	eval := mock.Eval()
	eval.JobID = bEval.JobID
	eval.Status = structs.EvalStatusComplete
	eval.FailedTGAllocs = make(map[string]*structs.AllocMetric)
	eval.FailedTGAllocs["test"] = new(structs.AllocMetric)

	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("not found!")
	}
	if out.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out.CreateIndex)
	}

	// Verify the eval wasn't enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 0 {
		t.Fatalf("bad: %#v %#v", stats, out)
	}

	// Verify the eval was not untracked in the blocked tracker.
	bStats := fsm.blockedEvals.Stats()
	if bStats.TotalBlocked != 1 {
		t.Fatalf("bad: %#v %#v", bStats, out)
	}
}

func TestFSM_DeleteEval(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	eval := mock.Eval()
	req := structs.EvalUpdateRequest{
		Evals: []*structs.Evaluation{eval},
	}
	buf, err := structs.Encode(structs.EvalUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.EvalDeleteRequest{
		Evals: []string{eval.ID},
	}
	buf, err = structs.Encode(structs.EvalDeleteRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	eval, err = fsm.State().EvalByID(ws, req.Evals[0].ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval != nil {
		t.Fatalf("eval found!")
	}
}

func TestFSM_UpsertAllocs(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	alloc := mock.Alloc()
	fsm.State().UpsertJobSummary(1, mock.JobSummary(alloc.JobID))
	req := structs.AllocUpdateRequest{
		Alloc: []*structs.Allocation{alloc},
	}
	buf, err := structs.Encode(structs.AllocUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	alloc.CreateIndex = out.CreateIndex
	alloc.ModifyIndex = out.ModifyIndex
	alloc.AllocModifyIndex = out.AllocModifyIndex
	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	evictAlloc := new(structs.Allocation)
	*evictAlloc = *alloc
	evictAlloc.DesiredStatus = structs.AllocDesiredStatusEvict
	req2 := structs.AllocUpdateRequest{
		Alloc: []*structs.Allocation{evictAlloc},
	}
	buf, err = structs.Encode(structs.AllocUpdateRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are evicted
	out, err = fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.DesiredStatus != structs.AllocDesiredStatusEvict {
		t.Fatalf("alloc found!")
	}
}

func TestFSM_UpsertAllocs_SharedJob(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	alloc := mock.Alloc()
	fsm.State().UpsertJobSummary(1, mock.JobSummary(alloc.JobID))
	job := alloc.Job
	alloc.Job = nil
	req := structs.AllocUpdateRequest{
		Job:   job,
		Alloc: []*structs.Allocation{alloc},
	}
	buf, err := structs.Encode(structs.AllocUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	alloc.CreateIndex = out.CreateIndex
	alloc.ModifyIndex = out.ModifyIndex
	alloc.AllocModifyIndex = out.AllocModifyIndex

	// Job should be re-attached
	alloc.Job = job
	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	// Ensure that the original job is used
	evictAlloc := new(structs.Allocation)
	*evictAlloc = *alloc
	job = mock.Job()
	job.Priority = 123

	evictAlloc.Job = nil
	evictAlloc.DesiredStatus = structs.AllocDesiredStatusEvict
	req2 := structs.AllocUpdateRequest{
		Job:   job,
		Alloc: []*structs.Allocation{evictAlloc},
	}
	buf, err = structs.Encode(structs.AllocUpdateRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are evicted
	out, err = fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.DesiredStatus != structs.AllocDesiredStatusEvict {
		t.Fatalf("alloc found!")
	}
	if out.Job == nil || out.Job.Priority == 123 {
		t.Fatalf("bad job")
	}
}

func TestFSM_UpsertAllocs_StrippedResources(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	alloc := mock.Alloc()
	fsm.State().UpsertJobSummary(1, mock.JobSummary(alloc.JobID))
	job := alloc.Job
	resources := alloc.Resources
	alloc.Resources = nil
	req := structs.AllocUpdateRequest{
		Job:   job,
		Alloc: []*structs.Allocation{alloc},
	}
	buf, err := structs.Encode(structs.AllocUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	alloc.CreateIndex = out.CreateIndex
	alloc.ModifyIndex = out.ModifyIndex
	alloc.AllocModifyIndex = out.AllocModifyIndex

	// Resources should be recomputed
	resources.DiskMB = alloc.Job.TaskGroups[0].EphemeralDisk.SizeMB
	alloc.Resources = resources
	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}
}

func TestFSM_UpdateAllocFromClient_Unblock(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)
	state := fsm.State()

	node := mock.Node()
	state.UpsertNode(1, node)

	// Mark an eval as blocked.
	eval := mock.Eval()
	eval.ClassEligibility = map[string]bool{node.ComputedClass: true}
	fsm.blockedEvals.Block(eval)

	bStats := fsm.blockedEvals.Stats()
	if bStats.TotalBlocked != 1 {
		t.Fatalf("bad: %#v", bStats)
	}

	// Create a completed eval
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	state.UpsertJobSummary(8, mock.JobSummary(alloc.JobID))
	state.UpsertJobSummary(9, mock.JobSummary(alloc2.JobID))
	state.UpsertAllocs(10, []*structs.Allocation{alloc, alloc2})

	clientAlloc := new(structs.Allocation)
	*clientAlloc = *alloc
	clientAlloc.ClientStatus = structs.AllocClientStatusComplete
	update2 := &structs.Allocation{
		ID:           alloc2.ID,
		ClientStatus: structs.AllocClientStatusRunning,
	}

	req := structs.AllocUpdateRequest{
		Alloc: []*structs.Allocation{clientAlloc, update2},
	}
	buf, err := structs.Encode(structs.AllocClientUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are updated
	ws := memdb.NewWatchSet()
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	clientAlloc.CreateIndex = out.CreateIndex
	clientAlloc.ModifyIndex = out.ModifyIndex
	if !reflect.DeepEqual(clientAlloc, out) {
		t.Fatalf("bad: %#v %#v", clientAlloc, out)
	}

	out, err = fsm.State().AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	alloc2.CreateIndex = out.CreateIndex
	alloc2.ModifyIndex = out.ModifyIndex
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	alloc2.TaskStates = nil
	if !reflect.DeepEqual(alloc2, out) {
		t.Fatalf("bad: %#v %#v", alloc2, out)
	}

	// Verify the eval was unblocked.
	testutil.WaitForResult(func() (bool, error) {
		bStats = fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v %#v", bStats, out)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestFSM_UpdateAllocFromClient(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	state := fsm.State()

	alloc := mock.Alloc()
	state.UpsertJobSummary(9, mock.JobSummary(alloc.JobID))
	state.UpsertAllocs(10, []*structs.Allocation{alloc})

	clientAlloc := new(structs.Allocation)
	*clientAlloc = *alloc
	clientAlloc.ClientStatus = structs.AllocClientStatusFailed

	req := structs.AllocUpdateRequest{
		Alloc: []*structs.Allocation{clientAlloc},
	}
	buf, err := structs.Encode(structs.AllocClientUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	clientAlloc.CreateIndex = out.CreateIndex
	clientAlloc.ModifyIndex = out.ModifyIndex
	if !reflect.DeepEqual(clientAlloc, out) {
		t.Fatalf("err: %#v,%#v", clientAlloc, out)
	}
}

func TestFSM_UpsertVaultAccessor(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	va := mock.VaultAccessor()
	va2 := mock.VaultAccessor()
	req := structs.VaultAccessorsRequest{
		Accessors: []*structs.VaultAccessor{va, va2},
	}
	buf, err := structs.Encode(structs.VaultAccessorRegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out1, err := fsm.State().VaultAccessor(ws, va.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out1 == nil {
		t.Fatalf("not found!")
	}
	if out1.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out1.CreateIndex)
	}
	out2, err := fsm.State().VaultAccessor(ws, va2.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out2 == nil {
		t.Fatalf("not found!")
	}
	if out1.CreateIndex != 1 {
		t.Fatalf("bad index: %d", out2.CreateIndex)
	}

	tt := fsm.TimeTable()
	index := tt.NearestIndex(time.Now().UTC())
	if index != 1 {
		t.Fatalf("bad: %d", index)
	}
}

func TestFSM_DeregisterVaultAccessor(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.blockedEvals.SetEnabled(true)

	va := mock.VaultAccessor()
	va2 := mock.VaultAccessor()
	accessors := []*structs.VaultAccessor{va, va2}

	// Insert the accessors
	if err := fsm.State().UpsertVaultAccessor(1000, accessors); err != nil {
		t.Fatalf("bad: %v", err)
	}

	req := structs.VaultAccessorsRequest{
		Accessors: accessors,
	}
	buf, err := structs.Encode(structs.VaultAccessorDegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	ws := memdb.NewWatchSet()
	out1, err := fsm.State().VaultAccessor(ws, va.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out1 != nil {
		t.Fatalf("not deleted!")
	}

	tt := fsm.TimeTable()
	index := tt.NearestIndex(time.Now().UTC())
	if index != 1 {
		t.Fatalf("bad: %d", index)
	}
}

func TestFSM_ApplyPlanResults(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	// Create the request and create a deployment
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	d := mock.Deployment()
	d.JobID = job.ID
	d.JobModifyIndex = job.ModifyIndex
	d.JobVersion = job.Version

	alloc.DeploymentID = d.ID

	fsm.State().UpsertJobSummary(1, mock.JobSummary(alloc.JobID))
	req := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Job:   job,
			Alloc: []*structs.Allocation{alloc},
		},
		Deployment: d,
	}
	buf, err := structs.Encode(structs.ApplyPlanResultsRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify the allocation is registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	alloc.CreateIndex = out.CreateIndex
	alloc.ModifyIndex = out.ModifyIndex
	alloc.AllocModifyIndex = out.AllocModifyIndex

	// Job should be re-attached
	alloc.Job = job
	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	dout, err := fsm.State().DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tg, ok := dout.TaskGroups[alloc.TaskGroup]; !ok || tg.PlacedAllocs != 1 {
		t.Fatalf("err: %v %v", tg, err)
	}

	// Ensure that the original job is used
	evictAlloc := alloc.Copy()
	job = mock.Job()
	job.Priority = 123

	evictAlloc.Job = nil
	evictAlloc.DesiredStatus = structs.AllocDesiredStatusEvict
	req2 := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Job:   job,
			Alloc: []*structs.Allocation{evictAlloc},
		},
	}
	buf, err = structs.Encode(structs.ApplyPlanResultsRequestType, req2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are evicted
	out, err = fsm.State().AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.DesiredStatus != structs.AllocDesiredStatusEvict {
		t.Fatalf("alloc found!")
	}
	if out.Job == nil || out.Job.Priority == 123 {
		t.Fatalf("bad job")
	}
}

func TestFSM_DeploymentStatusUpdate(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	state := fsm.State()

	// Upsert a deployment
	d := mock.Deployment()
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a request to update the deployment, create an eval and job
	e := mock.Eval()
	j := mock.Job()
	status, desc := structs.DeploymentStatusFailed, "foo"
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            status,
			StatusDescription: desc,
		},
		Job:  j,
		Eval: e,
	}
	buf, err := structs.Encode(structs.DeploymentStatusUpdateRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if dout.Status != status || dout.StatusDescription != desc {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, j.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}

	// Assert the eval was enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v %#v", stats, e)
	}
}

func TestFSM_JobStabilityUpdate(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	state := fsm.State()

	// Upsert a deployment
	job := mock.Job()
	if err := state.UpsertJob(1, job); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a request to update the job to stable
	req := &structs.JobStabilityRequest{
		JobID:      job.ID,
		JobVersion: job.Version,
		Stable:     true,
	}
	buf, err := structs.Encode(structs.JobStabilityRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Check that the stability was updated properly
	ws := memdb.NewWatchSet()
	jout, _ := state.JobByIDAndVersion(ws, job.ID, job.Version)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil || !jout.Stable {
		t.Fatalf("bad: %#v", jout)
	}
}

func TestFSM_DeploymentPromotion(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	state := fsm.State()

	// Create a job with two task groups
	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	if err := state.UpsertJob(1, j); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a deployment
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": &structs.DeploymentState{
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
		"foo": &structs.DeploymentState{
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
	}
	if err := state.UpsertDeployment(2, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a set of allocations
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}

	if err := state.UpsertAllocs(3, []*structs.Allocation{c1, c2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create an eval
	e := mock.Eval()

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: e,
	}
	buf, err := structs.Encode(structs.DeploymentPromoteRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Check that the status per task group was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if len(dout.TaskGroups) != 2 {
		t.Fatalf("bad: %#v", dout.TaskGroups)
	}
	for tg, state := range dout.TaskGroups {
		if !state.Promoted {
			t.Fatalf("bad: group %q not promoted %#v", tg, state)
		}
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Assert the eval was enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v %#v", stats, e)
	}
}

func TestFSM_DeploymentAllocHealth(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	fsm.evalBroker.SetEnabled(true)
	state := fsm.State()

	// Insert a deployment
	d := mock.Deployment()
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Insert two allocations
	a1 := mock.Alloc()
	a1.DeploymentID = d.ID
	a2 := mock.Alloc()
	a2.DeploymentID = d.ID
	if err := state.UpsertAllocs(2, []*structs.Allocation{a1, a2}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a job to roll back to
	j := mock.Job()

	// Create an eval that should be upserted
	e := mock.Eval()

	// Create a status update for the deployment
	status, desc := structs.DeploymentStatusFailed, "foo"
	u := &structs.DeploymentStatusUpdate{
		DeploymentID:      d.ID,
		Status:            status,
		StatusDescription: desc,
	}

	// Set health against the deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			HealthyAllocationIDs:   []string{a1.ID},
			UnhealthyAllocationIDs: []string{a2.ID},
		},
		Job:              j,
		Eval:             e,
		DeploymentUpdate: u,
	}
	buf, err := structs.Encode(structs.DeploymentAllocHealthRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if dout.Status != status || dout.StatusDescription != desc {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, j.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}

	// Check the status of the allocs
	out1, err := state.AllocByID(ws, a1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	out2, err := state.AllocByID(ws, a2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !out1.DeploymentStatus.IsHealthy() {
		t.Fatalf("bad: alloc %q not healthy", out1.ID)
	}
	if !out2.DeploymentStatus.IsUnhealthy() {
		t.Fatalf("bad: alloc %q not unhealthy", out2.ID)
	}

	// Assert the eval was enqueued
	stats := fsm.evalBroker.Stats()
	if stats.TotalReady != 1 {
		t.Fatalf("bad: %#v %#v", stats, e)
	}
}

func TestFSM_DeleteDeployment(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)
	state := fsm.State()

	// Upsert a deployments
	d := mock.Deployment()
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	req := structs.DeploymentDeleteRequest{
		Deployments: []string{d.ID},
	}
	buf, err := structs.Encode(structs.DeploymentDeleteRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	deployment, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if deployment != nil {
		t.Fatalf("deployment found!")
	}
}

func testSnapshotRestore(t *testing.T, fsm *nomadFSM) *nomadFSM {
	// Snapshot
	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Release()

	// Persist
	buf := bytes.NewBuffer(nil)
	sink := &MockSink{buf, false}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try to restore on a new FSM
	fsm2 := testFSM(t)
	snap, err = fsm2.Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Release()

	abandonCh := fsm2.State().AbandonCh()

	// Do a restore
	if err := fsm2.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case <-abandonCh:
	default:
		t.Fatalf("bad")
	}

	return fsm2
}

func TestFSM_SnapshotRestore_Nodes(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	node1 := mock.Node()
	state.UpsertNode(1000, node1)
	node2 := mock.Node()
	state.UpsertNode(1001, node2)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.NodeByID(ws, node1.ID)
	out2, _ := state2.NodeByID(ws, node2.ID)
	if !reflect.DeepEqual(node1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, node1)
	}
	if !reflect.DeepEqual(node2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, node2)
	}
}

func TestFSM_SnapshotRestore_Jobs(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	job1 := mock.Job()
	state.UpsertJob(1000, job1)
	job2 := mock.Job()
	state.UpsertJob(1001, job2)

	// Verify the contents
	ws := memdb.NewWatchSet()
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out1, _ := state2.JobByID(ws, job1.ID)
	out2, _ := state2.JobByID(ws, job2.ID)
	if !reflect.DeepEqual(job1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, job1)
	}
	if !reflect.DeepEqual(job2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, job2)
	}
}

func TestFSM_SnapshotRestore_Evals(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	eval1 := mock.Eval()
	state.UpsertEvals(1000, []*structs.Evaluation{eval1})
	eval2 := mock.Eval()
	state.UpsertEvals(1001, []*structs.Evaluation{eval2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.EvalByID(ws, eval1.ID)
	out2, _ := state2.EvalByID(ws, eval2.ID)
	if !reflect.DeepEqual(eval1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, eval1)
	}
	if !reflect.DeepEqual(eval2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, eval2)
	}
}

func TestFSM_SnapshotRestore_Allocs(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
	state.UpsertAllocs(1000, []*structs.Allocation{alloc1})
	state.UpsertAllocs(1001, []*structs.Allocation{alloc2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.AllocByID(ws, alloc1.ID)
	out2, _ := state2.AllocByID(ws, alloc2.ID)
	if !reflect.DeepEqual(alloc1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, alloc1)
	}
	if !reflect.DeepEqual(alloc2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, alloc2)
	}
}

func TestFSM_SnapshotRestore_Allocs_NoSharedResources(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc1.SharedResources = nil
	alloc2.SharedResources = nil
	state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
	state.UpsertAllocs(1000, []*structs.Allocation{alloc1})
	state.UpsertAllocs(1001, []*structs.Allocation{alloc2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.AllocByID(ws, alloc1.ID)
	out2, _ := state2.AllocByID(ws, alloc2.ID)
	alloc1.SharedResources = &structs.Resources{DiskMB: 150}
	alloc2.SharedResources = &structs.Resources{DiskMB: 150}

	if !reflect.DeepEqual(alloc1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, alloc1)
	}
	if !reflect.DeepEqual(alloc2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, alloc2)
	}
}

func TestFSM_SnapshotRestore_Indexes(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	node1 := mock.Node()
	state.UpsertNode(1000, node1)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()

	index, err := state2.Index("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}

func TestFSM_SnapshotRestore_TimeTable(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)

	tt := fsm.TimeTable()
	start := time.Now().UTC()
	tt.Witness(1000, start)
	tt.Witness(2000, start.Add(10*time.Minute))

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)

	tt2 := fsm2.TimeTable()
	if tt2.NearestTime(1500) != start {
		t.Fatalf("bad")
	}
	if tt2.NearestIndex(start.Add(15*time.Minute)) != 2000 {
		t.Fatalf("bad")
	}
}

func TestFSM_SnapshotRestore_PeriodicLaunches(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	job1 := mock.Job()
	launch1 := &structs.PeriodicLaunch{ID: job1.ID, Launch: time.Now()}
	state.UpsertPeriodicLaunch(1000, launch1)
	job2 := mock.Job()
	launch2 := &structs.PeriodicLaunch{ID: job2.ID, Launch: time.Now()}
	state.UpsertPeriodicLaunch(1001, launch2)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.PeriodicLaunchByID(ws, launch1.ID)
	out2, _ := state2.PeriodicLaunchByID(ws, launch2.ID)
	if !reflect.DeepEqual(launch1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, job1)
	}
	if !reflect.DeepEqual(launch2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, job2)
	}
}

func TestFSM_SnapshotRestore_JobSummary(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()

	job1 := mock.Job()
	state.UpsertJob(1000, job1)
	ws := memdb.NewWatchSet()
	js1, _ := state.JobSummaryByID(ws, job1.ID)

	job2 := mock.Job()
	state.UpsertJob(1001, job2)
	js2, _ := state.JobSummaryByID(ws, job2.ID)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out1, _ := state2.JobSummaryByID(ws, job1.ID)
	out2, _ := state2.JobSummaryByID(ws, job2.ID)
	if !reflect.DeepEqual(js1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", js1, out1)
	}
	if !reflect.DeepEqual(js2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", js2, out2)
	}
}

func TestFSM_SnapshotRestore_VaultAccessors(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	a1 := mock.VaultAccessor()
	a2 := mock.VaultAccessor()
	state.UpsertVaultAccessor(1000, []*structs.VaultAccessor{a1, a2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.VaultAccessor(ws, a1.Accessor)
	out2, _ := state2.VaultAccessor(ws, a2.Accessor)
	if !reflect.DeepEqual(a1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, a1)
	}
	if !reflect.DeepEqual(a2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, a2)
	}
}

func TestFSM_SnapshotRestore_JobVersions(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	job1 := mock.Job()
	state.UpsertJob(1000, job1)
	job2 := mock.Job()
	job2.ID = job1.ID
	state.UpsertJob(1001, job2)

	// Verify the contents
	ws := memdb.NewWatchSet()
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out1, _ := state2.JobByIDAndVersion(ws, job1.ID, job1.Version)
	out2, _ := state2.JobByIDAndVersion(ws, job2.ID, job2.Version)
	if !reflect.DeepEqual(job1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, job1)
	}
	if !reflect.DeepEqual(job2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, job2)
	}
	if job2.Version != 1 {
		t.Fatalf("bad: \n%#v\n%#v", 1, job2)
	}
}

func TestFSM_SnapshotRestore_Deployments(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	state.UpsertDeployment(1000, d1)
	state.UpsertDeployment(1001, d2)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.DeploymentByID(ws, d1.ID)
	out2, _ := state2.DeploymentByID(ws, d2.ID)
	if !reflect.DeepEqual(d1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, d1)
	}
	if !reflect.DeepEqual(d2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, d2)
	}
}

func TestFSM_SnapshotRestore_AddMissingSummary(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()

	// make an allocation
	alloc := mock.Alloc()
	state.UpsertJob(1010, alloc.Job)
	state.UpsertAllocs(1011, []*structs.Allocation{alloc})

	// Delete the summary
	state.DeleteJobSummary(1040, alloc.Job.ID)

	// Delete the index
	if err := state.RemoveIndex("job_summary"); err != nil {
		t.Fatalf("err: %v", err)
	}

	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	latestIndex, _ := state.LatestIndex()

	ws := memdb.NewWatchSet()
	out, _ := state2.JobSummaryByID(ws, alloc.Job.ID)
	expected := structs.JobSummary{
		JobID: alloc.Job.ID,
		Summary: map[string]structs.TaskGroupSummary{
			"web": structs.TaskGroupSummary{
				Starting: 1,
			},
		},
		CreateIndex: 1010,
		ModifyIndex: latestIndex,
	}
	if !reflect.DeepEqual(&expected, out) {
		t.Fatalf("expected: %#v, actual: %#v", &expected, out)
	}
}

func TestFSM_ReconcileSummaries(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()

	// Add a node
	node := mock.Node()
	state.UpsertNode(800, node)

	// Make a job so that none of the tasks can be placed
	job1 := mock.Job()
	job1.TaskGroups[0].Tasks[0].Resources.CPU = 5000
	state.UpsertJob(1000, job1)

	// make a job which can make partial progress
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	state.UpsertJob(1010, alloc.Job)
	state.UpsertAllocs(1011, []*structs.Allocation{alloc})

	// Delete the summaries
	state.DeleteJobSummary(1030, job1.ID)
	state.DeleteJobSummary(1040, alloc.Job.ID)

	req := structs.GenericRequest{}
	buf, err := structs.Encode(structs.ReconcileJobSummariesRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	ws := memdb.NewWatchSet()
	out1, _ := state.JobSummaryByID(ws, job1.ID)
	expected := structs.JobSummary{
		JobID: job1.ID,
		Summary: map[string]structs.TaskGroupSummary{
			"web": structs.TaskGroupSummary{
				Queued: 10,
			},
		},
		CreateIndex: 1000,
		ModifyIndex: out1.ModifyIndex,
	}
	if !reflect.DeepEqual(&expected, out1) {
		t.Fatalf("expected: %#v, actual: %#v", &expected, out1)
	}

	// This exercises the code path which adds the allocations made by the
	// planner and the number of unplaced allocations in the reconcile summaries
	// codepath
	out2, _ := state.JobSummaryByID(ws, alloc.Job.ID)
	expected = structs.JobSummary{
		JobID: alloc.Job.ID,
		Summary: map[string]structs.TaskGroupSummary{
			"web": structs.TaskGroupSummary{
				Queued:   9,
				Starting: 1,
			},
		},
		CreateIndex: 1010,
		ModifyIndex: out2.ModifyIndex,
	}
	if !reflect.DeepEqual(&expected, out2) {
		t.Fatalf("Diff % #v", pretty.Diff(&expected, out2))
	}
}
