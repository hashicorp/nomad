package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHTTP_JobsList(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		for i := 0; i < 3; i++ {
			// Create the job
			job := mock.Job()
			args := structs.JobRegisterRequest{
				Job:          job,
				WriteRequest: structs.WriteRequest{Region: "global"},
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
	httpTest(t, nil, func(s *TestServer) {
		for i := 0; i < 3; i++ {
			// Create the job
			job := mock.Job()
			job.ID = ids[i]
			job.TaskGroups[0].Count = 1
			args := structs.JobRegisterRequest{
				Job:          job,
				WriteRequest: structs.WriteRequest{Region: "global"},
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
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
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
			JobID:        job.ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
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

func TestHTTP_JobQuery(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
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

func TestHTTP_JobUpdate(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/job/"+job.ID, buf)
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
			JobID:        job.ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
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

func TestHTTP_JobDelete(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
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

		// Check the job is gone
		getReq := structs.JobSpecificRequest{
			JobID:        job.ID,
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		var getResp structs.SingleJobResponse
		if err := s.Agent.RPC("Job.GetJob", &getReq, &getResp); err != nil {
			t.Fatalf("err: %v", err)
		}
		if getResp.Job != nil {
			t.Fatalf("job still exists")
		}
	})
}

func TestHTTP_JobForceEvaluate(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
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

func TestHTTP_JobEvaluations(t *testing.T) {
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
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
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		job := mock.Job()
		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.JobRegisterResponse
		if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		alloc1.JobID = job.ID
		err := state.UpsertAllocs(1000, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/job/"+job.ID+"/allocations", nil)
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
	httpTest(t, nil, func(s *TestServer) {
		// Create and register a periodic job.
		job := mock.PeriodicJob()
		args := structs.JobRegisterRequest{
			Job:          job,
			WriteRequest: structs.WriteRequest{Region: "global"},
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
	httpTest(t, nil, func(s *TestServer) {
		// Create the job
		job := mock.Job()
		args := structs.JobPlanRequest{
			Job:          job,
			Diff:         true,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest("PUT", "/v1/job/"+job.ID+"/plan", buf)
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
