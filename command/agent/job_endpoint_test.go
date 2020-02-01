package agent

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTP_JobsList(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("GET", "/v1/jobs", nil)
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
		j := obj.([]*structs.JobListStub)
		if len(j) != 3 {
			t.Fatalf("bad: %#v", j)
		}
	})
}

func TestHTTP_PrefixJobsList(t *testing.T) {
	ids := []string{
		"aaaaaaaa-e8f7-fd38-c855-ab94ceb89706",
		"aabbbbbb-e8f7-fd38-c855-ab94ceb89706",
		"aabbcccc-e8f7-fd38-c855-ab94ceb89706",
	}
	t.Parallel()
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
		req, err := http.NewRequest("GET", "/v1/jobs?prefix=aabb", nil)
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
		j := obj.([]*structs.JobListStub)
		if len(j) != 2 {
			t.Fatalf("bad: %#v", j)
		}
	})
}

func TestHTTP_JobsRegister(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := MockJob()
		args := api.JobRegisterRequest{
			Job:          job,
			WriteRequest: api.WriteRequest{Region: "global"},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/jobs", buf)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
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

// Test that ACL token is properly threaded through to the RPC endpoint
func TestHTTP_JobsRegister_ACL(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("PUT", "/v1/jobs", buf)
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
	t.Parallel()
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
		req, err := http.NewRequest("PUT", "/v1/jobs", buf)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
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
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		buf := encodeReq(api.JobsParseRequest{JobHCL: mock.HCL()})
		req, err := http.NewRequest("POST", "/v1/jobs/parse", buf)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		respW := httptest.NewRecorder()

		obj, err := s.Server.JobsParseRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if obj == nil {
			t.Fatal("response should not be nil")
		}

		job := obj.(*api.Job)
		expected := mock.Job()
		if job.Name == nil || *job.Name != expected.Name {
			t.Fatalf("job name is '%s', expected '%s'", *job.Name, expected.Name)
		}

		if job.Datacenters == nil ||
			job.Datacenters[0] != expected.Datacenters[0] {
			t.Fatalf("job datacenters is '%s', expected '%s'",
				job.Datacenters[0], expected.Datacenters[0])
		}
	})
}
func TestHTTP_JobQuery(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("GET", "/v1/job/"+job.ID, nil)
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
		j := obj.(*structs.Job)
		if j.ID != job.ID {
			t.Fatalf("bad: %#v", j)
		}
	})
}

func TestHTTP_JobQuery_Payload(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := mock.Job()

		// Insert Payload compressed
		expected := []byte("hello world")
		compressed := snappy.Encode(nil, expected)
		job.Payload = compressed

		// Directly manipulate the state
		state := s.Agent.server.State()
		if err := state.UpsertJob(1000, job); err != nil {
			t.Fatalf("Failed to upsert job: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/job/"+job.ID, nil)
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

func TestHTTP_JobUpdate(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("PUT", "/v1/job/"+*job.ID, buf)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
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

func TestHTTP_JobUpdateRegion(t *testing.T) {
	t.Parallel()

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

				req, err := http.NewRequest("PUT", url, buf)
				require.NoError(t, err)
				respW := httptest.NewRecorder()

				// Make the request
				obj, err := s.Server.JobSpecificRequest(respW, req)
				require.NoError(t, err)

				// Check the response
				dereg := obj.(structs.JobRegisterResponse)
				require.NotEmpty(t, dereg.EvalID)

				// Check for the index
				require.NotEmpty(t, respW.HeaderMap.Get("X-Nomad-Index"), "missing index")

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
	t.Parallel()
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
		req, err := http.NewRequest("DELETE", "/v1/job/"+job.ID, nil)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
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
		req2, err := http.NewRequest("DELETE", "/v1/job/"+job.ID+"?purge=true", nil)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
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

func TestHTTP_JobForceEvaluate(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("POST", "/v1/job/"+job.ID+"/evaluate", nil)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
	})
}

func TestHTTP_JobEvaluate_ForceReschedule(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("POST", "/v1/job/"+job.ID+"/evaluate", buf)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
	})
}

