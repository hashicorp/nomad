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
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalEndpoint_GetEval(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})

	// Lookup the eval
	get := &structs.EvalSpecificRequest{
		EvalID:       eval1.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.SingleEvalResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.GetEval", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if !reflect.DeepEqual(eval1, resp.Eval) {
		t.Fatalf("bad: %#v %#v", eval1, resp.Eval)
	}

	// Lookup non-existing node
	get.EvalID = uuid.Generate()
	if err := msgpackrpc.CallWithCodec(codec, "Eval.GetEval", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}
	if resp.Eval != nil {
		t.Fatalf("unexpected eval")
	}
}

func TestEvalEndpoint_GetEval_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	eval1 := mock.Eval()
	state := s1.fsm.State()
	state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})

	// Create ACL tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get := &structs.EvalSpecificRequest{
		EvalID:       eval1.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.SingleEvalResponse
		err := msgpackrpc.CallWithCodec(codec, "Eval.GetEval", get, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		get.AuthToken = invalidToken.SecretID
		var resp structs.SingleEvalResponse
		err := msgpackrpc.CallWithCodec(codec, "Eval.GetEval", get, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Lookup the eval using a valid token
	{
		get.AuthToken = validToken.SecretID
		var resp structs.SingleEvalResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Eval.GetEval", get, &resp))
		assert.Equal(uint64(1000), resp.Index, "Bad index: %d %d", resp.Index, 1000)
		assert.Equal(eval1, resp.Eval)
	}

	// Lookup the eval using a root token
	{
		get.AuthToken = root.SecretID
		var resp structs.SingleEvalResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Eval.GetEval", get, &resp))
		assert.Equal(uint64(1000), resp.Index, "Bad index: %d %d", resp.Index, 1000)
		assert.Equal(eval1, resp.Eval)
	}
}

func TestEvalEndpoint_GetEval_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the evals
	eval1 := mock.Eval()
	eval2 := mock.Eval()

	// First create an unrelated eval
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 100, []*structs.Evaluation{eval1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the eval we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 200, []*structs.Evaluation{eval2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the eval
	req := &structs.EvalSpecificRequest{
		EvalID: eval2.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleEvalResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Eval.GetEval", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Eval == nil || resp.Eval.ID != eval2.ID {
		t.Fatalf("bad: %#v", resp.Eval)
	}

	// Eval delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.DeleteEval(300, []string{eval2.ID}, []string{})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleEvalResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Eval.GetEval", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Eval != nil {
		t.Fatalf("bad: %#v", resp2.Eval)
	}
}

func TestEvalEndpoint_Dequeue(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	// Dequeue the eval
	get := &structs.EvalDequeueRequest{
		Schedulers:       defaultSched,
		SchedulerVersion: scheduler.SchedulerVersion,
		WriteRequest:     structs.WriteRequest{Region: "global"},
	}
	var resp structs.EvalDequeueResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Dequeue", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval1, resp.Eval) {
		t.Fatalf("bad: %v %v", eval1, resp.Eval)
	}

	// Ensure outstanding
	token, ok := s1.evalBroker.Outstanding(eval1.ID)
	if !ok {
		t.Fatalf("should be outstanding")
	}
	if token != resp.Token {
		t.Fatalf("bad token: %#v %#v", token, resp.Token)
	}

	if resp.WaitIndex != eval1.ModifyIndex {
		t.Fatalf("bad wait index; got %d; want %d", resp.WaitIndex, eval1.ModifyIndex)
	}
}

