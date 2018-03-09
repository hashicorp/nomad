package nomad

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	vapi "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientEndpoint_Register(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Check that we have no client connections
	require.Empty(s1.connectedNodes())

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

	// Check that we have the client connections
	nodes := s1.connectedNodes()
	require.Len(nodes, 1)
	require.Contains(nodes, node.ID)

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected node")
	}
	if out.CreateIndex != resp.Index {
		t.Fatalf("index mis-match")
	}
	if out.ComputedClass == "" {
		t.Fatal("ComputedClass not set")
	}

	// Close the connection and check that we remove the client connections
	require.Nil(codec.Close())
	testutil.WaitForResult(func() (bool, error) {
		nodes := s1.connectedNodes()
		return len(nodes) == 0, nil
	}, func(err error) {
		t.Fatalf("should have no clients")
	})
}

func TestClientEndpoint_EmitEvent(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1 := TestServer(t, nil)
	state := s1.fsm.State()
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// create a node that we can register our event to
	node := mock.Node()
	err := state.UpsertNode(2, node)
	require.Nil(err)

	nodeEvent := &structs.NodeEvent{
		Message:   "Registration failed",
		Subsystem: "Server",
		Timestamp: time.Now().Unix(),
	}

	nodeEvents := map[string][]*structs.NodeEvent{node.ID: {nodeEvent}}
	req := structs.EmitNodeEventRequest{
		NodeEvents:   nodeEvents,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	var resp structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "Node.EmitEvent", &req, &resp)
	require.Nil(err)
	require.NotEqual(0, resp.Index)

	// Check for the node in the FSM
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)
	require.False(len(out.NodeEvents) < 2)
}

func TestClientEndpoint_Register_SecretMismatch(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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

	// Update the nodes SecretID
	node.SecretID = uuid.Generate()
	err := msgpackrpc.CallWithCodec(codec, "Node.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), "Not registering") {
		t.Fatalf("Expecting error regarding mismatching secret id: %v", err)
	}
}

func TestClientEndpoint_Deregister(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected node")
	}
}

