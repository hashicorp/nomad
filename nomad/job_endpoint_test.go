package nomad

import (
	"fmt"
	"reflect"
	"strings"
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

func TestJobEndpoint_Register_InvalidDriverConfig(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job containing an invalid driver
	// config
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Config["foo"] = 1
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil {
		t.Fatalf("expected a validation error")
	}

	if !strings.Contains(err.Error(), "-> config:") {
		t.Fatalf("expected a driver config validation error but got: %v", err)
	}
}

func TestJobEndpoint_Register_Payload(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job containing an invalid driver
	// config
	job := mock.Job()
	job.Payload = []byte{0x1}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil {
		t.Fatalf("expected a validation error")
	}

	if !strings.Contains(err.Error(), "payload") {
		t.Fatalf("expected a payload error but got: %v", err)
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
	job := mock.PeriodicJob()
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

func TestJobEndpoint_Register_Constructor(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request for a constructor job.
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Constructor = &structs.ConstructorConfig{}
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

	// Check for the job in the FSM
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
	if resp.EvalID != "" {
		t.Fatalf("Register created an eval for a constructor job")
	}
}

func TestJobEndpoint_Register_EnforceIndex(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request and enforcing an incorrect index
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: 100, // Not registered yet so not possible
		WriteRequest:   structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), RegisterEnforceIndexErrPrefix) {
		t.Fatalf("expected enforcement error")
	}

	// Create the register request and enforcing it is new
	req = &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: 0,
		WriteRequest:   structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	curIndex := resp.JobModifyIndex

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

	// Reregister request and enforcing it be a new job
	req = &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: 0,
		WriteRequest:   structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), RegisterEnforceIndexErrPrefix) {
		t.Fatalf("expected enforcement error")
	}

	// Reregister request and enforcing it be at an incorrect index
	req = &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: curIndex - 1,
		WriteRequest:   structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), RegisterEnforceIndexErrPrefix) {
		t.Fatalf("expected enforcement error")
	}

	// Reregister request and enforcing it be at the correct index
	job.Priority = job.Priority + 1
	req = &structs.JobRegisterRequest{
		Job:            job,
		EnforceIndex:   true,
		JobModifyIndex: curIndex,
		WriteRequest:   structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	out, err = state.JobByID(job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.Priority != job.Priority {
		t.Fatalf("priority mis-match")
	}
}

func TestJobEndpoint_Register_Vault_Disabled(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
		f := false
		c.VaultConfig.Enabled = &f
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job asking for a vault policy
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{"foo"},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), "Vault not enabled") {
		t.Fatalf("expected Vault not enabled error: %v", err)
	}
}

func TestJobEndpoint_Register_Vault_AllowUnauthenticated(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault and allow authenticated
	tr := true
	s1.config.VaultConfig.Enabled = &tr
	s1.config.VaultConfig.AllowUnauthenticated = &tr

	// Replace the Vault Client on the server
	s1.vault = &TestVaultClient{}

	// Create the register request with a job asking for a vault policy
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{"foo"},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
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
}

func TestJobEndpoint_Register_Vault_NoToken(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.VaultConfig.Enabled = &tr
	s1.config.VaultConfig.AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	s1.vault = &TestVaultClient{}

	// Create the register request with a job asking for a vault policy but
	// don't send a Vault token
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{"foo"},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), "missing Vault Token") {
		t.Fatalf("expected Vault not enabled error: %v", err)
	}
}

