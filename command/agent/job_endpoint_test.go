// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/acl"
	api "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_JobsList(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		for i := 0; i < 3; i++ {
			// Create the job
			job := mock.Job()
			args := structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
			}
			var resp structs.JobRegisterResponse
			if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/jobs", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobsRequest(respW, req)
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
		j := obj.([]*structs.JobListStub)
		if len(j) != 3 {
			t.Fatalf("bad: %#v", j)
		}
	})
}

func TestHTTP_PrefixJobsList(t *testing.T) {
	ci.Parallel(t)

	ids := []string{
		"aaaaaaaa-e8f7-fd38-c855-ab94ceb89706",
		"aabbbbbb-e8f7-fd38-c855-ab94ceb89706",
		"aabbcccc-e8f7-fd38-c855-ab94ceb89706",
	}
	httpTest(t, nil, func(s *TestAgent) {
		for i := 0; i < 3; i++ {
			// Create the job
			job := mock.Job()
			job.ID = ids[i]
			job.TaskGroups[0].Count = 1
			args := structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
			}
			var resp structs.JobRegisterResponse
			if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/jobs?prefix=aabb", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobsRequest(respW, req)
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
		j := obj.([]*structs.JobListStub)
		if len(j) != 2 {
			t.Fatalf("bad: %#v", j)
		}
	})
}

func TestHTTP_JobsList_AllNamespaces_OSS(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		for i := 0; i < 3; i++ {
			// Create the job
			job := mock.Job()
			args := structs.JobRegisterRequest{
				Job: job,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
			}
			var resp structs.JobRegisterResponse
			err := s.Agent.RPC("Job.Register", &args, &resp)
			require.NoError(t, err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/jobs?namespace=*", nil)
		require.NoError(t, err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobsRequest(respW, req)
		require.NoError(t, err)

		// Check for the index
		require.NotEmpty(t, respW.Result().Header.Get("X-Nomad-Index"), "missing index")
		require.Equal(t, "true", respW.Result().Header.Get("X-Nomad-KnownLeader"), "missing known leader")
		require.NotEmpty(t, respW.Result().Header.Get("X-Nomad-LastContact"), "missing last contact")

		// Check the job
		j := obj.([]*structs.JobListStub)
		require.Len(t, j, 3)

		require.Equal(t, "default", j[0].Namespace)
	})
}

func TestHTTP_JobsRegister(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := MockJob()
		args := api.JobRegisterRequest{
			Job:          job,
			WriteRequest: api.WriteRequest{Region: "global"},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/jobs", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobsRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		dereg := obj.(structs.JobRegisterResponse)
		if dereg.EvalID == "" {
			t.Fatalf("bad: %v", dereg)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the job is registered
		getReq := structs.JobSpecificRequest{
			JobID: *job.ID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var getResp structs.SingleJobResponse
		if err := s.Agent.RPC("Job.GetJob", &getReq, &getResp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if getResp.Job == nil {
			t.Fatalf("job does not exist")
		}
	})
}

func TestHTTP_JobsRegister_IgnoresParentID(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := MockJob()
		parentID := "somebadparentid"
		job.ParentID = &parentID
		args := api.JobRegisterRequest{
			Job:          job,
			WriteRequest: api.WriteRequest{Region: "global"},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/jobs", buf)
		require.NoError(t, err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobsRequest(respW, req)
		require.NoError(t, err)

		// Check the response
		reg := obj.(structs.JobRegisterResponse)
		require.NotEmpty(t, reg.EvalID)

		// Check for the index
		require.NotEmpty(t, respW.Result().Header.Get("X-Nomad-Index"))

		// Check the job is registered
		getReq := structs.JobSpecificRequest{
			JobID: *job.ID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var getResp structs.SingleJobResponse
		err = s.Agent.RPC("Job.GetJob", &getReq, &getResp)
		require.NoError(t, err)

		require.NotNil(t, getResp.Job)
		require.Equal(t, *job.ID, getResp.Job.ID)
		require.Empty(t, getResp.Job.ParentID)

		// check the eval exists
		evalReq := structs.EvalSpecificRequest{
			EvalID: reg.EvalID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var evalResp structs.SingleEvalResponse
		err = s.Agent.RPC("Eval.GetEval", &evalReq, &evalResp)
		require.NoError(t, err)

		require.NotNil(t, evalResp.Eval)
		require.Equal(t, reg.EvalID, evalResp.Eval.ID)
	})
}

// Test that ACL token is properly threaded through to the RPC endpoint
func TestHTTP_JobsRegister_ACL(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := MockJob()
		args := api.JobRegisterRequest{
			Job: job,
			WriteRequest: api.WriteRequest{
				Region: "global",
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/jobs", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()
		setToken(req, s.RootToken)

		// Make the request
		obj, err := s.Server.JobsRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		assert.NotNil(t, obj)
	})
}

func TestHTTP_JobsRegister_Defaulting(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := MockJob()

		// Do not set its priority
		job.Priority = nil

		args := api.JobRegisterRequest{
			Job:          job,
			WriteRequest: api.WriteRequest{Region: "global"},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/jobs", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobsRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		dereg := obj.(structs.JobRegisterResponse)
		if dereg.EvalID == "" {
			t.Fatalf("bad: %v", dereg)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the job is registered
		getReq := structs.JobSpecificRequest{
			JobID: *job.ID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var getResp structs.SingleJobResponse
		if err := s.Agent.RPC("Job.GetJob", &getReq, &getResp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if getResp.Job == nil {
			t.Fatalf("job does not exist")
		}
		if getResp.Job.Priority != 50 {
			t.Fatalf("job didn't get defaulted")
		}
	})
}

func TestHTTP_JobsParse(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		buf := encodeReq(api.JobsParseRequest{JobHCL: mock.HCL()})
		req, err := http.NewRequest(http.MethodPost, "/v1/jobs/parse", buf)
		must.NoError(t, err)

		respW := httptest.NewRecorder()

		obj, err := s.Server.JobsParseRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, obj)

		job := obj.(*api.Job)
		expected := mock.Job()
		must.Eq(t, expected.Name, *job.Name)
		must.Eq(t, expected.Datacenters[0], job.Datacenters[0])
	})
}

func TestHTTP_JobsParse_HCLVar(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		hclJob, hclVar := mock.HCLVar()
		buf := encodeReq(api.JobsParseRequest{
			JobHCL:    hclJob,
			Variables: hclVar,
		})
		req, err := http.NewRequest(http.MethodPost, "/v1/jobs/parse", buf)
		must.NoError(t, err)

		respW := httptest.NewRecorder()

		obj, err := s.Server.JobsParseRequest(respW, req)
		must.NoError(t, err)
		must.NotNil(t, obj)

		job := obj.(*api.Job)

		must.Eq(t, "var-job", *job.Name)
		must.Eq(t, map[string]any{
			"command": "echo",
			"args":    []any{"S is stringy, N is 42, B is true"},
		}, job.TaskGroups[0].Tasks[0].Config)
	})
}

func TestHTTP_JobsParse_ACL(t *testing.T) {
	ci.Parallel(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// ACL tokens used in tests.
		nodeToken := mock.CreatePolicyAndToken(
			t, state, 1000, "node",
			mock.NodePolicy(acl.PolicyWrite),
		)
		parseJobDevToken := mock.CreatePolicyAndToken(
			t, state, 1002, "parse-job-dev",
			mock.NamespacePolicy("dev", "", []string{"parse-job"}),
		)
		readNsDevToken := mock.CreatePolicyAndToken(
			t, state, 1004, "read-dev",
			mock.NamespacePolicy("dev", "read", nil),
		)
		parseJobDefaultToken := mock.CreatePolicyAndToken(
			t, state, 1006, "parse-job-default",
			mock.NamespacePolicy("default", "", []string{"parse-job"}),
		)
		submitJobDefaultToken := mock.CreatePolicyAndToken(
			t, state, 1008, "submit-job-default",
			mock.NamespacePolicy("default", "", []string{"submit-job"}),
		)
		readNsDefaultToken := mock.CreatePolicyAndToken(
			t, state, 1010, "read-default",
			mock.NamespacePolicy("default", "read", nil),
		)

		testCases := []struct {
			name        string
			token       *structs.ACLToken
			namespace   string
			expectError bool
		}{
			{
				name:        "missing ACL token",
				token:       nil,
				expectError: true,
			},
			{
				name:        "wrong permissions",
				token:       nodeToken,
				expectError: true,
			},
			{
				name:        "wrong namespace",
				token:       readNsDevToken,
				expectError: true,
			},
			{
				name:        "wrong namespace capability",
				token:       parseJobDevToken,
				expectError: true,
			},
			{
				name:        "default namespace read",
				token:       readNsDefaultToken,
				expectError: false,
			},
			{
				name:        "non-default namespace read",
				token:       readNsDevToken,
				namespace:   "dev",
				expectError: false,
			},
			{
				name:        "default namespace parse-job capability",
				token:       parseJobDefaultToken,
				expectError: false,
			},
			{
				name:        "default namespace submit-job capability",
				token:       submitJobDefaultToken,
				expectError: false,
			},
			{
				name:        "non-default namespace capability",
				token:       parseJobDevToken,
				namespace:   "dev",
				expectError: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				buf := encodeReq(api.JobsParseRequest{JobHCL: mock.HCL()})
				req, err := http.NewRequest(http.MethodPost, "/v1/jobs/parse", buf)
				require.NoError(t, err)

				if tc.namespace != "" {
					setNamespace(req, tc.namespace)
				}

				if tc.token != nil {
					setToken(req, tc.token)
				}

				respW := httptest.NewRecorder()
				obj, err := s.Server.JobsParseRequest(respW, req)

				if tc.expectError {
					require.Error(t, err)
					require.Equal(t, structs.ErrPermissionDenied.Error(), err.Error())
				} else {
					require.NoError(t, err)
					require.NotNil(t, obj)

					job := obj.(*api.Job)
					expected := mock.Job()
					require.Equal(t, expected.Name, *job.Name)
					require.ElementsMatch(t, expected.Datacenters, job.Datacenters)
				}
			})
		}
	})
}

func TestHTTP_JobQuery(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+job.ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
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
		j := obj.(*structs.Job)
		if j.ID != job.ID {
			t.Fatalf("bad: %#v", j)
		}
	})
}

func TestHTTP_JobQuery_Payload(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()

		// Insert Payload compressed
		expected := []byte("hello world")
		compressed := snappy.Encode(nil, expected)
		job.Payload = compressed

		// Directly manipulate the state
		state := s.Agent.server.State()
		if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job); err != nil {
			t.Fatalf("Failed to upsert job: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+job.ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
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
		j := obj.(*structs.Job)
		if j.ID != job.ID {
			t.Fatalf("bad: %#v", j)
		}

		// Check the payload is decompressed
		if !reflect.DeepEqual(j.Payload, expected) {
			t.Fatalf("Payload not decompressed properly; got %#v; want %#v", j.Payload, expected)
		}
	})
}

func TestHTTP_jobUpdate_systemScaling(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := MockJob()
		job.Type = pointer.Of("system")
		job.TaskGroups[0].Scaling = &api.ScalingPolicy{Enabled: pointer.Of(true)}
		args := api.JobRegisterRequest{
			Job: job,
			WriteRequest: api.WriteRequest{
				Region:    "global",
				Namespace: api.DefaultNamespace,
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/job/"+*job.ID, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		assert.Nil(t, obj)
		assert.Equal(t, CodedError(400, "Task groups with job type system do not support scaling blocks"), err)
	})
}

