package nomad

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPeriodicEndpoint_Force(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	state := s1.fsm.State()
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create and insert a periodic job.
	job := mock.PeriodicJob()
	job.Periodic.ProhibitOverlap = true // Shouldn't affect anything.
	if err := state.UpsertJob(100, job); err != nil {
		t.Fatalf("err: %v", err)
	}
	s1.periodicDispatcher.Add(job)

	// Force launch it.
	req := &structs.PeriodicForceRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.PeriodicForceResponse
	if err := msgpackrpc.CallWithCodec(codec, "Periodic.Force", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}

	// Lookup the evaluation
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
}

func TestPeriodicEndpoint_Force_ACL(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	state := s1.fsm.State()
	assert := assert.New(t)
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create and insert a periodic job.
	job := mock.PeriodicJob()
	job.Periodic.ProhibitOverlap = true // Shouldn't affect anything.
	assert.Nil(state.UpsertJob(100, job))
	_, err := s1.periodicDispatcher.Add(job)
	assert.Nil(err)

	// Force launch it.
	req := &structs.PeriodicForceRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.PeriodicForceResponse
		err := msgpackrpc.CallWithCodec(codec, "Periodic.Force", req, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "invalid", mock.NodePolicy(acl.PolicyWrite))
		req.AuthToken = invalidToken.SecretID
		var resp structs.PeriodicForceResponse
		err := msgpackrpc.CallWithCodec(codec, "Periodic.Force", req, &resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Fetch the response with a valid token
	{
		policy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob})
		token := mock.CreatePolicyAndToken(t, state, 1005, "valid", policy)
		req.AuthToken = token.SecretID
		var resp structs.PeriodicForceResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Periodic.Force", req, &resp))
		assert.NotEqual(0, resp.Index)

		// Lookup the evaluation
		ws := memdb.NewWatchSet()
		eval, err := state.EvalByID(ws, resp.EvalID)
		assert.Nil(err)
		assert.NotNil(eval)
		assert.Equal(eval.CreateIndex, resp.EvalCreateIndex)
	}

	// Fetch the response with management token
	{
		req.AuthToken = root.SecretID
		var resp structs.PeriodicForceResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Periodic.Force", req, &resp))
		assert.NotEqual(0, resp.Index)

		// Lookup the evaluation
		ws := memdb.NewWatchSet()
		eval, err := state.EvalByID(ws, resp.EvalID)
		assert.Nil(err)
		assert.NotNil(eval)
		assert.Equal(eval.CreateIndex, resp.EvalCreateIndex)
	}
}

func TestPeriodicEndpoint_Force_NonPeriodic(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	state := s1.fsm.State()
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create and insert a non-periodic job.
	job := mock.Job()
	if err := state.UpsertJob(100, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Force launch it.
	req := &structs.PeriodicForceRequest{
		JobID: job.ID,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.PeriodicForceResponse
	if err := msgpackrpc.CallWithCodec(codec, "Periodic.Force", req, &resp); err == nil {
		t.Fatalf("Force on non-perodic job should err")
	}
}
