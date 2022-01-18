package nomad

import (
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentEndpoint_GetDeployment(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Lookup the deployments
	get := &structs.DeploymentSpecificRequest{
		DeploymentID: d.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp structs.SingleDeploymentResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.GetDeployment", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1000, "resp.Index")
	assert.Equal(d, resp.Deployment, "Returned deployment not equal")
}

func TestDeploymentEndpoint_GetDeployment_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	// Lookup the deployments without a token and expect failure
	get := &structs.DeploymentSpecificRequest{
		DeploymentID: d.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp structs.SingleDeploymentResponse
	assert.NotNil(msgpackrpc.CallWithCodec(codec, "Deployment.GetDeployment", get, &resp), "RPC")

	// Try with a good token
	get.AuthToken = validToken.SecretID
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.GetDeployment", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1000, "resp.Index")
	assert.Equal(d, resp.Deployment, "Returned deployment not equal")

	// Try with a bad token
	get.AuthToken = invalidToken.SecretID
	err := msgpackrpc.CallWithCodec(codec, "Deployment.GetDeployment", get, &resp)
	assert.NotNil(err, "RPC")
	assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

	// Try with a root token
	get.AuthToken = root.SecretID
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.GetDeployment", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1000, "resp.Index")
	assert.Equal(d, resp.Deployment, "Returned deployment not equal")
}

func TestDeploymentEndpoint_GetDeployment_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()
	assert := assert.New(t)

	// Create the deployments
	j1 := mock.Job()
	j2 := mock.Job()
	d1 := mock.Deployment()
	d1.JobID = j1.ID
	d2 := mock.Deployment()
	d2.JobID = j2.ID

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 98, j1), "UpsertJob")
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 99, j2), "UpsertJob")

	// Upsert a deployment we are not interested in first.
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertDeployment(100, d1), "UpsertDeployment")
	})

	// Upsert another deployment later which should trigger the watch.
	time.AfterFunc(200*time.Millisecond, func() {
		assert.Nil(state.UpsertDeployment(200, d2), "UpsertDeployment")
	})

	// Lookup the deployments
	get := &structs.DeploymentSpecificRequest{
		DeploymentID: d2.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     structs.DefaultNamespace,
			MinQueryIndex: 150,
		},
	}
	start := time.Now()
	var resp structs.SingleDeploymentResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.GetDeployment", get, &resp), "RPC")
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	assert.EqualValues(resp.Index, 200, "resp.Index")
	assert.Equal(d2, resp.Deployment, "deployments equal")
}

