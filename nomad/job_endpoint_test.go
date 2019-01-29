package nomad

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func TestJobEndpoint_Register(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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
	eval, err := state.EvalByID(ws, resp.EvalID)
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

func TestJobEndpoint_Register_ACL(t *testing.T) {
	t.Parallel()
	s1, root := TestACLServer(t, func(c *Config) {
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

	// Try without a token, expect failure
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err == nil {
		t.Fatalf("expected error")
	}

	// Try with a token
	req.AuthToken = root.SecretID
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check for the node in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
}

func TestJobEndpoint_Register_InvalidNamespace(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Namespace = "foo"
	req := &structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Try without a token, expect failure
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), "nonexistent namespace") {
		t.Fatalf("expected namespace error: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected no job")
	}
}

func TestJobEndpoint_Register_Payload(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
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
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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
	if out.Version != 1 {
		t.Fatalf("expected update")
	}

	// Lookup the evaluation
	eval, err := state.EvalByID(ws, resp.EvalID)
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

	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check to ensure the job version didn't get bumped because we submitted
	// the same job
	state = s1.fsm.State()
	ws = memdb.NewWatchSet()
	out, err = state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.Version != 1 {
		t.Fatalf("expected no update; got %v; diff %v", out.Version, pretty.Diff(job2, out))
	}
}

func TestJobEndpoint_Register_Periodic(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request for a periodic job.
	job := mock.PeriodicJob()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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

func TestJobEndpoint_Register_ParameterizedJob(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request for a parameterized job.
	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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
		t.Fatalf("Register created an eval for a parameterized job")
	}
}

func TestJobEndpoint_Register_Dispatched(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job with 'Dispatch' set to true
	job := mock.Job()
	job.Dispatched = true
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "job can't be submitted with 'Dispatched'")
}
func TestJobEndpoint_Register_EnforceIndex(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
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
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	out, err = state.JobByID(ws, job.Namespace, job.ID)
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
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
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
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), "Vault not enabled") {
		t.Fatalf("expected Vault not enabled error: %v", err)
	}
}

func TestJobEndpoint_Register_Vault_AllowUnauthenticated(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
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
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
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
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	if err == nil || !strings.Contains(err.Error(), "missing Vault Token") {
		t.Fatalf("expected Vault not enabled error: %v", err)
	}
}

func TestJobEndpoint_Register_Vault_Policies(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
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

	badToken := uuid.Generate()
	badPolicies := []string{"a", "b", "c"}
	tvc.SetLookupTokenAllowedPolicies(badToken, badPolicies)

	goodToken := uuid.Generate()
	goodPolicies := []string{"foo", "bar", "baz"}
	tvc.SetLookupTokenAllowedPolicies(goodToken, goodPolicies)

	rootToken := uuid.Generate()
	rootPolicies := []string{"root"}
	tvc.SetLookupTokenAllowedPolicies(rootToken, rootPolicies)

	errToken := uuid.Generate()
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
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	out, err = state.JobByID(ws, job2.Namespace, job2.ID)
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

func TestJobEndpoint_Revert(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the initial register request
	job := mock.Job()
	job.Priority = 100
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Reregister again to get another version
	job2 := job.Copy()
	job2.Priority = 1
	req = &structs.JobRegisterRequest{
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Create revert request and enforcing it be at an incorrect version
	revertReq := &structs.JobRevertRequest{
		JobID:               job.ID,
		JobVersion:          0,
		EnforcePriorVersion: helper.Uint64ToPtr(10),
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	if err == nil || !strings.Contains(err.Error(), "enforcing version 10") {
		t.Fatalf("expected enforcement error")
	}

	// Create revert request and enforcing it be at the current version
	revertReq = &structs.JobRevertRequest{
		JobID:      job.ID,
		JobVersion: 1,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	if err == nil || !strings.Contains(err.Error(), "current version") {
		t.Fatalf("expected current version err: %v", err)
	}

	// Create revert request and enforcing it be at version 1
	revertReq = &structs.JobRevertRequest{
		JobID:               job.ID,
		JobVersion:          0,
		EnforcePriorVersion: helper.Uint64ToPtr(1),
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}
	if resp.EvalID == "" || resp.EvalCreateIndex == 0 {
		t.Fatalf("bad created eval: %+v", resp)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad job modify index: %d", resp.JobModifyIndex)
	}

	// Create revert request and don't enforce. We are at version 2 but it is
	// the same as version 0
	revertReq = &structs.JobRevertRequest{
		JobID:      job.ID,
		JobVersion: 0,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}
	if resp.EvalID == "" || resp.EvalCreateIndex == 0 {
		t.Fatalf("bad created eval: %+v", resp)
	}
	if resp.JobModifyIndex == 0 {
		t.Fatalf("bad job modify index: %d", resp.JobModifyIndex)
	}

	// Check that the job is at the correct version and that the eval was
	// created
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if out.Priority != job.Priority {
		t.Fatalf("priority mis-match")
	}
	if out.Version != 2 {
		t.Fatalf("got version %d; want %d", out.Version, 2)
	}

	eout, err := state.EvalByID(ws, resp.EvalID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if eout == nil {
		t.Fatalf("expected eval")
	}
	if eout.JobID != job.ID {
		t.Fatalf("job id mis-match")
	}

	versions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("got %d versions; want %d", len(versions), 3)
	}
}

func TestJobEndpoint_Revert_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	state := s1.fsm.State()
	testutil.WaitForLeader(t, s1.RPC)

	// Create the job
	job := mock.Job()
	err := state.UpsertJob(300, job)
	require.Nil(err)

	job2 := job.Copy()
	job2.Priority = 1
	err = state.UpsertJob(400, job2)
	require.Nil(err)

	// Create revert request and enforcing it be at the current version
	revertReq := &structs.JobRevertRequest{
		JobID:      job.ID,
		JobVersion: 0,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch the response without a valid token
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	revertReq.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Fetch the response with a valid management token
	revertReq.AuthToken = root.SecretID
	var validResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &validResp)
	require.Nil(err)

	// Try with a valid non-management token
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))

	revertReq.AuthToken = validToken.SecretID
	var validResp2 structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Revert", revertReq, &validResp2)
	require.Nil(err)
}

func TestJobEndpoint_Stable(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the initial register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Create stability request
	stableReq := &structs.JobStabilityRequest{
		JobID:      job.ID,
		JobVersion: 0,
		Stable:     true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var stableResp structs.JobStabilityResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &stableResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if stableResp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Check that the job is marked stable
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("expected job")
	}
	if !out.Stable {
		t.Fatalf("Job is not marked stable")
	}
}

func TestJobEndpoint_Stable_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	state := s1.fsm.State()
	testutil.WaitForLeader(t, s1.RPC)

	// Register the job
	job := mock.Job()
	err := state.UpsertJob(1000, job)
	require.Nil(err)

	// Create stability request
	stableReq := &structs.JobStabilityRequest{
		JobID:      job.ID,
		JobVersion: 0,
		Stable:     true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch the token without a token
	var stableResp structs.JobStabilityResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &stableResp)
	require.NotNil(err)
	require.Contains("Permission denied", err.Error())

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	stableReq.AuthToken = invalidToken.SecretID
	var invalidStableResp structs.JobStabilityResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &invalidStableResp)
	require.NotNil(err)
	require.Contains("Permission denied", err.Error())

	// Attempt to fetch with a management token
	stableReq.AuthToken = root.SecretID
	var validStableResp structs.JobStabilityResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &validStableResp)
	require.Nil(err)

	// Attempt to fetch with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))

	stableReq.AuthToken = validToken.SecretID
	var validStableResp2 structs.JobStabilityResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Stable", stableReq, &validStableResp2)
	require.Nil(err)

	// Check that the job is marked stable
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	require.Nil(err)
	require.NotNil(job)
	require.Equal(true, out.Stable)
}