func TestClientEndpoint_Deregister_ACL(t *testing.T) {
	t.Parallel()
	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the node
	node := mock.Node()
	node1 := mock.Node()
	state := s1.fsm.State()
	if err := state.UpsertNode(1, node); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertNode(2, node1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid", mock.NodePolicy(acl.PolicyWrite))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid", mock.NodePolicy(acl.PolicyRead))

	// Deregister without any token and expect it to fail
	dereg := &structs.NodeDeregisterRequest{
		NodeID:       node.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.Deregister", dereg, &resp); err == nil {
		t.Fatalf("node de-register succeeded")
	}

	// Deregister with a valid token
	dereg.AuthToken = validToken.SecretID
	if err := msgpackrpc.CallWithCodec(codec, "Node.Deregister", dereg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check for the node in the FSM
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected node")
	}

	// Deregister with an invalid token.
	dereg1 := &structs.NodeDeregisterRequest{
		NodeID:       node1.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	dereg1.AuthToken = invalidToken.SecretID
	if err := msgpackrpc.CallWithCodec(codec, "Node.Deregister", dereg1, &resp); err == nil {
		t.Fatalf("rpc should not have succeeded")
	}

	// Try with a root token
	dereg1.AuthToken = root.SecretID
	if err := msgpackrpc.CallWithCodec(codec, "Node.Deregister", dereg1, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestClientEndpoint_Deregister_Vault(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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

	// Swap the servers Vault Client
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Put some Vault accessors in the state store for that node
	state := s1.fsm.State()
	va1 := mock.VaultAccessor()
	va1.NodeID = node.ID
	va2 := mock.VaultAccessor()
	va2.NodeID = node.ID
	state.UpsertVaultAccessor(100, []*structs.VaultAccessor{va1, va2})

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
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected node")
	}

	// Check that the endpoint revoked the tokens
	if l := len(tvc.RevokedTokens); l != 2 {
		t.Fatalf("Deregister revoked %d tokens; want 2", l)
	}
}

func TestClientEndpoint_UpdateStatus(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Check that we have no client connections
	require.Empty(s1.connectedNodes())

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

	// Check that we have the client connections
	nodes := s1.connectedNodes()
	require.Len(nodes, 1)
	require.Contains(nodes, node.ID)

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected node")
	}
	if out.ModifyIndex != resp2.Index {
		t.Fatalf("index mis-match")
	}

	// Close the connection and check that we remove the client connections
	require.Nil(codec.Close())
	testutil.WaitForResult(func() (bool, error) {
		nodes := s1.connectedNodes()
		return len(nodes) == 0, nil
	}, func(err error) {
		t.Fatalf("should have no clients")
	})
}

func TestClientEndpoint_UpdateStatus_Vault(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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

	// Swap the servers Vault Client
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Put some Vault accessors in the state store for that node
	state := s1.fsm.State()
	va1 := mock.VaultAccessor()
	va1.NodeID = node.ID
	va2 := mock.VaultAccessor()
	va2.NodeID = node.ID
	state.UpsertVaultAccessor(100, []*structs.VaultAccessor{va1, va2})

	// Update the status to be down
	dereg := &structs.NodeUpdateStatusRequest{
		NodeID:       node.ID,
		Status:       structs.NodeStatusDown,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.NodeUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateStatus", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check that the endpoint revoked the tokens
	if l := len(tvc.RevokedTokens); l != 2 {
		t.Fatalf("Deregister revoked %d tokens; want 2", l)
	}
}

func TestClientEndpoint_Register_GetEvals(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Register a system job.
	job := mock.SystemJob()
	state := s1.fsm.State()
	if err := state.UpsertJob(1, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the register request going directly to ready
	node := mock.Node()
	node.Status = structs.NodeStatusReady
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

	// Check for an eval caused by the system job.
	if len(resp.EvalIDs) != 1 {
		t.Fatalf("expected one eval; got %#v", resp.EvalIDs)
	}

	evalID := resp.EvalIDs[0]
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, evalID)
	if err != nil {
		t.Fatalf("could not get eval %v", evalID)
	}

	if eval.Type != "system" {
		t.Fatalf("unexpected eval type; got %v; want %q", eval.Type, "system")
	}

	// Check for the node in the FSM
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected node")
	}
	if out.ModifyIndex != resp.Index {
		t.Fatalf("index mis-match")
	}

	// Transition it to down and then ready
	node.Status = structs.NodeStatusDown
	reg = &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(resp.EvalIDs) != 1 {
		t.Fatalf("expected one eval; got %#v", resp.EvalIDs)
	}

	node.Status = structs.NodeStatusReady
	reg = &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(resp.EvalIDs) != 1 {
		t.Fatalf("expected one eval; got %#v", resp.EvalIDs)
	}
}

func TestClientEndpoint_UpdateStatus_GetEvals(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Register a system job.
	job := mock.SystemJob()
	state := s1.fsm.State()
	if err := state.UpsertJob(1, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the register request
	node := mock.Node()
	node.Status = structs.NodeStatusInit
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
	update := &structs.NodeUpdateStatusRequest{
		NodeID:       node.ID,
		Status:       structs.NodeStatusReady,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.NodeUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateStatus", update, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for an eval caused by the system job.
	if len(resp2.EvalIDs) != 1 {
		t.Fatalf("expected one eval; got %#v", resp2.EvalIDs)
	}

	evalID := resp2.EvalIDs[0]
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, evalID)
	if err != nil {
		t.Fatalf("could not get eval %v", evalID)
	}

	if eval.Type != "system" {
		t.Fatalf("unexpected eval type; got %v; want %q", eval.Type, "system")
	}

	// Check for heartbeat interval
	ttl = resp2.HeartbeatTTL
	if ttl < s1.config.MinHeartbeatTTL || ttl > 2*s1.config.MinHeartbeatTTL {
		t.Fatalf("bad: %#v", ttl)
	}

	// Check for the node in the FSM
	out, err := state.NodeByID(ws, node.ID)
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
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()

	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()

	s3 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s3.Shutdown()
	servers := []*Server{s1, s2, s3}
	TestJoin(t, s1, s2, s3)

	for _, s := range servers {
		testutil.WaitForResult(func() (bool, error) {
			peers, _ := s.numPeers()
			return peers == 3, nil
		}, func(err error) {
			t.Fatalf("should have 3 peers")
		})
	}

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

	// Check for heartbeat servers
	serverAddrs := resp.Servers
	if len(serverAddrs) == 0 {
		t.Fatalf("bad: %#v", serverAddrs)
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
	t.Parallel()
	s1 := TestServer(t, nil)
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
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.Drain {
		t.Fatalf("bad: %#v", out)
	}
}

func TestClientEndpoint_UpdateDrain_ACL(t *testing.T) {
	t.Parallel()
	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the node
	node := mock.Node()
	state := s1.fsm.State()

	assert.Nil(state.UpsertNode(1, node), "UpsertNode")

	// Create the policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid", mock.NodePolicy(acl.PolicyWrite))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid", mock.NodePolicy(acl.PolicyRead))

	// Update the status without a token and expect failure
	dereg := &structs.NodeUpdateDrainRequest{
		NodeID:       node.ID,
		Drain:        true,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	{
		var resp structs.NodeDrainUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", dereg, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	dereg.AuthToken = validToken.SecretID
	{
		var resp structs.NodeDrainUpdateResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", dereg, &resp), "RPC")
	}

	// Try with a invalid token
	dereg.AuthToken = invalidToken.SecretID
	{
		var resp structs.NodeDrainUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", dereg, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a root token
	dereg.AuthToken = root.SecretID
	{
		var resp structs.NodeDrainUpdateResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", dereg, &resp), "RPC")
	}
}

// This test ensures that Nomad marks client state of allocations which are in
// pending/running state to lost when a node is marked as down.
func TestClientEndpoint_Drain_Down(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Register a node
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

	// Register a service job
	var jobResp structs.JobRegisterResponse
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	jobReq := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", jobReq, &jobResp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a system job
	var jobResp1 structs.JobRegisterResponse
	job1 := mock.Job()
	job1.TaskGroups[0].Count = 1
	job1.Type = structs.JobTypeSystem
	jobReq1 := &structs.JobRegisterRequest{
		Job: job1,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job1.Namespace,
		},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", jobReq1, &jobResp1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the scheduler to create an allocation
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		allocs, err := s1.fsm.state.AllocsByJob(ws, job.Namespace, job.ID, true)
		if err != nil {
			return false, err
		}
		allocs1, err := s1.fsm.state.AllocsByJob(ws, job1.Namespace, job1.ID, true)
		if err != nil {
			return false, err
		}
		return len(allocs) > 0 && len(allocs1) > 0, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Drain the node
	dereg := &structs.NodeUpdateDrainRequest{
		NodeID:       node.ID,
		Drain:        true,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.NodeDrainUpdateResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Mark the node as down
	node.Status = structs.NodeStatusDown
	reg = &structs.NodeRegisterRequest{
		Node:         node,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that the allocation has transitioned to lost
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		summary, err := s1.fsm.state.JobSummaryByID(ws, job.Namespace, job.ID)
		if err != nil {
			return false, err
		}
		expectedSummary := &structs.JobSummary{
			JobID:     job.ID,
			Namespace: job.Namespace,
			Summary: map[string]structs.TaskGroupSummary{
				"web": {
					Queued: 1,
					Lost:   1,
				},
			},
			Children:    new(structs.JobChildrenSummary),
			CreateIndex: jobResp.JobModifyIndex,
			ModifyIndex: summary.ModifyIndex,
		}
		if !reflect.DeepEqual(summary, expectedSummary) {
			return false, fmt.Errorf("expected: %#v, actual: %#v", expectedSummary, summary)
		}

		summary1, err := s1.fsm.state.JobSummaryByID(ws, job1.Namespace, job1.ID)
		if err != nil {
			return false, err
		}
		expectedSummary1 := &structs.JobSummary{
			JobID:     job1.ID,
			Namespace: job1.Namespace,
			Summary: map[string]structs.TaskGroupSummary{
				"web": {
					Lost: 1,
				},
			},
			Children:    new(structs.JobChildrenSummary),
			CreateIndex: jobResp1.JobModifyIndex,
			ModifyIndex: summary1.ModifyIndex,
		}
		if !reflect.DeepEqual(summary1, expectedSummary1) {
			return false, fmt.Errorf("expected: %#v, actual: %#v", expectedSummary1, summary1)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestClientEndpoint_GetNode(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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

	if resp2.Node.ComputedClass == "" {
		t.Fatalf("bad ComputedClass: %#v", resp2.Node)
	}

	// Update the status updated at value
	node.StatusUpdatedAt = resp2.Node.StatusUpdatedAt
	node.SecretID = ""
	node.NodeEvents = resp2.Node.NodeEvents
	if !reflect.DeepEqual(node, resp2.Node) {
		t.Fatalf("bad: %#v \n %#v", node, resp2.Node)
	}

	// assert that the node register event was set correctly
	if len(resp2.Node.NodeEvents) != 1 {
		t.Fatalf("Did not set node events: %#v", resp2.Node)
	}
	if resp2.Node.NodeEvents[0].Message != "Node Registered" {
		t.Fatalf("Did not set node register event correctly: %#v", resp2.Node)
	}

	// Lookup non-existing node
	get.NodeID = "12345678-abcd-efab-cdef-123456789abc"
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

func TestClientEndpoint_GetNode_ACL(t *testing.T) {
	t.Parallel()
	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the node
	node := mock.Node()
	state := s1.fsm.State()
	assert.Nil(state.UpsertNode(1, node), "UpsertNode")

	// Create the policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid", mock.NodePolicy(acl.PolicyRead))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid", mock.NodePolicy(acl.PolicyDeny))

	// Lookup the node without a token and expect failure
	req := &structs.NodeSpecificRequest{
		NodeID:       node.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	{
		var resp structs.SingleNodeResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	req.AuthToken = validToken.SecretID
	{
		var resp structs.SingleNodeResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp), "RPC")
		assert.Equal(node.ID, resp.Node.ID)
	}

	// Try with a Node.SecretID
	req.AuthToken = node.SecretID
	{
		var resp structs.SingleNodeResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp), "RPC")
		assert.Equal(node.ID, resp.Node.ID)
	}

	// Try with a invalid token
	req.AuthToken = invalidToken.SecretID
	{
		var resp structs.SingleNodeResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a root token
	req.AuthToken = root.SecretID
	{
		var resp structs.SingleNodeResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp), "RPC")
		assert.Equal(node.ID, resp.Node.ID)
	}
}

func TestClientEndpoint_GetNode_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the node
	node1 := mock.Node()
	node2 := mock.Node()

	// First create an unrelated node.
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertNode(100, node1); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the node we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		if err := state.UpsertNode(200, node2); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the node
	req := &structs.NodeSpecificRequest{
		NodeID: node2.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleNodeResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Node == nil || resp.Node.ID != node2.ID {
		t.Fatalf("bad: %#v", resp.Node)
	}

	// Node update triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		nodeUpdate := mock.Node()
		nodeUpdate.ID = node2.ID
		nodeUpdate.Status = structs.NodeStatusDown
		if err := state.UpsertNode(300, nodeUpdate); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleNodeResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Node == nil || resp2.Node.Status != structs.NodeStatusDown {
		t.Fatalf("bad: %#v", resp2.Node)
	}

	// Node delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteNode(400, node2.ID); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 350
	var resp3 structs.SingleNodeResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp3.Index != 400 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 400)
	}
	if resp3.Node != nil {
		t.Fatalf("bad: %#v", resp3.Node)
	}
}

func TestClientEndpoint_GetAllocs(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
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

func TestClientEndpoint_GetAllocs_ACL_Basic(t *testing.T) {
	t.Parallel()
	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the node
	allocDefaultNS := mock.Alloc()
	node := mock.Node()
	allocDefaultNS.NodeID = node.ID
	state := s1.fsm.State()
	assert.Nil(state.UpsertNode(1, node), "UpsertNode")
	assert.Nil(state.UpsertJobSummary(2, mock.JobSummary(allocDefaultNS.JobID)), "UpsertJobSummary")
	allocs := []*structs.Allocation{allocDefaultNS}
	assert.Nil(state.UpsertAllocs(5, allocs), "UpsertAllocs")

	// Create the namespace policy and tokens
	validDefaultToken := mock.CreatePolicyAndToken(t, state, 1001, "test-default-valid", mock.NodePolicy(acl.PolicyRead)+
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1004, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	req := &structs.NodeSpecificRequest{
		NodeID: node.ID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	// Lookup the node without a token and expect failure
	{
		var resp structs.NodeAllocsResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token for the default namespace
	req.AuthToken = validDefaultToken.SecretID
	{
		var resp structs.NodeAllocsResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp), "RPC")
		assert.Len(resp.Allocs, 1)
		assert.Equal(allocDefaultNS.ID, resp.Allocs[0].ID)
	}

	// Try with a invalid token
	req.AuthToken = invalidToken.SecretID
	{
		var resp structs.NodeAllocsResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a root token
	req.AuthToken = root.SecretID
	{
		var resp structs.NodeAllocsResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp), "RPC")
		assert.Len(resp.Allocs, 1)
		for _, alloc := range resp.Allocs {
			switch alloc.ID {
			case allocDefaultNS.ID:
				// expected
			default:
				t.Errorf("unexpected alloc %q for namespace %q", alloc.ID, alloc.Namespace)
			}
		}
	}
}

func TestClientEndpoint_GetClientAllocs(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Check that we have no client connections
	require.Empty(s1.connectedNodes())

	// Create the register request
	node := mock.Node()
	state := s1.fsm.State()
	require.Nil(state.UpsertNode(98, node))

	// Inject fake evaluations
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	err := state.UpsertAllocs(100, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the allocs
	get := &structs.NodeSpecificRequest{
		NodeID:       node.ID,
		SecretID:     node.SecretID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.NodeClientAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetClientAllocs", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 100 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 100)
	}

	if len(resp2.Allocs) != 1 || resp2.Allocs[alloc.ID] != 100 {
		t.Fatalf("bad: %#v", resp2.Allocs)
	}

	// Check that we have the client connections
	nodes := s1.connectedNodes()
	require.Len(nodes, 1)
	require.Contains(nodes, node.ID)

	// Lookup node with bad SecretID
	get.SecretID = "foobarbaz"
	var resp3 structs.NodeClientAllocsResponse
	err = msgpackrpc.CallWithCodec(codec, "Node.GetClientAllocs", get, &resp3)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("err: %v", err)
	}

	// Lookup non-existing node
	get.NodeID = uuid.Generate()
	var resp4 structs.NodeClientAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetClientAllocs", get, &resp4); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp4.Index != 100 {
		t.Fatalf("Bad index: %d %d", resp3.Index, 100)
	}
	if len(resp4.Allocs) != 0 {
		t.Fatalf("unexpected node %#v", resp3.Allocs)
	}

	// Close the connection and check that we remove the client connections
	require.Nil(codec.Close())
	testutil.WaitForResult(func() (bool, error) {
		nodes := s1.connectedNodes()
		return len(nodes) == 0, nil
	}, func(err error) {
		t.Fatalf("should have no clients")
	})
}

func TestClientEndpoint_GetClientAllocs_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
	now := time.Now().UTC().UnixNano()
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.ModifyTime = now
	state := s1.fsm.State()
	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	start := time.Now()
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertAllocs(100, []*structs.Allocation{alloc})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the allocs in a blocking query
	req := &structs.NodeSpecificRequest{
		NodeID:   node.ID,
		SecretID: node.SecretID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 50,
			MaxQueryTime:  time.Second,
		},
	}
	var resp2 structs.NodeClientAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetClientAllocs", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block at least 100ms
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	if resp2.Index != 100 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 100)
	}

	if len(resp2.Allocs) != 1 || resp2.Allocs[alloc.ID] != 100 {
		t.Fatalf("bad: %#v", resp2.Allocs)
	}

	iter, err := state.AllocsByIDPrefix(nil, structs.DefaultNamespace, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	getAllocs := func(iter memdb.ResultIterator) []*structs.Allocation {
		var allocs []*structs.Allocation
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			allocs = append(allocs, raw.(*structs.Allocation))
		}
		return allocs
	}
	out := getAllocs(iter)

	if len(out) != 1 {
		t.Fatalf("Expected to get one allocation but got:%v", out)
	}

	if out[0].ModifyTime != now {
		t.Fatalf("Invalid modify time %v", out[0].ModifyTime)
	}

	// Alloc updates fire watches
	time.AfterFunc(100*time.Millisecond, func() {
		allocUpdate := mock.Alloc()
		allocUpdate.NodeID = alloc.NodeID
		allocUpdate.ID = alloc.ID
		allocUpdate.ClientStatus = structs.AllocClientStatusRunning
		state.UpsertJobSummary(199, mock.JobSummary(allocUpdate.JobID))
		err := state.UpsertAllocs(200, []*structs.Allocation{allocUpdate})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 150
	var resp3 structs.NodeClientAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetClientAllocs", req, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}

	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}
	if resp3.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp3.Index, 200)
	}
	if len(resp3.Allocs) != 1 || resp3.Allocs[alloc.ID] != 200 {
		t.Fatalf("bad: %#v", resp3.Allocs)
	}
}

func TestClientEndpoint_GetClientAllocs_Blocking_GC(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	s1 := TestServer(t, nil)
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
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", reg, &resp))
	node.CreateIndex = resp.Index
	node.ModifyIndex = resp.Index

	// Inject fake allocations async
	alloc1 := mock.Alloc()
	alloc1.NodeID = node.ID
	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	state := s1.fsm.State()
	state.UpsertJobSummary(99, mock.JobSummary(alloc1.JobID))
	start := time.Now()
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertAllocs(100, []*structs.Allocation{alloc1, alloc2}))
	})

	// Lookup the allocs in a blocking query
	req := &structs.NodeSpecificRequest{
		NodeID:   node.ID,
		SecretID: node.SecretID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 50,
			MaxQueryTime:  time.Second,
		},
	}
	var resp2 structs.NodeClientAllocsResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetClientAllocs", req, &resp2))

	// Should block at least 100ms
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	assert.EqualValues(100, resp2.Index)
	if assert.Len(resp2.Allocs, 2) {
		assert.EqualValues(100, resp2.Allocs[alloc1.ID])
	}

	// Delete an allocation
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.DeleteEval(200, nil, []string{alloc2.ID}))
	})

	req.QueryOptions.MinQueryIndex = 150
	var resp3 structs.NodeClientAllocsResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetClientAllocs", req, &resp3))

	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}
	assert.EqualValues(200, resp3.Index)
	if assert.Len(resp3.Allocs, 1) {
		assert.EqualValues(100, resp3.Allocs[alloc1.ID])
	}
}

