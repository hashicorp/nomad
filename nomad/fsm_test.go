package nomad

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
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

func testFSM(t *testing.T) *nomadFSM {
	fsm, err := NewFSM(os.Stderr)
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

func TestFSM_RegisterNode(t *testing.T) {
	fsm := testFSM(t)

	req := structs.NodeRegisterRequest{
		Node: mockNode(),
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
	node, err := fsm.State().GetNodeByID(req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node == nil {
		t.Fatalf("not found!")
	}
	if node.CreateIndex != 1 {
		t.Fatalf("bad index: %d", node.CreateIndex)
	}
}

func TestFSM_DeregisterNode(t *testing.T) {
	fsm := testFSM(t)

	node := mockNode()
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
	node, err = fsm.State().GetNodeByID(req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node != nil {
		t.Fatalf("node found!")
	}
}

func TestFSM_UpdateNodeStatus(t *testing.T) {
	fsm := testFSM(t)

	node := mockNode()
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

	// Verify we are NOT registered
	node, err = fsm.State().GetNodeByID(req.Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if node.Status != structs.NodeStatusReady {
		t.Fatalf("bad node: %#v", node)
	}
}

func TestFSM_RegisterJob(t *testing.T) {
	fsm := testFSM(t)

	req := structs.JobRegisterRequest{
		Job: mockJob(),
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
	job, err := fsm.State().GetJobByID(req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if job == nil {
		t.Fatalf("not found!")
	}
	if job.CreateIndex != 1 {
		t.Fatalf("bad index: %d", job.CreateIndex)
	}
}

func TestFSM_DeregisterJob(t *testing.T) {
	fsm := testFSM(t)

	job := mockJob()
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
	job, err = fsm.State().GetJobByID(req.Job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if job != nil {
		t.Fatalf("job found!")
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

	// Do a restore
	if err := fsm2.Restore(sink); err != nil {
		t.Fatalf("err: %v", err)
	}
	return fsm2
}

func TestFSM_SnapshotRestore_Nodes(t *testing.T) {
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	node1 := mockNode()
	state.RegisterNode(1000, node1)
	node2 := mockNode()
	state.RegisterNode(1001, node2)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out1, _ := state2.GetNodeByID(node1.ID)
	out2, _ := state2.GetNodeByID(node2.ID)
	if !reflect.DeepEqual(node1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, node1)
	}
	if !reflect.DeepEqual(node2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, node2)
	}
}

func TestFSM_SnapshotRestore_Jobs(t *testing.T) {
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	job1 := mockJob()
	state.RegisterJob(1000, job1)
	job2 := mockJob()
	state.RegisterJob(1001, job2)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	out1, _ := state2.GetJobByID(job1.ID)
	out2, _ := state2.GetJobByID(job2.ID)
	if !reflect.DeepEqual(job1, out1) {
		t.Fatalf("bad: \n%#v\n%#v", out1, job1)
	}
	if !reflect.DeepEqual(job2, out2) {
		t.Fatalf("bad: \n%#v\n%#v", out2, job2)
	}
}

func TestFSM_SnapshotRestore_Indexes(t *testing.T) {
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	node1 := mockNode()
	state.RegisterNode(1000, node1)

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()

	index, err := state2.GetIndex("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
}