func TestJobEndpoint_Evaluate(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
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

func TestJobEndpoint_ForceRescheduleEvaluate(t *testing.T) {
	require := require.New(t)
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp)
	require.Nil(err)
	require.NotEqual(0, resp.Index)

	state := s1.fsm.State()
	job, err = state.JobByID(nil, structs.DefaultNamespace, job.ID)
	require.Nil(err)

	// Create a failed alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.Namespace = job.Namespace
	alloc.ClientStatus = structs.AllocClientStatusFailed
	err = s1.State().UpsertAllocs(resp.Index+1, []*structs.Allocation{alloc})
	require.Nil(err)

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID:       job.ID,
		EvalOptions: structs.EvalOptions{ForceReschedule: true},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp)
	require.Nil(err)
	require.NotEqual(0, resp.Index)

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	require.Nil(err)
	require.NotNil(eval)
	require.Equal(eval.CreateIndex, resp.EvalCreateIndex)
	require.Equal(eval.Priority, job.Priority)
	require.Equal(eval.Type, job.Type)
	require.Equal(eval.TriggeredBy, structs.EvalTriggerJobRegister)
	require.Equal(eval.JobID, job.ID)
	require.Equal(eval.JobModifyIndex, resp.JobModifyIndex)
	require.Equal(eval.Status, structs.EvalStatusPending)

	// Lookup the alloc, verify DesiredTransition ForceReschedule
	alloc, err = state.AllocByID(ws, alloc.ID)
	require.NotNil(alloc)
	require.Nil(err)
	require.True(*alloc.DesiredTransition.ForceReschedule)
}

func TestJobEndpoint_Evaluate_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create the job
	job := mock.Job()
	err := state.UpsertJob(300, job)
	require.Nil(err)

	// Force a re-evaluation
	reEval := &structs.JobEvaluateRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch the response without a token
	var resp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	reEval.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Fetch the response with a valid management token
	reEval.AuthToken = root.SecretID
	var validResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &validResp)
	require.Nil(err)

	// Fetch the response with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	reEval.AuthToken = validToken.SecretID
	var validResp2 structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &validResp2)
	require.Nil(err)

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, validResp2.EvalID)
	require.Nil(err)
	require.NotNil(eval)

	require.Equal(eval.CreateIndex, validResp2.EvalCreateIndex)
	require.Equal(eval.Priority, job.Priority)
	require.Equal(eval.Type, job.Type)
	require.Equal(eval.TriggeredBy, structs.EvalTriggerJobRegister)
	require.Equal(eval.JobID, job.ID)
	require.Equal(eval.JobModifyIndex, validResp2.JobModifyIndex)
	require.Equal(eval.Status, structs.EvalStatusPending)
}

func TestJobEndpoint_Evaluate_Periodic(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.PeriodicJob()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp); err == nil {
		t.Fatal("expect an err")
	}
}

func TestJobEndpoint_Evaluate_ParameterizedJob(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	if err := msgpackrpc.CallWithCodec(codec, "Job.Evaluate", reEval, &resp); err == nil {
		t.Fatal("expect an err")
	}
}