// A MigrateToken should not be created if an allocation shares the same node
// with its previous allocation
func TestClientEndpoint_GetClientAllocs_WithoutMigrateTokens(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	s1 := TestServer(t, nil)
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
	prevAlloc := mock.Alloc()
	prevAlloc.NodeID = node.ID
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.PreviousAllocation = prevAlloc.ID
	alloc.DesiredStatus = structs.AllocClientStatusComplete
	state := s1.fsm.State()
	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	err := state.UpsertAllocs(100, []*structs.Allocation{prevAlloc, alloc})
	assert.Nil(err)

	// Lookup the allocs
	get := &structs.NodeSpecificRequest{
		NodeID:       node.ID,
		SecretID:     node.SecretID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.NodeClientAllocsResponse

	err = msgpackrpc.CallWithCodec(codec, "Node.GetClientAllocs", get, &resp2)
	assert.Nil(err)

	assert.Equal(uint64(100), resp2.Index)
	assert.Equal(2, len(resp2.Allocs))
	assert.Equal(uint64(100), resp2.Allocs[alloc.ID])
	assert.Equal(0, len(resp2.MigrateTokens))
}

func TestClientEndpoint_GetAllocs_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	start := time.Now()
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertAllocs(100, []*structs.Allocation{alloc})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the allocs in a blocking query
	req := &structs.NodeSpecificRequest{
		NodeID: node.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 50,
			MaxQueryTime:  time.Second,
		},
	}
	var resp2 structs.NodeAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp2); err != nil {
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

	// Alloc updates fire watches
	time.AfterFunc(100*time.Millisecond, func() {
		allocUpdate := mock.Alloc()
		allocUpdate.NodeID = alloc.NodeID
		allocUpdate.ID = alloc.ID
		allocUpdate.ClientStatus = structs.AllocClientStatusRunning
		state.UpsertJobSummary(199, mock.JobSummary(allocUpdate.JobID))
		err := state.UpdateAllocsFromClient(200, []*structs.Allocation{allocUpdate})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 150
	var resp3 structs.NodeAllocsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}

	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}
	if resp3.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp3.Index, 200)
	}
	if len(resp3.Allocs) != 1 || resp3.Allocs[0].ClientStatus != structs.AllocClientStatusRunning {
		t.Fatalf("bad: %#v", resp3.Allocs[0])
	}
}