func TestDeploymentEndpoint_Fail(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Mark the deployment as failed
	req := &structs.DeploymentFailRequest{
		DeploymentID: d.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.DeploymentUpdateResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Fail", req, &resp), "RPC")
	assert.NotEqual(resp.Index, uint64(0), "bad response index")

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	assert.Nil(err, "EvalByID failed")
	assert.NotNil(eval, "Expect eval")
	assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
	assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
	assert.Equal(eval.JobID, d.JobID, "eval job id")
	assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
	assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

	// Lookup the deployment
	dout, err := state.DeploymentByID(ws, d.ID)
	assert.Nil(err, "DeploymentByID failed")
	assert.Equal(dout.Status, structs.DeploymentStatusFailed, "wrong status")
	assert.Equal(dout.StatusDescription, structs.DeploymentStatusDescriptionFailedByUser, "wrong status description")
	assert.Equal(dout.ModifyIndex, resp.DeploymentModifyIndex, "wrong modify index")
}

func TestDeploymentEndpoint_Fail_ACL(t *testing.T) {
	t.Parallel()

	s1, _, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	// Mark the deployment as failed
	req := &structs.DeploymentFailRequest{
		DeploymentID: d.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.Fail", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	{
		req.AuthToken = invalidToken.SecretID
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.Fail", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	{
		req.AuthToken = validToken.SecretID
		var resp structs.DeploymentUpdateResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Fail", req, &resp), "RPC")
		assert.NotEqual(resp.Index, uint64(0), "bad response index")

		// Lookup the evaluation
		ws := memdb.NewWatchSet()
		eval, err := state.EvalByID(ws, resp.EvalID)
		assert.Nil(err, "EvalByID failed")
		assert.NotNil(eval, "Expect eval")
		assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
		assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
		assert.Equal(eval.JobID, d.JobID, "eval job id")
		assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
		assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

		// Lookup the deployment
		dout, err := state.DeploymentByID(ws, d.ID)
		assert.Nil(err, "DeploymentByID failed")
		assert.Equal(dout.Status, structs.DeploymentStatusFailed, "wrong status")
		assert.Equal(dout.StatusDescription, structs.DeploymentStatusDescriptionFailedByUser, "wrong status description")
		assert.Equal(dout.ModifyIndex, resp.DeploymentModifyIndex, "wrong modify index")
	}
}

func TestDeploymentEndpoint_Fail_Rollback(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create the original job
	j := mock.Job()
	j.Stable = true
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 998, j), "UpsertJob")

	// Create the second job, deployment and alloc
	j2 := j.Copy()
	j2.Stable = false
	// Modify the job to make its specification different
	j2.Meta["foo"] = "bar"

	d := mock.Deployment()
	d.TaskGroups["web"].AutoRevert = true
	d.JobID = j2.ID
	d.JobVersion = j2.Version

	a := mock.Alloc()
	a.JobID = j.ID
	a.DeploymentID = d.ID

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j2), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Mark the deployment as failed
	req := &structs.DeploymentFailRequest{
		DeploymentID: d.ID,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.DeploymentUpdateResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Fail", req, &resp), "RPC")
	assert.NotEqual(resp.Index, uint64(0), "bad response index")
	assert.NotNil(resp.RevertedJobVersion, "bad revert version")
	assert.EqualValues(0, *resp.RevertedJobVersion, "bad revert version")

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	assert.Nil(err, "EvalByID failed")
	assert.NotNil(eval, "Expect eval")
	assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
	assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
	assert.Equal(eval.JobID, d.JobID, "eval job id")
	assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
	assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

	// Lookup the deployment
	expectedDesc := structs.DeploymentStatusDescriptionRollback(structs.DeploymentStatusDescriptionFailedByUser, 0)
	dout, err := state.DeploymentByID(ws, d.ID)
	assert.Nil(err, "DeploymentByID failed")
	assert.Equal(dout.Status, structs.DeploymentStatusFailed, "wrong status")
	assert.Equal(dout.StatusDescription, expectedDesc, "wrong status description")
	assert.Equal(resp.DeploymentModifyIndex, dout.ModifyIndex, "wrong modify index")

	// Lookup the job
	jout, err := state.JobByID(ws, j.Namespace, j.ID)
	assert.Nil(err, "JobByID")
	assert.NotNil(jout, "job")
	assert.EqualValues(2, jout.Version, "reverted job version")
}

func TestDeploymentEndpoint_Pause(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Mark the deployment as failed
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.DeploymentUpdateResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Pause", req, &resp), "RPC")
	assert.NotEqual(resp.Index, uint64(0), "bad response index")
	assert.Zero(resp.EvalCreateIndex, "Shouldn't create eval")
	assert.Zero(resp.EvalID, "Shouldn't create eval")

	// Lookup the deployment
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	assert.Nil(err, "DeploymentByID failed")
	assert.Equal(dout.Status, structs.DeploymentStatusPaused, "wrong status")
	assert.Equal(dout.StatusDescription, structs.DeploymentStatusDescriptionPaused, "wrong status description")
	assert.Equal(dout.ModifyIndex, resp.DeploymentModifyIndex, "wrong modify index")
}