func TestJobEndpoint_Deregister(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register requests
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp))

	// Deregister but don't purge
	dereg := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobDeregisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg, &resp2))
	require.NotZero(resp2.Index)

	// Check for the job in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.Nil(err)
	require.NotNil(out)
	require.True(out.Stop)

	// Lookup the evaluation
	eval, err := state.EvalByID(nil, resp2.EvalID)
	require.Nil(err)
	require.NotNil(eval)
	require.EqualValues(resp2.EvalCreateIndex, eval.CreateIndex)
	require.Equal(job.Priority, eval.Priority)
	require.Equal(job.Type, eval.Type)
	require.Equal(structs.EvalTriggerJobDeregister, eval.TriggeredBy)
	require.Equal(job.ID, eval.JobID)
	require.Equal(structs.EvalStatusPending, eval.Status)

	// Deregister and purge
	dereg2 := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp3 structs.JobDeregisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deregister", dereg2, &resp3))
	require.NotZero(resp3.Index)

	// Check for the job in the FSM
	out, err = state.JobByID(nil, job.Namespace, job.ID)
	require.Nil(err)
	require.Nil(out)

	// Lookup the evaluation
	eval, err = state.EvalByID(nil, resp3.EvalID)
	require.Nil(err)
	require.NotNil(eval)

	require.EqualValues(resp3.EvalCreateIndex, eval.CreateIndex)
	require.Equal(job.Priority, eval.Priority)
	require.Equal(job.Type, eval.Type)
	require.Equal(structs.EvalTriggerJobDeregister, eval.TriggeredBy)
	require.Equal(job.ID, eval.JobID)
	require.Equal(structs.EvalStatusPending, eval.Status)
}

func TestJobEndpoint_Deregister_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create and register a job
	job := mock.Job()
	err := state.UpsertJob(100, job)

	// Deregister and purge
	req := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Expect failure for request without a token
	var resp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	req.AuthToken = invalidToken.SecretID

	var invalidResp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect success with a valid management token
	req.AuthToken = root.SecretID

	var validResp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req, &validResp)
	require.Nil(err)
	require.NotEqual(validResp.Index, 0)

	// Expect success with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
	req.AuthToken = validToken.SecretID

	var validResp2 structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deregister", req, &validResp2)
	require.Nil(err)
	require.NotEqual(validResp2.Index, 0)

	// Check for the job in the FSM
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.Nil(err)
	require.Nil(out)

	// Lookup the evaluation
	eval, err := state.EvalByID(nil, validResp2.EvalID)
	require.Nil(err)
	require.NotNil(eval, nil)

	require.Equal(eval.CreateIndex, validResp2.EvalCreateIndex)
	require.Equal(eval.Priority, structs.JobDefaultPriority)
	require.Equal(eval.Type, structs.JobTypeService)
	require.Equal(eval.TriggeredBy, structs.EvalTriggerJobDeregister)
	require.Equal(eval.JobID, job.ID)
	require.Equal(eval.JobModifyIndex, validResp2.JobModifyIndex)
	require.Equal(eval.Status, structs.EvalStatusPending)
}

func TestJobEndpoint_Deregister_Nonexistent(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Deregister
	jobID := "foo"
	dereg := &structs.JobDeregisterRequest{
		JobID: jobID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
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
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp2.EvalID)
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
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.PeriodicJob()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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

func TestJobEndpoint_Deregister_ParameterizedJob(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Deregister
	dereg := &structs.JobDeregisterRequest{
		JobID: job.ID,
		Purge: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("unexpected job")
	}

	if resp.EvalID != "" {
		t.Fatalf("Deregister created an eval for a parameterized job")
	}
}

func TestJobEndpoint_BatchDeregister(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register requests
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp))

	job2 := mock.Job()
	job2.Priority = 1
	reg2 := &structs.JobRegisterRequest{
		Job: job2,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job2.Namespace,
		},
	}

	// Fetch the response
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", reg2, &resp))

	// Deregister
	dereg := &structs.JobBatchDeregisterRequest{
		Jobs: map[structs.NamespacedID]*structs.JobDeregisterOptions{
			{
				ID:        job.ID,
				Namespace: job.Namespace,
			}: {},
			{
				ID:        job2.ID,
				Namespace: job2.Namespace,
			}: {
				Purge: true,
			},
		},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobBatchDeregisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.BatchDeregister", dereg, &resp2))
	require.NotZero(resp2.Index)

	// Check for the job in the FSM
	state := s1.fsm.State()
	out, err := state.JobByID(nil, job.Namespace, job.ID)
	require.Nil(err)
	require.NotNil(out)
	require.True(out.Stop)

	out, err = state.JobByID(nil, job2.Namespace, job2.ID)
	require.Nil(err)
	require.Nil(out)

	// Lookup the evaluation
	for jobNS, eval := range resp2.JobEvals {
		expectedJob := job
		if jobNS.ID != job.ID {
			expectedJob = job2
		}

		eval, err := state.EvalByID(nil, eval)
		require.Nil(err)
		require.NotNil(eval)
		require.EqualValues(resp2.Index, eval.CreateIndex)
		require.Equal(expectedJob.Priority, eval.Priority)
		require.Equal(expectedJob.Type, eval.Type)
		require.Equal(structs.EvalTriggerJobDeregister, eval.TriggeredBy)
		require.Equal(expectedJob.ID, eval.JobID)
		require.Equal(structs.EvalStatusPending, eval.Status)
	}
}

func TestJobEndpoint_BatchDeregister_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create and register a job
	job, job2 := mock.Job(), mock.Job()
	require.Nil(state.UpsertJob(100, job))
	require.Nil(state.UpsertJob(101, job2))

	// Deregister
	req := &structs.JobBatchDeregisterRequest{
		Jobs: map[structs.NamespacedID]*structs.JobDeregisterOptions{
			{
				ID:        job.ID,
				Namespace: job.Namespace,
			}: {},
			{
				ID:        job2.ID,
				Namespace: job2.Namespace,
			}: {},
		},
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}

	// Expect failure for request without a token
	var resp structs.JobBatchDeregisterResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.BatchDeregister", req, &resp)
	require.NotNil(err)
	require.True(structs.IsErrPermissionDenied(err))

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	req.AuthToken = invalidToken.SecretID

	var invalidResp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.BatchDeregister", req, &invalidResp)
	require.NotNil(err)
	require.True(structs.IsErrPermissionDenied(err))

	// Expect success with a valid management token
	req.AuthToken = root.SecretID

	var validResp structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.BatchDeregister", req, &validResp)
	require.Nil(err)
	require.NotEqual(validResp.Index, 0)

	// Expect success with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
	req.AuthToken = validToken.SecretID

	var validResp2 structs.JobDeregisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.BatchDeregister", req, &validResp2)
	require.Nil(err)
	require.NotEqual(validResp2.Index, 0)
}

