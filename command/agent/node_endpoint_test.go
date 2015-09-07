package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHTTP_NodesList(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		for i := 0; i < 3; i++ {
			// Create the node
			node := mock.Node()
			args := structs.NodeRegisterRequest{
				Node:         node,
				WriteRequest: structs.WriteRequest{Region: "region1"},
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

		// Check the job
		n := obj.([]*structs.NodeListStub)
		if len(n) < 3 { // Maybe 4 including client
			t.Fatalf("bad: %#v", n)
		}
	})
}

func TestHTTP_NodeForceEval(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		// Create the node
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "region1"},
		}
		var resp structs.NodeUpdateResponse
		if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		alloc1.NodeID = node.ID
		err := state.UpdateAllocations(1000, []*structs.Allocation{alloc1})
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
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "region1"},
		}
		var resp structs.NodeUpdateResponse
		if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		alloc1.NodeID = node.ID
		err := state.UpdateAllocations(1000, []*structs.Allocation{alloc1})
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
	})
}

func TestHTTP_NodeDrain(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		// Create the node
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "region1"},
		}
		var resp structs.NodeUpdateResponse
		if err := s.Agent.RPC("Node.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		alloc1.NodeID = node.ID
		err := state.UpdateAllocations(1000, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("POST", "/v1/node/"+node.ID+"/drain?enable=1", nil)
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
		upd := obj.(structs.NodeDrainUpdateResponse)
		if len(upd.EvalIDs) == 0 {
			t.Fatalf("bad: %v", upd)
		}
	})
}

func TestHTTP_NodeQuery(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		node := mock.Node()
		args := structs.NodeRegisterRequest{
			Node:         node,
			WriteRequest: structs.WriteRequest{Region: "region1"},
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
	})
}