func TestHTTP_JobUpdate(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := MockJob()
		args := api.JobRegisterRequest{
			Job: job,
			WriteRequest: api.WriteRequest{
				Region:    "global",
				Namespace: api.DefaultNamespace,
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/job/"+*job.ID, buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		dereg := obj.(structs.JobRegisterResponse)
		if dereg.EvalID == "" {
			t.Fatalf("bad: %v", dereg)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the job is registered
		getReq := structs.JobSpecificRequest{
			JobID: *job.ID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var getResp structs.SingleJobResponse
		if err := s.Agent.RPC("Job.GetJob", &getReq, &getResp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if getResp.Job == nil {
			t.Fatalf("job does not exist")
		}
	})
}

func TestHTTP_JobUpdate_EvalPriority(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputEvalPriority int
		expectedError     bool
		name              string
	}{
		{
			inputEvalPriority: 95,
			expectedError:     false,
			name:              "valid input eval priority",
		},
		{
			inputEvalPriority: 99999999999,
			expectedError:     true,
			name:              "invalid input eval priority",
		},
		{
			inputEvalPriority: 0,
			expectedError:     false,
			name:              "no input eval priority",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			httpTest(t, nil, func(s *TestAgent) {
				// Create the job
				job := MockJob()
				args := api.JobRegisterRequest{
					Job: job,
					WriteRequest: api.WriteRequest{
						Region:    "global",
						Namespace: api.DefaultNamespace,
					},
				}

				// Add our eval priority query param if set.
				if tc.inputEvalPriority > 0 {
					args.EvalPriority = tc.inputEvalPriority
				}
				buf := encodeReq(args)

				// Make the HTTP request
				req, err := http.NewRequest(http.MethodPut, "/v1/job/"+*job.ID, buf)
				assert.Nil(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.JobSpecificRequest(respW, req)
				if tc.expectedError {
					assert.NotNil(t, err)
					return
				} else {
					assert.Nil(t, err)
				}

				// Check the response
				regResp := obj.(structs.JobRegisterResponse)
				assert.NotEmpty(t, regResp.EvalID)
				assert.NotEmpty(t, respW.Result().Header.Get("X-Nomad-Index"))

				// Check the job is registered
				getReq := structs.JobSpecificRequest{
					JobID: *job.ID,
					QueryOptions: structs.QueryOptions{
						Region:    "global",
						Namespace: structs.DefaultNamespace,
					},
				}
				var getResp structs.SingleJobResponse
				assert.Nil(t, s.Agent.RPC("Job.GetJob", &getReq, &getResp))
				assert.NotNil(t, getResp.Job)

				// Check the evaluation that resulted from the job register.
				evalInfoReq, err := http.NewRequest(http.MethodGet, "/v1/evaluation/"+regResp.EvalID, nil)
				assert.Nil(t, err)
				respW.Flush()

				evalRaw, err := s.Server.EvalSpecificRequest(respW, evalInfoReq)
				assert.Nil(t, err)
				evalRespObj := evalRaw.(*structs.Evaluation)

				if tc.inputEvalPriority > 0 {
					assert.Equal(t, tc.inputEvalPriority, evalRespObj.Priority)
				} else {
					assert.Equal(t, *job.Priority, evalRespObj.Priority)
				}
			})
		})
	}
}

func TestHTTP_JobUpdateRegion(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name           string
		ConfigRegion   string
		APIRegion      string
		ExpectedRegion string
	}{
		{
			Name:           "api region takes precedence",
			ConfigRegion:   "not-global",
			APIRegion:      "north-america",
			ExpectedRegion: "north-america",
		},
		{
			Name:           "config region is set",
			ConfigRegion:   "north-america",
			APIRegion:      "",
			ExpectedRegion: "north-america",
		},
		{
			Name:           "api region is set",
			ConfigRegion:   "",
			APIRegion:      "north-america",
			ExpectedRegion: "north-america",
		},
		{
			Name:           "defaults to node region global if no region is provided",
			ConfigRegion:   "",
			APIRegion:      "",
			ExpectedRegion: "global",
		},
		{
			Name:           "defaults to node region not-global if no region is provided",
			ConfigRegion:   "",
			APIRegion:      "",
			ExpectedRegion: "not-global",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			httpTest(t, func(c *Config) { c.Region = tc.ExpectedRegion }, func(s *TestAgent) {
				// Create the job
				job := MockRegionalJob()

				if tc.ConfigRegion == "" {
					job.Region = nil
				} else {
					job.Region = &tc.ConfigRegion
				}

				args := api.JobRegisterRequest{
					Job: job,
					WriteRequest: api.WriteRequest{
						Namespace: api.DefaultNamespace,
						Region:    tc.APIRegion,
					},
				}

				buf := encodeReq(args)

				// Make the HTTP request
				url := "/v1/job/" + *job.ID

				req, err := http.NewRequest(http.MethodPut, url, buf)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.JobSpecificRequest(respW, req)
				require.NoError(t, err)

				// Check the response
				dereg := obj.(structs.JobRegisterResponse)
				require.NotEmpty(t, dereg.EvalID)

				// Check for the index
				require.NotEmpty(t, respW.Result().Header.Get("X-Nomad-Index"), "missing index")

				// Check the job is registered
				getReq := structs.JobSpecificRequest{
					JobID: *job.ID,
					QueryOptions: structs.QueryOptions{
						Region:    tc.ExpectedRegion,
						Namespace: structs.DefaultNamespace,
					},
				}
				var getResp structs.SingleJobResponse
				err = s.Agent.RPC("Job.GetJob", &getReq, &getResp)
				require.NoError(t, err)
				require.NotNil(t, getResp.Job, "job does not exist")
				require.Equal(t, tc.ExpectedRegion, getResp.Job.Region)
			})
		})
	}
}

func TestHTTP_JobDelete(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request to do a soft delete
		req, err := http.NewRequest(http.MethodDelete, "/v1/job/"+job.ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		dereg := obj.(structs.JobDeregisterResponse)
		if dereg.EvalID == "" {
			t.Fatalf("bad: %v", dereg)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the job is still queryable
		getReq1 := structs.JobSpecificRequest{
			JobID: job.ID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var getResp1 structs.SingleJobResponse
		if err := s.Agent.RPC("Job.GetJob", &getReq1, &getResp1); err != nil {
			t.Fatalf("err: %v", err)
		}
		if getResp1.Job == nil {
			t.Fatalf("job doesn't exists")
		}
		if !getResp1.Job.Stop {
			t.Fatalf("job should be marked as stop")
		}

		// Make the HTTP request to do a purge delete
		req2, err := http.NewRequest(http.MethodDelete, "/v1/job/"+job.ID+"?purge=true", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW.Flush()

		// Make the request
		obj, err = s.Server.JobSpecificRequest(respW, req2)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		dereg = obj.(structs.JobDeregisterResponse)
		if dereg.EvalID == "" {
			t.Fatalf("bad: %v", dereg)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the job is gone
		getReq2 := structs.JobSpecificRequest{
			JobID: job.ID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var getResp2 structs.SingleJobResponse
		if err := s.Agent.RPC("Job.GetJob", &getReq2, &getResp2); err != nil {
			t.Fatalf("err: %v", err)
		}
		if getResp2.Job != nil {
			t.Fatalf("job still exists")
		}
	})
}

func TestHTTP_JobDelete_EvalPriority(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputEvalPriority int
		expectedError     bool
		name              string
	}{
		{
			inputEvalPriority: 95,
			expectedError:     false,
			name:              "valid input eval priority",
		},
		{
			inputEvalPriority: 99999999999,
			expectedError:     true,
			name:              "invalid input eval priority",
		},
		{
			inputEvalPriority: 0,
			expectedError:     false,
			name:              "no input eval priority",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			httpTest(t, nil, func(s *TestAgent) {
				// Create the job
				job := MockJob()
				args := api.JobRegisterRequest{
					Job: job,
					WriteRequest: api.WriteRequest{
						Region:    "global",
						Namespace: api.DefaultNamespace,
					},
				}
				buf := encodeReq(args)

				// Make the HTTP request
				regReq, err := http.NewRequest(http.MethodPut, "/v1/job/"+*job.ID, buf)
				assert.Nil(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.JobSpecificRequest(respW, regReq)
				assert.Nil(t, err)

				// Check the response
				regResp := obj.(structs.JobRegisterResponse)
				assert.NotEmpty(t, regResp.EvalID)
				assert.NotEmpty(t, respW.Result().Header.Get("X-Nomad-Index"))

				// Check the job is registered
				getReq := structs.JobSpecificRequest{
					JobID: *job.ID,
					QueryOptions: structs.QueryOptions{
						Region:    "global",
						Namespace: structs.DefaultNamespace,
					},
				}
				var getResp structs.SingleJobResponse
				assert.Nil(t, s.Agent.RPC("Job.GetJob", &getReq, &getResp))
				assert.NotNil(t, getResp.Job)

				// Delete the job.
				deleteReq, err := http.NewRequest(http.MethodDelete, "/v1/job/"+*job.ID+"?purge=true", nil)
				assert.Nil(t, err)
				respW.Flush()

				// Add our eval priority query param if set.
				if tc.inputEvalPriority > 0 {
					q := deleteReq.URL.Query()
					q.Add("eval_priority", strconv.Itoa(tc.inputEvalPriority))
					deleteReq.URL.RawQuery = q.Encode()
				}

				// Make the request
				obj, err = s.Server.JobSpecificRequest(respW, deleteReq)
				if tc.expectedError {
					assert.NotNil(t, err)
					return
				} else {
					assert.Nil(t, err)
				}

				// Check the response
				dereg := obj.(structs.JobDeregisterResponse)
				assert.NotEmpty(t, dereg.EvalID)
				assert.NotEmpty(t, respW.Result().Header.Get("X-Nomad-Index"))

				// Check the evaluation that resulted from the job register.
				evalInfoReq, err := http.NewRequest(http.MethodGet, "/v1/evaluation/"+dereg.EvalID, nil)
				assert.Nil(t, err)
				respW.Flush()

				evalRaw, err := s.Server.EvalSpecificRequest(respW, evalInfoReq)
				assert.Nil(t, err)
				evalRespObj := evalRaw.(*structs.Evaluation)

				if tc.inputEvalPriority > 0 {
					assert.Equal(t, tc.inputEvalPriority, evalRespObj.Priority)
				} else {
					assert.Equal(t, *job.Priority, evalRespObj.Priority)
				}
			})
		})
	}
}

func TestHTTP_Job_ScaleTaskGroup(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		require.NoError(s.Agent.RPC("Job.Register", &args, &resp))

		newCount := job.TaskGroups[0].Count + 1
		scaleReq := &api.ScalingRequest{
			Count:   pointer.Of(int64(newCount)),
			Message: "testing",
			Target: map[string]string{
				"Job":   job.ID,
				"Group": job.TaskGroups[0].Name,
			},
		}
		buf := encodeReq(scaleReq)

		// Make the HTTP request to scale the job group
		req, err := http.NewRequest(http.MethodPost, "/v1/job/"+job.ID+"/scale", buf)
		require.NoError(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		require.NoError(err)

		// Check the response
		resp = obj.(structs.JobRegisterResponse)
		require.NotEmpty(resp.EvalID)

		// Check for the index
		require.NotEmpty(respW.Header().Get("X-Nomad-Index"))

		// Check that the group count was changed
		getReq := structs.JobSpecificRequest{
			JobID: job.ID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var getResp structs.SingleJobResponse
		err = s.Agent.RPC("Job.GetJob", &getReq, &getResp)
		require.NoError(err)
		require.NotNil(getResp.Job)
		require.Equal(newCount, getResp.Job.TaskGroups[0].Count)
	})
}

func TestHTTP_Job_ScaleStatus(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request to scale the job group
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+job.ID+"/scale", nil)
		require.NoError(err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		require.NoError(err)

		// Check the response
		status := obj.(*structs.JobScaleStatus)
		require.NotEmpty(resp.EvalID)
		require.Equal(job.TaskGroups[0].Count, status.TaskGroups[job.TaskGroups[0].Name].Desired)

		// Check for the index
		require.NotEmpty(respW.Header().Get("X-Nomad-Index"))
	})
}

func TestHTTP_JobForceEvaluate(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPost, "/v1/job/"+job.ID+"/evaluate", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		reg := obj.(structs.JobRegisterResponse)
		if reg.EvalID == "" {
			t.Fatalf("bad: %v", reg)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
	})
}