func TestJobEndpoint_GetJob(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
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

	// Clear the submit times
	j.SubmitTime = 0
	resp2.Job.SubmitTime = 0

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

func TestJobEndpoint_GetJob_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create the job
	job := mock.Job()
	err := state.UpsertJob(1000, job)
	require.Nil(err)

	// Lookup the job
	get := &structs.JobSpecificRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Looking up the job without a token should fail
	var resp structs.SingleJobResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.SingleJobResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Looking up the job with a management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.SingleJobResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &validResp)
	require.Nil(err)
	require.Equal(job.ID, validResp.Job.ID)

	// Looking up the job with a valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.SingleJobResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJob", get, &validResp2)
	require.Nil(err)
	require.Equal(job.ID, validResp2.Job.ID)
}

func TestJobEndpoint_GetJob_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
			Namespace:     job2.Namespace,
			MinQueryIndex: 150,
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
		if err := state.DeleteJob(300, job2.Namespace, job2.ID); err != nil {
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

func TestJobEndpoint_GetJobVersions(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Priority = 88
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register the job again to create another version
	job.Priority = 100
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the job
	get := &structs.JobVersionsRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var versionsResp structs.JobVersionsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &versionsResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if versionsResp.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", versionsResp.Index, resp.Index)
	}

	// Make sure there are two job versions
	versions := versionsResp.Versions
	if l := len(versions); l != 2 {
		t.Fatalf("Got %d versions; want 2", l)
	}

	if v := versions[0]; v.Priority != 100 || v.ID != job.ID || v.Version != 1 {
		t.Fatalf("bad: %+v", v)
	}
	if v := versions[1]; v.Priority != 88 || v.ID != job.ID || v.Version != 0 {
		t.Fatalf("bad: %+v", v)
	}

	// Lookup non-existing job
	get.JobID = "foobarbaz"
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &versionsResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if versionsResp.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", versionsResp.Index, resp.Index)
	}
	if l := len(versionsResp.Versions); l != 0 {
		t.Fatalf("unexpected versions: %d", l)
	}
}

func TestJobEndpoint_GetJobVersions_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create two versions of a job with different priorities
	job := mock.Job()
	job.Priority = 88
	err := state.UpsertJob(10, job)
	require.Nil(err)

	job.Priority = 100
	err = state.UpsertJob(100, job)
	require.Nil(err)

	// Lookup the job
	get := &structs.JobVersionsRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch without a token should fail
	var resp structs.JobVersionsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobVersionsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect success for request with a valid management token
	get.AuthToken = root.SecretID
	var validResp structs.JobVersionsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &validResp)
	require.Nil(err)

	// Expect success for request with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.JobVersionsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &validResp2)
	require.Nil(err)

	// Make sure there are two job versions
	versions := validResp2.Versions
	require.Equal(2, len(versions))
	require.Equal(versions[0].ID, job.ID)
	require.Equal(versions[1].ID, job.ID)
}

func TestJobEndpoint_GetJobVersions_Diff(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	job.Priority = 88
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register the job again to create another version
	job.Priority = 90
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register the job again to create another version
	job.Priority = 100
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the job
	get := &structs.JobVersionsRequest{
		JobID: job.ID,
		Diffs: true,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var versionsResp structs.JobVersionsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", get, &versionsResp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if versionsResp.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", versionsResp.Index, resp.Index)
	}

	// Make sure there are two job versions
	versions := versionsResp.Versions
	if l := len(versions); l != 3 {
		t.Fatalf("Got %d versions; want 3", l)
	}

	if v := versions[0]; v.Priority != 100 || v.ID != job.ID || v.Version != 2 {
		t.Fatalf("bad: %+v", v)
	}
	if v := versions[1]; v.Priority != 90 || v.ID != job.ID || v.Version != 1 {
		t.Fatalf("bad: %+v", v)
	}
	if v := versions[2]; v.Priority != 88 || v.ID != job.ID || v.Version != 0 {
		t.Fatalf("bad: %+v", v)
	}

	// Ensure we got diffs
	diffs := versionsResp.Diffs
	if l := len(diffs); l != 2 {
		t.Fatalf("Got %d diffs; want 2", l)
	}
	d1 := diffs[0]
	if len(d1.Fields) != 1 {
		t.Fatalf("Got too many diffs: %#v", d1)
	}
	if d1.Fields[0].Name != "Priority" {
		t.Fatalf("Got wrong field: %#v", d1)
	}
	if d1.Fields[0].Old != "90" && d1.Fields[0].New != "100" {
		t.Fatalf("Got wrong field values: %#v", d1)
	}
	d2 := diffs[1]
	if len(d2.Fields) != 1 {
		t.Fatalf("Got too many diffs: %#v", d2)
	}
	if d2.Fields[0].Name != "Priority" {
		t.Fatalf("Got wrong field: %#v", d2)
	}
	if d2.Fields[0].Old != "88" && d1.Fields[0].New != "90" {
		t.Fatalf("Got wrong field values: %#v", d2)
	}
}

func TestJobEndpoint_GetJobVersions_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the jobs
	job1 := mock.Job()
	job2 := mock.Job()
	job3 := mock.Job()
	job3.ID = job2.ID
	job3.Priority = 1

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

	req := &structs.JobVersionsRequest{
		JobID: job2.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     job2.Namespace,
			MinQueryIndex: 150,
		},
	}
	start := time.Now()
	var resp structs.JobVersionsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Versions) != 1 || resp.Versions[0].ID != job2.ID {
		t.Fatalf("bad: %#v", resp.Versions)
	}

	// Upsert the job again which should trigger the watch.
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertJob(300, job3); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req2 := &structs.JobVersionsRequest{
		JobID: job3.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     job3.Namespace,
			MinQueryIndex: 250,
		},
	}
	var resp2 structs.JobVersionsResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Job.GetJobVersions", req2, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp.Index, 300)
	}
	if len(resp2.Versions) != 2 {
		t.Fatalf("bad: %#v", resp2.Versions)
	}
}