// TestEvalEndpoint_Dequeue_WaitIndex_Snapshot asserts that an eval's wait
// index will be equal to the highest eval modify index in the state store.
func TestEvalEndpoint_Dequeue_WaitIndex_Snapshot(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})
	s1.evalBroker.Enqueue(eval1)
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval2})

	// Dequeue the eval
	get := &structs.EvalDequeueRequest{
		Schedulers:       defaultSched,
		SchedulerVersion: scheduler.SchedulerVersion,
		WriteRequest:     structs.WriteRequest{Region: "global"},
	}
	var resp structs.EvalDequeueResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Dequeue", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval1, resp.Eval) {
		t.Fatalf("bad: %v %v", eval1, resp.Eval)
	}

	// Ensure outstanding
	token, ok := s1.evalBroker.Outstanding(eval1.ID)
	if !ok {
		t.Fatalf("should be outstanding")
	}
	if token != resp.Token {
		t.Fatalf("bad token: %#v %#v", token, resp.Token)
	}

	if resp.WaitIndex != 1001 {
		t.Fatalf("bad wait index; got %d; want %d", resp.WaitIndex, 1001)
	}
}

// TestEvalEndpoint_Dequeue_WaitIndex_Eval asserts that an eval's wait index
// will be its own modify index if its modify index is greater than all of the
// indexes in the state store. This can happen if Dequeue receives an eval that
// has not yet been applied from the Raft log to the local node's state store.
func TestEvalEndpoint_Dequeue_WaitIndex_Eval(t *testing.T) {
	t.Parallel()
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request but only upsert 1 into the state store
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})
	eval2.ModifyIndex = 1001
	s1.evalBroker.Enqueue(eval2)

	// Dequeue the eval
	get := &structs.EvalDequeueRequest{
		Schedulers:       defaultSched,
		SchedulerVersion: scheduler.SchedulerVersion,
		WriteRequest:     structs.WriteRequest{Region: "global"},
	}
	var resp structs.EvalDequeueResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Eval.Dequeue", get, &resp))
	require.Equal(t, eval2, resp.Eval)

	// Ensure outstanding
	token, ok := s1.evalBroker.Outstanding(eval2.ID)
	require.True(t, ok)
	require.Equal(t, resp.Token, token)

	// WaitIndex should be equal to the max ModifyIndex - even when that
	// modify index is of the dequeued eval which has yet to be applied to
	// the state store.
	require.Equal(t, eval2.ModifyIndex, resp.WaitIndex)
}

func TestEvalEndpoint_Dequeue_UpdateWaitIndex(t *testing.T) {
	// test enqueuing an eval, updating a plan result for the same eval and de-queueing the eval
	t.Parallel()
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	state := s1.fsm.State()

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	if err := state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	s1.evalBroker.Enqueue(eval)

	// Create a plan result and apply it with a later index
	res := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc},
			Job:   job,
		},
		EvalID: eval.ID,
	}
	assert := assert.New(t)
	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	assert.Nil(err)

	// Dequeue the eval
	get := &structs.EvalDequeueRequest{
		Schedulers:       defaultSched,
		SchedulerVersion: scheduler.SchedulerVersion,
		WriteRequest:     structs.WriteRequest{Region: "global"},
	}
	var resp structs.EvalDequeueResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Dequeue", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure outstanding
	token, ok := s1.evalBroker.Outstanding(eval.ID)
	if !ok {
		t.Fatalf("should be outstanding")
	}
	if token != resp.Token {
		t.Fatalf("bad token: %#v %#v", token, resp.Token)
	}

	if resp.WaitIndex != 1000 {
		t.Fatalf("bad wait index; got %d; want %d", resp.WaitIndex, 1000)
	}
}

func TestEvalEndpoint_Dequeue_Version_Mismatch(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	// Dequeue the eval
	get := &structs.EvalDequeueRequest{
		Schedulers:       defaultSched,
		SchedulerVersion: 0,
		WriteRequest:     structs.WriteRequest{Region: "global"},
	}
	var resp structs.EvalDequeueResponse
	err := msgpackrpc.CallWithCodec(codec, "Eval.Dequeue", get, &resp)
	if err == nil || !strings.Contains(err.Error(), "scheduler version is 0") {
		t.Fatalf("err: %v", err)
	}
}