func TestHTTP_JobEvaluate_ForceReschedule(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}
		jobEvalReq := api.JobEvaluateRequest{
			JobID: job.ID,
			EvalOptions: api.EvalOptions{
				ForceReschedule: true,
			},
		}

		buf := encodeReq(jobEvalReq)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPost, "/v1/job/"+job.ID+"/evaluate", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		reg := obj.(structs.JobRegisterResponse)
		if reg.EvalID == "" {
			t.Fatalf("bad: %v", reg)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
	})
}

func TestHTTP_JobEvaluations(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+job.ID+"/evaluations", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		evals := obj.([]*structs.Evaluation)
		// Can be multiple evals, use the last one, since they are in order
		idx := len(evals) - 1
		if len(evals) < 0 || evals[idx].ID != resp.EvalID {
			t.Fatalf("bad: %v", evals)
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
	})
}

func TestHTTP_JobAllocations(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		alloc1 := mock.Alloc()
		args := structs.JobRegisterRequest{
			Job: alloc1.Job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Directly manipulate the state
		expectedDisplayMsg := "test message"
		testEvent := structs.NewTaskEvent("test event").SetMessage(expectedDisplayMsg)
		var events []*structs.TaskEvent
		events = append(events, testEvent)
		taskState := &structs.TaskState{Events: events}
		alloc1.TaskStates = make(map[string]*structs.TaskState)
		alloc1.TaskStates["test"] = taskState
		state := s.Agent.server.State()
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+alloc1.Job.ID+"/allocations?all=true", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		allocs := obj.([]*structs.AllocListStub)
		if len(allocs) != 1 && allocs[0].ID != alloc1.ID {
			t.Fatalf("bad: %v", allocs)
		}
		displayMsg := allocs[0].TaskStates["test"].Events[0].DisplayMessage
		assert.Equal(t, expectedDisplayMsg, displayMsg)

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
	})
}

func TestHTTP_JobDeployments(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		j := mock.Job()
		args := structs.JobRegisterRequest{
			Job: j,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		assert.Nil(s.Agent.RPC("Job.Register", &args, &resp), "JobRegister")

		// Directly manipulate the state
		state := s.Agent.server.State()
		d := mock.Deployment()
		d.JobID = j.ID
		d.JobCreateIndex = resp.JobModifyIndex

		assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+j.ID+"/deployments", nil)
		assert.Nil(err, "HTTP")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		assert.Nil(err, "JobSpecificRequest")

		// Check the response
		deploys := obj.([]*structs.Deployment)
		assert.Len(deploys, 1, "deployments")
		assert.Equal(d.ID, deploys[0].ID, "deployment id")

		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
		assert.Equal("true", respW.Result().Header.Get("X-Nomad-KnownLeader"), "missing known leader")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-LastContact"), "missing last contact")
	})
}

func TestHTTP_JobDeployment(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		j := mock.Job()
		args := structs.JobRegisterRequest{
			Job: j,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		assert.Nil(s.Agent.RPC("Job.Register", &args, &resp), "JobRegister")

		// Directly manipulate the state
		state := s.Agent.server.State()
		d := mock.Deployment()
		d.JobID = j.ID
		d.JobCreateIndex = resp.JobModifyIndex
		assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+j.ID+"/deployment", nil)
		assert.Nil(err, "HTTP")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		assert.Nil(err, "JobSpecificRequest")

		// Check the response
		out := obj.(*structs.Deployment)
		assert.NotNil(out, "deployment")
		assert.Equal(d.ID, out.ID, "deployment id")

		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
		assert.Equal("true", respW.Result().Header.Get("X-Nomad-KnownLeader"), "missing known leader")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-LastContact"), "missing last contact")
	})
}

func TestHTTP_JobVersions(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		job2 := mock.Job()
		job2.ID = job.ID
		job2.Priority = 100

		args2 := structs.JobRegisterRequest{
			Job: job2,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp2 structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args2, &resp2); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+job.ID+"/versions?diffs=true", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		vResp := obj.(structs.JobVersionsResponse)
		versions := vResp.Versions
		if len(versions) != 2 {
			t.Fatalf("got %d versions; want 2", len(versions))
		}

		if v := versions[0]; v.Version != 1 || v.Priority != 100 {
			t.Fatalf("bad %v", v)
		}

		if v := versions[1]; v.Version != 0 {
			t.Fatalf("bad %v", v)
		}

		if len(vResp.Diffs) != 1 {
			t.Fatalf("bad %v", vResp)
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
	})
}

func TestHTTP_JobSubmission(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
			Submission: &structs.JobSubmission{
				Source: mock.HCL(),
				Format: "hcl2",
			},
		}
		var resp structs.JobRegisterResponse
		must.NoError(t, s.Agent.RPC("Job.Register", &args, &resp))

		respW := httptest.NewRecorder()

		// make request for job submission @ v0
		req, err := http.NewRequest(http.MethodGet, "/v1/job/"+job.ID+"/submission?version=0", nil)
		must.NoError(t, err)
		submission, err := s.Server.jobSubmissionCRUD(respW, req, job.ID)
		must.NoError(t, err)
		must.Eq(t, "hcl2", submission.Format)
		must.StrContains(t, submission.Source, `job "my-job" {`)

		// make request for job submission @v1 (does not exist)
		req, err = http.NewRequest(http.MethodGet, "/v1/job/"+job.ID+"/submission?version=1", nil)
		must.NoError(t, err)
		_, err = s.Server.jobSubmissionCRUD(respW, req, job.ID)
		must.ErrorContains(t, err, "job source not found")

		// make POST request (invalid method)
		req, err = http.NewRequest(http.MethodPost, "/v1/job/"+job.ID+"/submission?version=0", nil)
		must.NoError(t, err)
		_, err = s.Server.jobSubmissionCRUD(respW, req, job.ID)
		must.ErrorContains(t, err, "Invalid method")
	})
}

func TestHTTP_PeriodicForce(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create and register a periodic job.
		job := mock.PeriodicJob()
		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPost, "/v1/job/"+job.ID+"/periodic/force", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}

		// Check the response
		r := obj.(structs.PeriodicForceResponse)
		if r.EvalID == "" {
			t.Fatalf("bad: %#v", r)
		}
	})
}

func TestHTTP_JobPlan(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := MockJob()
		args := api.JobPlanRequest{
			Job:  job,
			Diff: true,
			WriteRequest: api.WriteRequest{
				Region:    "global",
				Namespace: api.DefaultNamespace,
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/job/"+*job.ID+"/plan", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		plan := obj.(structs.JobPlanResponse)
		if plan.Annotations == nil {
			t.Fatalf("bad: %v", plan)
		}

		if plan.Diff == nil {
			t.Fatalf("bad: %v", plan)
		}
	})
}

func TestHTTP_JobPlanRegion(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		Name           string
		ConfigRegion   string
		APIRegion      string
		ExpectedRegion string
	}{
		{
			Name:           "api region takes precedence",
			ConfigRegion:   "not-global",
			APIRegion:      "north-america",
			ExpectedRegion: "north-america",
		},
		{
			Name:           "config region is set",
			ConfigRegion:   "north-america",
			APIRegion:      "",
			ExpectedRegion: "north-america",
		},
		{
			Name:           "api region is set",
			ConfigRegion:   "",
			APIRegion:      "north-america",
			ExpectedRegion: "north-america",
		},
		{
			Name:           "falls back to default if no region is provided",
			ConfigRegion:   "",
			APIRegion:      "",
			ExpectedRegion: "global",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			httpTest(t, func(c *Config) { c.Region = tc.ExpectedRegion }, func(s *TestAgent) {
				// Create the job
				job := MockRegionalJob()

				if tc.ConfigRegion == "" {
					job.Region = nil
				} else {
					job.Region = &tc.ConfigRegion
				}

				args := api.JobPlanRequest{
					Job:  job,
					Diff: true,
					WriteRequest: api.WriteRequest{
						Region:    tc.APIRegion,
						Namespace: api.DefaultNamespace,
					},
				}
				buf := encodeReq(args)

				// Make the HTTP request
				req, err := http.NewRequest(http.MethodPut, "/v1/job/"+*job.ID+"/plan", buf)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.JobSpecificRequest(respW, req)
				require.NoError(t, err)

				// Check the response
				plan := obj.(structs.JobPlanResponse)
				require.NotNil(t, plan.Annotations)
				require.NotNil(t, plan.Diff)
			})
		})
	}
}

func TestHTTP_JobDispatch(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the parameterized job
		job := mock.BatchJob()
		job.ParameterizedJob = &structs.ParameterizedJobConfig{}

		args := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the request
		respW := httptest.NewRecorder()
		args2 := structs.JobDispatchRequest{
			WriteRequest: structs.WriteRequest{
				Region:           "global",
				Namespace:        structs.DefaultNamespace,
				IdempotencyToken: "foo",
			},
		}
		buf := encodeReq(args2)

		// Make the HTTP request
		req2, err := http.NewRequest(http.MethodPut, "/v1/job/"+job.ID+"/dispatch", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW.Flush()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req2)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		dispatch := obj.(structs.JobDispatchResponse)
		if dispatch.EvalID == "" {
			t.Fatalf("bad: %v", dispatch)
		}

		if dispatch.DispatchedJobID == "" {
			t.Fatalf("bad: %v", dispatch)
		}
	})
}

func TestHTTP_JobRevert(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job and register it twice
		job := mock.Job()
		regReq := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var regResp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &regReq, &regResp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Change the job to get a new version
		job.Datacenters = append(job.Datacenters, "foo")
		if err := s.Agent.RPC("Job.Register", &regReq, &regResp); err != nil {
			t.Fatalf("err: %v", err)
		}

		args := structs.JobRevertRequest{
			JobID:      job.ID,
			JobVersion: 0,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/job/"+job.ID+"/revert", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		revertResp := obj.(structs.JobRegisterResponse)
		if revertResp.EvalID == "" {
			t.Fatalf("bad: %v", revertResp)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
	})
}

func TestHTTP_JobStable(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job and register it twice
		job := mock.Job()
		regReq := structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		var regResp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &regReq, &regResp); err != nil {
			t.Fatalf("err: %v", err)
		}

		if err := s.Agent.RPC("Job.Register", &regReq, &regResp); err != nil {
			t.Fatalf("err: %v", err)
		}

		args := structs.JobStabilityRequest{
			JobID:      job.ID,
			JobVersion: 0,
			Stable:     true,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/job/"+job.ID+"/stable", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check the response
		stableResp := obj.(structs.JobStabilityResponse)
		if stableResp.Index == 0 {
			t.Fatalf("bad: %v", stableResp)
		}

		// Check for the index
		if respW.Result().Header.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
	})
}