func TestJobEndpoint_GetJobSummary(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp2 structs.JobSummaryResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Summary", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != resp.JobModifyIndex {
		t.Fatalf("Bad index: %d %d", resp2.Index, resp.Index)
	}

	expectedJobSummary := structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: job.CreateIndex,
		ModifyIndex: job.CreateIndex,
	}

	if !reflect.DeepEqual(resp2.JobSummary, &expectedJobSummary) {
		t.Fatalf("expected: %v, actual: %v", expectedJobSummary, resp2.JobSummary)
	}
}

func TestJobEndpoint_Summary_ACL(t *testing.T) {
	require := require.New(t)
	t.Parallel()

	srv, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer srv.Shutdown()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)

	// Create the job
	job := mock.Job()
	reg := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	reg.AuthToken = root.SecretID

	var err error

	// Register the job with a valid token
	var regResp structs.JobRegisterResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Register", reg, &regResp)
	require.Nil(err)

	job.CreateIndex = regResp.JobModifyIndex
	job.ModifyIndex = regResp.JobModifyIndex
	job.JobModifyIndex = regResp.JobModifyIndex

	req := &structs.JobSummaryRequest{
		JobID: job.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Expect failure for request without a token
	var resp structs.JobSummaryResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &resp)
	require.NotNil(err)

	expectedJobSummary := &structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: job.CreateIndex,
		ModifyIndex: job.ModifyIndex,
	}

	// Expect success when using a management token
	req.AuthToken = root.SecretID
	var mgmtResp structs.JobSummaryResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &mgmtResp)
	require.Nil(err)
	require.Equal(expectedJobSummary, mgmtResp.JobSummary)

	// Create the namespace policy and tokens
	state := srv.fsm.State()

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	req.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobSummaryResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &invalidResp)
	require.NotNil(err)

	// Try with a valid token
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	req.AuthToken = validToken.SecretID
	var authResp structs.JobSummaryResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Summary", req, &authResp)
	require.Nil(err)
	require.Equal(expectedJobSummary, authResp.JobSummary)
}

func TestJobEndpoint_GetJobSummary_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
			Namespace:     job1.Namespace,
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
			Namespace:     job1.Namespace,
			MinQueryIndex: 199,
		},
	}
	start = time.Now()
	var resp1 structs.JobSummaryResponse
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
		if err := state.DeleteJob(300, job1.Namespace, job1.ID); err != nil {
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

func TestJobEndpoint_ListJobs(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
			Prefix:    resp2.Jobs[0].ID[:4],
		},
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

func TestJobEndpoint_ListJobs_WithACL(t *testing.T) {
	require := require.New(t)
	t.Parallel()

	srv, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer srv.Shutdown()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)
	state := srv.fsm.State()

	var err error

	// Create the register request
	job := mock.Job()
	err = state.UpsertJob(1000, job)
	require.Nil(err)

	req := &structs.JobListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Expect failure for request without a token
	var resp structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", req, &resp)
	require.NotNil(err)

	// Expect success for request with a management token
	var mgmtResp structs.JobListResponse
	req.AuthToken = root.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Job.List", req, &mgmtResp)
	require.Nil(err)
	require.Equal(1, len(mgmtResp.Jobs))
	require.Equal(job.ID, mgmtResp.Jobs[0].ID)

	// Expect failure for request with a token that has incorrect permissions
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	req.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.List", req, &invalidResp)
	require.NotNil(err)

	// Try with a valid token with correct permissions
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	var validResp structs.JobListResponse
	req.AuthToken = validToken.SecretID

	err = msgpackrpc.CallWithCodec(codec, "Job.List", req, &validResp)
	require.Nil(err)
	require.Equal(1, len(validResp.Jobs))
	require.Equal(job.ID, validResp.Jobs[0].ID)
}

func TestJobEndpoint_ListJobs_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
			Namespace:     job.Namespace,
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
		t.Fatalf("bad: %#v", resp)
	}

	// Job deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteJob(200, job.Namespace, job.ID); err != nil {
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
		t.Fatalf("bad: %#v", resp2)
	}
}

func TestJobEndpoint_Allocations(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
		JobID: alloc1.JobID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: alloc1.Job.Namespace,
		},
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

func TestJobEndpoint_Allocations_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create allocations for a job
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.JobID = alloc1.JobID
	state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
	err := state.UpsertAllocs(1000,
		[]*structs.Allocation{alloc1, alloc2})
	require.Nil(err)

	// Look up allocations for that job
	get := &structs.JobSpecificRequest{
		JobID: alloc1.JobID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: alloc1.Job.Namespace,
		},
	}

	// Attempt to fetch the response without a token should fail
	var resp structs.JobAllocationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token should fail
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobAllocationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with valid management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.JobAllocationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &validResp)
	require.Nil(err)

	// Attempt to fetch the response with valid management token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.JobAllocationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Allocations", get, &validResp2)
	require.Nil(err)

	require.Equal(2, len(validResp2.Allocations))
}

func TestJobEndpoint_Allocations_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
			Namespace:     alloc1.Job.Namespace,
			MinQueryIndex: 150,
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
	t.Parallel()
	s1 := TestServer(t, nil)
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
		JobID: eval1.JobID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
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

func TestJobEndpoint_Evaluations_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create evaluations for the same job
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	err := state.UpsertEvals(1000,
		[]*structs.Evaluation{eval1, eval2})
	require.Nil(err)

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: eval1.JobID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	// Attempt to fetch without providing a token
	var resp structs.JobEvaluationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.JobEvaluationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch with valid management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.JobEvaluationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &validResp)
	require.Nil(err)
	require.Equal(2, len(validResp.Evaluations))

	// Attempt to fetch with valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.JobEvaluationsResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Evaluations", get, &validResp2)
	require.Nil(err)
	require.Equal(2, len(validResp2.Evaluations))
}