func TestHTTP_JobEvaluations(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("GET", "/v1/job/"+job.ID+"/evaluations", nil)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}
	})
}

func TestHTTP_JobAllocations(t *testing.T) {
	t.Parallel()
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
		err := state.UpsertAllocs(1000, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/job/"+alloc1.Job.ID+"/allocations?all=true", nil)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}
	})
}

func TestHTTP_JobDeployments(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
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
		req, err := http.NewRequest("GET", "/v1/job/"+j.ID+"/deployments", nil)
		assert.Nil(err, "HTTP")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		assert.Nil(err, "JobSpecificRequest")

		// Check the response
		deploys := obj.([]*structs.Deployment)
		assert.Len(deploys, 1, "deployments")
		assert.Equal(d.ID, deploys[0].ID, "deployment id")

		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"), "missing index")
		assert.Equal("true", respW.HeaderMap.Get("X-Nomad-KnownLeader"), "missing known leader")
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-LastContact"), "missing last contact")
	})
}

func TestHTTP_JobDeployment(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
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
		req, err := http.NewRequest("GET", "/v1/job/"+j.ID+"/deployment", nil)
		assert.Nil(err, "HTTP")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.JobSpecificRequest(respW, req)
		assert.Nil(err, "JobSpecificRequest")

		// Check the response
		out := obj.(*structs.Deployment)
		assert.NotNil(out, "deployment")
		assert.Equal(d.ID, out.ID, "deployment id")

		assert.NotZero(respW.HeaderMap.Get("X-Nomad-Index"), "missing index")
		assert.Equal("true", respW.HeaderMap.Get("X-Nomad-KnownLeader"), "missing known leader")
		assert.NotZero(respW.HeaderMap.Get("X-Nomad-LastContact"), "missing last contact")
	})
}

func TestHTTP_JobVersions(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("GET", "/v1/job/"+job.ID+"/versions?diffs=true", nil)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}
	})
}

func TestHTTP_PeriodicForce(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("POST", "/v1/job/"+job.ID+"/periodic/force", nil)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
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
	t.Parallel()
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
		req, err := http.NewRequest("PUT", "/v1/job/"+*job.ID+"/plan", buf)
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
	t.Parallel()

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
				req, err := http.NewRequest("PUT", "/v1/job/"+*job.ID+"/plan", buf)
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
	t.Parallel()
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
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		buf := encodeReq(args2)

		// Make the HTTP request
		req2, err := http.NewRequest("PUT", "/v1/job/"+job.ID+"/dispatch", buf)
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
	t.Parallel()
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
		req, err := http.NewRequest("PUT", "/v1/job/"+job.ID+"/revert", buf)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
	})
}

func TestHTTP_JobStable(t *testing.T) {
	t.Parallel()
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
		req, err := http.NewRequest("PUT", "/v1/job/"+job.ID+"/stable", buf)
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
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
	})
}