func TestJobs_ParsingWriteRequest(t *testing.T) {
	ci.Parallel(t)

	// defaults
	agentRegion := "agentRegion"

	cases := []struct {
		name                  string
		jobRegion             string
		multiregion           *api.Multiregion
		queryRegion           string
		queryNamespace        string
		queryToken            string
		apiRegion             string
		apiNamespace          string
		apiToken              string
		expectedRequestRegion string
		expectedJobRegion     string
		expectedToken         string
		expectedNamespace     string
	}{
		{
			name:                  "no region provided at all",
			jobRegion:             "",
			multiregion:           nil,
			queryRegion:           "",
			expectedRequestRegion: agentRegion,
			expectedJobRegion:     agentRegion,
			expectedToken:         "",
			expectedNamespace:     "default",
		},
		{
			name:                  "no region provided but multiregion safe",
			jobRegion:             "",
			multiregion:           &api.Multiregion{},
			queryRegion:           "",
			expectedRequestRegion: agentRegion,
			expectedJobRegion:     api.GlobalRegion,
			expectedToken:         "",
			expectedNamespace:     "default",
		},
		{
			name:                  "region flag provided",
			jobRegion:             "",
			multiregion:           nil,
			queryRegion:           "west",
			expectedRequestRegion: "west",
			expectedJobRegion:     "west",
			expectedToken:         "",
			expectedNamespace:     "default",
		},
		{
			name:                  "job region provided",
			jobRegion:             "west",
			multiregion:           nil,
			queryRegion:           "",
			expectedRequestRegion: "west",
			expectedJobRegion:     "west",
			expectedToken:         "",
			expectedNamespace:     "default",
		},
		{
			name:                  "job region overridden by region flag",
			jobRegion:             "west",
			multiregion:           nil,
			queryRegion:           "east",
			expectedRequestRegion: "east",
			expectedJobRegion:     "east",
			expectedToken:         "",
			expectedNamespace:     "default",
		},
		{
			name:      "multiregion to valid region",
			jobRegion: "",
			multiregion: &api.Multiregion{Regions: []*api.MultiregionRegion{
				{Name: "west"},
				{Name: "east"},
			}},
			queryRegion:           "east",
			expectedRequestRegion: "east",
			expectedJobRegion:     api.GlobalRegion,
			expectedToken:         "",
			expectedNamespace:     "default",
		},
		{
			name:      "multiregion sent to wrong region",
			jobRegion: "",
			multiregion: &api.Multiregion{Regions: []*api.MultiregionRegion{
				{Name: "west"},
				{Name: "east"},
			}},
			queryRegion:           "north",
			expectedRequestRegion: "west",
			expectedJobRegion:     api.GlobalRegion,
			expectedToken:         "",
			expectedNamespace:     "default",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			// we need a valid agent config but we don't want to start up
			// a real server for this
			srv := &HTTPServer{}
			srv.agent = &Agent{config: &Config{Region: agentRegion}}

			job := &api.Job{
				Region:      pointer.Of(tc.jobRegion),
				Multiregion: tc.multiregion,
			}

			req, _ := http.NewRequest(http.MethodPost, "/", nil)
			if tc.queryToken != "" {
				req.Header.Set("X-Nomad-Token", tc.queryToken)
			}
			q := req.URL.Query()
			if tc.queryNamespace != "" {
				q.Add("namespace", tc.queryNamespace)
			}
			if tc.queryRegion != "" {
				q.Add("region", tc.queryRegion)
			}
			req.URL.RawQuery = q.Encode()

			apiReq := api.WriteRequest{
				Region:    tc.apiRegion,
				Namespace: tc.apiNamespace,
				SecretID:  tc.apiToken,
			}

			sJob, sWriteReq := srv.apiJobAndRequestToStructs(job, req, apiReq)
			must.Eq(t, tc.expectedJobRegion, sJob.Region)
			must.Eq(t, tc.expectedNamespace, sJob.Namespace)
			must.Eq(t, tc.expectedNamespace, sWriteReq.Namespace)
			must.Eq(t, tc.expectedRequestRegion, sWriteReq.Region)
			must.Eq(t, tc.expectedToken, sWriteReq.AuthToken)
		})
	}
}

func TestJobs_RegionForJob(t *testing.T) {
	ci.Parallel(t)

	// defaults
	agentRegion := "agentRegion"

	cases := []struct {
		name                  string
		jobRegion             string
		multiregion           *api.Multiregion
		queryRegion           string
		apiRegion             string
		agentRegion           string
		expectedRequestRegion string
		expectedJobRegion     string
	}{
		{
			name:                  "no region provided",
			jobRegion:             "",
			multiregion:           nil,
			queryRegion:           "",
			expectedRequestRegion: agentRegion,
			expectedJobRegion:     agentRegion,
		},
		{
			name:                  "no region provided but multiregion safe",
			jobRegion:             "",
			multiregion:           &api.Multiregion{},
			queryRegion:           "",
			expectedRequestRegion: agentRegion,
			expectedJobRegion:     api.GlobalRegion,
		},
		{
			name:                  "region flag provided",
			jobRegion:             "",
			multiregion:           nil,
			queryRegion:           "west",
			expectedRequestRegion: "west",
			expectedJobRegion:     "west",
		},
		{
			name:                  "job region provided",
			jobRegion:             "west",
			multiregion:           nil,
			queryRegion:           "",
			expectedRequestRegion: "west",
			expectedJobRegion:     "west",
		},
		{
			name:                  "job region overridden by region flag",
			jobRegion:             "west",
			multiregion:           nil,
			queryRegion:           "east",
			expectedRequestRegion: "east",
			expectedJobRegion:     "east",
		},
		{
			name:                  "job region overridden by api body",
			jobRegion:             "west",
			multiregion:           nil,
			apiRegion:             "east",
			expectedRequestRegion: "east",
			expectedJobRegion:     "east",
		},
		{
			name:      "multiregion to valid region",
			jobRegion: "",
			multiregion: &api.Multiregion{Regions: []*api.MultiregionRegion{
				{Name: "west"},
				{Name: "east"},
			}},
			queryRegion:           "east",
			expectedRequestRegion: "east",
			expectedJobRegion:     api.GlobalRegion,
		},
		{
			name:      "multiregion sent to wrong region",
			jobRegion: "",
			multiregion: &api.Multiregion{Regions: []*api.MultiregionRegion{
				{Name: "west"},
				{Name: "east"},
			}},
			queryRegion:           "north",
			expectedRequestRegion: "west",
			expectedJobRegion:     api.GlobalRegion,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := &api.Job{
				Region:      pointer.Of(tc.jobRegion),
				Multiregion: tc.multiregion,
			}
			requestRegion, jobRegion := regionForJob(
				job, tc.queryRegion, tc.apiRegion, agentRegion)
			require.Equal(t, tc.expectedRequestRegion, requestRegion)
			require.Equal(t, tc.expectedJobRegion, jobRegion)
		})
	}
}

func TestJobs_NamespaceForJob(t *testing.T) {
	ci.Parallel(t)

	// test namespace for pointer inputs
	ns := "dev"

	cases := []struct {
		name           string
		job            *api.Job
		queryNamespace string
		apiNamespace   string
		expected       string
	}{
		{
			name:     "no namespace provided",
			job:      &api.Job{},
			expected: structs.DefaultNamespace,
		},

		{
			name:     "jobspec has namespace",
			job:      &api.Job{Namespace: &ns},
			expected: "dev",
		},

		{
			name:           "-namespace flag overrides empty job namespace",
			job:            &api.Job{},
			queryNamespace: "prod",
			expected:       "prod",
		},

		{
			name:           "-namespace flag overrides job namespace",
			job:            &api.Job{Namespace: &ns},
			queryNamespace: "prod",
			expected:       "prod",
		},

		{
			name:           "-namespace flag overrides job namespace even if default",
			job:            &api.Job{Namespace: &ns},
			queryNamespace: structs.DefaultNamespace,
			expected:       structs.DefaultNamespace,
		},

		{
			name:         "API param overrides empty job namespace",
			job:          &api.Job{},
			apiNamespace: "prod",
			expected:     "prod",
		},

		{
			name:           "-namespace flag overrides API param",
			job:            &api.Job{Namespace: &ns},
			queryNamespace: "prod",
			apiNamespace:   "whatever",
			expected:       "prod",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected,
				namespaceForJob(tc.job.Namespace, tc.queryNamespace, tc.apiNamespace),
			)
		})
	}
}

func TestHTTPServer_jobServiceRegistrations(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		testFn func(srv *TestAgent)
		name   string
	}{
		{
			testFn: func(s *TestAgent) {

				// Grab the state, so we can manipulate it and test against it.
				testState := s.Agent.server.State()

				// Generate a job and upsert this.
				job := mock.Job()
				require.NoError(t, testState.UpsertJob(structs.MsgTypeTestSetup, 10, nil, job))

				// Generate a service registration, assigned the jobID to the
				// mocked jobID, and upsert this.
				serviceReg := mock.ServiceRegistrations()[0]
				serviceReg.JobID = job.ID
				require.NoError(t, testState.UpsertServiceRegistrations(
					structs.MsgTypeTestSetup, 20, []*structs.ServiceRegistration{serviceReg}))

				// Build the HTTP request.
				path := fmt.Sprintf("/v1/job/%s/services", job.ID)
				req, err := http.NewRequest(http.MethodGet, path, nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := s.Server.JobSpecificRequest(respW, req)
				require.NoError(t, err)

				// Check the response.
				require.Equal(t, "20", respW.Header().Get("X-Nomad-Index"))
				require.ElementsMatch(t, []*structs.ServiceRegistration{serviceReg},
					obj.([]*structs.ServiceRegistration))
			},
			name: "job has registrations",
		},
		{
			testFn: func(s *TestAgent) {

				// Grab the state, so we can manipulate it and test against it.
				testState := s.Agent.server.State()

				// Generate a job and upsert this.
				job := mock.Job()
				require.NoError(t, testState.UpsertJob(structs.MsgTypeTestSetup, 10, nil, job))

				// Build the HTTP request.
				path := fmt.Sprintf("/v1/job/%s/services", job.ID)
				req, err := http.NewRequest(http.MethodGet, path, nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := s.Server.JobSpecificRequest(respW, req)
				require.NoError(t, err)

				// Check the response.
				require.Equal(t, "1", respW.Header().Get("X-Nomad-Index"))
				require.ElementsMatch(t, []*structs.ServiceRegistration{}, obj.([]*structs.ServiceRegistration))
			},
			name: "job without registrations",
		},
		{
			testFn: func(s *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodGet, "/v1/job/example/services", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := s.Server.JobSpecificRequest(respW, req)
				require.Error(t, err)
				require.Contains(t, err.Error(), "job not found")
				require.Nil(t, obj)
			},
			name: "job not found",
		},
		{
			testFn: func(s *TestAgent) {

				// Build the HTTP request.
				req, err := http.NewRequest(http.MethodHead, "/v1/job/example/services", nil)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Send the HTTP request.
				obj, err := s.Server.JobSpecificRequest(respW, req)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Invalid method")
				require.Nil(t, obj)
			},
			name: "incorrect method",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpTest(t, nil, tc.testFn)
		})
	}
}