func TestJobEndpoint_Evaluations_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
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
			Namespace:     eval1.Namespace,
			MinQueryIndex: 150,
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

func TestJobEndpoint_Deployments(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	require := require.New(t)

	// Create the register request
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d1.JobID = j.ID
	d2.JobID = j.ID
	require.Nil(state.UpsertJob(1000, j), "UpsertJob")
	require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
	require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: j.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: j.Namespace,
		},
	}
	var resp structs.DeploymentListResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &resp), "RPC")
	require.EqualValues(1002, resp.Index, "response index")
	require.Len(resp.Deployments, 2, "deployments for job")
}

func TestJobEndpoint_Deployments_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create a job and corresponding deployments
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d1.JobID = j.ID
	d2.JobID = j.ID
	require.Nil(state.UpsertJob(1000, j), "UpsertJob")
	require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
	require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: j.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: j.Namespace,
		},
	}
	// Lookup with no token should fail
	var resp structs.DeploymentListResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.DeploymentListResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Lookup with valid management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.DeploymentListResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &validResp), "RPC")
	require.EqualValues(1002, validResp.Index, "response index")
	require.Len(validResp.Deployments, 2, "deployments for job")

	// Lookup with valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1005, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.DeploymentListResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &validResp2), "RPC")
	require.EqualValues(1002, validResp2.Index, "response index")
	require.Len(validResp2.Deployments, 2, "deployments for job")
}

func TestJobEndpoint_Deployments_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	require := require.New(t)

	// Create the register request
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d2.JobID = j.ID
	require.Nil(state.UpsertJob(50, j), "UpsertJob")

	// First upsert an unrelated eval
	time.AfterFunc(100*time.Millisecond, func() {
		require.Nil(state.UpsertDeployment(100, d1), "UpsertDeployment")
	})

	// Upsert an eval for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		require.Nil(state.UpsertDeployment(200, d2), "UpsertDeployment")
	})

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: d2.JobID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     d2.Namespace,
			MinQueryIndex: 150,
		},
	}
	var resp structs.DeploymentListResponse
	start := time.Now()
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Deployments", get, &resp), "RPC")
	require.EqualValues(200, resp.Index, "response index")
	require.Len(resp.Deployments, 1, "deployments for job")
	require.Equal(d2.ID, resp.Deployments[0].ID, "returned deployment")
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
}

func TestJobEndpoint_LatestDeployment(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	require := require.New(t)

	// Create the register request
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d1.JobID = j.ID
	d2.JobID = j.ID
	d2.CreateIndex = d1.CreateIndex + 100
	d2.ModifyIndex = d2.CreateIndex + 100
	require.Nil(state.UpsertJob(1000, j), "UpsertJob")
	require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
	require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: j.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: j.Namespace,
		},
	}
	var resp structs.SingleDeploymentResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &resp), "RPC")
	require.EqualValues(1002, resp.Index, "response index")
	require.NotNil(resp.Deployment, "want a deployment")
	require.Equal(d2.ID, resp.Deployment.ID, "latest deployment for job")
}

func TestJobEndpoint_LatestDeployment_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create a job and deployments
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d1.JobID = j.ID
	d2.JobID = j.ID
	d2.CreateIndex = d1.CreateIndex + 100
	d2.ModifyIndex = d2.CreateIndex + 100
	require.Nil(state.UpsertJob(1000, j), "UpsertJob")
	require.Nil(state.UpsertDeployment(1001, d1), "UpsertDeployment")
	require.Nil(state.UpsertDeployment(1002, d2), "UpsertDeployment")

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: j.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: j.Namespace,
		},
	}

	// Attempt to fetch the response without a token should fail
	var resp structs.SingleDeploymentResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token should fail
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get.AuthToken = invalidToken.SecretID
	var invalidResp structs.SingleDeploymentResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Fetching latest deployment with a valid management token should succeed
	get.AuthToken = root.SecretID
	var validResp structs.SingleDeploymentResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &validResp), "RPC")
	require.EqualValues(1002, validResp.Index, "response index")
	require.NotNil(validResp.Deployment, "want a deployment")
	require.Equal(d2.ID, validResp.Deployment.ID, "latest deployment for job")

	// Fetching latest deployment with a valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1004, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	get.AuthToken = validToken.SecretID
	var validResp2 structs.SingleDeploymentResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &validResp2), "RPC")
	require.EqualValues(1002, validResp2.Index, "response index")
	require.NotNil(validResp2.Deployment, "want a deployment")
	require.Equal(d2.ID, validResp2.Deployment.ID, "latest deployment for job")
}

