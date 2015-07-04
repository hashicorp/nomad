package nomad

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

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

	req := structs.RegisterRequest{
		Node: mockNode(),
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
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
	req := structs.RegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.DeregisterRequest{
		NodeID: node.ID,
	}
	buf, err = structs.Encode(structs.DeregisterRequestType, req2)
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
	req := structs.RegisterRequest{
		Node: node,
	}
	buf, err := structs.Encode(structs.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	req2 := structs.UpdateStatusRequest{
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