func TestJobEndpoint_Register_Vault_Policies(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.VaultConfig.Enabled = &tr
	s1.config.VaultConfig.AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	// Add three tokens: one that allows the requesting policy, one that does
	// not and one that returns an error
	policy := "foo"

	badToken := structs.GenerateUUID()
	badPolicies := []string{"a", "b", "c"}
	tvc.SetLookupTokenAllowedPolicies(badToken, badPolicies)

	goodToken := structs.GenerateUUID()
	goodPolicies := []string{"foo", "bar", "baz"}
	tvc.SetLookupTokenAllowedPolicies(goodToken, goodPolicies)

	rootToken := structs.GenerateUUID()
	rootPolicies := []string{"root"}
	tvc.SetLookupTokenAllowedPolicies(rootToken, rootPolicies)

	errToken := structs.GenerateUUID()
	expectedErr := fmt.Errorf("return errors from vault")
	tvc.SetLookupTokenError(errToken, expectedErr)

	// Create the register request with a job asking for a vault policy but
	// send the bad Vault token
	job := mock.Job()
	job.VaultToken = badToken
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{policy},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(),
		"doesn't allow access to the following policies: "+policy) {
		t.Fatalf("expected permission denied error: %v", err)
	}

	// Use the err token
	job.VaultToken = errToken
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("expected permission denied error: %v", err)
	}

	// Use the good token
	job.VaultToken = goodToken

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
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
	if out.VaultToken != "" {
		t.Fatalf("vault token not cleared")
	}

	// Check that an implicit constraint was created
	constraints := out.TaskGroups[0].Constraints
	if l := len(constraints); l != 1 {
		t.Fatalf("Unexpected number of tests: %v", l)
	}

	if !constraints[0].Equal(vaultConstraint) {
		t.Fatalf("bad constraint; got %#v; want %#v", constraints[0], vaultConstraint)
	}

	// Create the register request with another job asking for a vault policy but
	// send the root Vault token
	job2 := mock.Job()
	job2.VaultToken = rootToken
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{policy},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req = &structs.JobRegisterRequest{
		Job:          job2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	out, err = state.JobByID(job2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.CreateIndex != resp.JobModifyIndex {
		t.Fatalf("index mis-match")
	}
	if out.VaultToken != "" {
		t.Fatalf("vault token not cleared")
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
	job := mock.PeriodicJob()
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

func TestJobEndpoint_Evaluate_Constructor(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Constructor = &structs.ConstructorConfig{}
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

func TestJobEndpoint_Deregister_NonExistent(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Deregister
	jobID := "foo"
	dereg := &structs.JobDeregisterRequest{
		JobID:        jobID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp2 structs.JobDeregisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.JobModifyIndex == 0 {
		t.Fatalf("bad index: %d", resp2.Index)
	}

	// Lookup the evaluation
	state := s1.fsm.State()
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
	if eval.JobID != jobID {
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
	job := mock.PeriodicJob()
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

func TestJobEndpoint_Deregister_Constructor(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.Constructor = &structs.ConstructorConfig{}
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
		t.Fatalf("Deregister created an eval for a constructor job")
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
	job.JobModifyIndex = resp.JobModifyIndex

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
				for cidx, check := range service.Checks {
					check.Name = resp2.Job.TaskGroups[tgix].Tasks[tidx].Services[sidx].Checks[cidx].Name
				}
			}
		}
	}

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

func TestJobEndpoint_GetJobSummary(t *testing.T) {
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
	job.CreateIndex = resp.JobModifyIndex
	job.ModifyIndex = resp.JobModifyIndex
	job.JobModifyIndex = resp.JobModifyIndex

	// Lookup the job summary
	get := &structs.JobSummaryRequest{
		JobID:        job.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp2 structs.JobSummaryResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}

	expectedJobSummary := structs.JobSummary{
		JobID: job.ID,
		Summary: map[string]structs.TaskGroupSummary{
			"web": structs.TaskGroupSummary{},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: job.CreateIndex,
		ModifyIndex: job.CreateIndex,
	}

	if !reflect.DeepEqual(resp2.JobSummary, &expectedJobSummary) {
		t.Fatalf("exptected: %v, actual: %v", expectedJobSummary, resp2.JobSummary)
	}
}

func TestJobEndpoint_GetJobSummary_Blocking(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create a job and insert it
	job1 := mock.Job()
	time.AfterFunc(200*time.Millisecond, func() {
		if err := state.UpsertJob(100, job1); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Ensure the job summary request gets fired
	req := &structs.JobSummaryRequest{
		JobID: job1.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 50,
		},
	}
	var resp structs.JobSummaryResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Upsert an allocation for the job which should trigger the watch.
	time.AfterFunc(200*time.Millisecond, func() {
		alloc := mock.Alloc()
		alloc.JobID = job1.ID
		alloc.Job = job1
		if err := state.UpsertAllocs(200, []*structs.Allocation{alloc}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})
	req = &structs.JobSummaryRequest{
		JobID: job1.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 199,
		},
	}
	start = time.Now()
	var resp1 structs.JobSummaryResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &resp1); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp1.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp1.JobSummary == nil {
		t.Fatalf("bad: %#v", resp)
	}

	// Job delete fires watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteJob(300, job1.ID); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	start = time.Now()

	var resp2 structs.SingleJobResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &resp2); err != nil {
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

	// Lookup the jobs by prefix
	get = &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{Region: "global", Prefix: resp2.Jobs[0].ID[:4]},
	}
	var resp3 structs.JobListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.List", get, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp3.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp3.Index, 1000)
	}

	if len(resp3.Jobs) != 1 {
		t.Fatalf("bad: %#v", resp3.Jobs)
	}
	if resp3.Jobs[0].ID != job.ID {
		t.Fatalf("bad: %#v", resp3.Jobs[0])
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
	state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
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
		state.UpsertJobSummary(99, mock.JobSummary(alloc1.JobID))
		err := state.UpsertAllocs(100, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert an alloc for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		state.UpsertJobSummary(199, mock.JobSummary(alloc2.JobID))
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

func TestJobEndpoint_Evaluations_Blocking(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = "job1"
	state := s1.fsm.State()

	// First upsert an unrelated eval
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertEvals(100, []*structs.Evaluation{eval1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert an eval for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertEvals(200, []*structs.Evaluation{eval2})
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
	var resp structs.JobEvaluationsResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Evaluations) != 1 || resp.Evaluations[0].JobID != "job1" {
		t.Fatalf("bad: %#v", resp.Evaluations)
	}
}

func TestJobEndpoint_Plan_WithDiff(t *testing.T) {
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

	// Create a plan request
	planReq := &structs.JobPlanRequest{
		Job:          job,
		Diff:         true,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var planResp structs.JobPlanResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the response
	if planResp.JobModifyIndex == 0 {
		t.Fatalf("bad cas: %d", planResp.JobModifyIndex)
	}
	if planResp.Annotations == nil {
		t.Fatalf("no annotations")
	}
	if planResp.Diff == nil {
		t.Fatalf("no diff")
	}
	if len(planResp.FailedTGAllocs) == 0 {
		t.Fatalf("no failed task group alloc metrics")
	}
}

func TestJobEndpoint_Plan_NoDiff(t *testing.T) {
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

	// Create a plan request
	planReq := &structs.JobPlanRequest{
		Job:          job,
		Diff:         false,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var planResp structs.JobPlanResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the response
	if planResp.JobModifyIndex == 0 {
		t.Fatalf("bad cas: %d", planResp.JobModifyIndex)
	}
	if planResp.Annotations == nil {
		t.Fatalf("no annotations")
	}
	if planResp.Diff != nil {
		t.Fatalf("got diff")
	}
	if len(planResp.FailedTGAllocs) == 0 {
		t.Fatalf("no failed task group alloc metrics")
	}
}

func TestJobEndpoint_ImplicitConstraints_Vault(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Enable vault
	tr, f := true, false
	s1.config.VaultConfig.Enabled = &tr
	s1.config.VaultConfig.AllowUnauthenticated = &f

	// Replace the Vault Client on the server
	tvc := &TestVaultClient{}
	s1.vault = tvc

	policy := "foo"
	goodToken := structs.GenerateUUID()
	goodPolicies := []string{"foo", "bar", "baz"}
	tvc.SetLookupTokenAllowedPolicies(goodToken, goodPolicies)

	// Create the register request with a job asking for a vault policy
	job := mock.Job()
	job.VaultToken = goodToken
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:   []string{policy},
		ChangeMode: structs.VaultChangeModeRestart,
	}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
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

	// Check that there is an implicit vault constraint
	constraints := out.TaskGroups[0].Constraints
	if len(constraints) != 1 {
		t.Fatalf("Expected an implicit constraint")
	}

	if !constraints[0].Equal(vaultConstraint) {
		t.Fatalf("Expected implicit vault constraint")
	}
}

func TestJobEndpoint_ImplicitConstraints_Signals(t *testing.T) {
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job asking for a template that sends a
	// signal
	job := mock.Job()
	signal := "SIGUSR1"
	job.TaskGroups[0].Tasks[0].Templates = []*structs.Template{
		&structs.Template{
			SourcePath:   "foo",
			DestPath:     "bar",
			ChangeMode:   structs.TemplateChangeModeSignal,
			ChangeSignal: signal,
		},
	}
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
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

	// Check that there is an implicit signal constraint
	constraints := out.TaskGroups[0].Constraints
	if len(constraints) != 1 {
		t.Fatalf("Expected an implicit constraint")
	}

	sigConstraint := getSignalConstraint([]string{signal})

	if !constraints[0].Equal(sigConstraint) {
		t.Fatalf("Expected implicit vault constraint")
	}
}

func TestJobEndpoint_ValidateJob_InvalidDriverConf(t *testing.T) {
	// Create a mock job with an invalid config
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"foo": "bar",
	}

	if err := validateJob(job); err == nil || !strings.Contains(err.Error(), "-> config") {
		t.Fatalf("Expected config error; got %v", err)
	}
}

func TestJobEndpoint_ValidateJob_InvalidSignals(t *testing.T) {
	// Create a mock job that wants to send a signal to a driver that can't
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Driver = "qemu"
	job.TaskGroups[0].Tasks[0].Vault = &structs.Vault{
		Policies:     []string{"foo"},
		ChangeMode:   structs.VaultChangeModeSignal,
		ChangeSignal: "SIGUSR1",
	}

	if err := validateJob(job); err == nil || !strings.Contains(err.Error(), "support sending signals") {
		t.Fatalf("Expected signal feasibility error; got %v", err)
	}
}

func TestJobEndpoint_Dispatch(t *testing.T) {

	// No requirements
	d1 := mock.Job()
	d1.Type = structs.JobTypeBatch
	d1.Constructor = &structs.ConstructorConfig{}

	// Require input data
	d2 := mock.Job()
	d2.Type = structs.JobTypeBatch
	d2.Constructor = &structs.ConstructorConfig{
		Payload: structs.DispatchPayloadRequired,
	}

	// Disallow input data
	d3 := mock.Job()
	d3.Type = structs.JobTypeBatch
	d3.Constructor = &structs.ConstructorConfig{
		Payload: structs.DispatchPayloadForbidden,
	}

	// Require meta
	d4 := mock.Job()
	d4.Type = structs.JobTypeBatch
	d4.Constructor = &structs.ConstructorConfig{
		MetaRequired: []string{"foo", "bar"},
	}

	// Optional meta
	d5 := mock.Job()
	d5.Type = structs.JobTypeBatch
	d5.Constructor = &structs.ConstructorConfig{
		MetaOptional: []string{"foo", "bar"},
	}

	reqNoInputNoMeta := &structs.JobDispatchRequest{}
	reqInputDataNoMeta := &structs.JobDispatchRequest{
		Payload: []byte("hello world"),
	}
	reqNoInputDataMeta := &structs.JobDispatchRequest{
		Meta: map[string]string{
			"foo": "f1",
			"bar": "f2",
		},
	}
	reqInputDataMeta := &structs.JobDispatchRequest{
		Payload: []byte("hello world"),
		Meta: map[string]string{
			"foo": "f1",
			"bar": "f2",
		},
	}
	reqBadMeta := &structs.JobDispatchRequest{
		Payload: []byte("hello world"),
		Meta: map[string]string{
			"foo": "f1",
			"bar": "f2",
			"baz": "f3",
		},
	}
	reqInputDataTooLarge := &structs.JobDispatchRequest{
		Payload: make([]byte, DispatchPayloadSizeLimit+100),
	}

	type testCase struct {
		name        string
		constructor *structs.Job
		dispatchReq *structs.JobDispatchRequest
		err         bool
		errStr      string
	}
	cases := []testCase{
		{
			name:        "optional input data w/ data",
			constructor: d1,
			dispatchReq: reqInputDataNoMeta,
			err:         false,
		},
		{
			name:        "optional input data w/o data",
			constructor: d1,
			dispatchReq: reqNoInputNoMeta,
			err:         false,
		},
		{
			name:        "require input data w/ data",
			constructor: d2,
			dispatchReq: reqInputDataNoMeta,
			err:         false,
		},
		{
			name:        "require input data w/o data",
			constructor: d2,
			dispatchReq: reqNoInputNoMeta,
			err:         true,
			errStr:      "not provided but required",
		},
		{
			name:        "disallow input data w/o data",
			constructor: d3,
			dispatchReq: reqNoInputNoMeta,
			err:         false,
		},
		{
			name:        "disallow input data w/ data",
			constructor: d3,
			dispatchReq: reqInputDataNoMeta,
			err:         true,
			errStr:      "provided but forbidden",
		},
		{
			name:        "require meta w/ meta",
			constructor: d4,
			dispatchReq: reqInputDataMeta,
			err:         false,
		},
		{
			name:        "require meta w/o meta",
			constructor: d4,
			dispatchReq: reqNoInputNoMeta,
			err:         true,
			errStr:      "did not provide required meta keys",
		},
		{
			name:        "optional meta w/ meta",
			constructor: d5,
			dispatchReq: reqNoInputDataMeta,
			err:         false,
		},
		{
			name:        "optional meta w/o meta",
			constructor: d5,
			dispatchReq: reqNoInputNoMeta,
			err:         false,
		},
		{
			name:        "optional meta w/ bad meta",
			constructor: d5,
			dispatchReq: reqBadMeta,
			err:         true,
			errStr:      "unpermitted metadata keys",
		},
		{
			name:        "optional input w/ too big of input",
			constructor: d1,
			dispatchReq: reqInputDataTooLarge,
			err:         true,
			errStr:      "Payload exceeds maximum size",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s1 := testServer(t, func(c *Config) {
				c.NumSchedulers = 0 // Prevent automatic dequeue
			})
			defer s1.Shutdown()
			codec := rpcClient(t, s1)
			testutil.WaitForLeader(t, s1.RPC)

			// Create the register request
			regReq := &structs.JobRegisterRequest{
				Job:          tc.constructor,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}

			// Fetch the response
			var regResp structs.JobRegisterResponse
			if err := msgpackrpc.CallWithCodec(codec, "Job.Register", regReq, &regResp); err != nil {
				t.Fatalf("err: %v", err)
			}

			// Now try to dispatch
			tc.dispatchReq.JobID = tc.constructor.ID
			tc.dispatchReq.WriteRequest = structs.WriteRequest{Region: "global"}

			var dispatchResp structs.JobDispatchResponse
			dispatchErr := msgpackrpc.CallWithCodec(codec, "Job.Dispatch", tc.dispatchReq, &dispatchResp)

			if dispatchErr == nil {
				if tc.err {
					t.Fatalf("Expected error: %v", dispatchErr)
				}

				// Check that we got an eval and job id back
				if dispatchResp.EvalID == "" || dispatchResp.DispatchedJobID == "" {
					t.Fatalf("Bad response")
				}

				state := s1.fsm.State()
				out, err := state.JobByID(dispatchResp.DispatchedJobID)
				if err != nil {
					t.Fatalf("err: %v", err)
				}
				if out == nil {
					t.Fatalf("expected job")
				}
				if out.CreateIndex != dispatchResp.JobCreateIndex {
					t.Fatalf("index mis-match")
				}
				if out.ParentID != tc.constructor.ID {
					t.Fatalf("bad parent ID")
				}

				// Lookup the evaluation
				eval, err := state.EvalByID(dispatchResp.EvalID)
				if err != nil {
					t.Fatalf("err: %v", err)
				}
				if eval == nil {
					t.Fatalf("expected eval")
				}
				if eval.CreateIndex != dispatchResp.EvalCreateIndex {
					t.Fatalf("index mis-match")
				}
			} else {
				if !tc.err {
					t.Fatalf("Got unexpected error: %v", dispatchErr)
				} else if !strings.Contains(dispatchErr.Error(), tc.errStr) {
					t.Fatalf("Expected err to include %q; got %v", tc.errStr, dispatchErr)
				}
			}
		})
	}
}
