package nomad

import (
	"reflect"
	"testing"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestJobEndpoint_Register(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "region1"},
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
	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}

	// Lookup the evaluation
	eval, err := state.GetEvalByID(resp.EvalID)
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "region1"},
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
	out, err := state.GetJobByID(job.ID)
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
	eval, err := state.GetEvalByID(resp.EvalID)
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

func TestJobEndpoint_Evaluate(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "region1"},
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
		WriteRequest: structs.WriteRequest{Region: "region1"},
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
	eval, err := state.GetEvalByID(resp.EvalID)
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

func TestJobEndpoint_Deregister(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.JobDeregisterRequest{
		JobID:        job.ID,
		WriteRequest: structs.WriteRequest{Region: "region1"},
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
	out, err := state.GetJobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected job")
	}

	// Lookup the evaluation
	eval, err := state.GetEvalByID(resp2.EvalID)
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

func TestJobEndpoint_GetJob(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "region1"},
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
		QueryOptions: structs.QueryOptions{Region: "region1"},
	}
	var resp2 structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}

	if !reflect.DeepEqual(job, resp2.Job) {
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

func TestJobEndpoint_ListJobs(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	state := s1.fsm.State()
	err := state.RegisterJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the jobs
	get := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{Region: "region1"},
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
	err := state.UpdateAllocations(1000,
		[]*structs.Allocation{alloc1, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID:        alloc1.JobID,
		QueryOptions: structs.QueryOptions{Region: "region1"},
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
		QueryOptions: structs.QueryOptions{Region: "region1"},
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