func TestDeploymentEndpoint_Pause_ACL(t *testing.T) {
	t.Parallel()

	s1, _, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	// Mark the deployment as failed
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.Pause", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	{
		req.AuthToken = invalidToken.SecretID
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.Pause", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Fetch the response with a valid token
	{
		req.AuthToken = validToken.SecretID
		var resp structs.DeploymentUpdateResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Pause", req, &resp), "RPC")
		assert.NotEqual(resp.Index, uint64(0), "bad response index")
		assert.Zero(resp.EvalCreateIndex, "Shouldn't create eval")
		assert.Zero(resp.EvalID, "Shouldn't create eval")

		// Lookup the deployment
		ws := memdb.NewWatchSet()
		dout, err := state.DeploymentByID(ws, d.ID)
		assert.Nil(err, "DeploymentByID failed")
		assert.Equal(dout.Status, structs.DeploymentStatusPaused, "wrong status")
		assert.Equal(dout.StatusDescription, structs.DeploymentStatusDescriptionPaused, "wrong status description")
		assert.Equal(dout.ModifyIndex, resp.DeploymentModifyIndex, "wrong modify index")
	}
}

func TestDeploymentEndpoint_Promote(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment, job and canary
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 1
	d := mock.Deployment()
	d.TaskGroups["web"].DesiredCanaries = 1
	d.JobID = j.ID
	a := mock.Alloc()
	d.TaskGroups[a.TaskGroup].PlacedCanaries = []string{a.ID}
	a.DeploymentID = d.ID
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}

	state := s1.fsm.State()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Promote the deployment
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.DeploymentUpdateResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Promote", req, &resp), "RPC")
	assert.NotEqual(resp.Index, uint64(0), "bad response index")

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	assert.Nil(err, "EvalByID failed")
	assert.NotNil(eval, "Expect eval")
	assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
	assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
	assert.Equal(eval.JobID, d.JobID, "eval job id")
	assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
	assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

	// Lookup the deployment
	dout, err := state.DeploymentByID(ws, d.ID)
	assert.Nil(err, "DeploymentByID failed")
	assert.Equal(dout.Status, structs.DeploymentStatusRunning, "wrong status")
	assert.Equal(dout.StatusDescription, structs.DeploymentStatusDescriptionRunning, "wrong status description")
	assert.Equal(dout.ModifyIndex, resp.DeploymentModifyIndex, "wrong modify index")
	assert.Len(dout.TaskGroups, 1, "should have one group")
	assert.Contains(dout.TaskGroups, "web", "should have web group")
	assert.True(dout.TaskGroups["web"].Promoted, "web group should be promoted")
}

func TestDeploymentEndpoint_Promote_ACL(t *testing.T) {
	t.Parallel()

	s1, _, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment, job and canary
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 1
	d := mock.Deployment()
	d.TaskGroups["web"].DesiredCanaries = 1
	d.JobID = j.ID
	a := mock.Alloc()
	d.TaskGroups[a.TaskGroup].PlacedCanaries = []string{a.ID}
	a.DeploymentID = d.ID
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}

	state := s1.fsm.State()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	// Promote the deployment
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.Promote", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	{
		req.AuthToken = invalidToken.SecretID
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.Promote", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Fetch the response with a valid token
	{
		req.AuthToken = validToken.SecretID
		var resp structs.DeploymentUpdateResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Promote", req, &resp), "RPC")
		assert.NotEqual(resp.Index, uint64(0), "bad response index")

		// Lookup the evaluation
		ws := memdb.NewWatchSet()
		eval, err := state.EvalByID(ws, resp.EvalID)
		assert.Nil(err, "EvalByID failed")
		assert.NotNil(eval, "Expect eval")
		assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
		assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
		assert.Equal(eval.JobID, d.JobID, "eval job id")
		assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
		assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

		// Lookup the deployment
		dout, err := state.DeploymentByID(ws, d.ID)
		assert.Nil(err, "DeploymentByID failed")
		assert.Equal(dout.Status, structs.DeploymentStatusRunning, "wrong status")
		assert.Equal(dout.StatusDescription, structs.DeploymentStatusDescriptionRunning, "wrong status description")
		assert.Equal(dout.ModifyIndex, resp.DeploymentModifyIndex, "wrong modify index")
		assert.Len(dout.TaskGroups, 1, "should have one group")
		assert.Contains(dout.TaskGroups, "web", "should have web group")
		assert.True(dout.TaskGroups["web"].Promoted, "web group should be promoted")
	}
}

