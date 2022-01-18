package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHTTP_EvalList(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
		require.NoError(t, err)

		// simple list request
		req, err := http.NewRequest("GET", "/v1/evaluations", nil)
		require.NoError(t, err)
		respW := httptest.NewRecorder()
		obj, err := s.Server.EvalsRequest(respW, req)
		require.NoError(t, err)

		// check headers and response body
		require.NotEqual(t, "", respW.Result().Header.Get("X-Nomad-Index"), "missing index")
		require.Equal(t, "true", respW.Result().Header.Get("X-Nomad-KnownLeader"), "missing known leader")
		require.NotEqual(t, "", respW.Result().Header.Get("X-Nomad-LastContact"), "missing last contact")
		require.Len(t, obj.([]*structs.Evaluation), 2, "expected 2 evals")

		// paginated list request
		req, err = http.NewRequest("GET", "/v1/evaluations?per_page=1", nil)
		require.NoError(t, err)
		respW = httptest.NewRecorder()
		obj, err = s.Server.EvalsRequest(respW, req)
		require.NoError(t, err)

		// check response body
		require.Len(t, obj.([]*structs.Evaluation), 1, "expected 1 eval")

		// filtered list request
		req, err = http.NewRequest("GET",
			fmt.Sprintf("/v1/evaluations?per_page=10&job=%s", eval2.JobID), nil)
		require.NoError(t, err)
		respW = httptest.NewRecorder()
		obj, err = s.Server.EvalsRequest(respW, req)
		require.NoError(t, err)

		// check response body
		require.Len(t, obj.([]*structs.Evaluation), 1, "expected 1 eval")

	})
}

func TestHTTP_EvalPrefixList(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval1.ID = "aaabbbbb-e8f7-fd38-c855-ab94ceb89706"
		eval2 := mock.Eval()
		eval2.ID = "aaabbbbb-e8f7-fd38-c855-ab94ceb89706"
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/evaluations?prefix=aaab", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.EvalsRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Result().Header.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Result().Header.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the eval
		e := obj.([]*structs.Evaluation)
		if len(e) != 1 {
			t.Fatalf("bad: %#v", e)
		}

		// Check the identifier
		if e[0].ID != eval2.ID {
			t.Fatalf("expected eval ID: %v, Actual: %v", eval2.ID, e[0].ID)
		}
	})
}

func TestHTTP_EvalAllocations(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		alloc2 := mock.Alloc()
		alloc2.EvalID = alloc1.EvalID
		state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
		state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET",
			"/v1/evaluation/"+alloc1.EvalID+"/allocations", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.EvalSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Result().Header.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Result().Header.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the output
		allocs := obj.([]*structs.AllocListStub)
		if len(allocs) != 2 {
			t.Fatalf("bad: %#v", allocs)
		}
	})
}

func TestHTTP_EvalQuery(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		eval := mock.Eval()
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/evaluation/"+eval.ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.EvalSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.Result().Header.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.Result().Header.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the job
		e := obj.(*structs.Evaluation)
		if e.ID != eval.ID {
			t.Fatalf("bad: %#v", e)
		}
	})
}