func TestClientEndpoint_UpdateAlloc(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	require := require.New(t)

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

	state := s1.fsm.State()
	// Inject mock job
	job := mock.Job()
	err := state.UpsertJob(101, job)
	require.Nil(err)

	// Inject fake allocations
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	err = state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	require.Nil(err)
	alloc.TaskGroup = job.TaskGroups[0].Name
	err = state.UpsertAllocs(100, []*structs.Allocation{alloc})
	require.Nil(err)

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
	start := time.Now()
	err = msgpackrpc.CallWithCodec(codec, "Node.UpdateAlloc", update, &resp2)
	require.Nil(err)
	require.NotEqual(0, resp2.Index)

	if diff := time.Since(start); diff < batchUpdateInterval {
		t.Fatalf("too fast: %v", diff)
	}

	// Lookup the alloc
	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	require.Nil(err)
	require.Equal(structs.AllocClientStatusFailed, out.ClientStatus)
	require.True(out.ModifyTime > 0)

	// Assert that one eval with TriggeredBy EvalTriggerRetryFailedAlloc exists
	evaluations, err := state.EvalsByJob(ws, job.Namespace, job.ID)
	require.Nil(err)
	require.True(len(evaluations) != 0)
	found := false
	for _, resultEval := range evaluations {
		if resultEval.TriggeredBy == structs.EvalTriggerRetryFailedAlloc {
			found = true
		}
	}
	require.True(found, "Should create an eval for failed alloc")

}