func TestJobs_ApiJobToStructsJob(t *testing.T) {
	ci.Parallel(t)

	apiJob := &api.Job{
		Stop:        pointer.Of(true),
		Region:      pointer.Of("global"),
		Namespace:   pointer.Of("foo"),
		ID:          pointer.Of("foo"),
		ParentID:    pointer.Of("lol"),
		Name:        pointer.Of("name"),
		Type:        pointer.Of("service"),
		Priority:    pointer.Of(50),
		AllAtOnce:   pointer.Of(true),
		Datacenters: []string{"dc1", "dc2"},
		Constraints: []*api.Constraint{
			{
				LTarget: "a",
				RTarget: "b",
				Operand: "c",
			},
		},
		Affinities: []*api.Affinity{
			{
				LTarget: "a",
				RTarget: "b",
				Operand: "c",
				Weight:  pointer.Of(int8(50)),
			},
		},
		Update: &api.UpdateStrategy{
			Stagger:          pointer.Of(1 * time.Second),
			MaxParallel:      pointer.Of(5),
			HealthCheck:      pointer.Of(structs.UpdateStrategyHealthCheck_Manual),
			MinHealthyTime:   pointer.Of(1 * time.Minute),
			HealthyDeadline:  pointer.Of(3 * time.Minute),
			ProgressDeadline: pointer.Of(3 * time.Minute),
			AutoRevert:       pointer.Of(false),
			Canary:           pointer.Of(1),
		},
		Spreads: []*api.Spread{
			{
				Attribute: "${meta.rack}",
				Weight:    pointer.Of(int8(100)),
				SpreadTarget: []*api.SpreadTarget{
					{
						Value:   "r1",
						Percent: 50,
					},
				},
			},
		},
		Periodic: &api.PeriodicConfig{
			Enabled:         pointer.Of(true),
			Spec:            pointer.Of("spec"),
			Specs:           []string{"spec"},
			SpecType:        pointer.Of("cron"),
			ProhibitOverlap: pointer.Of(true),
			TimeZone:        pointer.Of("test zone"),
		},
		ParameterizedJob: &api.ParameterizedJobConfig{
			Payload:      "payload",
			MetaRequired: []string{"a", "b"},
			MetaOptional: []string{"c", "d"},
		},
		Payload: []byte("payload"),
		Meta: map[string]string{
			"foo": "bar",
		},
		Multiregion: &api.Multiregion{
			Strategy: &api.MultiregionStrategy{
				MaxParallel: pointer.Of(2),
				OnFailure:   pointer.Of("fail_all"),
			},
			Regions: []*api.MultiregionRegion{
				{
					Name:        "west",
					Count:       pointer.Of(1),
					Datacenters: []string{"dc1", "dc2"},
					Meta:        map[string]string{"region_code": "W"},
				},
			},
		},
		TaskGroups: []*api.TaskGroup{
			{
				Name:  pointer.Of("group1"),
				Count: pointer.Of(5),
				Constraints: []*api.Constraint{
					{
						LTarget: "x",
						RTarget: "y",
						Operand: "z",
					},
				},
				Affinities: []*api.Affinity{
					{
						LTarget: "x",
						RTarget: "y",
						Operand: "z",
						Weight:  pointer.Of(int8(100)),
					},
				},
				RestartPolicy: &api.RestartPolicy{
					Interval:        pointer.Of(1 * time.Second),
					Attempts:        pointer.Of(5),
					Delay:           pointer.Of(10 * time.Second),
					Mode:            pointer.Of("delay"),
					RenderTemplates: pointer.Of(false),
				},
				ReschedulePolicy: &api.ReschedulePolicy{
					Interval:      pointer.Of(12 * time.Hour),
					Attempts:      pointer.Of(5),
					DelayFunction: pointer.Of("constant"),
					Delay:         pointer.Of(30 * time.Second),
					Unlimited:     pointer.Of(true),
					MaxDelay:      pointer.Of(20 * time.Minute),
				},
				Migrate: &api.MigrateStrategy{
					MaxParallel:     pointer.Of(12),
					HealthCheck:     pointer.Of("task_events"),
					MinHealthyTime:  pointer.Of(12 * time.Hour),
					HealthyDeadline: pointer.Of(12 * time.Hour),
				},
				Spreads: []*api.Spread{
					{
						Attribute: "${node.datacenter}",
						Weight:    pointer.Of(int8(100)),
						SpreadTarget: []*api.SpreadTarget{
							{
								Value:   "dc1",
								Percent: 100,
							},
						},
					},
				},
				EphemeralDisk: &api.EphemeralDisk{
					SizeMB:  pointer.Of(100),
					Sticky:  pointer.Of(true),
					Migrate: pointer.Of(true),
				},
				Update: &api.UpdateStrategy{
					HealthCheck:      pointer.Of(structs.UpdateStrategyHealthCheck_Checks),
					MinHealthyTime:   pointer.Of(2 * time.Minute),
					HealthyDeadline:  pointer.Of(5 * time.Minute),
					ProgressDeadline: pointer.Of(5 * time.Minute),
					AutoRevert:       pointer.Of(true),
				},
				Meta: map[string]string{
					"key": "value",
				},
				Consul: &api.Consul{
					Namespace: "team-foo",
				},
				Services: []*api.Service{
					{
						Name:              "groupserviceA",
						Tags:              []string{"a", "b"},
						CanaryTags:        []string{"d", "e"},
						EnableTagOverride: true,
						PortLabel:         "1234",
						Address:           "group.example.com",
						Meta: map[string]string{
							"servicemeta": "foobar",
						},
						TaggedAddresses: map[string]string{
							"wan": "1.2.3.4",
						},
						CheckRestart: &api.CheckRestart{
							Limit: 4,
							Grace: pointer.Of(11 * time.Second),
						},
						Checks: []api.ServiceCheck{
							{
								Name:          "bar",
								Type:          "http",
								Command:       "foo",
								Args:          []string{"a", "b"},
								Path:          "/check",
								Protocol:      "http",
								Method:        http.MethodPost,
								Body:          "{\"check\":\"mem\"}",
								PortLabel:     "foo",
								AddressMode:   "driver",
								GRPCService:   "foo.Bar",
								GRPCUseTLS:    true,
								Interval:      4 * time.Second,
								Timeout:       2 * time.Second,
								InitialStatus: "ok",
								CheckRestart: &api.CheckRestart{
									Limit:          3,
									IgnoreWarnings: true,
								},
								TaskName:               "task1",
								SuccessBeforePassing:   2,
								FailuresBeforeCritical: 3,
							},
						},
						Connect: &api.ConsulConnect{
							Native: false,
							SidecarService: &api.ConsulSidecarService{
								Tags:                   []string{"f", "g"},
								Port:                   "9000",
								DisableDefaultTCPCheck: true,
								Meta: map[string]string{
									"test-key": "test-value",
								},
							},
						},
					},
				},
				MaxClientDisconnect: pointer.Of(30 * time.Second),
				Tasks: []*api.Task{
					{
						Name:   "task1",
						Leader: true,
						Driver: "docker",
						User:   "mary",
						Config: map[string]interface{}{
							"lol": "code",
						},
						Env: map[string]string{
							"hello": "world",
						},
						Constraints: []*api.Constraint{
							{
								LTarget: "x",
								RTarget: "y",
								Operand: "z",
							},
						},
						Affinities: []*api.Affinity{
							{
								LTarget: "a",
								RTarget: "b",
								Operand: "c",
								Weight:  pointer.Of(int8(50)),
							},
						},
						VolumeMounts: []*api.VolumeMount{
							{
								Volume:          pointer.Of("vol"),
								Destination:     pointer.Of("dest"),
								ReadOnly:        pointer.Of(false),
								PropagationMode: pointer.Of("a"),
							},
						},
						RestartPolicy: &api.RestartPolicy{
							Interval:        pointer.Of(2 * time.Second),
							Attempts:        pointer.Of(10),
							Delay:           pointer.Of(20 * time.Second),
							Mode:            pointer.Of("delay"),
							RenderTemplates: pointer.Of(false),
						},
						Services: []*api.Service{
							{
								Name:              "serviceA",
								Tags:              []string{"1", "2"},
								CanaryTags:        []string{"3", "4"},
								EnableTagOverride: true,
								PortLabel:         "foo",
								Address:           "task.example.com",
								Meta: map[string]string{
									"servicemeta": "foobar",
								},
								CheckRestart: &api.CheckRestart{
									Limit: 4,
									Grace: pointer.Of(11 * time.Second),
								},
								Checks: []api.ServiceCheck{
									{
										Name:                   "bar",
										Type:                   "http",
										Command:                "foo",
										Args:                   []string{"a", "b"},
										Path:                   "/check",
										Protocol:               "http",
										PortLabel:              "foo",
										AddressMode:            "driver",
										GRPCService:            "foo.Bar",
										GRPCUseTLS:             true,
										Interval:               4 * time.Second,
										Timeout:                2 * time.Second,
										InitialStatus:          "ok",
										SuccessBeforePassing:   3,
										FailuresBeforeCritical: 4,
										CheckRestart: &api.CheckRestart{
											Limit:          3,
											IgnoreWarnings: true,
										},
									},
									{
										Name:      "check2",
										Type:      "tcp",
										PortLabel: "foo",
										Interval:  4 * time.Second,
										Timeout:   2 * time.Second,
									},
								},
							},
						},
						Resources: &api.Resources{
							CPU:      pointer.Of(100),
							MemoryMB: pointer.Of(10),
							Networks: []*api.NetworkResource{
								{
									IP:       "10.10.11.1",
									MBits:    pointer.Of(10),
									Hostname: "foobar",
									ReservedPorts: []api.Port{
										{
											Label: "http",
											Value: 80,
										},
									},
									DynamicPorts: []api.Port{
										{
											Label: "ssh",
											Value: 2000,
										},
									},
								},
							},
							Devices: []*api.RequestedDevice{
								{
									Name:  "nvidia/gpu",
									Count: pointer.Of(uint64(4)),
									Constraints: []*api.Constraint{
										{
											LTarget: "x",
											RTarget: "y",
											Operand: "z",
										},
									},
									Affinities: []*api.Affinity{
										{
											LTarget: "a",
											RTarget: "b",
											Operand: "c",
											Weight:  pointer.Of(int8(50)),
										},
									},
								},
								{
									Name:  "gpu",
									Count: nil,
								},
							},
						},
						Meta: map[string]string{
							"lol": "code",
						},
						KillTimeout: pointer.Of(10 * time.Second),
						KillSignal:  "SIGQUIT",
						LogConfig: &api.LogConfig{
							Disabled:      pointer.Of(true),
							MaxFiles:      pointer.Of(10),
							MaxFileSizeMB: pointer.Of(100),
						},
						Artifacts: []*api.TaskArtifact{
							{
								GetterSource: pointer.Of("source"),
								GetterOptions: map[string]string{
									"a": "b",
								},
								GetterMode:   pointer.Of("dir"),
								RelativeDest: pointer.Of("dest"),
							},
						},
						Vault: &api.Vault{
							Role:         "nomad-task",
							Namespace:    pointer.Of("ns1"),
							Policies:     []string{"a", "b", "c"},
							Env:          pointer.Of(true),
							DisableFile:  pointer.Of(false),
							ChangeMode:   pointer.Of("c"),
							ChangeSignal: pointer.Of("sighup"),
						},
						Templates: []*api.Template{
							{
								SourcePath:   pointer.Of("source"),
								DestPath:     pointer.Of("dest"),
								EmbeddedTmpl: pointer.Of("embedded"),
								ChangeMode:   pointer.Of("change"),
								ChangeSignal: pointer.Of("signal"),
								ChangeScript: &api.ChangeScript{
									Command:     pointer.Of("/bin/foo"),
									Args:        []string{"-h"},
									Timeout:     pointer.Of(5 * time.Second),
									FailOnError: pointer.Of(false),
								},
								Splay:      pointer.Of(1 * time.Minute),
								Perms:      pointer.Of("666"),
								Uid:        pointer.Of(1000),
								Gid:        pointer.Of(1000),
								LeftDelim:  pointer.Of("abc"),
								RightDelim: pointer.Of("def"),
								Envvars:    pointer.Of(true),
								Wait: &api.WaitConfig{
									Min: pointer.Of(5 * time.Second),
									Max: pointer.Of(10 * time.Second),
								},
								ErrMissingKey: pointer.Of(true),
							},
						},
						DispatchPayload: &api.DispatchPayloadConfig{
							File: "fileA",
						},
					},
				},
			},
		},
		ConsulToken:       pointer.Of("abc123"),
		VaultToken:        pointer.Of("def456"),
		VaultNamespace:    pointer.Of("ghi789"),
		Status:            pointer.Of("status"),
		StatusDescription: pointer.Of("status_desc"),
		Version:           pointer.Of(uint64(10)),
		CreateIndex:       pointer.Of(uint64(1)),
		ModifyIndex:       pointer.Of(uint64(3)),
		JobModifyIndex:    pointer.Of(uint64(5)),
	}

	expected := &structs.Job{
		Stop:           true,
		Region:         "global",
		Namespace:      "foo",
		VaultNamespace: "ghi789",
		ID:             "foo",
		Name:           "name",
		Type:           "service",
		Priority:       50,
		AllAtOnce:      true,
		Datacenters:    []string{"dc1", "dc2"},
		NodePool:       "",
		Constraints: []*structs.Constraint{
			{
				LTarget: "a",
				RTarget: "b",
				Operand: "c",
			},
		},
		Affinities: []*structs.Affinity{
			{
				LTarget: "a",
				RTarget: "b",
				Operand: "c",
				Weight:  50,
			},
		},
		Spreads: []*structs.Spread{
			{
				Attribute: "${meta.rack}",
				Weight:    100,
				SpreadTarget: []*structs.SpreadTarget{
					{
						Value:   "r1",
						Percent: 50,
					},
				},
			},
		},
		Update: structs.UpdateStrategy{
			Stagger:     1 * time.Second,
			MaxParallel: 5,
		},
		Periodic: &structs.PeriodicConfig{
			Enabled:         true,
			Spec:            "spec",
			Specs:           []string{"spec"},
			SpecType:        "cron",
			ProhibitOverlap: true,
			TimeZone:        "test zone",
		},
		ParameterizedJob: &structs.ParameterizedJobConfig{
			Payload:      "payload",
			MetaRequired: []string{"a", "b"},
			MetaOptional: []string{"c", "d"},
		},
		Payload: []byte("payload"),
		Meta: map[string]string{
			"foo": "bar",
		},
		Multiregion: &structs.Multiregion{
			Strategy: &structs.MultiregionStrategy{
				MaxParallel: 2,
				OnFailure:   "fail_all",
			},
			Regions: []*structs.MultiregionRegion{
				{
					Name:        "west",
					Count:       1,
					Datacenters: []string{"dc1", "dc2"},
					Meta:        map[string]string{"region_code": "W"},
				},
			},
		},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:  "group1",
				Count: 5,
				Constraints: []*structs.Constraint{
					{
						LTarget: "x",
						RTarget: "y",
						Operand: "z",
					},
				},
				Affinities: []*structs.Affinity{
					{
						LTarget: "x",
						RTarget: "y",
						Operand: "z",
						Weight:  100,
					},
				},
				RestartPolicy: &structs.RestartPolicy{
					Interval:        1 * time.Second,
					Attempts:        5,
					Delay:           10 * time.Second,
					Mode:            "delay",
					RenderTemplates: false,
				},
				Spreads: []*structs.Spread{
					{
						Attribute: "${node.datacenter}",
						Weight:    100,
						SpreadTarget: []*structs.SpreadTarget{
							{
								Value:   "dc1",
								Percent: 100,
							},
						},
					},
				},
				ReschedulePolicy: &structs.ReschedulePolicy{
					Interval:      12 * time.Hour,
					Attempts:      5,
					DelayFunction: "constant",
					Delay:         30 * time.Second,
					Unlimited:     true,
					MaxDelay:      20 * time.Minute,
				},
				Migrate: &structs.MigrateStrategy{
					MaxParallel:     12,
					HealthCheck:     "task_events",
					MinHealthyTime:  12 * time.Hour,
					HealthyDeadline: 12 * time.Hour,
				},
				EphemeralDisk: &structs.EphemeralDisk{
					SizeMB:  100,
					Sticky:  true,
					Migrate: true,
				},
				Update: &structs.UpdateStrategy{
					Stagger:          1 * time.Second,
					MaxParallel:      5,
					HealthCheck:      structs.UpdateStrategyHealthCheck_Checks,
					MinHealthyTime:   2 * time.Minute,
					HealthyDeadline:  5 * time.Minute,
					ProgressDeadline: 5 * time.Minute,
					AutoRevert:       true,
					AutoPromote:      false,
					Canary:           1,
				},
				Meta: map[string]string{
					"key": "value",
				},
				Consul: &structs.Consul{
					Namespace: "team-foo",
				},
				Services: []*structs.Service{
					{
						Name:              "groupserviceA",
						Provider:          "consul",
						Tags:              []string{"a", "b"},
						CanaryTags:        []string{"d", "e"},
						EnableTagOverride: true,
						PortLabel:         "1234",
						AddressMode:       "auto",
						Address:           "group.example.com",
						Meta: map[string]string{
							"servicemeta": "foobar",
						},
						TaggedAddresses: map[string]string{
							"wan": "1.2.3.4",
						},
						OnUpdate: structs.OnUpdateRequireHealthy,
						Checks: []*structs.ServiceCheck{
							{
								Name:          "bar",
								Type:          "http",
								Command:       "foo",
								Args:          []string{"a", "b"},
								Path:          "/check",
								Protocol:      "http",
								Method:        http.MethodPost,
								Body:          "{\"check\":\"mem\"}",
								PortLabel:     "foo",
								AddressMode:   "driver",
								GRPCService:   "foo.Bar",
								GRPCUseTLS:    true,
								Interval:      4 * time.Second,
								Timeout:       2 * time.Second,
								InitialStatus: "ok",
								CheckRestart: &structs.CheckRestart{
									Grace:          11 * time.Second,
									Limit:          3,
									IgnoreWarnings: true,
								},
								TaskName:               "task1",
								OnUpdate:               structs.OnUpdateRequireHealthy,
								SuccessBeforePassing:   2,
								FailuresBeforeCritical: 3,
							},
						},
						Connect: &structs.ConsulConnect{
							Native: false,
							SidecarService: &structs.ConsulSidecarService{
								Tags:                   []string{"f", "g"},
								Port:                   "9000",
								DisableDefaultTCPCheck: true,
								Meta: map[string]string{
									"test-key": "test-value",
								},
							},
						},
					},
				},
				MaxClientDisconnect: pointer.Of(30 * time.Second),
				Tasks: []*structs.Task{
					{
						Name:   "task1",
						Driver: "docker",
						Leader: true,
						User:   "mary",
						Config: map[string]interface{}{
							"lol": "code",
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: "x",
								RTarget: "y",
								Operand: "z",
							},
						},
						Affinities: []*structs.Affinity{
							{
								LTarget: "a",
								RTarget: "b",
								Operand: "c",
								Weight:  50,
							},
						},
						Env: map[string]string{
							"hello": "world",
						},
						VolumeMounts: []*structs.VolumeMount{
							{
								Volume:          "vol",
								Destination:     "dest",
								ReadOnly:        false,
								PropagationMode: "a",
							},
						},
						RestartPolicy: &structs.RestartPolicy{
							Interval:        2 * time.Second,
							Attempts:        10,
							Delay:           20 * time.Second,
							Mode:            "delay",
							RenderTemplates: false,
						},
						Services: []*structs.Service{
							{
								Name:              "serviceA",
								Provider:          "consul",
								Tags:              []string{"1", "2"},
								CanaryTags:        []string{"3", "4"},
								EnableTagOverride: true,
								PortLabel:         "foo",
								AddressMode:       "auto",
								Address:           "task.example.com",
								Meta: map[string]string{
									"servicemeta": "foobar",
								},
								OnUpdate: structs.OnUpdateRequireHealthy,
								Checks: []*structs.ServiceCheck{
									{
										Name:                   "bar",
										Type:                   "http",
										Command:                "foo",
										Args:                   []string{"a", "b"},
										Path:                   "/check",
										Protocol:               "http",
										PortLabel:              "foo",
										AddressMode:            "driver",
										Interval:               4 * time.Second,
										Timeout:                2 * time.Second,
										InitialStatus:          "ok",
										GRPCService:            "foo.Bar",
										GRPCUseTLS:             true,
										SuccessBeforePassing:   3,
										FailuresBeforeCritical: 4,
										CheckRestart: &structs.CheckRestart{
											Limit:          3,
											Grace:          11 * time.Second,
											IgnoreWarnings: true,
										},
										OnUpdate: structs.OnUpdateRequireHealthy,
									},
									{
										Name:      "check2",
										Type:      "tcp",
										PortLabel: "foo",
										Interval:  4 * time.Second,
										Timeout:   2 * time.Second,
										CheckRestart: &structs.CheckRestart{
											Limit: 4,
											Grace: 11 * time.Second,
										},
										OnUpdate: structs.OnUpdateRequireHealthy,
									},
								},
							},
						},
						Resources: &structs.Resources{
							CPU:      100,
							MemoryMB: 10,
							Networks: []*structs.NetworkResource{
								{
									IP:       "10.10.11.1",
									MBits:    10,
									Hostname: "foobar",
									ReservedPorts: []structs.Port{
										{
											Label: "http",
											Value: 80,
										},
									},
									DynamicPorts: []structs.Port{
										{
											Label: "ssh",
											Value: 2000,
										},
									},
								},
							},
							Devices: []*structs.RequestedDevice{
								{
									Name:  "nvidia/gpu",
									Count: 4,
									Constraints: []*structs.Constraint{
										{
											LTarget: "x",
											RTarget: "y",
											Operand: "z",
										},
									},
									Affinities: []*structs.Affinity{
										{
											LTarget: "a",
											RTarget: "b",
											Operand: "c",
											Weight:  50,
										},
									},
								},
								{
									Name:  "gpu",
									Count: 1,
								},
							},
						},
						Meta: map[string]string{
							"lol": "code",
						},
						KillTimeout: 10 * time.Second,
						KillSignal:  "SIGQUIT",
						LogConfig: &structs.LogConfig{
							Disabled:      true,
							MaxFiles:      10,
							MaxFileSizeMB: 100,
						},
						Artifacts: []*structs.TaskArtifact{
							{
								GetterSource: "source",
								GetterOptions: map[string]string{
									"a": "b",
								},
								GetterMode:   "dir",
								RelativeDest: "dest",
							},
						},
						Vault: &structs.Vault{
							Role:         "nomad-task",
							Namespace:    "ns1",
							Policies:     []string{"a", "b", "c"},
							Env:          true,
							DisableFile:  false,
							ChangeMode:   "c",
							ChangeSignal: "sighup",
						},
						Templates: []*structs.Template{
							{
								SourcePath:   "source",
								DestPath:     "dest",
								EmbeddedTmpl: "embedded",
								ChangeMode:   "change",
								ChangeSignal: "SIGNAL",
								ChangeScript: &structs.ChangeScript{
									Command:     "/bin/foo",
									Args:        []string{"-h"},
									Timeout:     5 * time.Second,
									FailOnError: false,
								},
								Splay:      1 * time.Minute,
								Perms:      "666",
								Uid:        pointer.Of(1000),
								Gid:        pointer.Of(1000),
								LeftDelim:  "abc",
								RightDelim: "def",
								Envvars:    true,
								Wait: &structs.WaitConfig{
									Min: pointer.Of(5 * time.Second),
									Max: pointer.Of(10 * time.Second),
								},
								ErrMissingKey: true,
							},
						},
						DispatchPayload: &structs.DispatchPayloadConfig{
							File: "fileA",
						},
					},
				},
			},
		},

		ConsulToken: "abc123",
		VaultToken:  "def456",
	}

	structsJob := ApiJobToStructJob(apiJob)

	require.Equal(t, expected, structsJob)

	systemAPIJob := &api.Job{
		Stop:        pointer.Of(true),
		Region:      pointer.Of("global"),
		Namespace:   pointer.Of("foo"),
		ID:          pointer.Of("foo"),
		ParentID:    pointer.Of("lol"),
		Name:        pointer.Of("name"),
		Type:        pointer.Of("system"),
		Priority:    pointer.Of(50),
		AllAtOnce:   pointer.Of(true),
		Datacenters: []string{"dc1", "dc2"},
		NodePool:    pointer.Of("default"),
		Constraints: []*api.Constraint{
			{
				LTarget: "a",
				RTarget: "b",
				Operand: "c",
			},
		},
		TaskGroups: []*api.TaskGroup{
			{
				Name:  pointer.Of("group1"),
				Count: pointer.Of(5),
				Constraints: []*api.Constraint{
					{
						LTarget: "x",
						RTarget: "y",
						Operand: "z",
					},
				},
				RestartPolicy: &api.RestartPolicy{
					Interval:        pointer.Of(1 * time.Second),
					Attempts:        pointer.Of(5),
					Delay:           pointer.Of(10 * time.Second),
					Mode:            pointer.Of("delay"),
					RenderTemplates: pointer.Of(false),
				},
				EphemeralDisk: &api.EphemeralDisk{
					SizeMB:  pointer.Of(100),
					Sticky:  pointer.Of(true),
					Migrate: pointer.Of(true),
				},
				Meta: map[string]string{
					"key": "value",
				},
				Consul: &api.Consul{
					Namespace: "foo",
				},
				Tasks: []*api.Task{
					{
						Name:   "task1",
						Leader: true,
						Driver: "docker",
						User:   "mary",
						Config: map[string]interface{}{
							"lol": "code",
						},
						Env: map[string]string{
							"hello": "world",
						},
						Constraints: []*api.Constraint{
							{
								LTarget: "x",
								RTarget: "y",
								Operand: "z",
							},
						},
						Resources: &api.Resources{
							CPU:      pointer.Of(100),
							MemoryMB: pointer.Of(10),
							Networks: []*api.NetworkResource{
								{
									IP:    "10.10.11.1",
									MBits: pointer.Of(10),
									ReservedPorts: []api.Port{
										{
											Label: "http",
											Value: 80,
										},
									},
									DynamicPorts: []api.Port{
										{
											Label: "ssh",
											Value: 2000,
										},
									},
								},
							},
						},
						Meta: map[string]string{
							"lol": "code",
						},
						KillTimeout: pointer.Of(10 * time.Second),
						KillSignal:  "SIGQUIT",
						LogConfig: &api.LogConfig{
							Disabled:      pointer.Of(true),
							MaxFiles:      pointer.Of(10),
							MaxFileSizeMB: pointer.Of(100),
						},
						Artifacts: []*api.TaskArtifact{
							{
								GetterSource:  pointer.Of("source"),
								GetterOptions: map[string]string{"a": "b"},
								GetterHeaders: map[string]string{"User-Agent": "nomad"},
								GetterMode:    pointer.Of("dir"),
								RelativeDest:  pointer.Of("dest"),
							},
						},
						DispatchPayload: &api.DispatchPayloadConfig{
							File: "fileA",
						},
					},
				},
			},
		},
		Status:            pointer.Of("status"),
		StatusDescription: pointer.Of("status_desc"),
		Version:           pointer.Of(uint64(10)),
		CreateIndex:       pointer.Of(uint64(1)),
		ModifyIndex:       pointer.Of(uint64(3)),
		JobModifyIndex:    pointer.Of(uint64(5)),
	}

	expectedSystemJob := &structs.Job{
		Stop:        true,
		Region:      "global",
		Namespace:   "foo",
		ID:          "foo",
		Name:        "name",
		Type:        "system",
		Priority:    50,
		AllAtOnce:   true,
		Datacenters: []string{"dc1", "dc2"},
		NodePool:    "default",
		Constraints: []*structs.Constraint{
			{
				LTarget: "a",
				RTarget: "b",
				Operand: "c",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:  "group1",
				Count: 5,
				Constraints: []*structs.Constraint{
					{
						LTarget: "x",
						RTarget: "y",
						Operand: "z",
					},
				},
				RestartPolicy: &structs.RestartPolicy{
					Interval:        1 * time.Second,
					Attempts:        5,
					Delay:           10 * time.Second,
					Mode:            "delay",
					RenderTemplates: false,
				},
				EphemeralDisk: &structs.EphemeralDisk{
					SizeMB:  100,
					Sticky:  true,
					Migrate: true,
				},
				Meta: map[string]string{
					"key": "value",
				},
				Consul: &structs.Consul{
					Namespace: "foo",
				},
				Tasks: []*structs.Task{
					{
						Name:   "task1",
						Driver: "docker",
						Leader: true,
						User:   "mary",
						Config: map[string]interface{}{
							"lol": "code",
						},
						Constraints: []*structs.Constraint{
							{
								LTarget: "x",
								RTarget: "y",
								Operand: "z",
							},
						},
						Env: map[string]string{
							"hello": "world",
						},
						Resources: &structs.Resources{
							CPU:      100,
							MemoryMB: 10,
							Networks: []*structs.NetworkResource{
								{
									IP:    "10.10.11.1",
									MBits: 10,
									ReservedPorts: []structs.Port{
										{
											Label: "http",
											Value: 80,
										},
									},
									DynamicPorts: []structs.Port{
										{
											Label: "ssh",
											Value: 2000,
										},
									},
								},
							},
						},
						RestartPolicy: &structs.RestartPolicy{
							Interval:        1 * time.Second,
							Attempts:        5,
							Delay:           10 * time.Second,
							Mode:            "delay",
							RenderTemplates: false,
						},
						Meta: map[string]string{
							"lol": "code",
						},
						KillTimeout: 10 * time.Second,
						KillSignal:  "SIGQUIT",
						LogConfig: &structs.LogConfig{
							Disabled:      true,
							MaxFiles:      10,
							MaxFileSizeMB: 100,
						},
						Artifacts: []*structs.TaskArtifact{
							{
								GetterSource:  "source",
								GetterOptions: map[string]string{"a": "b"},
								GetterHeaders: map[string]string{"User-Agent": "nomad"},
								GetterMode:    "dir",
								RelativeDest:  "dest",
							},
						},
						DispatchPayload: &structs.DispatchPayloadConfig{
							File: "fileA",
						},
					},
				},
			},
		},
	}

	systemStructsJob := ApiJobToStructJob(systemAPIJob)
	require.Equal(t, expectedSystemJob, systemStructsJob)
}