func TestJobEndpoint_LatestDeployment_Blocking(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	require := require.New(t)

	// Create the register request
	j := mock.Job()
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	d2.JobID = j.ID
	require.Nil(state.UpsertJob(50, j), "UpsertJob")

	// First upsert an unrelated eval
	time.AfterFunc(100*time.Millisecond, func() {
		require.Nil(state.UpsertDeployment(100, d1), "UpsertDeployment")
	})

	// Upsert an eval for the job we are interested in later
	time.AfterFunc(200*time.Millisecond, func() {
		require.Nil(state.UpsertDeployment(200, d2), "UpsertDeployment")
	})

	// Lookup the jobs
	get := &structs.JobSpecificRequest{
		JobID: d2.JobID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     d2.Namespace,
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleDeploymentResponse
	start := time.Now()
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.LatestDeployment", get, &resp), "RPC")
	require.EqualValues(200, resp.Index, "response index")
	require.NotNil(resp.Deployment, "deployment for job")
	require.Equal(d2.ID, resp.Deployment.ID, "returned deployment")
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
}

func TestJobEndpoint_Plan_ACL(t *testing.T) {
	t.Parallel()
	s1, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create a plan request
	job := mock.Job()
	planReq := &structs.JobPlanRequest{
		Job:  job,
		Diff: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Try without a token, expect failure
	var planResp structs.JobPlanResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp); err == nil {
		t.Fatalf("expected error")
	}

	// Try with a token
	planReq.AuthToken = root.SecretID
	if err := msgpackrpc.CallWithCodec(codec, "Job.Plan", planReq, &planResp); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestJobEndpoint_Plan_WithDiff(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		Job:  job,
		Diff: true,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	job := mock.Job()
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
		Job:  job,
		Diff: false,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
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
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
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
	goodToken := uuid.Generate()
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
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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
	t.Parallel()
	s1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request with a job asking for a template that sends a
	// signal
	job := mock.Job()
	signal1 := "SIGUSR1"
	signal2 := "SIGHUP"
	job.TaskGroups[0].Tasks[0].Templates = []*structs.Template{
		{
			SourcePath:   "foo",
			DestPath:     "bar",
			ChangeMode:   structs.TemplateChangeModeSignal,
			ChangeSignal: signal1,
		},
		{
			SourcePath:   "foo",
			DestPath:     "baz",
			ChangeMode:   structs.TemplateChangeModeSignal,
			ChangeSignal: signal2,
		},
	}
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check for the job in the FSM
	state := s1.fsm.State()
	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
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

	sigConstraint := getSignalConstraint([]string{signal1, signal2})
	if !strings.HasPrefix(sigConstraint.RTarget, "SIGHUP") {
		t.Fatalf("signals not sorted: %v", sigConstraint.RTarget)
	}

	if !constraints[0].Equal(sigConstraint) {
		t.Fatalf("Expected implicit vault constraint")
	}
}

func TestJobEndpoint_ValidateJobUpdate(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	old := mock.Job()
	new := mock.Job()

	if err := validateJobUpdate(old, new); err != nil {
		t.Errorf("expected update to be valid but got: %v", err)
	}

	new.Type = "batch"
	if err := validateJobUpdate(old, new); err == nil {
		t.Errorf("expected err when setting new job to a different type")
	} else {
		t.Log(err)
	}

	new = mock.Job()
	new.Periodic = &structs.PeriodicConfig{Enabled: true}
	if err := validateJobUpdate(old, new); err == nil {
		t.Errorf("expected err when setting new job to periodic")
	} else {
		t.Log(err)
	}

	new = mock.Job()
	new.ParameterizedJob = &structs.ParameterizedJobConfig{}
	if err := validateJobUpdate(old, new); err == nil {
		t.Errorf("expected err when setting new job to parameterized")
	} else {
		t.Log(err)
	}

	new = mock.Job()
	new.Dispatched = true
	require.Error(validateJobUpdate(old, new),
		"expected err when setting new job to dispatched")
	require.Error(validateJobUpdate(nil, new),
		"expected err when setting new job to dispatched")
	require.Error(validateJobUpdate(new, old),
		"expected err when setting dispatched to false")
	require.NoError(validateJobUpdate(nil, old))
}

func TestJobEndpoint_ValidateJobUpdate_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	job := mock.Job()

	req := &structs.JobValidateRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to update without providing a valid token
	var resp structs.JobValidateResponse
	err := msgpackrpc.CallWithCodec(codec, "Job.Validate", req, &resp)
	require.NotNil(err)

	// Update with a valid token
	req.AuthToken = root.SecretID
	var validResp structs.JobValidateResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Validate", req, &validResp)
	require.Nil(err)

	require.Equal("", validResp.Error)
	require.Equal("", validResp.Warnings)
}

func TestJobEndpoint_Dispatch_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})

	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create a parameterized job
	job := mock.BatchJob()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}
	err := state.UpsertJob(400, job)
	require.Nil(err)

	req := &structs.JobDispatchRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Attempt to fetch the response without a token should fail
	var resp structs.JobDispatchResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Attempt to fetch the response with an invalid token should fail
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
	req.AuthToken = invalidToken.SecretID

	var invalidResp structs.JobDispatchResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", req, &invalidResp)
	require.NotNil(err)
	require.Contains(err.Error(), "Permission denied")

	// Dispatch with a valid management token should succeed
	req.AuthToken = root.SecretID

	var validResp structs.JobDispatchResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", req, &validResp)
	require.Nil(err)
	require.NotNil(validResp.EvalID)
	require.NotNil(validResp.DispatchedJobID)
	require.NotEqual(validResp.DispatchedJobID, "")

	// Dispatch with a valid token should succeed
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityDispatchJob}))
	req.AuthToken = validToken.SecretID

	var validResp2 structs.JobDispatchResponse
	err = msgpackrpc.CallWithCodec(codec, "Job.Dispatch", req, &validResp2)
	require.Nil(err)
	require.NotNil(validResp2.EvalID)
	require.NotNil(validResp2.DispatchedJobID)
	require.NotEqual(validResp2.DispatchedJobID, "")

	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, validResp2.DispatchedJobID)
	require.Nil(err)
	require.NotNil(out)
	require.Equal(out.ParentID, job.ID)

	// Look up the evaluation
	eval, err := state.EvalByID(ws, validResp2.EvalID)
	require.Nil(err)
	require.NotNil(eval)
	require.Equal(eval.CreateIndex, validResp2.EvalCreateIndex)
}