func TestJobs_ApiJobToStructsJob(t *testing.T) {
	apiJob := &api.Job{
		Stop:        helper.BoolToPtr(true),
		Region:      helper.StringToPtr("global"),
		Namespace:   helper.StringToPtr("foo"),
		ID:          helper.StringToPtr("foo"),
		ParentID:    helper.StringToPtr("lol"),
		Name:        helper.StringToPtr("name"),
		Type:        helper.StringToPtr("service"),
		Priority:    helper.IntToPtr(50),
		AllAtOnce:   helper.BoolToPtr(true),
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
				Weight:  helper.Int8ToPtr(50),
			},
		},
		Update: &api.UpdateStrategy{
			Stagger:          helper.TimeToPtr(1 * time.Second),
			MaxParallel:      helper.IntToPtr(5),
			HealthCheck:      helper.StringToPtr(structs.UpdateStrategyHealthCheck_Manual),
			MinHealthyTime:   helper.TimeToPtr(1 * time.Minute),
			HealthyDeadline:  helper.TimeToPtr(3 * time.Minute),
			ProgressDeadline: helper.TimeToPtr(3 * time.Minute),
			AutoRevert:       helper.BoolToPtr(false),
			Canary:           helper.IntToPtr(1),
		},
		Spreads: []*api.Spread{
			{
				Attribute: "${meta.rack}",
				Weight:    helper.Int8ToPtr(100),
				SpreadTarget: []*api.SpreadTarget{
					{
						Value:   "r1",
						Percent: 50,
					},
				},
			},
		},
		Periodic: &api.PeriodicConfig{
			Enabled:         helper.BoolToPtr(true),
			Spec:            helper.StringToPtr("spec"),
			SpecType:        helper.StringToPtr("cron"),
			ProhibitOverlap: helper.BoolToPtr(true),
			TimeZone:        helper.StringToPtr("test zone"),
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
		TaskGroups: []*api.TaskGroup{
			{
				Name:  helper.StringToPtr("group1"),
				Count: helper.IntToPtr(5),
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
						Weight:  helper.Int8ToPtr(100),
					},
				},
				RestartPolicy: &api.RestartPolicy{
					Interval: helper.TimeToPtr(1 * time.Second),
					Attempts: helper.IntToPtr(5),
					Delay:    helper.TimeToPtr(10 * time.Second),
					Mode:     helper.StringToPtr("delay"),
				},
				ReschedulePolicy: &api.ReschedulePolicy{
					Interval:      helper.TimeToPtr(12 * time.Hour),
					Attempts:      helper.IntToPtr(5),
					DelayFunction: helper.StringToPtr("constant"),
					Delay:         helper.TimeToPtr(30 * time.Second),
					Unlimited:     helper.BoolToPtr(true),
					MaxDelay:      helper.TimeToPtr(20 * time.Minute),
				},
				Migrate: &api.MigrateStrategy{
					MaxParallel:     helper.IntToPtr(12),
					HealthCheck:     helper.StringToPtr("task_events"),
					MinHealthyTime:  helper.TimeToPtr(12 * time.Hour),
					HealthyDeadline: helper.TimeToPtr(12 * time.Hour),
				},
				Spreads: []*api.Spread{
					{
						Attribute: "${node.datacenter}",
						Weight:    helper.Int8ToPtr(100),
						SpreadTarget: []*api.SpreadTarget{
							{
								Value:   "dc1",
								Percent: 100,
							},
						},
					},
				},
				EphemeralDisk: &api.EphemeralDisk{
					SizeMB:  helper.IntToPtr(100),
					Sticky:  helper.BoolToPtr(true),
					Migrate: helper.BoolToPtr(true),
				},
				Update: &api.UpdateStrategy{
					HealthCheck:      helper.StringToPtr(structs.UpdateStrategyHealthCheck_Checks),
					MinHealthyTime:   helper.TimeToPtr(2 * time.Minute),
					HealthyDeadline:  helper.TimeToPtr(5 * time.Minute),
					ProgressDeadline: helper.TimeToPtr(5 * time.Minute),
					AutoRevert:       helper.BoolToPtr(true),
				},
				Meta: map[string]string{
					"key": "value",
				},
				Services: []*api.Service{
					{
						Name:       "groupserviceA",
						Tags:       []string{"a", "b"},
						CanaryTags: []string{"d", "e"},
						PortLabel:  "1234",
						Meta: map[string]string{
							"servicemeta": "foobar",
						},
						CheckRestart: &api.CheckRestart{
							Limit: 4,
							Grace: helper.TimeToPtr(11 * time.Second),
						},
						Checks: []api.ServiceCheck{
							{
								Id:            "hello",
								Name:          "bar",
								Type:          "http",
								Command:       "foo",
								Args:          []string{"a", "b"},
								Path:          "/check",
								Protocol:      "http",
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
								TaskName: "task1",
							},
						},
						Connect: &api.ConsulConnect{
							Native: false,
							SidecarService: &api.ConsulSidecarService{
								Tags: []string{"f", "g"},
								Port: "9000",
							},
						},
					},
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
						Affinities: []*api.Affinity{
							{
								LTarget: "a",
								RTarget: "b",
								Operand: "c",
								Weight:  helper.Int8ToPtr(50),
							},
						},

						Services: []*api.Service{
							{
								Id:         "id",
								Name:       "serviceA",
								Tags:       []string{"1", "2"},
								CanaryTags: []string{"3", "4"},
								PortLabel:  "foo",
								Meta: map[string]string{
									"servicemeta": "foobar",
								},
								CheckRestart: &api.CheckRestart{
									Limit: 4,
									Grace: helper.TimeToPtr(11 * time.Second),
								},
								Checks: []api.ServiceCheck{
									{
										Id:            "hello",
										Name:          "bar",
										Type:          "http",
										Command:       "foo",
										Args:          []string{"a", "b"},
										Path:          "/check",
										Protocol:      "http",
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
									},
									{
										Id:        "check2id",
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
							CPU:      helper.IntToPtr(100),
							MemoryMB: helper.IntToPtr(10),
							Networks: []*api.NetworkResource{
								{
									IP:    "10.10.11.1",
									MBits: helper.IntToPtr(10),
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
									Count: helper.Uint64ToPtr(4),
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
											Weight:  helper.Int8ToPtr(50),
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
						KillTimeout: helper.TimeToPtr(10 * time.Second),
						KillSignal:  "SIGQUIT",
						LogConfig: &api.LogConfig{
							MaxFiles:      helper.IntToPtr(10),
							MaxFileSizeMB: helper.IntToPtr(100),
						},
						Artifacts: []*api.TaskArtifact{
							{
								GetterSource: helper.StringToPtr("source"),
								GetterOptions: map[string]string{
									"a": "b",
								},
								GetterMode:   helper.StringToPtr("dir"),
								RelativeDest: helper.StringToPtr("dest"),
							},
						},
						Vault: &api.Vault{
							Policies:     []string{"a", "b", "c"},
							Env:          helper.BoolToPtr(true),
							ChangeMode:   helper.StringToPtr("c"),
							ChangeSignal: helper.StringToPtr("sighup"),
						},
						Templates: []*api.Template{
							{
								SourcePath:   helper.StringToPtr("source"),
								DestPath:     helper.StringToPtr("dest"),
								EmbeddedTmpl: helper.StringToPtr("embedded"),
								ChangeMode:   helper.StringToPtr("change"),
								ChangeSignal: helper.StringToPtr("signal"),
								Splay:        helper.TimeToPtr(1 * time.Minute),
								Perms:        helper.StringToPtr("666"),
								LeftDelim:    helper.StringToPtr("abc"),
								RightDelim:   helper.StringToPtr("def"),
								Envvars:      helper.BoolToPtr(true),
								VaultGrace:   helper.TimeToPtr(3 * time.Second),
							},
						},
						DispatchPayload: &api.DispatchPayloadConfig{
							File: "fileA",
						},
					},
				},
			},
		},
		ConsulToken:       helper.StringToPtr("abc123"),
		VaultToken:        helper.StringToPtr("def456"),
		Status:            helper.StringToPtr("status"),
		StatusDescription: helper.StringToPtr("status_desc"),
		Version:           helper.Uint64ToPtr(10),
		CreateIndex:       helper.Uint64ToPtr(1),
		ModifyIndex:       helper.Uint64ToPtr(3),
		JobModifyIndex:    helper.Uint64ToPtr(5),
	}

	expected := &structs.Job{
		Stop:        true,
		Region:      "global",
		Namespace:   "foo",
		ID:          "foo",
		ParentID:    "lol",
		Name:        "name",
		Type:        "service",
		Priority:    50,
		AllAtOnce:   true,
		Datacenters: []string{"dc1", "dc2"},
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
					Interval: 1 * time.Second,
					Attempts: 5,
					Delay:    10 * time.Second,
					Mode:     "delay",
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
				Services: []*structs.Service{
					{
						Name:        "groupserviceA",
						Tags:        []string{"a", "b"},
						CanaryTags:  []string{"d", "e"},
						PortLabel:   "1234",
						AddressMode: "auto",
						Meta: map[string]string{
							"servicemeta": "foobar",
						},
						Checks: []*structs.ServiceCheck{
							{
								Name:          "bar",
								Type:          "http",
								Command:       "foo",
								Args:          []string{"a", "b"},
								Path:          "/check",
								Protocol:      "http",
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
								TaskName: "task1",
							},
						},
						Connect: &structs.ConsulConnect{
							Native: false,
							SidecarService: &structs.ConsulSidecarService{
								Tags: []string{"f", "g"},
								Port: "9000",
							},
						},
					},
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
						Services: []*structs.Service{
							{
								Name:        "serviceA",
								Tags:        []string{"1", "2"},
								CanaryTags:  []string{"3", "4"},
								PortLabel:   "foo",
								AddressMode: "auto",
								Meta: map[string]string{
									"servicemeta": "foobar",
								},
								Checks: []*structs.ServiceCheck{
									{
										Name:          "bar",
										Type:          "http",
										Command:       "foo",
										Args:          []string{"a", "b"},
										Path:          "/check",
										Protocol:      "http",
										PortLabel:     "foo",
										AddressMode:   "driver",
										Interval:      4 * time.Second,
										Timeout:       2 * time.Second,
										InitialStatus: "ok",
										GRPCService:   "foo.Bar",
										GRPCUseTLS:    true,
										CheckRestart: &structs.CheckRestart{
											Limit:          3,
											Grace:          11 * time.Second,
											IgnoreWarnings: true,
										},
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
									},
								},
							},
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
							Policies:     []string{"a", "b", "c"},
							Env:          true,
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
								Splay:        1 * time.Minute,
								Perms:        "666",
								LeftDelim:    "abc",
								RightDelim:   "def",
								Envvars:      true,
								VaultGrace:   3 * time.Second,
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

	if diff := pretty.Diff(expected, structsJob); len(diff) > 0 {
		t.Fatalf("bad:\n%s", strings.Join(diff, "\n"))
	}

	systemAPIJob := &api.Job{
		Stop:        helper.BoolToPtr(true),
		Region:      helper.StringToPtr("global"),
		Namespace:   helper.StringToPtr("foo"),
		ID:          helper.StringToPtr("foo"),
		ParentID:    helper.StringToPtr("lol"),
		Name:        helper.StringToPtr("name"),
		Type:        helper.StringToPtr("system"),
		Priority:    helper.IntToPtr(50),
		AllAtOnce:   helper.BoolToPtr(true),
		Datacenters: []string{"dc1", "dc2"},
		Constraints: []*api.Constraint{
			{
				LTarget: "a",
				RTarget: "b",
				Operand: "c",
			},
		},
		TaskGroups: []*api.TaskGroup{
			{
				Name:  helper.StringToPtr("group1"),
				Count: helper.IntToPtr(5),
				Constraints: []*api.Constraint{
					{
						LTarget: "x",
						RTarget: "y",
						Operand: "z",
					},
				},
				RestartPolicy: &api.RestartPolicy{
					Interval: helper.TimeToPtr(1 * time.Second),
					Attempts: helper.IntToPtr(5),
					Delay:    helper.TimeToPtr(10 * time.Second),
					Mode:     helper.StringToPtr("delay"),
				},
				EphemeralDisk: &api.EphemeralDisk{
					SizeMB:  helper.IntToPtr(100),
					Sticky:  helper.BoolToPtr(true),
					Migrate: helper.BoolToPtr(true),
				},
				Meta: map[string]string{
					"key": "value",
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
							CPU:      helper.IntToPtr(100),
							MemoryMB: helper.IntToPtr(10),
							Networks: []*api.NetworkResource{
								{
									IP:    "10.10.11.1",
									MBits: helper.IntToPtr(10),
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
						KillTimeout: helper.TimeToPtr(10 * time.Second),
						KillSignal:  "SIGQUIT",
						LogConfig: &api.LogConfig{
							MaxFiles:      helper.IntToPtr(10),
							MaxFileSizeMB: helper.IntToPtr(100),
						},
						Artifacts: []*api.TaskArtifact{
							{
								GetterSource: helper.StringToPtr("source"),
								GetterOptions: map[string]string{
									"a": "b",
								},
								GetterMode:   helper.StringToPtr("dir"),
								RelativeDest: helper.StringToPtr("dest"),
							},
						},
						DispatchPayload: &api.DispatchPayloadConfig{
							File: "fileA",
						},
					},
				},
			},
		},
		Status:            helper.StringToPtr("status"),
		StatusDescription: helper.StringToPtr("status_desc"),
		Version:           helper.Uint64ToPtr(10),
		CreateIndex:       helper.Uint64ToPtr(1),
		ModifyIndex:       helper.Uint64ToPtr(3),
		JobModifyIndex:    helper.Uint64ToPtr(5),
	}

	expectedSystemJob := &structs.Job{
		Stop:        true,
		Region:      "global",
		Namespace:   "foo",
		ID:          "foo",
		ParentID:    "lol",
		Name:        "name",
		Type:        "system",
		Priority:    50,
		AllAtOnce:   true,
		Datacenters: []string{"dc1", "dc2"},
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
					Interval: 1 * time.Second,
					Attempts: 5,
					Delay:    10 * time.Second,
					Mode:     "delay",
				},
				EphemeralDisk: &structs.EphemeralDisk{
					SizeMB:  100,
					Sticky:  true,
					Migrate: true,
				},
				Meta: map[string]string{
					"key": "value",
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
						Meta: map[string]string{
							"lol": "code",
						},
						KillTimeout: 10 * time.Second,
						KillSignal:  "SIGQUIT",
						LogConfig: &structs.LogConfig{
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
						DispatchPayload: &structs.DispatchPayloadConfig{
							File: "fileA",
						},
					},
				},
			},
		},
	}

	systemStructsJob := ApiJobToStructJob(systemAPIJob)

	if diff := pretty.Diff(expectedSystemJob, systemStructsJob); len(diff) > 0 {
		t.Fatalf("bad:\n%s", strings.Join(diff, "\n"))
	}
}

func TestJobs_ApiJobToStructsJobUpdate(t *testing.T) {
	apiJob := &api.Job{
		Update: &api.UpdateStrategy{
			Stagger:          helper.TimeToPtr(1 * time.Second),
			MaxParallel:      helper.IntToPtr(5),
			HealthCheck:      helper.StringToPtr(structs.UpdateStrategyHealthCheck_Manual),
			MinHealthyTime:   helper.TimeToPtr(1 * time.Minute),
			HealthyDeadline:  helper.TimeToPtr(3 * time.Minute),
			ProgressDeadline: helper.TimeToPtr(3 * time.Minute),
			AutoRevert:       helper.BoolToPtr(false),
			AutoPromote:      nil,
			Canary:           helper.IntToPtr(1),
		},
		TaskGroups: []*api.TaskGroup{
			{
				Update: &api.UpdateStrategy{
					Canary:     helper.IntToPtr(2),
					AutoRevert: helper.BoolToPtr(true),
				},
			}, {
				Update: &api.UpdateStrategy{
					Canary:      helper.IntToPtr(3),
					AutoPromote: helper.BoolToPtr(true),
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

// TestHTTP_JobValidate_SystemMigrate asserts that a system job with a migrate
// stanza fails to validate but does not panic (see #5477).
func TestHTTP_JobValidate_SystemMigrate(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Create the job
		job := &api.Job{
			Region:      helper.StringToPtr("global"),
			Datacenters: []string{"dc1"},
			ID:          helper.StringToPtr("systemmigrate"),
			Name:        helper.StringToPtr("systemmigrate"),
			TaskGroups: []*api.TaskGroup{
				{Name: helper.StringToPtr("web")},
			},

			// System job...
			Type: helper.StringToPtr("system"),

			// ...with an empty migrate stanza
			Migrate: &api.MigrateStrategy{},
		}

		args := api.JobValidateRequest{
			Job:          job,
			WriteRequest: api.WriteRequest{Region: "global"},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/validate/job", buf)
		require.NoError(t, err)
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.ValidateJobRequest(respW, req)
		require.NoError(t, err)

		// Check the response
		resp := obj.(structs.JobValidateResponse)
		require.Contains(t, resp.Error, `Job type "system" does not allow migrate block`)
	})
}