func TestJobs_ApiJobToStructsJobUpdate(t *testing.T) {
	ci.Parallel(t)

	apiJob := &api.Job{
		Update: &api.UpdateStrategy{
			Stagger:          pointer.Of(1 * time.Second),
			MaxParallel:      pointer.Of(5),
			HealthCheck:      pointer.Of(structs.UpdateStrategyHealthCheck_Manual),
			MinHealthyTime:   pointer.Of(1 * time.Minute),
			HealthyDeadline:  pointer.Of(3 * time.Minute),
			ProgressDeadline: pointer.Of(3 * time.Minute),
			AutoRevert:       pointer.Of(false),
			AutoPromote:      nil,
			Canary:           pointer.Of(1),
		},
		TaskGroups: []*api.TaskGroup{
			{
				Update: &api.UpdateStrategy{
					Canary:     pointer.Of(2),
					AutoRevert: pointer.Of(true),
				},
			}, {
				Update: &api.UpdateStrategy{
					Canary:      pointer.Of(3),
					AutoPromote: pointer.Of(true),
				},
			},
		},
	}

	structsJob := ApiJobToStructJob(apiJob)

	// Update has been moved from job down to the groups
	jobUpdate := structs.UpdateStrategy{
		Stagger:          1000000000,
		MaxParallel:      5,
		HealthCheck:      "",
		MinHealthyTime:   0,
		HealthyDeadline:  0,
		ProgressDeadline: 0,
		AutoRevert:       false,
		AutoPromote:      false,
		Canary:           0,
	}

	// But the groups inherit settings from the job update
	group1 := structs.UpdateStrategy{
		Stagger:          1000000000,
		MaxParallel:      5,
		HealthCheck:      "manual",
		MinHealthyTime:   60000000000,
		HealthyDeadline:  180000000000,
		ProgressDeadline: 180000000000,
		AutoRevert:       true,
		AutoPromote:      false,
		Canary:           2,
	}

	group2 := structs.UpdateStrategy{
		Stagger:          1000000000,
		MaxParallel:      5,
		HealthCheck:      "manual",
		MinHealthyTime:   60000000000,
		HealthyDeadline:  180000000000,
		ProgressDeadline: 180000000000,
		AutoRevert:       false,
		AutoPromote:      true,
		Canary:           3,
	}

	require.Equal(t, jobUpdate, structsJob.Update)
	require.Equal(t, group1, *structsJob.TaskGroups[0].Update)
	require.Equal(t, group2, *structsJob.TaskGroups[1].Update)
}