func TestJobEndpoint_Dispatch(t *testing.T) {
	t.Parallel()

	// No requirements
	d1 := mock.BatchJob()
	d1.ParameterizedJob = &structs.ParameterizedJobConfig{}

	// Require input data
	d2 := mock.BatchJob()
	d2.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadRequired,
	}

	// Disallow input data
	d3 := mock.BatchJob()
	d3.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadForbidden,
	}

	// Require meta
	d4 := mock.BatchJob()
	d4.ParameterizedJob = &structs.ParameterizedJobConfig{
		MetaRequired: []string{"foo", "bar"},
	}

	// Optional meta
	d5 := mock.BatchJob()
	d5.ParameterizedJob = &structs.ParameterizedJobConfig{
		MetaOptional: []string{"foo", "bar"},
	}

	// Periodic dispatch job
	d6 := mock.PeriodicJob()
	d6.ParameterizedJob = &structs.ParameterizedJobConfig{}

	d7 := mock.BatchJob()
	d7.ParameterizedJob = &structs.ParameterizedJobConfig{}
	d7.Stop = true

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
		name             string
		parameterizedJob *structs.Job
		dispatchReq      *structs.JobDispatchRequest
		noEval           bool
		err              bool
		errStr           string
	}
	cases := []testCase{
		{
			name:             "optional input data w/ data",
			parameterizedJob: d1,
			dispatchReq:      reqInputDataNoMeta,
			err:              false,
		},
		{
			name:             "optional input data w/o data",
			parameterizedJob: d1,
			dispatchReq:      reqNoInputNoMeta,
			err:              false,
		},
		{
			name:             "require input data w/ data",
			parameterizedJob: d2,
			dispatchReq:      reqInputDataNoMeta,
			err:              false,
		},
		{
			name:             "require input data w/o data",
			parameterizedJob: d2,
			dispatchReq:      reqNoInputNoMeta,
			err:              true,
			errStr:           "not provided but required",
		},
		{
			name:             "disallow input data w/o data",
			parameterizedJob: d3,
			dispatchReq:      reqNoInputNoMeta,
			err:              false,
		},
		{
			name:             "disallow input data w/ data",
			parameterizedJob: d3,
			dispatchReq:      reqInputDataNoMeta,
			err:              true,
			errStr:           "provided but forbidden",
		},
		{
			name:             "require meta w/ meta",
			parameterizedJob: d4,
			dispatchReq:      reqInputDataMeta,
			err:              false,
		},
		{
			name:             "require meta w/o meta",
			parameterizedJob: d4,
			dispatchReq:      reqNoInputNoMeta,
			err:              true,
			errStr:           "did not provide required meta keys",
		},
		{
			name:             "optional meta w/ meta",
			parameterizedJob: d5,
			dispatchReq:      reqNoInputDataMeta,
			err:              false,
		},
		{
			name:             "optional meta w/o meta",
			parameterizedJob: d5,
			dispatchReq:      reqNoInputNoMeta,
			err:              false,
		},
		{
			name:             "optional meta w/ bad meta",
			parameterizedJob: d5,
			dispatchReq:      reqBadMeta,
			err:              true,
			errStr:           "unpermitted metadata keys",
		},
		{
			name:             "optional input w/ too big of input",
			parameterizedJob: d1,
			dispatchReq:      reqInputDataTooLarge,
			err:              true,
			errStr:           "Payload exceeds maximum size",
		},
		{
			name:             "periodic job dispatched, ensure no eval",
			parameterizedJob: d6,
			dispatchReq:      reqNoInputNoMeta,
			noEval:           true,
		},
		{
			name:             "periodic job stopped, ensure error",
			parameterizedJob: d7,
			dispatchReq:      reqNoInputNoMeta,
			err:              true,
			errStr:           "stopped",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s1 := TestServer(t, func(c *Config) {
				c.NumSchedulers = 0 // Prevent automatic dequeue
			})
			defer s1.Shutdown()
			codec := rpcClient(t, s1)
			testutil.WaitForLeader(t, s1.RPC)

			// Create the register request
			regReq := &structs.JobRegisterRequest{
				Job: tc.parameterizedJob,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: tc.parameterizedJob.Namespace,
				},
			}

			// Fetch the response
			var regResp structs.JobRegisterResponse
			if err := msgpackrpc.CallWithCodec(codec, "Job.Register", regReq, &regResp); err != nil {
				t.Fatalf("err: %v", err)
			}

			// Now try to dispatch
			tc.dispatchReq.JobID = tc.parameterizedJob.ID
			tc.dispatchReq.WriteRequest = structs.WriteRequest{
				Region:    "global",
				Namespace: tc.parameterizedJob.Namespace,
			}

			var dispatchResp structs.JobDispatchResponse
			dispatchErr := msgpackrpc.CallWithCodec(codec, "Job.Dispatch", tc.dispatchReq, &dispatchResp)

			if dispatchErr == nil {
				if tc.err {
					t.Fatalf("Expected error: %v", dispatchErr)
				}

				// Check that we got an eval and job id back
				switch dispatchResp.EvalID {
				case "":
					if !tc.noEval {
						t.Fatalf("Bad response")
					}
				default:
					if tc.noEval {
						t.Fatalf("Got eval %q", dispatchResp.EvalID)
					}
				}

				if dispatchResp.DispatchedJobID == "" {
					t.Fatalf("Bad response")
				}

				state := s1.fsm.State()
				ws := memdb.NewWatchSet()
				out, err := state.JobByID(ws, tc.parameterizedJob.Namespace, dispatchResp.DispatchedJobID)
				if err != nil {
					t.Fatalf("err: %v", err)
				}
				if out == nil {
					t.Fatalf("expected job")
				}
				if out.CreateIndex != dispatchResp.JobCreateIndex {
					t.Fatalf("index mis-match")
				}
				if out.ParentID != tc.parameterizedJob.ID {
					t.Fatalf("bad parent ID")
				}
				if !out.Dispatched {
					t.Fatal("expected dispatched job")
				}
				if out.IsParameterized() {
					t.Fatal("dispatched job should not be parameterized")
				}
				if out.ParameterizedJob == nil {
					t.Fatal("parameter job config should exist")
				}

				if tc.noEval {
					return
				}

				// Lookup the evaluation
				eval, err := state.EvalByID(ws, dispatchResp.EvalID)
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