func TestEvalEndpoint_Ack(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)
	out, token, err := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing eval")
	}

	// Ack the eval
	get := &structs.EvalAckRequest{
		EvalID:       out.ID,
		Token:        token,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Ack", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure outstanding
	if _, ok := s1.evalBroker.Outstanding(eval1.ID); ok {
		t.Fatalf("should not be outstanding")
	}
}

func TestEvalEndpoint_Nack(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		// Disable all of the schedulers so we can manually dequeue
		// evals and check the queue status
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)
	out, token, _ := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if out == nil {
		t.Fatalf("missing eval")
	}

	// Nack the eval
	get := &structs.EvalAckRequest{
		EvalID:       out.ID,
		Token:        token,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Nack", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure outstanding
	if _, ok := s1.evalBroker.Outstanding(eval1.ID); ok {
		t.Fatalf("should not be outstanding")
	}

	// Should get it back
	testutil.WaitForResult(func() (bool, error) {
		out2, _, _ := s1.evalBroker.Dequeue(defaultSched, time.Second)
		if out2 != out {
			return false, fmt.Errorf("nack failed")
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})
}

func TestEvalEndpoint_Update(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)
	out, token, err := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing eval")
	}

	// Update the eval
	eval2 := eval1.Copy()
	eval2.Status = structs.EvalStatusComplete

	get := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval2},
		EvalToken:    token,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Update", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure updated
	ws := memdb.NewWatchSet()
	outE, err := s1.fsm.State().EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE.Status != structs.EvalStatusComplete {
		t.Fatalf("Bad: %#v", out)
	}
}

func TestEvalEndpoint_Create(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the register request
	prev := mock.Eval()
	s1.evalBroker.Enqueue(prev)
	out, token, err := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing eval")
	}

	// Create the register request
	eval1 := mock.Eval()
	eval1.PreviousEval = prev.ID
	get := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval1},
		EvalToken:    token,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Create", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure created
	ws := memdb.NewWatchSet()
	outE, err := s1.fsm.State().EvalByID(ws, eval1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	eval1.CreateIndex = resp.Index
	eval1.ModifyIndex = resp.Index
	if !reflect.DeepEqual(eval1, outE) {
		t.Fatalf("Bad: %#v %#v", outE, eval1)
	}
}

func TestEvalEndpoint_Reap(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})

	// Reap the eval
	get := &structs.EvalDeleteRequest{
		Evals:        []string{eval1.ID},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Reap", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("Bad index: %d", resp.Index)
	}

	// Ensure deleted
	ws := memdb.NewWatchSet()
	outE, err := s1.fsm.State().EvalByID(ws, eval1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if outE != nil {
		t.Fatalf("Bad: %#v", outE)
	}
}

func TestEvalEndpoint_List(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	eval1.ID = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	eval2 := mock.Eval()
	eval2.ID = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})

	// Lookup the eval
	get := &structs.EvalListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp structs.EvalListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.List", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if len(resp.Evaluations) != 2 {
		t.Fatalf("bad: %#v", resp.Evaluations)
	}

	// Lookup the eval by prefix
	get = &structs.EvalListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
			Prefix:    "aaaabb",
		},
	}
	var resp2 structs.EvalListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.List", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 1000)
	}

	if len(resp2.Evaluations) != 1 {
		t.Fatalf("bad: %#v", resp2.Evaluations)
	}

}

func TestEvalEndpoint_ListAllNamespaces(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	eval1.ID = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	eval2 := mock.Eval()
	eval2.ID = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})

	// Lookup the eval
	get := &structs.EvalListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: "*",
		},
	}
	var resp structs.EvalListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.List", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if len(resp.Evaluations) != 2 {
		t.Fatalf("bad: %#v", resp.Evaluations)
	}
}