func TestDeploymentEndpoint_SetAllocHealth(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment, job and canary
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.JobID = j.ID
	a.DeploymentID = d.ID

	state := s1.fsm.State()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Set the alloc as healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
		WriteRequest:         structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.DeploymentUpdateResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.SetAllocHealth", req, &resp), "RPC")
	assert.NotZero(resp.Index, "bad response index")

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	assert.Nil(err, "EvalByID failed")
	assert.NotNil(eval, "Expect eval")
	assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
	assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
	assert.Equal(eval.JobID, d.JobID, "eval job id")
	assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
	assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

	// Lookup the deployment
	dout, err := state.DeploymentByID(ws, d.ID)
	assert.Nil(err, "DeploymentByID failed")
	assert.Equal(dout.Status, structs.DeploymentStatusRunning, "wrong status")
	assert.Equal(dout.StatusDescription, structs.DeploymentStatusDescriptionRunning, "wrong status description")
	assert.Equal(resp.DeploymentModifyIndex, dout.ModifyIndex, "wrong modify index")
	assert.Len(dout.TaskGroups, 1, "should have one group")
	assert.Contains(dout.TaskGroups, "web", "should have web group")
	assert.Equal(1, dout.TaskGroups["web"].HealthyAllocs, "should have one healthy")

	// Lookup the allocation
	aout, err := state.AllocByID(ws, a.ID)
	assert.Nil(err, "AllocByID")
	assert.NotNil(aout, "alloc")
	assert.NotNil(aout.DeploymentStatus, "alloc deployment status")
	assert.NotNil(aout.DeploymentStatus.Healthy, "alloc deployment healthy")
	assert.True(*aout.DeploymentStatus.Healthy, "alloc deployment healthy")
}

func TestDeploymentEndpoint_SetAllocHealth_ACL(t *testing.T) {
	t.Parallel()

	s1, _, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment, job and canary
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.JobID = j.ID
	a.DeploymentID = d.ID

	state := s1.fsm.State()
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	// Set the alloc as healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
		WriteRequest:         structs.WriteRequest{Region: "global"},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.SetAllocHealth", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	{
		req.AuthToken = invalidToken.SecretID
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.SetAllocHealth", req, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Fetch the response with a valid token
	{
		req.AuthToken = validToken.SecretID
		var resp structs.DeploymentUpdateResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.SetAllocHealth", req, &resp), "RPC")
		assert.NotZero(resp.Index, "bad response index")

		// Lookup the evaluation
		ws := memdb.NewWatchSet()
		eval, err := state.EvalByID(ws, resp.EvalID)
		assert.Nil(err, "EvalByID failed")
		assert.NotNil(eval, "Expect eval")
		assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
		assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
		assert.Equal(eval.JobID, d.JobID, "eval job id")
		assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
		assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

		// Lookup the deployment
		dout, err := state.DeploymentByID(ws, d.ID)
		assert.Nil(err, "DeploymentByID failed")
		assert.Equal(dout.Status, structs.DeploymentStatusRunning, "wrong status")
		assert.Equal(dout.StatusDescription, structs.DeploymentStatusDescriptionRunning, "wrong status description")
		assert.Equal(resp.DeploymentModifyIndex, dout.ModifyIndex, "wrong modify index")
		assert.Len(dout.TaskGroups, 1, "should have one group")
		assert.Contains(dout.TaskGroups, "web", "should have web group")
		assert.Equal(1, dout.TaskGroups["web"].HealthyAllocs, "should have one healthy")

		// Lookup the allocation
		aout, err := state.AllocByID(ws, a.ID)
		assert.Nil(err, "AllocByID")
		assert.NotNil(aout, "alloc")
		assert.NotNil(aout.DeploymentStatus, "alloc deployment status")
		assert.NotNil(aout.DeploymentStatus.Healthy, "alloc deployment healthy")
		assert.True(*aout.DeploymentStatus.Healthy, "alloc deployment healthy")
	}
}

