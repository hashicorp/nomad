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

func TestJobEndpoint_Register(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	serviceName := out.TaskGroups[0].Tasks[0].Services[0].Name
	expectedServiceName := "web-frontend"
	if serviceName != expectedServiceName {
		t.Fatalf("Expected Service Name: %s, Actual: %s", expectedServiceName, serviceName)
	}

	// Lookup the evaluation
	eval, err := state.EvalByID(resp.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != resp.EvalCreateIndex {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != job.Priority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != job.Type {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerJobRegister {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != job.ID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobModifyIndex != resp.JobModifyIndex {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
}

func TestJobEndpoint_Register_Existing(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Update the job definition
	job2 := mock.Job()
	job2.Priority = 100
	job2.ID = job.ID
	req.Job = job2

	// Attempt update
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.ModifyIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	if out.Priority != 100 {
		t.Fatalf("expected update")
	}

	// Lookup the evaluation
	eval, err := state.EvalByID(resp.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != resp.EvalCreateIndex {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != job2.Priority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != job2.Type {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerJobRegister {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != job2.ID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobModifyIndex != resp.JobModifyIndex {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
}

func TestJobEndpoint_Register_Periodic(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request for a periodic job.
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Periodic = &structs.PeriodicConfig{Enabled: true}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	serviceName := out.TaskGroups[0].Tasks[0].Services[0].Name
	expectedServiceName := "web-frontend"
	if serviceName != expectedServiceName {
		t.Fatalf("Expected Service Name: %s, Actual: %s", expectedServiceName, serviceName)
	}

	if resp.EvalID != "" {
		t.Fatalf("Register created an eval for a periodic job")
	}
}

func TestJobEndpoint_Evaluate(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID:        job.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Lookup the evaluation
	state := s1.fsm.State()
	eval, err := state.EvalByID(resp.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != resp.EvalCreateIndex {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != job.Priority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != job.Type {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerJobRegister {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != job.ID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobModifyIndex != resp.JobModifyIndex {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
}

func TestJobEndpoint_Evaluate_Periodic(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Periodic = &structs.PeriodicConfig{Enabled: true}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID:        job.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp); err == nil {
		t.Fatal("expect an err")
	}
}

func TestJobEndpoint_Deregister(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.JobDeregisterRequest{
		JobID:        job.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.JobDeregisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected job")
	}

	// Lookup the evaluation
	eval, err := state.EvalByID(resp2.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eval == nil {
		t.Fatalf("expected eval")
	}
	if eval.CreateIndex != resp2.EvalCreateIndex {
		t.Fatalf("index mis-match")
	}

	if eval.Priority != structs.JobDefaultPriority {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Type != structs.JobTypeService {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.TriggeredBy != structs.EvalTriggerJobDeregister {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobID != job.ID {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.JobModifyIndex != resp2.JobModifyIndex {
		t.Fatalf("bad: %#v", eval)
	}
	if eval.Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", eval)
	}
}

func TestJobEndpoint_Deregister_Periodic(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Periodic = &structs.PeriodicConfig{Enabled: true}
	reg := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.JobDeregisterRequest{
		JobID:        job.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.JobDeregisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected job")
	}

	if resp.EvalID != "" {
		t.Fatalf("Deregister created an eval for a periodic job")
	}
}

func TestJobEndpoint_GetJob(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	job.CreateIndex = resp.JobModifyIndex
	job.ModifyIndex = resp.JobModifyIndex

	// Lookup the job
	get := &structs.JobSpecificRequest{
		JobID:        job.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}

	// Make a copy of the origin job and change the service name so that we can
	// do a deep equal with the response from the GET JOB Api
	j := job
	j.TaskGroups[0].Tasks[0].Services[0].Name = "web-frontend"
	for tgix, tg := range j.TaskGroups {
		for tidx, t := range tg.Tasks {
			for sidx, service := range t.Services {
				service.Id = resp2.Job.TaskGroups[tgix].Tasks[tidx].Services[sidx].Id
				for cidx, check := range service.Checks {
					check.Name = resp2.Job.TaskGroups[tgix].Tasks[tidx].Services[sidx].Checks[cidx].Name
					check.Id = resp2.Job.TaskGroups[tgix].Tasks[tidx].Services[sidx].Checks[cidx].Id
				}
			}
		}
	}
	j.TaskGroups[0].Tasks[0].Services[0].Id = resp2.Job.TaskGroups[0].Tasks[0].Services[0].Id

	if !reflect.DeepEqual(j, resp2.Job) {
		t.Fatalf("bad: %#v %#v", job, resp2.Job)
	}

	// Lookup non-existing job
	get.JobID = "foobarbaz"
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}
	if resp2.Job != nil {
		t.Fatalf("unexpected job")
	}
}

func TestJobEndpoint_GetJob_Blocking(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the jobs
	job1 := mock.Job()
	job2 := mock.Job()

	// Upsert a job we are not interested in first.
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertJob(100, job1); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert another job later which should trigger the watch.
	time.AfterFunc(200*time.Millisecond, func() {
		if err := state.UpsertJob(200, job2); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.JobSpecificRequest{
		JobID: job2.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 50,
		},
	}
	start := time.Now()
	var resp structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Job == nil || resp.Job.ID != job2.ID {
		t.Fatalf("bad: %#v", resp.Job)
	}

	// Job delete fires watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteJob(300, job2.ID); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	start = time.Now()

	var resp2 structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Job != nil {
		t.Fatalf("bad: %#v", resp2.Job)
	}
}

func TestJobEndpoint_ListJobs(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	state := s1.fsm.State()
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the jobs
	get := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.JobListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 1000)
	}

	if len(resp2.Jobs) != 1 {
		t.Fatalf("bad: %#v", resp2.Jobs)
	}
	if resp2.Jobs[0].ID != job.ID {
		t.Fatalf("bad: %#v", resp2.Jobs[0])
	}
}

func TestJobEndpoint_ListJobs_Blocking(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the job
	job := mock.Job()

	// Upsert job triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertJob(100, job); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 50,
		},
	}
	start := time.Now()
	var resp structs.JobListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 100 {
		t.Fatalf("Bad index: %d %d", resp.Index, 100)
	}
	if len(resp.Jobs) != 1 || resp.Jobs[0].ID != job.ID {
		t.Fatalf("bad: %#v", resp.Jobs)
	}

	// Job deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteJob(200, job.ID); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 150
	start = time.Now()
	var resp2 structs.JobListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.List", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 200)
	}
	if len(resp2.Jobs) != 0 {
		t.Fatalf("bad: %#v", resp2.Jobs)
	}
}

func TestJobEndpoint_Allocations(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.JobID = alloc1.JobID
	state := s1.fsm.State()
	err := state.UpsertAllocs(1000,
		[]*structs.Allocation{alloc1, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID:        alloc1.JobID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.JobAllocationsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 1000)
	}

	if len(resp2.Allocations) != 2 {
		t.Fatalf("bad: %#v", resp2.Allocations)
	}
}

func TestJobEndpoint_Allocations_Blocking(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.JobID = "job1"
	state := s1.fsm.State()

	// First upsert an unrelated alloc
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertAllocs(100, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert an alloc for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertAllocs(200, []*structs.Allocation{alloc2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: "job1",
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 50,
		},
	}
	var resp structs.JobAllocationsResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Allocations) != 1 || resp.Allocations[0].JobID != "job1" {
		t.Fatalf("bad: %#v", resp.Allocations)
	}
}

func TestJobEndpoint_Evaluations(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	state := s1.fsm.State()
	err := state.UpsertEvals(1000,
		[]*structs.Evaluation{eval1, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID:        eval1.JobID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.JobEvaluationsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 1000)
	}

	if len(resp2.Evaluations) != 2 {
		t.Fatalf("bad: %#v", resp2.Evaluations)
	}
}
