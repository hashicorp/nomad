package nomad

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestClientEndpoint_Register(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	req := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.NodeByID(node.ID)
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
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.NodeDeregisterRequest{
		NodeID:       node.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Deregister", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected node")
	}
}

func TestClientEndpoint_UpdateStatus(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.NodeUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check for heartbeat interval
	ttl := resp.HeartbeatTTL
	if ttl < s1.config.MinHeartbeatTTL || ttl > 2*s1.config.MinHeartbeatTTL {
		t.Fatalf("bad: %#v", ttl)
	}

	// Update the status
	dereg := &structs.NodeUpdateStatusRequest{
		NodeID:       node.ID,
		Status:       structs.NodeStatusInit,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.NodeUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateStatus", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for heartbeat interval
	ttl = resp2.HeartbeatTTL
	if ttl < s1.config.MinHeartbeatTTL || ttl > 2*s1.config.MinHeartbeatTTL {
		t.Fatalf("bad: %#v", ttl)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected node")
	}
	if out.ModifyIndex != resp2.Index {
		t.Fatalf("index mis-match")
	}
}

func TestClientEndpoint_UpdateStatus_HeartbeatOnly(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.NodeUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check for heartbeat interval
	ttl := resp.HeartbeatTTL
	if ttl < s1.config.MinHeartbeatTTL || ttl > 2*s1.config.MinHeartbeatTTL {
		t.Fatalf("bad: %#v", ttl)
	}

	// Update the status, static state
	dereg := &structs.NodeUpdateStatusRequest{
		NodeID:       node.ID,
		Status:       node.Status,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.NodeUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateStatus", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for heartbeat interval
	ttl = resp2.HeartbeatTTL
	if ttl < s1.config.MinHeartbeatTTL || ttl > 2*s1.config.MinHeartbeatTTL {
		t.Fatalf("bad: %#v", ttl)
	}
}

func TestClientEndpoint_UpdateDrain(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.NodeUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the status
	dereg := &structs.NodeUpdateDrainRequest{
		NodeID:       node.ID,
		Drain:        true,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.NodeDrainUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.NodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.Drain {
		t.Fatalf("bad: %#v", out)
	}
}

func TestClientEndpoint_GetNode(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	node.CreateIndex = resp.Index
	node.ModifyIndex = resp.Index

	// Lookup the node
	get := &structs.NodeSpecificRequest{
		NodeID:       node.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.SingleNodeResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.Index {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}

	if !reflect.DeepEqual(node, resp2.Node) {
		t.Fatalf("bad: %#v %#v", node, resp2.Node)
	}

	// Lookup non-existing node
	get.NodeID = "foobarbaz"
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.Index {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}
	if resp2.Node != nil {
		t.Fatalf("unexpected node")
	}
}

func TestClientEndpoint_GetAllocs(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	node.CreateIndex = resp.Index
	node.ModifyIndex = resp.Index

	// Inject fake evaluations
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	state := s1.fsm.State()
	err := state.UpsertAllocs(100, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the allocs
	get := &structs.NodeSpecificRequest{
		NodeID:       node.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.NodeAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 100 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 100)
	}

	if len(resp2.Allocs) != 1 || resp2.Allocs[0].ID != alloc.ID {
		t.Fatalf("bad: %#v", resp2.Allocs)
	}

	// Lookup non-existing node
	get.NodeID = "foobarbaz"
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 100 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 100)
	}
	if len(resp2.Allocs) != 0 {
		t.Fatalf("unexpected node")
	}
}

func TestClientEndpoint_GetAllocs_Blocking(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	node.CreateIndex = resp.Index
	node.ModifyIndex = resp.Index

	// Inject fake evaluations async
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	state := s1.fsm.State()
	start := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := state.UpsertAllocs(100, []*structs.Allocation{alloc})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	// Lookup the allocs in a blocking query
	get := &structs.NodeSpecificRequest{
		NodeID: node.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 50,
			MaxQueryTime:  time.Second,
		},
	}
	var resp2 structs.NodeAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block at least 100ms
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	if resp2.Index != 100 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 100)
	}

	if len(resp2.Allocs) != 1 || resp2.Allocs[0].ID != alloc.ID {
		t.Fatalf("bad: %#v", resp2.Allocs)
	}
}

func TestClientEndpoint_UpdateAlloc(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Inject fake evaluations
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	state := s1.fsm.State()
	err := state.UpsertAllocs(100, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Attempt update
	clientAlloc := new(structs.Allocation)
	*clientAlloc = *alloc
	clientAlloc.ClientStatus = structs.AllocClientStatusFailed

	// Update the alloc
	update := &structs.AllocUpdateRequest{
		Alloc:        []*structs.Allocation{clientAlloc},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.NodeAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateAlloc", update, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("Bad index: %d", resp2.Index)
	}

	// Lookup the alloc
	out, err := state.AllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.ClientStatus != structs.AllocClientStatusFailed {
		t.Fatalf("Bad: %#v", out)
	}
}

func TestClientEndpoint_CreateNodeEvals(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Inject fake evaluations
	alloc := mock.Alloc()
	state := s1.fsm.State()
	err := state.UpsertAllocs(1, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some evaluations
	ids, index, err := s1.endpoints.Node.createNodeEvals(alloc.NodeID, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index == 0 {
		t.Fatalf("bad: %d", index)
	}
	if len(ids) != 1 {
		t.Fatalf("bad: %s", ids)
	}

	// Lookup the evaluation
	eval, err := state.EvalByID(ids[0])
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != index {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != alloc.Job.Priority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != alloc.Job.Type {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerNodeUpdate {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != alloc.JobID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.NodeID != alloc.NodeID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.NodeModifyIndex != 1 {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
}

func TestClientEndpoint_Evaluate(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Inject fake evaluations
	alloc := mock.Alloc()
	node := mock.Node()
	node.ID = alloc.NodeID
	state := s1.fsm.State()
	err := state.UpsertNode(1, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = state.UpsertAllocs(2, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Re-evaluate
	req := &structs.NodeEvaluateRequest{
		NodeID:       alloc.NodeID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.NodeUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Evaluate", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Create some evaluations
	ids := resp.EvalIDs
	if len(ids) != 1 {
		t.Fatalf("bad: %s", ids)
	}

	// Lookup the evaluation
	eval, err := state.EvalByID(ids[0])
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != resp.Index {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != alloc.Job.Priority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != alloc.Job.Type {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerNodeUpdate {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != alloc.JobID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.NodeID != alloc.NodeID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.NodeModifyIndex != 1 {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
}

func TestClientEndpoint_ListNodes(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	node := mock.Node()
	reg := &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	node.CreateIndex = resp.Index
	node.ModifyIndex = resp.Index

	// Lookup the node
	get := &structs.NodeListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.NodeListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.List", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.Index {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}

	if len(resp2.Nodes) != 1 {
		t.Fatalf("bad: %#v", resp2.Nodes)
	}
	if resp2.Nodes[0].ID != node.ID {
		t.Fatalf("bad: %#v", resp2.Nodes[0])
	}
}