func TestDeploymentEndpoint_SetAllocHealth_Rollback(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create the original job
	j := mock.Job()
	j.Stable = true
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 998, j), "UpsertJob")

	// Create the second job, deployment and alloc
	j2 := j.Copy()
	j2.Stable = false
	// Modify the job to make its specification different
	j2.Meta["foo"] = "bar"
	d := mock.Deployment()
	d.TaskGroups["web"].AutoRevert = true
	d.JobID = j2.ID
	d.JobVersion = j2.Version

	a := mock.Alloc()
	a.JobID = j.ID
	a.DeploymentID = d.ID

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j2), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Set the alloc as unhealthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
		WriteRequest:           structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.DeploymentUpdateResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.SetAllocHealth", req, &resp), "RPC")
	assert.NotZero(resp.Index, "bad response index")
	assert.NotNil(resp.RevertedJobVersion, "bad revert version")
	assert.EqualValues(0, *resp.RevertedJobVersion, "bad revert version")

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	assert.Nil(err, "EvalByID failed")
	assert.NotNil(eval, "Expect eval")
	assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
	assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
	assert.Equal(eval.JobID, d.JobID, "eval job id")
	assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
	assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

	// Lookup the deployment
	expectedDesc := structs.DeploymentStatusDescriptionRollback(structs.DeploymentStatusDescriptionFailedAllocations, 0)
	dout, err := state.DeploymentByID(ws, d.ID)
	assert.Nil(err, "DeploymentByID failed")
	assert.Equal(dout.Status, structs.DeploymentStatusFailed, "wrong status")
	assert.Equal(dout.StatusDescription, expectedDesc, "wrong status description")
	assert.Equal(resp.DeploymentModifyIndex, dout.ModifyIndex, "wrong modify index")
	assert.Len(dout.TaskGroups, 1, "should have one group")
	assert.Contains(dout.TaskGroups, "web", "should have web group")
	assert.Equal(1, dout.TaskGroups["web"].UnhealthyAllocs, "should have one healthy")

	// Lookup the allocation
	aout, err := state.AllocByID(ws, a.ID)
	assert.Nil(err, "AllocByID")
	assert.NotNil(aout, "alloc")
	assert.NotNil(aout.DeploymentStatus, "alloc deployment status")
	assert.NotNil(aout.DeploymentStatus.Healthy, "alloc deployment healthy")
	assert.False(*aout.DeploymentStatus.Healthy, "alloc deployment healthy")

	// Lookup the job
	jout, err := state.JobByID(ws, j.Namespace, j.ID)
	assert.Nil(err, "JobByID")
	assert.NotNil(jout, "job")
	assert.EqualValues(2, jout.Version, "reverted job version")
}