func TestEvalEndpoint_List_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	eval1 := mock.Eval()
	eval1.ID = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	eval2 := mock.Eval()
	eval2.ID = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	state := s1.fsm.State()
	assert.Nil(state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2}))

	// Create ACL tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get := &structs.EvalListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	// Try without a token and expect permission denied
	{
		var resp structs.EvalListResponse
		err := msgpackrpc.CallWithCodec(codec, "Eval.List", get, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		get.AuthToken = invalidToken.SecretID
		var resp structs.EvalListResponse
		err := msgpackrpc.CallWithCodec(codec, "Eval.List", get, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// List evals with a valid token
	{
		get.AuthToken = validToken.SecretID
		var resp structs.EvalListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Eval.List", get, &resp))
		assert.Equal(uint64(1000), resp.Index, "Bad index: %d %d", resp.Index, 1000)
		assert.Lenf(resp.Evaluations, 2, "bad: %#v", resp.Evaluations)
	}

	// List evals with a root token
	{
		get.AuthToken = root.SecretID
		var resp structs.EvalListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Eval.List", get, &resp))
		assert.Equal(uint64(1000), resp.Index, "Bad index: %d %d", resp.Index, 1000)
		assert.Lenf(resp.Evaluations, 2, "bad: %#v", resp.Evaluations)
	}
}

func TestEvalEndpoint_List_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the ieval
	eval := mock.Eval()

	// Upsert eval triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{eval}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.EvalListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     structs.DefaultNamespace,
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.EvalListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 2 {
		t.Fatalf("Bad index: %d %d", resp.Index, 2)
	}
	if len(resp.Evaluations) != 1 || resp.Evaluations[0].ID != eval.ID {
		t.Fatalf("bad: %#v", resp.Evaluations)
	}

	// Eval deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteEval(3, []string{eval.ID}, nil); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 2
	start = time.Now()
	var resp2 structs.EvalListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.List", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 3 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 3)
	}
	if len(resp2.Evaluations) != 0 {
		t.Fatalf("bad: %#v", resp2.Evaluations)
	}
}

