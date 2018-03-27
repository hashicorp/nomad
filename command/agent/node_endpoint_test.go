package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_NodesList(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		for i := 0; i < 3; i++ {
			// Create the node
			node := mock.Node()
			args := structs.NodeRegisterRequest{
				Node:         node,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			var resp structs.NodeUpdateResponse
			if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/nodes", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NodesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the nodes
		n := obj.([]*structs.NodeListStub)
		if len(n) < 3 { // Maybe 4 including client
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_NodesPrefixList(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		ids := []string{
			"12345678-abcd-efab-cdef-123456789abc",
			"12345678-aaaa-efab-cdef-123456789abc",
			"1234aaaa-abcd-efab-cdef-123456789abc",
			"1234bbbb-abcd-efab-cdef-123456789abc",
			"1234cccc-abcd-efab-cdef-123456789abc",
			"1234dddd-abcd-efab-cdef-123456789abc",
		}
		for i := 0; i < 5; i++ {
			// Create the node
			node := mock.Node()
			node.ID = ids[i]
			args := structs.NodeRegisterRequest{
				Node:         node,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			var resp structs.NodeUpdateResponse
			if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/nodes?prefix=12345678", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NodesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the nodes
		n := obj.([]*structs.NodeListStub)
		if len(n) != 2 {
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_NodeForceEval(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Create the node
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.NodeUpdateResponse
		if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		alloc1.NodeID = node.ID
		if err := state.UpsertJobSummary(999, mock.JobSummary(alloc1.JobID)); err != nil {
			t.Fatal(err)
		}
		err := state.UpsertAllocs(1000, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("POST", "/v1/node/"+node.ID+"/evaluate", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NodeSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the response
		upd := obj.(structs.NodeUpdateResponse)
		if len(upd.EvalIDs) == 0 {
			t.Fatalf("bad: %v", upd)
		}
	})
}

func TestHTTP_NodeAllocations(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.NodeUpdateResponse
		if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		alloc1.NodeID = node.ID
		if err := state.UpsertJobSummary(999, mock.JobSummary(alloc1.JobID)); err != nil {
			t.Fatal(err)
		}
		// Create a test event for the allocation
		testEvent := structs.NewTaskEvent(structs.TaskStarted)
		var events []*structs.TaskEvent
		events = append(events, testEvent)
		taskState := &structs.TaskState{Events: events}
		alloc1.TaskStates = make(map[string]*structs.TaskState)
		alloc1.TaskStates["test"] = taskState

		err := state.UpsertAllocs(1000, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/node/"+node.ID+"/allocations", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NodeSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the node
		allocs := obj.([]*structs.Allocation)
		if len(allocs) != 1 || allocs[0].ID != alloc1.ID {
			t.Fatalf("bad: %#v", allocs)
		}
		expectedDisplayMsg := "Task started by client"
		displayMsg := allocs[0].TaskStates["test"].Events[0].DisplayMessage
		assert.Equal(t, expectedDisplayMsg, displayMsg)
	})
}

func TestHTTP_NodeDrain(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the node
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.NodeUpdateResponse
		require.Nil(s.Agent.RPC("Node.Register", &args, &resp))

		drainReq := api.NodeUpdateDrainRequest{
			NodeID: node.ID,
			DrainSpec: &api.DrainSpec{
				Deadline: 10 * time.Second,
			},
		}

		// Make the HTTP request
		buf := encodeReq(drainReq)
		req, err := http.NewRequest("POST", "/v1/node/"+node.ID+"/drain", buf)
		require.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NodeSpecificRequest(respW, req)
		require.Nil(err)

		// Check for the index
		require.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))

		// Check the response
		_, ok := obj.(structs.NodeDrainUpdateResponse)
		require.True(ok)

		// Check that the node has been updated
		state := s.Agent.server.State()
		out, err := state.NodeByID(nil, node.ID)
		require.Nil(err)
		require.True(out.Drain)
		require.NotNil(out.DrainStrategy)
		require.Equal(10*time.Second, out.DrainStrategy.Deadline)

		// Make the HTTP request to unset drain
		drainReq.DrainSpec = nil
		buf = encodeReq(drainReq)
		req, err = http.NewRequest("POST", "/v1/node/"+node.ID+"/drain", buf)
		require.Nil(err)
		respW = httptest.NewRecorder()

		// Make the request
		_, err = s.Server.NodeSpecificRequest(respW, req)
		require.Nil(err)

		out, err = state.NodeByID(nil, node.ID)
		require.Nil(err)
		require.False(out.Drain)
		require.Nil(out.DrainStrategy)
	})
}

func TestHTTP_NodeEligible(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the node
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.NodeUpdateResponse
		require.Nil(s.Agent.RPC("Node.Register", &args, &resp))

		drainReq := api.NodeUpdateEligibilityRequest{
			NodeID:      node.ID,
			Eligibility: structs.NodeSchedulingIneligible,
		}

		// Make the HTTP request
		buf := encodeReq(drainReq)
		req, err := http.NewRequest("POST", "/v1/node/"+node.ID+"/eligibility", buf)
		require.Nil(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NodeSpecificRequest(respW, req)
		require.Nil(err)

		// Check for the index
		require.NotZero(respW.HeaderMap.Get("X-Nomad-Index"))

		// Check the response
		_, ok := obj.(structs.NodeEligibilityUpdateResponse)
		require.True(ok)

		// Check that the node has been updated
		state := s.Agent.server.State()
		out, err := state.NodeByID(nil, node.ID)
		require.Nil(err)
		require.Equal(structs.NodeSchedulingIneligible, out.SchedulingEligibility)

		// Make the HTTP request to set something invalid
		drainReq.Eligibility = "foo"
		buf = encodeReq(drainReq)
		req, err = http.NewRequest("POST", "/v1/node/"+node.ID+"/eligibility", buf)
		require.Nil(err)
		respW = httptest.NewRecorder()

		// Make the request
		_, err = s.Server.NodeSpecificRequest(respW, req)
		require.NotNil(err)
		require.Contains(err.Error(), "invalid")
	})
}

func TestHTTP_NodePurge(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Create the node
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.NodeUpdateResponse
		if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Add some allocations to the node
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		alloc1.NodeID = node.ID
		if err := state.UpsertJobSummary(999, mock.JobSummary(alloc1.JobID)); err != nil {
			t.Fatal(err)
		}
		err := state.UpsertAllocs(1000, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request to purge it
		req, err := http.NewRequest("POST", "/v1/node/"+node.ID+"/purge", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NodeSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the response
		upd := obj.(structs.NodeUpdateResponse)
		if len(upd.EvalIDs) == 0 {
			t.Fatalf("bad: %v", upd)
		}

		// Ensure that the node is not present anymore
		args1 := structs.NodeSpecificRequest{
			NodeID:       node.ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		var resp1 structs.SingleNodeResponse
		if err := s.Agent.RPC("Node.GetNode", &args1, &resp1); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp1.Node != nil {
			t.Fatalf("node still exists after purging: %#v", resp1.Node)
		}
	})
}

func TestHTTP_NodeQuery(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.NodeUpdateResponse
		if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/node/"+node.ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.NodeSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the node
		n := obj.(*structs.Node)
		if n.ID != node.ID {
			t.Fatalf("bad: %#v", n)
		}
		if len(n.Events) < 1 {
			t.Fatalf("Expected node registration event to be populated: %#v", n)
		}
		if n.Events[0].Message != "Node Registered" {
			t.Fatalf("Expected node registration event to be first node event: %#v", n)
		}
	})
}