// TestJobs_Matching_Resources asserts:
//
//	api.{Default,Min}Resources == structs.{Default,Min}Resources
//
// While this is an odd place to test that, this is where both are imported,
// validated, and converted.
func TestJobs_Matching_Resources(t *testing.T) {
	ci.Parallel(t)

	// api.MinResources == structs.MinResources
	structsMinRes := ApiResourcesToStructs(api.MinResources())
	assert.Equal(t, structs.MinResources(), structsMinRes)

	// api.DefaultResources == structs.DefaultResources
	structsDefaultRes := ApiResourcesToStructs(api.DefaultResources())
	assert.Equal(t, structs.DefaultResources(), structsDefaultRes)
}

// TestHTTP_JobValidate_SystemMigrate asserts that a system job with a migrate
// block fails to validate but does not panic (see #5477).
func TestHTTP_JobValidate_SystemMigrate(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := &api.Job{
			Region:      pointer.Of("global"),
			Datacenters: []string{"dc1"},
			ID:          pointer.Of("systemmigrate"),
			Name:        pointer.Of("systemmigrate"),
			TaskGroups: []*api.TaskGroup{
				{Name: pointer.Of("web")},
			},

			// System job...
			Type: pointer.Of("system"),

			// ...with an empty migrate block
			Migrate: &api.MigrateStrategy{},
		}

		args := api.JobValidateRequest{
			Job:          job,
			WriteRequest: api.WriteRequest{Region: "global"},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/validate/job", buf)
		must.NoError(t, err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.ValidateJobRequest(respW, req)
		must.NoError(t, err)

		// Check the response
		resp := obj.(structs.JobValidateResponse)
		must.StrContains(t, resp.Error, `Job type "system" does not allow migrate block`)
		must.Len(t, 1, resp.ValidationErrors)
	})
}

func TestConversion_dereferenceInt(t *testing.T) {
	ci.Parallel(t)
	require.Equal(t, 0, dereferenceInt(nil))
	require.Equal(t, 42, dereferenceInt(pointer.Of(42)))
}

func TestConversion_apiLogConfigToStructs(t *testing.T) {
	ci.Parallel(t)
	must.Nil(t, apiLogConfigToStructs(nil))
	must.Eq(t, &structs.LogConfig{
		Disabled:      true,
		MaxFiles:      2,
		MaxFileSizeMB: 8,
	}, apiLogConfigToStructs(&api.LogConfig{
		Disabled:      pointer.Of(true),
		MaxFiles:      pointer.Of(2),
		MaxFileSizeMB: pointer.Of(8),
	}))

	// COMPAT(1.6.0): verify backwards compatibility fixes
	// Note: we're intentionally ignoring the Enabled: false case
	must.Eq(t, &structs.LogConfig{Disabled: false},
		apiLogConfigToStructs(&api.LogConfig{
			Enabled: pointer.Of(false),
		}))
	must.Eq(t, &structs.LogConfig{Disabled: false},
		apiLogConfigToStructs(&api.LogConfig{
			Enabled: pointer.Of(true),
		}))
	must.Eq(t, &structs.LogConfig{Disabled: false},
		apiLogConfigToStructs(&api.LogConfig{}))
	must.Eq(t, &structs.LogConfig{Disabled: false},
		apiLogConfigToStructs(&api.LogConfig{
			Disabled: pointer.Of(false),
		}))
	must.Eq(t, &structs.LogConfig{Disabled: false},
		apiLogConfigToStructs(&api.LogConfig{
			Enabled:  pointer.Of(false),
			Disabled: pointer.Of(false),
		}))

}

func TestConversion_apiResourcesToStructs(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		input    *api.Resources
		expected *structs.Resources
	}{
		{
			"nil",
			nil,
			nil,
		},
		{
			"plain",
			&api.Resources{
				CPU:      pointer.Of(100),
				MemoryMB: pointer.Of(200),
			},
			&structs.Resources{
				CPU:      100,
				MemoryMB: 200,
			},
		},
		{
			"with memory max",
			&api.Resources{
				CPU:         pointer.Of(100),
				MemoryMB:    pointer.Of(200),
				MemoryMaxMB: pointer.Of(300),
			},
			&structs.Resources{
				CPU:         100,
				MemoryMB:    200,
				MemoryMaxMB: 300,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			found := ApiResourcesToStructs(c.input)
			require.Equal(t, c.expected, found)
		})
	}
}