func TestEvalEndpoint_List_PaginationFiltering(t *testing.T) {
	t.Parallel()
	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// create a set of evals and field values to filter on. these are
	// in the order that the state store will return them from the
	// iterator (sorted by key), for ease of writing tests
	mocks := []struct {
		id        string
		namespace string
		jobID     string
		status    string
	}{
		{id: "aaaa1111-3350-4b4b-d185-0e1992ed43e9", jobID: "example"},
		{id: "aaaaaa22-3350-4b4b-d185-0e1992ed43e9", jobID: "example"},
		{id: "aaaaaa33-3350-4b4b-d185-0e1992ed43e9", namespace: "non-default"},
		{id: "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9", jobID: "example", status: "blocked"},
		{id: "aaaaaabb-3350-4b4b-d185-0e1992ed43e9"},
		{id: "aaaaaacc-3350-4b4b-d185-0e1992ed43e9"},
		{id: "aaaaaadd-3350-4b4b-d185-0e1992ed43e9", jobID: "example"},
		{id: "aaaaaaee-3350-4b4b-d185-0e1992ed43e9", jobID: "example"},
		{id: "aaaaaaff-3350-4b4b-d185-0e1992ed43e9"},
	}

	mockEvals := []*structs.Evaluation{}
	for _, m := range mocks {
		eval := mock.Eval()
		eval.ID = m.id
		if m.namespace != "" { // defaults to "default"
			eval.Namespace = m.namespace
		}
		if m.jobID != "" { // defaults to some random UUID
			eval.JobID = m.jobID
		}
		if m.status != "" { // defaults to "pending"
			eval.Status = m.status
		}
		mockEvals = append(mockEvals, eval)
	}

	state := s1.fsm.State()
	require.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1000, mockEvals))

	aclToken := mock.CreatePolicyAndToken(t, state, 1100, "test-valid-read",
		mock.NamespacePolicy(structs.DefaultNamespace, "read", nil)).
		SecretID

	cases := []struct {
		name              string
		namespace         string
		prefix            string
		nextToken         string
		filterJobID       string
		filterStatus      string
		pageSize          int32
		expectedNextToken string
		expectedIDs       []string
	}{
		{
			name:              "test01 size-2 page-1 default NS",
			pageSize:          2,
			expectedNextToken: "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test02 size-2 page-1 default NS with prefix",
			prefix:            "aaaa",
			pageSize:          2,
			expectedNextToken: "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test03 size-2 page-2 default NS",
			pageSize:          2,
			nextToken:         "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
			expectedNextToken: "aaaaaacc-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaabb-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test04 size-2 page-2 default NS with prefix",
			prefix:            "aaaa",
			pageSize:          2,
			nextToken:         "aaaaaabb-3350-4b4b-d185-0e1992ed43e9",
			expectedNextToken: "aaaaaadd-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaaaabb-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaacc-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:         "test05 size-2 page-1 with filters default NS",
			pageSize:     2,
			filterJobID:  "example",
			filterStatus: "pending",
			// aaaaaaaa, bb, and cc are filtered by status
			expectedNextToken: "aaaaaadd-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:         "test06 size-2 page-1 with filters default NS with short prefix",
			prefix:       "aaaa",
			pageSize:     2,
			filterJobID:  "example",
			filterStatus: "pending",
			// aaaaaaaa, bb, and cc are filtered by status
			expectedNextToken: "aaaaaadd-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test07 size-2 page-1 with filters default NS with longer prefix",
			prefix:            "aaaaaa",
			pageSize:          2,
			filterJobID:       "example",
			filterStatus:      "pending",
			expectedNextToken: "aaaaaaee-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaadd-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test08 size-2 page-2 filter skip nextToken",
			pageSize:          3, // reads off the end
			filterJobID:       "example",
			filterStatus:      "pending",
			nextToken:         "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
			expectedNextToken: "",
			expectedIDs: []string{
				"aaaaaadd-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaaee-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test09 size-2 page-2 filters skip nextToken with prefix",
			prefix:            "aaaaaa",
			pageSize:          3, // reads off the end
			filterJobID:       "example",
			filterStatus:      "pending",
			nextToken:         "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
			expectedNextToken: "",
			expectedIDs: []string{
				"aaaaaadd-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaaee-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:        "test10 no valid results with filters",
			pageSize:    2,
			filterJobID: "whatever",
			nextToken:   "",
			expectedIDs: []string{},
		},
		{
			name:        "test11 no valid results with filters and prefix",
			prefix:      "aaaa",
			pageSize:    2,
			filterJobID: "whatever",
			nextToken:   "",
			expectedIDs: []string{},
		},
		{
			name:        "test12 no valid results with filters page-2",
			filterJobID: "whatever",
			nextToken:   "aaaaaa11-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{},
		},
		{
			name:        "test13 no valid results with filters page-2 with prefix",
			prefix:      "aaaa",
			filterJobID: "whatever",
			nextToken:   "aaaaaa11-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.EvalListRequest{
				FilterJobID:      tc.filterJobID,
				FilterEvalStatus: tc.filterStatus,
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Namespace: tc.namespace,
					Prefix:    tc.prefix,
					PerPage:   tc.pageSize,
					NextToken: tc.nextToken,
				},
			}
			req.AuthToken = aclToken
			var resp structs.EvalListResponse
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "Eval.List", req, &resp))
			gotIDs := []string{}
			for _, eval := range resp.Evaluations {
				gotIDs = append(gotIDs, eval.ID)
			}
			require.Equal(t, tc.expectedIDs, gotIDs, "unexpected page of evals")
			require.Equal(t, tc.expectedNextToken, resp.QueryMeta.NextToken, "unexpected NextToken")
		})
	}
}

func TestEvalEndpoint_Allocations(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.EvalID = alloc1.EvalID
	state := s1.fsm.State()
	state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the eval
	get := &structs.EvalSpecificRequest{
		EvalID:       alloc1.EvalID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.EvalAllocationsResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Allocations", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if len(resp.Allocations) != 2 {
		t.Fatalf("bad: %#v", resp.Allocations)
	}
}