// tests rollback upon alloc health failure to job with identical spec does not succeed
func TestDeploymentEndpoint_SetAllocHealth_NoRollback(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create the original job
	j := mock.Job()
	j.Stable = true
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 998, j), "UpsertJob")

	// Create the second job, deployment and alloc. Job has same spec as original
	j2 := j.Copy()
	j2.Stable = false

	d := mock.Deployment()
	d.TaskGroups["web"].AutoRevert = true
	d.JobID = j2.ID
	d.JobVersion = j2.Version

	a := mock.Alloc()
	a.JobID = j.ID
	a.DeploymentID = d.ID

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j2), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Set the alloc as unhealthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
		WriteRequest:           structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.DeploymentUpdateResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.SetAllocHealth", req, &resp), "RPC")
	assert.NotZero(resp.Index, "bad response index")
	assert.Nil(resp.RevertedJobVersion, "revert version must be nil")

	// Lookup the evaluation
	ws := memdb.NewWatchSet()
	eval, err := state.EvalByID(ws, resp.EvalID)
	assert.Nil(err, "EvalByID failed")
	assert.NotNil(eval, "Expect eval")
	assert.Equal(eval.CreateIndex, resp.EvalCreateIndex, "eval index mismatch")
	assert.Equal(eval.TriggeredBy, structs.EvalTriggerDeploymentWatcher, "eval trigger")
	assert.Equal(eval.JobID, d.JobID, "eval job id")
	assert.Equal(eval.DeploymentID, d.ID, "eval deployment id")
	assert.Equal(eval.Status, structs.EvalStatusPending, "eval status")

	// Lookup the deployment
	expectedDesc := structs.DeploymentStatusDescriptionRollbackNoop(structs.DeploymentStatusDescriptionFailedAllocations, 0)
	dout, err := state.DeploymentByID(ws, d.ID)
	assert.Nil(err, "DeploymentByID failed")
	assert.Equal(dout.Status, structs.DeploymentStatusFailed, "wrong status")
	assert.Equal(dout.StatusDescription, expectedDesc, "wrong status description")
	assert.Equal(resp.DeploymentModifyIndex, dout.ModifyIndex, "wrong modify index")
	assert.Len(dout.TaskGroups, 1, "should have one group")
	assert.Contains(dout.TaskGroups, "web", "should have web group")
	assert.Equal(1, dout.TaskGroups["web"].UnhealthyAllocs, "should have one healthy")

	// Lookup the allocation
	aout, err := state.AllocByID(ws, a.ID)
	assert.Nil(err, "AllocByID")
	assert.NotNil(aout, "alloc")
	assert.NotNil(aout.DeploymentStatus, "alloc deployment status")
	assert.NotNil(aout.DeploymentStatus.Healthy, "alloc deployment healthy")
	assert.False(*aout.DeploymentStatus.Healthy, "alloc deployment healthy")

	// Lookup the job, its version should not have changed
	jout, err := state.JobByID(ws, j.Namespace, j.ID)
	assert.Nil(err, "JobByID")
	assert.NotNil(jout, "job")
	assert.EqualValues(1, jout.Version, "original job version")
}

func TestDeploymentEndpoint_List(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Lookup the deployments
	get := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp structs.DeploymentListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1000, "Wrong Index")
	assert.Len(resp.Deployments, 1, "Deployments")
	assert.Equal(resp.Deployments[0].ID, d.ID, "Deployment ID")

	// Lookup the deploys by prefix
	get = &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
			Prefix:    d.ID[:4],
		},
	}

	var resp2 structs.DeploymentListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp2), "RPC")
	assert.EqualValues(resp.Index, 1000, "Wrong Index")
	assert.Len(resp2.Deployments, 1, "Deployments")
	assert.Equal(resp2.Deployments[0].ID, d.ID, "Deployment ID")

	// add another deployment in another namespace

	j2 := mock.Job()
	d2 := mock.Deployment()
	j2.Namespace = "prod"
	d2.Namespace = "prod"
	d2.JobID = j2.ID
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{{Name: "prod"}}))
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1002, j2), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1003, d2), "UpsertDeployment")

	// Lookup the deployments with wildcard namespace
	get = &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.AllNamespacesSentinel,
		},
	}
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1003, "Wrong Index")
	assert.Len(resp.Deployments, 2, "Deployments")
}

func TestDeploymentEndpoint_List_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	{
		get.AuthToken = invalidToken.SecretID
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Lookup the deployments with a root token
	{
		get.AuthToken = root.SecretID
		var resp structs.DeploymentListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp), "RPC")
		assert.EqualValues(resp.Index, 1000, "Wrong Index")
		assert.Len(resp.Deployments, 1, "Deployments")
		assert.Equal(resp.Deployments[0].ID, d.ID, "Deployment ID")
	}

	// Lookup the deployments with a valid token
	{
		get.AuthToken = validToken.SecretID
		var resp structs.DeploymentListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp), "RPC")
		assert.EqualValues(resp.Index, 1000, "Wrong Index")
		assert.Len(resp.Deployments, 1, "Deployments")
		assert.Equal(resp.Deployments[0].ID, d.ID, "Deployment ID")
	}
}