func TestConversion_apiJobSubmissionToStructs(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		result := apiJobSubmissionToStructs(nil)
		must.Nil(t, result)
	})

	t.Run("not nil", func(t *testing.T) {
		result := apiJobSubmissionToStructs(&api.JobSubmission{
			Source:        "source",
			Format:        "hcl2",
			VariableFlags: map[string]string{"foo": "bar"},
			Variables:     "variable",
		})
		must.Eq(t, &structs.JobSubmission{
			Source:        "source",
			Format:        "hcl2",
			VariableFlags: map[string]string{"foo": "bar"},
			Variables:     "variable",
		}, result)
	})
}

func TestConversion_apiConnectSidecarTaskToStructs(t *testing.T) {
	ci.Parallel(t)
	require.Nil(t, apiConnectSidecarTaskToStructs(nil))
	delay := time.Duration(200)
	timeout := time.Duration(1000)
	config := make(map[string]interface{})
	env := make(map[string]string)
	meta := make(map[string]string)
	require.Equal(t, &structs.SidecarTask{
		Name:   "name",
		Driver: "driver",
		User:   "user",
		Config: config,
		Env:    env,
		Resources: &structs.Resources{
			CPU:      1,
			MemoryMB: 128,
		},
		Meta:        meta,
		KillTimeout: &timeout,
		LogConfig: &structs.LogConfig{
			Disabled:      true,
			MaxFiles:      2,
			MaxFileSizeMB: 8,
		},
		ShutdownDelay: &delay,
		KillSignal:    "SIGTERM",
	}, apiConnectSidecarTaskToStructs(&api.SidecarTask{
		Name:   "name",
		Driver: "driver",
		User:   "user",
		Config: config,
		Env:    env,
		Resources: &api.Resources{
			CPU:      pointer.Of(1),
			MemoryMB: pointer.Of(128),
		},
		Meta:        meta,
		KillTimeout: &timeout,
		LogConfig: &api.LogConfig{
			Disabled:      pointer.Of(true),
			MaxFiles:      pointer.Of(2),
			MaxFileSizeMB: pointer.Of(8),
		},
		ShutdownDelay: &delay,
		KillSignal:    "SIGTERM",
	}))
}

func TestConversion_apiConsulExposePathsToStructs(t *testing.T) {
	ci.Parallel(t)
	require.Nil(t, apiConsulExposePathsToStructs(nil))
	require.Nil(t, apiConsulExposePathsToStructs(make([]*api.ConsulExposePath, 0)))
	require.Equal(t, []structs.ConsulExposePath{{
		Path:          "/health",
		Protocol:      "http",
		LocalPathPort: 8080,
		ListenerPort:  "hcPort",
	}}, apiConsulExposePathsToStructs([]*api.ConsulExposePath{{
		Path:          "/health",
		Protocol:      "http",
		LocalPathPort: 8080,
		ListenerPort:  "hcPort",
	}}))
}

func TestConversion_apiConsulExposeConfigToStructs(t *testing.T) {
	ci.Parallel(t)
	require.Nil(t, apiConsulExposeConfigToStructs(nil))
	require.Equal(t, &structs.ConsulExposeConfig{
		Paths: []structs.ConsulExposePath{{Path: "/health"}},
	}, apiConsulExposeConfigToStructs(&api.ConsulExposeConfig{
		Paths: []*api.ConsulExposePath{{Path: "/health"}},
	}))
}

func TestConversion_apiUpstreamsToStructs(t *testing.T) {
	ci.Parallel(t)
	require.Nil(t, apiUpstreamsToStructs(nil))
	require.Nil(t, apiUpstreamsToStructs(make([]*api.ConsulUpstream, 0)))
	require.Equal(t, []structs.ConsulUpstream{{
		DestinationName:      "upstream",
		DestinationNamespace: "ns2",
		LocalBindPort:        8000,
		Datacenter:           "dc2",
		LocalBindAddress:     "127.0.0.2",
		MeshGateway:          structs.ConsulMeshGateway{Mode: "local"},
	}}, apiUpstreamsToStructs([]*api.ConsulUpstream{{
		DestinationName:      "upstream",
		DestinationNamespace: "ns2",
		LocalBindPort:        8000,
		Datacenter:           "dc2",
		LocalBindAddress:     "127.0.0.2",
		MeshGateway:          &api.ConsulMeshGateway{Mode: "local"},
	}}))
}

func TestConversion_apiConsulMeshGatewayToStructs(t *testing.T) {
	ci.Parallel(t)
	require.Equal(t, structs.ConsulMeshGateway{}, apiMeshGatewayToStructs(nil))
	require.Equal(t, structs.ConsulMeshGateway{Mode: "remote"},
		apiMeshGatewayToStructs(&api.ConsulMeshGateway{Mode: "remote"}))
}

func TestConversion_apiConnectSidecarServiceProxyToStructs(t *testing.T) {
	ci.Parallel(t)
	require.Nil(t, apiConnectSidecarServiceProxyToStructs(nil))
	config := make(map[string]interface{})
	require.Equal(t, &structs.ConsulProxy{
		LocalServiceAddress: "192.168.30.1",
		LocalServicePort:    9000,
		Config:              map[string]any{},
		Upstreams: []structs.ConsulUpstream{{
			DestinationName: "upstream",
		}},
		Expose: &structs.ConsulExposeConfig{
			Paths: []structs.ConsulExposePath{{Path: "/health"}},
		},
	}, apiConnectSidecarServiceProxyToStructs(&api.ConsulProxy{
		LocalServiceAddress: "192.168.30.1",
		LocalServicePort:    9000,
		Config:              config,
		Upstreams: []*api.ConsulUpstream{{
			DestinationName: "upstream",
		}},
		Expose: &api.ConsulExposeConfig{
			Paths: []*api.ConsulExposePath{{
				Path: "/health",
			}},
		},
	}))
}

func TestConversion_apiConnectSidecarServiceToStructs(t *testing.T) {
	ci.Parallel(t)
	require.Nil(t, apiConnectSidecarTaskToStructs(nil))
	require.Equal(t, &structs.ConsulSidecarService{
		Tags: []string{"foo"},
		Port: "myPort",
		Proxy: &structs.ConsulProxy{
			LocalServiceAddress: "192.168.30.1",
		},
		Meta: map[string]string{
			"test-key": "test-value",
		},
	}, apiConnectSidecarServiceToStructs(&api.ConsulSidecarService{
		Tags: []string{"foo"},
		Port: "myPort",
		Proxy: &api.ConsulProxy{
			LocalServiceAddress: "192.168.30.1",
		},
		Meta: map[string]string{
			"test-key": "test-value",
		},
	}))
}

func TestConversion_ApiConsulConnectToStructs(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		require.Nil(t, ApiConsulConnectToStructs(nil))
	})

	t.Run("sidecar", func(t *testing.T) {
		require.Equal(t, &structs.ConsulConnect{
			Native:         false,
			SidecarService: &structs.ConsulSidecarService{Port: "myPort"},
			SidecarTask:    &structs.SidecarTask{Name: "task"},
		}, ApiConsulConnectToStructs(&api.ConsulConnect{
			Native:         false,
			SidecarService: &api.ConsulSidecarService{Port: "myPort"},
			SidecarTask:    &api.SidecarTask{Name: "task"},
		}))
	})

	t.Run("gateway proxy", func(t *testing.T) {
		require.Equal(t, &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: &structs.ConsulGatewayProxy{
					ConnectTimeout:                  pointer.Of(3 * time.Second),
					EnvoyGatewayBindTaggedAddresses: true,
					EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
						"service": {
							Address: "10.0.0.1",
							Port:    9000,
						}},
					EnvoyGatewayNoDefaultBind: true,
					EnvoyDNSDiscoveryType:     "STRICT_DNS",
					Config: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		}, ApiConsulConnectToStructs(&api.ConsulConnect{
			Gateway: &api.ConsulGateway{
				Proxy: &api.ConsulGatewayProxy{
					ConnectTimeout:                  pointer.Of(3 * time.Second),
					EnvoyGatewayBindTaggedAddresses: true,
					EnvoyGatewayBindAddresses: map[string]*api.ConsulGatewayBindAddress{
						"service": {
							Address: "10.0.0.1",
							Port:    9000,
						},
					},
					EnvoyGatewayNoDefaultBind: true,
					EnvoyDNSDiscoveryType:     "STRICT_DNS",
					Config: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		}))
	})

	t.Run("gateway ingress", func(t *testing.T) {
		require.Equal(t, &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Ingress: &structs.ConsulIngressConfigEntry{
					TLS: &structs.ConsulGatewayTLSConfig{
						Enabled:       true,
						TLSMinVersion: "TLSv1_2",
						TLSMaxVersion: "TLSv1_3",
						CipherSuites:  []string{"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
					},
					Listeners: []*structs.ConsulIngressListener{{
						Port:     1111,
						Protocol: "http",
						Services: []*structs.ConsulIngressService{{
							Name:  "ingress1",
							Hosts: []string{"host1"},
						}},
					}},
				},
			},
		}, ApiConsulConnectToStructs(
			&api.ConsulConnect{
				Gateway: &api.ConsulGateway{
					Ingress: &api.ConsulIngressConfigEntry{
						TLS: &api.ConsulGatewayTLSConfig{
							Enabled:       true,
							TLSMinVersion: "TLSv1_2",
							TLSMaxVersion: "TLSv1_3",
							CipherSuites:  []string{"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
						},
						Listeners: []*api.ConsulIngressListener{{
							Port:     1111,
							Protocol: "http",
							Services: []*api.ConsulIngressService{{
								Name:  "ingress1",
								Hosts: []string{"host1"},
							}},
						}},
					},
				},
			},
		))
	})

	t.Run("gateway terminating", func(t *testing.T) {
		require.Equal(t, &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Terminating: &structs.ConsulTerminatingConfigEntry{
					Services: []*structs.ConsulLinkedService{{
						Name:     "linked-service",
						CAFile:   "ca.pem",
						CertFile: "cert.pem",
						KeyFile:  "key.pem",
						SNI:      "linked.consul",
					}},
				},
			},
		}, ApiConsulConnectToStructs(&api.ConsulConnect{
			Gateway: &api.ConsulGateway{
				Terminating: &api.ConsulTerminatingConfigEntry{
					Services: []*api.ConsulLinkedService{{
						Name:     "linked-service",
						CAFile:   "ca.pem",
						CertFile: "cert.pem",
						KeyFile:  "key.pem",
						SNI:      "linked.consul",
					}},
				},
			},
		}))
	})

	t.Run("gateway mesh", func(t *testing.T) {
		require.Equal(t, &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Mesh: &structs.ConsulMeshConfigEntry{
					// nothing
				},
			},
		}, ApiConsulConnectToStructs(&api.ConsulConnect{
			Gateway: &api.ConsulGateway{
				Mesh: &api.ConsulMeshConfigEntry{
					// nothing
				},
			},
		}))
	})

	t.Run("native", func(t *testing.T) {
		require.Equal(t, &structs.ConsulConnect{
			Native: true,
		}, ApiConsulConnectToStructs(&api.ConsulConnect{
			Native: true,
		}))
	})
}

func Test_apiWorkloadIdentityToStructs(t *testing.T) {
	ci.Parallel(t)
	must.Nil(t, apiWorkloadIdentityToStructs(nil))
	must.Eq(t, &structs.WorkloadIdentity{
		Name:        "consul/test",
		Audience:    []string{"consul.io"},
		Env:         false,
		File:        false,
		ServiceName: "web",
	}, apiWorkloadIdentityToStructs(&api.WorkloadIdentity{
		Name:        "consul/test",
		Audience:    []string{"consul.io"},
		Env:         false,
		File:        false,
		ServiceName: "web",
	}))
}