func TestEvalEndpoint_Allocations_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.EvalID = alloc1.EvalID
	state := s1.fsm.State()
	assert.Nil(state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID)))
	assert.Nil(state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID)))
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2}))

	// Create ACL tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1003, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get := &structs.EvalSpecificRequest{
		EvalID:       alloc1.EvalID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.EvalAllocationsResponse
		err := msgpackrpc.CallWithCodec(codec, "Eval.Allocations", get, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		get.AuthToken = invalidToken.SecretID
		var resp structs.EvalAllocationsResponse
		err := msgpackrpc.CallWithCodec(codec, "Eval.Allocations", get, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Lookup the eval with a valid token
	{
		get.AuthToken = validToken.SecretID
		var resp structs.EvalAllocationsResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Eval.Allocations", get, &resp))
		assert.Equal(uint64(1000), resp.Index, "Bad index: %d %d", resp.Index, 1000)
		assert.Lenf(resp.Allocations, 2, "bad: %#v", resp.Allocations)
	}

	// Lookup the eval with a root token
	{
		get.AuthToken = root.SecretID
		var resp structs.EvalAllocationsResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Eval.Allocations", get, &resp))
		assert.Equal(uint64(1000), resp.Index, "Bad index: %d %d", resp.Index, 1000)
		assert.Lenf(resp.Allocations, 2, "bad: %#v", resp.Allocations)
	}
}

func TestEvalEndpoint_Allocations_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the allocs
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()

	// Upsert an unrelated alloc first
	time.AfterFunc(100*time.Millisecond, func() {
		state.UpsertJobSummary(99, mock.JobSummary(alloc1.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 100, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert an alloc which will trigger the watch later
	time.AfterFunc(200*time.Millisecond, func() {
		state.UpsertJobSummary(199, mock.JobSummary(alloc2.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the eval
	get := &structs.EvalSpecificRequest{
		EvalID: alloc2.EvalID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.EvalAllocationsResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Allocations", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Allocations) != 1 || resp.Allocations[0].ID != alloc2.ID {
		t.Fatalf("bad: %#v", resp.Allocations)
	}
}

func TestEvalEndpoint_Reblock_Nonexistent(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)
	out, token, err := s1.evalBroker.Dequeue(defaultSched, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing eval")
	}

	get := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval1},
		EvalToken:    token,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Reblock", get, &resp); err == nil {
		t.Fatalf("expect error since eval does not exist")
	}
}

func TestEvalEndpoint_Reblock_NonBlocked(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the eval
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	// Insert it into the state store
	if err := s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1}); err != nil {
		t.Fatal(err)
	}

	out, token, err := s1.evalBroker.Dequeue(defaultSched, 2*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing eval")
	}

	get := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval1},
		EvalToken:    token,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Reblock", get, &resp); err == nil {
		t.Fatalf("should error since eval was not in blocked state: %v", err)
	}
}

func TestEvalEndpoint_Reblock(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)

	testutil.WaitForResult(func() (bool, error) {
		return s1.evalBroker.Enabled(), nil
	}, func(err error) {
		t.Fatalf("should enable eval broker")
	})

	// Create the eval
	eval1 := mock.Eval()
	eval1.Status = structs.EvalStatusBlocked
	s1.evalBroker.Enqueue(eval1)

	// Insert it into the state store
	if err := s1.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1}); err != nil {
		t.Fatal(err)
	}

	out, token, err := s1.evalBroker.Dequeue(defaultSched, 7*time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("bad: %v", out)
	}

	get := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval1},
		EvalToken:    token,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Eval.Reblock", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check that it is blocked
	bStats := s1.blockedEvals.Stats()
	if bStats.TotalBlocked+bStats.TotalEscaped == 0 {
		t.Fatalf("ReblockEval didn't insert eval into the blocked eval tracker")
	}
}