func TestDeploymentEndpoint_List_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, j), "UpsertJob")

	// Upsert alloc triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertDeployment(3, d), "UpsertDeployment")
	})

	req := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     structs.DefaultNamespace,
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.DeploymentListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", req, &resp), "RPC")
	assert.EqualValues(resp.Index, 3, "Wrong Index")
	assert.Len(resp.Deployments, 1, "Deployments")
	assert.Equal(resp.Deployments[0].ID, d.ID, "Deployment ID")
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Deployment updates trigger watches
	d2 := d.Copy()
	d2.Status = structs.DeploymentStatusPaused
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertDeployment(5, d2), "UpsertDeployment")
	})

	req.MinQueryIndex = 3
	start = time.Now()
	var resp2 structs.DeploymentListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", req, &resp2), "RPC")
	assert.EqualValues(5, resp2.Index, "Wrong Index")
	assert.Len(resp2.Deployments, 1, "Deployments")
	assert.Equal(d2.ID, resp2.Deployments[0].ID, "Deployment ID")
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
}

func TestDeploymentEndpoint_List_Pagination(t *testing.T) {
	t.Parallel()
	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// create a set of deployments. these are in the order that the
	// state store will return them from the iterator (sorted by key),
	// for ease of writing tests
	mocks := []struct {
		id        string
		namespace string
		jobID     string
		status    string
	}{
		{id: "aaaa1111-3350-4b4b-d185-0e1992ed43e9"},
		{id: "aaaaaa22-3350-4b4b-d185-0e1992ed43e9"},
		{id: "aaaaaa33-3350-4b4b-d185-0e1992ed43e9", namespace: "non-default"},
		{id: "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"},
		{id: "aaaaaabb-3350-4b4b-d185-0e1992ed43e9"},
		{id: "aaaaaacc-3350-4b4b-d185-0e1992ed43e9"},
		{id: "aaaaaadd-3350-4b4b-d185-0e1992ed43e9"},
	}

	state := s1.fsm.State()
	index := uint64(1000)

	for _, m := range mocks {
		index++
		deployment := mock.Deployment()
		deployment.Status = structs.DeploymentStatusCancelled
		deployment.ID = m.id
		if m.namespace != "" { // defaults to "default"
			deployment.Namespace = m.namespace
		}
		require.NoError(t, state.UpsertDeployment(index, deployment))
	}

	aclToken := mock.CreatePolicyAndToken(t, state, 1100, "test-valid-read",
		mock.NamespacePolicy(structs.DefaultNamespace, "read", nil)).
		SecretID

	cases := []struct {
		name              string
		namespace         string
		prefix            string
		nextToken         string
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
			name:        "test5 no valid results with filters and prefix",
			prefix:      "cccc",
			pageSize:    2,
			nextToken:   "",
			expectedIDs: []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.DeploymentListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Namespace: tc.namespace,
					Prefix:    tc.prefix,
					PerPage:   tc.pageSize,
					NextToken: tc.nextToken,
				},
			}
			req.AuthToken = aclToken
			var resp structs.DeploymentListResponse
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "Deployment.List", req, &resp))
			gotIDs := []string{}
			for _, deployment := range resp.Deployments {
				gotIDs = append(gotIDs, deployment.ID)
			}
			require.Equal(t, tc.expectedIDs, gotIDs, "unexpected page of deployments")
			require.Equal(t, tc.expectedNextToken, resp.QueryMeta.NextToken, "unexpected NextToken")
		})
	}
}

