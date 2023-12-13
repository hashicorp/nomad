// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestHTTP_EvalList(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
		require.NoError(t, err)

		// simple list request
		req, err := http.NewRequest(http.MethodGet, "/v1/evaluations", nil)
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
		req, err = http.NewRequest(http.MethodGet, "/v1/evaluations?per_page=1", nil)
		require.NoError(t, err)
		respW = httptest.NewRecorder()
		obj, err = s.Server.EvalsRequest(respW, req)
		require.NoError(t, err)

		// check response body
		require.Len(t, obj.([]*structs.Evaluation), 1, "expected 1 eval")

		// filtered list request
		req, err = http.NewRequest(http.MethodGet,
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
	ci.Parallel(t)
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
		req, err := http.NewRequest(http.MethodGet, "/v1/evaluations?prefix=aaab", nil)
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

func TestHTTP_EvalsDelete(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		testFn func()
		name   string
	}{
		{
			testFn: func() {
				httpTest(t, nil, func(s *TestAgent) {

					// Create an empty request object which doesn't contain any
					// eval IDs.
					deleteReq := api.EvalDeleteRequest{}
					buf := encodeReq(&deleteReq)

					// Generate the HTTP request.
					req, err := http.NewRequest(http.MethodDelete, "/v1/evaluations", buf)
					require.NoError(t, err)
					respW := httptest.NewRecorder()

					// Make the request and check the response.
					obj, err := s.Server.EvalsRequest(respW, req)
					require.Equal(t,
						CodedError(http.StatusBadRequest, "evals must be deleted by either ID or filter"), err)
					require.Nil(t, obj)
				})
			},
			name: "too few eval IDs",
		},
		{
			testFn: func() {
				httpTest(t, nil, func(s *TestAgent) {

					deleteReq := api.EvalDeleteRequest{EvalIDs: make([]string, 8000)}

					// Generate a UUID and add it 8000 times to the eval ID
					// request array.
					evalID := uuid.Generate()

					for i := 0; i < 8000; i++ {
						deleteReq.EvalIDs[i] = evalID
					}

					buf := encodeReq(&deleteReq)

					// Generate the HTTP request.
					req, err := http.NewRequest(http.MethodDelete, "/v1/evaluations", buf)
					require.NoError(t, err)
					respW := httptest.NewRecorder()

					// Make the request and check the response.
					obj, err := s.Server.EvalsRequest(respW, req)
					require.Equal(t,
						CodedError(http.StatusBadRequest,
							"request includes 8000 evaluation IDs, must be 7281 or fewer"), err)
					require.Nil(t, obj)
				})
			},
			name: "too many eval IDs",
		},
		{
			testFn: func() {
				httpTest(t, func(c *Config) {
					c.NomadConfig.DefaultSchedulerConfig.PauseEvalBroker = true
				}, func(s *TestAgent) {

					// Generate a request with an eval ID that doesn't exist
					// within state.
					deleteReq := api.EvalDeleteRequest{EvalIDs: []string{uuid.Generate()}}
					buf := encodeReq(&deleteReq)

					// Generate the HTTP request.
					req, err := http.NewRequest(http.MethodDelete, "/v1/evaluations", buf)
					require.NoError(t, err)
					respW := httptest.NewRecorder()

					// Make the request and check the response.
					obj, err := s.Server.EvalsRequest(respW, req)
					require.Contains(t, err.Error(), "eval not found")
					require.Nil(t, obj)
				})
			},
			name: "eval doesn't exist",
		},
		{
			testFn: func() {
				httpTest(t, func(c *Config) {
					c.NomadConfig.DefaultSchedulerConfig.PauseEvalBroker = true
				}, func(s *TestAgent) {

					// Upsert an eval into state.
					mockEval := mock.Eval()

					err := s.Agent.server.State().UpsertEvals(
						structs.MsgTypeTestSetup, 10, []*structs.Evaluation{mockEval})
					require.NoError(t, err)

					// Generate a request with the ID of the eval previously upserted.
					deleteReq := api.EvalDeleteRequest{EvalIDs: []string{mockEval.ID}}
					buf := encodeReq(&deleteReq)

					// Generate the HTTP request.
					req, err := http.NewRequest(http.MethodDelete, "/v1/evaluations", buf)
					require.NoError(t, err)
					respW := httptest.NewRecorder()

					// Make the request and check the response.
					obj, err := s.Server.EvalsRequest(respW, req)
					require.NoError(t, err)
					require.NotNil(t, obj)
					deleteResp := obj.(structs.EvalDeleteResponse)
					require.Equal(t, deleteResp.Count, 1)

					// Ensure the eval is not found.
					readEval, err := s.Agent.server.State().EvalByID(nil, mockEval.ID)
					require.NoError(t, err)
					require.Nil(t, readEval)
				})
			},
			name: "successfully delete eval",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn()
		})
	}
}

func TestHTTP_EvalAllocations(t *testing.T) {
	ci.Parallel(t)
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
		req, err := http.NewRequest(http.MethodGet,
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
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		eval := mock.Eval()
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/evaluation/"+eval.ID, nil)
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

func TestHTTP_EvalQueryWithRelated(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()

		// Link related evals
		eval1.NextEval = eval2.ID
		eval2.PreviousEval = eval1.ID

		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
		require.NoError(t, err)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/v1/evaluation/%s?related=true", eval1.ID), nil)
		require.NoError(t, err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.EvalSpecificRequest(respW, req)
		require.NoError(t, err)

		// Check for the index
		require.NotEmpty(t, respW.Result().Header.Get("X-Nomad-Index"))
		require.NotEmpty(t, respW.Result().Header.Get("X-Nomad-KnownLeader"))
		require.NotEmpty(t, respW.Result().Header.Get("X-Nomad-LastContact"))

		// Check the eval
		e := obj.(*structs.Evaluation)
		require.Equal(t, eval1.ID, e.ID)

		// Check for the related evals
		expected := []*structs.EvaluationStub{
			eval2.Stub(),
		}
		require.Equal(t, expected, e.RelatedEvals)
	})
}

func TestHTTP_EvalCount(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
		must.NoError(t, err)

		// simple count request
		req, err := http.NewRequest(http.MethodGet, "/v1/evaluations/count", nil)
		must.NoError(t, err)
		respW := httptest.NewRecorder()
		obj, err := s.Server.EvalsCountRequest(respW, req)
		must.NoError(t, err)

		// check headers and response body
		must.NotEq(t, "", respW.Result().Header.Get("X-Nomad-Index"),
			must.Sprint("missing index"))
		must.Eq(t, "true", respW.Result().Header.Get("X-Nomad-KnownLeader"),
			must.Sprint("missing known leader"))
		must.NotEq(t, "", respW.Result().Header.Get("X-Nomad-LastContact"),
			must.Sprint("missing last contact"))

		resp := obj.(*structs.EvalCountResponse)
		must.Eq(t, resp.Count, 2)

		// filtered count request
		v := url.Values{}
		v.Add("filter", fmt.Sprintf("JobID==\"%s\"", eval2.JobID))
		req, err = http.NewRequest(http.MethodGet, "/v1/evaluations/count?"+v.Encode(), nil)
		must.NoError(t, err)
		respW = httptest.NewRecorder()
		obj, err = s.Server.EvalsCountRequest(respW, req)
		must.NoError(t, err)
		resp = obj.(*structs.EvalCountResponse)
		must.Eq(t, resp.Count, 1)

	})
}