func TestClientEndpoint_BatchUpdate(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	err := state.UpsertAllocs(100, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Attempt update
	clientAlloc := new(structs.Allocation)
	*clientAlloc = *alloc
	clientAlloc.ClientStatus = structs.AllocClientStatusFailed

	// Call to do the batch update
	bf := NewBatchFuture()
	endpoint := s1.staticEndpoints.Node
	endpoint.batchUpdate(bf, []*structs.Allocation{clientAlloc}, nil)
	if err := bf.Wait(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if bf.Index() == 0 {
		t.Fatalf("Bad index: %d", bf.Index())
	}

	// Lookup the alloc
	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.ClientStatus != structs.AllocClientStatusFailed {
		t.Fatalf("Bad: %#v", out)
	}
}

func TestClientEndpoint_UpdateAlloc_Vault(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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

	// Swap the servers Vault Client
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Inject fake allocation and vault accessor
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	state := s1.fsm.State()
	state.UpsertJobSummary(99, mock.JobSummary(alloc.JobID))
	if err := state.UpsertAllocs(100, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	va := mock.VaultAccessor()
	va.NodeID = node.ID
	va.AllocID = alloc.ID
	if err := state.UpsertVaultAccessor(101, []*structs.VaultAccessor{va}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Inject mock job
	job := mock.Job()
	job.ID = alloc.JobID
	err := state.UpsertJob(101, job)
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
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateAlloc", update, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("Bad index: %d", resp2.Index)
	}
	if diff := time.Since(start); diff < batchUpdateInterval {
		t.Fatalf("too fast: %v", diff)
	}

	// Lookup the alloc
	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.ClientStatus != structs.AllocClientStatusFailed {
		t.Fatalf("Bad: %#v", out)
	}

	if l := len(tvc.RevokedTokens); l != 1 {
		t.Fatalf("Deregister revoked %d tokens; want 1", l)
	}
}

func TestClientEndpoint_CreateNodeEvals(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Inject fake evaluations
	alloc := mock.Alloc()
	state := s1.fsm.State()
	state.UpsertJobSummary(1, mock.JobSummary(alloc.JobID))
	if err := state.UpsertAllocs(2, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Inject a fake system job.
	job := mock.SystemJob()
	if err := state.UpsertJob(3, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create some evaluations
	ids, index, err := s1.staticEndpoints.Node.createNodeEvals(alloc.NodeID, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index == 0 {
		t.Fatalf("bad: %d", index)
	}
	if len(ids) != 2 {
		t.Fatalf("bad: %s", ids)
	}

	// Lookup the evaluations
	ws := memdb.NewWatchSet()
	evalByType := make(map[string]*structs.Evaluation, 2)
	for _, id := range ids {
		eval, err := state.EvalByID(ws, id)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if eval == nil {
			t.Fatalf("expected eval")
		}

		if old, ok := evalByType[eval.Type]; ok {
			t.Fatalf("multiple evals of the same type: %v and %v", old, eval)
		}

		evalByType[eval.Type] = eval
	}

	if len(evalByType) != 2 {
		t.Fatalf("Expected a service and system job; got %#v", evalByType)
	}

	// Ensure the evals are correct.
	for schedType, eval := range evalByType {
		expPriority := alloc.Job.Priority
		expJobID := alloc.JobID
		if schedType == "system" {
			expPriority = job.Priority
			expJobID = job.ID
		}

		if eval.CreateIndex != index {
			t.Fatalf("CreateIndex mis-match on type %v: %#v", schedType, eval)
		}
		if eval.TriggeredBy != structs.EvalTriggerNodeUpdate {
			t.Fatalf("TriggeredBy incorrect on type %v: %#v", schedType, eval)
		}
		if eval.NodeID != alloc.NodeID {
			t.Fatalf("NodeID incorrect on type %v: %#v", schedType, eval)
		}
		if eval.NodeModifyIndex != 1 {
			t.Fatalf("NodeModifyIndex incorrect on type %v: %#v", schedType, eval)
		}
		if eval.Status != structs.EvalStatusPending {
			t.Fatalf("Status incorrect on type %v: %#v", schedType, eval)
		}
		if eval.Priority != expPriority {
			t.Fatalf("Priority incorrect on type %v: %#v", schedType, eval)
		}
		if eval.JobID != expJobID {
			t.Fatalf("JobID incorrect on type %v: %#v", schedType, eval)
		}
	}
}

func TestClientEndpoint_Evaluate(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
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
	state.UpsertJobSummary(2, mock.JobSummary(alloc.JobID))
	err = state.UpsertAllocs(3, []*structs.Allocation{alloc})
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
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, ids[0])
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

func TestClientEndpoint_Evaluate_ACL(t *testing.T) {
	t.Parallel()
	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the node with an alloc
	alloc := mock.Alloc()
	node := mock.Node()
	node.ID = alloc.NodeID
	state := s1.fsm.State()

	assert.Nil(state.UpsertNode(1, node), "UpsertNode")
	assert.Nil(state.UpsertJobSummary(2, mock.JobSummary(alloc.JobID)), "UpsertJobSummary")
	assert.Nil(state.UpsertAllocs(3, []*structs.Allocation{alloc}), "UpsertAllocs")

	// Create the policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid", mock.NodePolicy(acl.PolicyWrite))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid", mock.NodePolicy(acl.PolicyRead))

	// Re-evaluate without a token and expect failure
	req := &structs.NodeEvaluateRequest{
		NodeID:       alloc.NodeID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	{
		var resp structs.NodeUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.Evaluate", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	req.AuthToken = validToken.SecretID
	{
		var resp structs.NodeUpdateResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.Evaluate", req, &resp), "RPC")
	}

	// Try with a invalid token
	req.AuthToken = invalidToken.SecretID
	{
		var resp structs.NodeUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.Evaluate", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a root token
	req.AuthToken = root.SecretID
	{
		var resp structs.NodeUpdateResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.Evaluate", req, &resp), "RPC")
	}
}

func TestClientEndpoint_ListNodes(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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

	// Lookup the node with prefix
	get = &structs.NodeListRequest{
		QueryOptions: structs.QueryOptions{Region: "global", Prefix: node.ID[:4]},
	}
	var resp3 structs.NodeListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.List", get, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp3.Index != resp.Index {
		t.Fatalf("Bad index: %d %d", resp3.Index, resp2.Index)
	}

	if len(resp3.Nodes) != 1 {
		t.Fatalf("bad: %#v", resp3.Nodes)
	}
	if resp3.Nodes[0].ID != node.ID {
		t.Fatalf("bad: %#v", resp3.Nodes[0])
	}
}

func TestClientEndpoint_ListNodes_ACL(t *testing.T) {
	t.Parallel()
	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the node
	node := mock.Node()
	state := s1.fsm.State()
	assert.Nil(state.UpsertNode(1, node), "UpsertNode")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid", mock.NodePolicy(acl.PolicyRead))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid", mock.NodePolicy(acl.PolicyDeny))

	// Lookup the node without a token and expect failure
	req := &structs.NodeListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	{
		var resp structs.NodeListResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.List", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	req.AuthToken = validToken.SecretID
	{
		var resp structs.NodeListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.List", req, &resp), "RPC")
		assert.Equal(node.ID, resp.Nodes[0].ID)
	}

	// Try with a invalid token
	req.AuthToken = invalidToken.SecretID
	{
		var resp structs.NodeListResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.List", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a root token
	req.AuthToken = root.SecretID
	{
		var resp structs.NodeListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.List", req, &resp), "RPC")
		assert.Equal(node.ID, resp.Nodes[0].ID)
	}
}

func TestClientEndpoint_ListNodes_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the node
	node := mock.Node()

	// Node upsert triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertNode(2, node); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.NodeListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.NodeListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 2 {
		t.Fatalf("Bad index: %d %d", resp.Index, 2)
	}
	if len(resp.Nodes) != 1 || resp.Nodes[0].ID != node.ID {
		t.Fatalf("bad: %#v", resp.Nodes)
	}

	// Node drain updates trigger watches.
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpdateNodeDrain(3, node.ID, true); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 2
	var resp2 structs.NodeListResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Node.List", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 3 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 3)
	}
	if len(resp2.Nodes) != 1 || !resp2.Nodes[0].Drain {
		t.Fatalf("bad: %#v", resp2.Nodes)
	}

	// Node status update triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpdateNodeStatus(4, node.ID, structs.NodeStatusDown); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 3
	var resp3 structs.NodeListResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Node.List", req, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp3)
	}
	if resp3.Index != 4 {
		t.Fatalf("Bad index: %d %d", resp3.Index, 4)
	}
	if len(resp3.Nodes) != 1 || resp3.Nodes[0].Status != structs.NodeStatusDown {
		t.Fatalf("bad: %#v", resp3.Nodes)
	}

	// Node delete triggers watches.
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteNode(5, node.ID); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 4
	var resp4 structs.NodeListResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Node.List", req, &resp4); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp4)
	}
	if resp4.Index != 5 {
		t.Fatalf("Bad index: %d %d", resp4.Index, 5)
	}
	if len(resp4.Nodes) != 0 {
		t.Fatalf("bad: %#v", resp4.Nodes)
	}
}

func TestBatchFuture(t *testing.T) {
	t.Parallel()
	bf := NewBatchFuture()

	// Async respond to the future
	expect := fmt.Errorf("testing")
	go func() {
		time.Sleep(10 * time.Millisecond)
		bf.Respond(1000, expect)
	}()

	// Block for the result
	start := time.Now()
	err := bf.Wait()
	diff := time.Since(start)
	if diff < 5*time.Millisecond {
		t.Fatalf("too fast")
	}

	// Check the results
	if err != expect {
		t.Fatalf("bad: %s", err)
	}
	if bf.Index() != 1000 {
		t.Fatalf("bad: %d", bf.Index())
	}
}

func TestClientEndpoint_DeriveVaultToken_Bad(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the node
	node := mock.Node()
	if err := state.UpsertNode(2, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create an alloc
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	tasks := []string{task.Name}
	if err := state.UpsertAllocs(3, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := &structs.DeriveVaultTokenRequest{
		NodeID:   node.ID,
		SecretID: uuid.Generate(),
		AllocID:  alloc.ID,
		Tasks:    tasks,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.DeriveVaultTokenResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.DeriveVaultToken", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	if resp.Error == nil || !strings.Contains(resp.Error.Error(), "SecretID mismatch") {
		t.Fatalf("Expected SecretID mismatch: %v", resp.Error)
	}

	// Put the correct SecretID
	req.SecretID = node.SecretID

	// Now we should get an error about the allocation not running on the node
	if err := msgpackrpc.CallWithCodec(codec, "Node.DeriveVaultToken", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if resp.Error == nil || !strings.Contains(resp.Error.Error(), "not running on Node") {
		t.Fatalf("Expected not running on node error: %v", resp.Error)
	}

	// Update to be running on the node
	alloc.NodeID = node.ID
	if err := state.UpsertAllocs(4, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now we should get an error about the job not needing any Vault secrets
	if err := msgpackrpc.CallWithCodec(codec, "Node.DeriveVaultToken", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if resp.Error == nil || !strings.Contains(resp.Error.Error(), "does not require") {
		t.Fatalf("Expected no policies error: %v", resp.Error)
	}

	// Update to be terminal
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	if err := state.UpsertAllocs(5, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now we should get an error about the job not needing any Vault secrets
	if err := msgpackrpc.CallWithCodec(codec, "Node.DeriveVaultToken", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if resp.Error == nil || !strings.Contains(resp.Error.Error(), "terminal") {
		t.Fatalf("Expected terminal allocation error: %v", resp.Error)
	}
}

func TestClientEndpoint_DeriveVaultToken(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault and allow authenticated
	tr := true
	s1.config.VaultConfig.Enabled = &tr
	s1.config.VaultConfig.AllowUnauthenticated = &tr

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Create the node
	node := mock.Node()
	if err := state.UpsertNode(2, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create an alloc an allocation that has vault policies required
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	task := alloc.Job.TaskGroups[0].Tasks[0]
	tasks := []string{task.Name}
	task.Vault = &structs.Vault{Policies: []string{"a", "b"}}
	if err := state.UpsertAllocs(3, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Return a secret for the task
	token := uuid.Generate()
	accessor := uuid.Generate()
	ttl := 10
	secret := &vapi.Secret{
		WrapInfo: &vapi.SecretWrapInfo{
			Token:           token,
			WrappedAccessor: accessor,
			TTL:             ttl,
		},
	}
	tvc.SetCreateTokenSecret(alloc.ID, task.Name, secret)

	req := &structs.DeriveVaultTokenRequest{
		NodeID:   node.ID,
		SecretID: node.SecretID,
		AllocID:  alloc.ID,
		Tasks:    tasks,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.DeriveVaultTokenResponse
	if err := msgpackrpc.CallWithCodec(codec, "Node.DeriveVaultToken", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("bad: %v", resp.Error)
	}

	// Check the state store and ensure that we created a VaultAccessor
	ws := memdb.NewWatchSet()
	va, err := state.VaultAccessor(ws, accessor)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if va == nil {
		t.Fatalf("bad: %v", va)
	}

	if va.CreateIndex == 0 {
		t.Fatalf("bad: %v", va)
	}

	va.CreateIndex = 0
	expected := &structs.VaultAccessor{
		AllocID:     alloc.ID,
		Task:        task.Name,
		NodeID:      alloc.NodeID,
		Accessor:    accessor,
		CreationTTL: ttl,
	}

	if !reflect.DeepEqual(expected, va) {
		t.Fatalf("Got %#v; want %#v", va, expected)
	}
}

func TestClientEndpoint_DeriveVaultToken_VaultError(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault and allow authenticated
	tr := true
	s1.config.VaultConfig.Enabled = &tr
	s1.config.VaultConfig.AllowUnauthenticated = &tr

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Create the node
	node := mock.Node()
	if err := state.UpsertNode(2, node); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create an alloc an allocation that has vault policies required
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	task := alloc.Job.TaskGroups[0].Tasks[0]
	tasks := []string{task.Name}
	task.Vault = &structs.Vault{Policies: []string{"a", "b"}}
	if err := state.UpsertAllocs(3, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Return an error when creating the token
	tvc.SetCreateTokenError(alloc.ID, task.Name,
		structs.NewRecoverableError(fmt.Errorf("recover"), true))

	req := &structs.DeriveVaultTokenRequest{
		NodeID:   node.ID,
		SecretID: node.SecretID,
		AllocID:  alloc.ID,
		Tasks:    tasks,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.DeriveVaultTokenResponse
	err := msgpackrpc.CallWithCodec(codec, "Node.DeriveVaultToken", req, &resp)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if resp.Error == nil || !resp.Error.IsRecoverable() {
		t.Fatalf("bad: %+v", resp.Error)
	}
}