func TestDeploymentEndpoint_Allocations(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	summary := mock.JobSummary(a.JobID)
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 998, j), "UpsertJob")
	assert.Nil(state.UpsertJobSummary(999, summary), "UpsertJobSummary")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Lookup the allocations
	get := &structs.DeploymentSpecificRequest{
		DeploymentID: d.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp structs.AllocListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", get, &resp), "RPC")
	assert.EqualValues(1001, resp.Index, "Wrong Index")
	assert.Len(resp.Allocations, 1, "Allocations")
	assert.Equal(a.ID, resp.Allocations[0].ID, "Allocation ID")
}

func TestDeploymentEndpoint_Allocations_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	summary := mock.JobSummary(a.JobID)
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 998, j), "UpsertJob")
	assert.Nil(state.UpsertJobSummary(999, summary), "UpsertJobSummary")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	get := &structs.DeploymentSpecificRequest{
		DeploymentID: d.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	// Try with no token and expect permission denied
	{
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token
	{
		get.AuthToken = invalidToken.SecretID
		var resp structs.DeploymentUpdateResponse
		err := msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", get, &resp)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Lookup the allocations with a valid token
	{
		get.AuthToken = validToken.SecretID
		var resp structs.AllocListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", get, &resp), "RPC")
		assert.EqualValues(1001, resp.Index, "Wrong Index")
		assert.Len(resp.Allocations, 1, "Allocations")
		assert.Equal(a.ID, resp.Allocations[0].ID, "Allocation ID")
	}

	// Lookup the allocations with a root token
	{
		get.AuthToken = root.SecretID
		var resp structs.AllocListResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", get, &resp), "RPC")
		assert.EqualValues(1001, resp.Index, "Wrong Index")
		assert.Len(resp.Allocations, 1, "Allocations")
		assert.Equal(a.ID, resp.Allocations[0].ID, "Allocation ID")
	}
}

func TestDeploymentEndpoint_Allocations_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the alloc
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	summary := mock.JobSummary(a.JobID)

	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(2, d), "UpsertDeployment")
	assert.Nil(state.UpsertJobSummary(3, summary), "UpsertJobSummary")

	// Upsert alloc triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{a}), "UpsertAllocs")
	})

	req := &structs.DeploymentSpecificRequest{
		DeploymentID: d.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     structs.DefaultNamespace,
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.AllocListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", req, &resp), "RPC")
	assert.EqualValues(4, resp.Index, "Wrong Index")
	assert.Len(resp.Allocations, 1, "Allocations")
	assert.Equal(a.ID, resp.Allocations[0].ID, "Allocation ID")
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}

	// Client updates trigger watches
	a2 := mock.Alloc()
	a2.ID = a.ID
	a2.DeploymentID = a.DeploymentID
	a2.ClientStatus = structs.AllocClientStatusRunning
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertJobSummary(5, mock.JobSummary(a2.JobID)), "UpsertJobSummary")
		assert.Nil(state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 6, []*structs.Allocation{a2}), "updateAllocsFromClient")
	})

	req.MinQueryIndex = 4
	start = time.Now()
	var resp2 structs.AllocListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", req, &resp2), "RPC")
	assert.EqualValues(6, resp2.Index, "Wrong Index")
	assert.Len(resp2.Allocations, 1, "Allocations")
	assert.Equal(a.ID, resp2.Allocations[0].ID, "Allocation ID")
	assert.Equal(structs.AllocClientStatusRunning, resp2.Allocations[0].ClientStatus, "Client Status")
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
}

func TestDeploymentEndpoint_Reap(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	d1 := mock.Deployment()
	assert.Nil(s1.fsm.State().UpsertDeployment(1000, d1), "UpsertDeployment")

	// Reap the eval
	get := &structs.DeploymentDeleteRequest{
		Deployments:  []string{d1.ID},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.GenericResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Reap", get, &resp), "RPC")
	assert.NotZero(resp.Index, "bad response index")

	// Ensure deleted
	ws := memdb.NewWatchSet()
	outD, err := s1.fsm.State().DeploymentByID(ws, d1.ID)
	assert.Nil(err, "DeploymentByID")
	assert.Nil(outD, "Deleted Deployment")
}
