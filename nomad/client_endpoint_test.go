package nomad

import (
	"testing"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestClientEndpoint_Register(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mockNode()
	req := &structs.RegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Client.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected node")
	}
	if out.CreateIndex != resp.Index {
		t.Fatalf("index mis-match")
	}
}

func TestClientEndpoint_Deregister(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mockNode()
	reg := &structs.RegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Client.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.DeregisterRequest{
		NodeID:       node.ID,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}
	var resp2 structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Client.Deregister", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected node")
	}
}
