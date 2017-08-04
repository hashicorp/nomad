package nomad

import (
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestDeploymentEndpoint_GetDeployment(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Lookup the deployments
	get := &structs.DeploymentSpecificRequest{
		DeploymentID: d.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.SingleDeploymentResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.GetDeployment", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1000, "resp.Index")
	assert.Equal(d, resp.Deployment, "Returned deployment not equal")
}

func TestDeploymentEndpoint_GetDeployment_Blocking(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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

	assert.Nil(state.UpsertJob(98, j1), "UpsertJob")
	assert.Nil(state.UpsertJob(99, j2), "UpsertJob")

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
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(999, j), "UpsertJob")
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

func TestDeploymentEndpoint_Fail_Rollback(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
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
	assert.Nil(state.UpsertJob(998, j), "UpsertJob")

	// Create the second job, deployment and alloc
	j2 := j.Copy()
	j2.Stable = false

	d := mock.Deployment()
	d.TaskGroups["web"].AutoRevert = true
	d.JobID = j2.ID
	d.JobVersion = j2.Version

	a := mock.Alloc()
	a.JobID = j.ID
	a.DeploymentID = d.ID

	assert.Nil(state.UpsertJob(999, j2), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(1001, []*structs.Allocation{a}), "UpsertAllocs")

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
	jout, err := state.JobByID(ws, j.ID)
	assert.Nil(err, "JobByID")
	assert.NotNil(jout, "job")
	assert.EqualValues(2, jout.Version, "reverted job version")
}

func TestDeploymentEndpoint_Pause(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(999, j), "UpsertJob")
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

func TestDeploymentEndpoint_Promote(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment, job and canary
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 2
	d := mock.Deployment()
	d.TaskGroups["web"].DesiredCanaries = 2
	d.JobID = j.ID
	a := mock.Alloc()
	d.TaskGroups[a.TaskGroup].PlacedCanaries = []string{a.ID}
	a.DeploymentID = d.ID
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}

	state := s1.fsm.State()
	assert.Nil(state.UpsertJob(999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(1001, []*structs.Allocation{a}), "UpsertAllocs")

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

func TestDeploymentEndpoint_SetAllocHealth(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
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
	assert.Nil(state.UpsertJob(999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(1001, []*structs.Allocation{a}), "UpsertAllocs")

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

func TestDeploymentEndpoint_SetAllocHealth_Rollback(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer s1.Shutdown()
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
	assert.Nil(state.UpsertJob(998, j), "UpsertJob")

	// Create the second job, deployment and alloc
	j2 := j.Copy()
	j2.Stable = false

	d := mock.Deployment()
	d.TaskGroups["web"].AutoRevert = true
	d.JobID = j2.ID
	d.JobVersion = j2.Version

	a := mock.Alloc()
	a.JobID = j.ID
	a.DeploymentID = d.ID

	assert.Nil(state.UpsertJob(999, j2), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(1001, []*structs.Allocation{a}), "UpsertAllocs")

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
	jout, err := state.JobByID(ws, j.ID)
	assert.Nil(err, "JobByID")
	assert.NotNil(jout, "job")
	assert.EqualValues(2, jout.Version, "reverted job version")
}

func TestDeploymentEndpoint_List(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the register request
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	state := s1.fsm.State()

	assert.Nil(state.UpsertJob(999, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

	// Lookup the deployments
	get := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.DeploymentListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1000, "Wrong Index")
	assert.Len(resp.Deployments, 1, "Deployments")
	assert.Equal(resp.Deployments[0].ID, d.ID, "Deployment ID")

	// Lookup the deploys by prefix
	get = &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{Region: "global", Prefix: d.ID[:4]},
	}

	var resp2 structs.DeploymentListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp2), "RPC")
	assert.EqualValues(resp.Index, 1000, "Wrong Index")
	assert.Len(resp2.Deployments, 1, "Deployments")
	assert.Equal(resp2.Deployments[0].ID, d.ID, "Deployment ID")
}

func TestDeploymentEndpoint_List_Blocking(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID

	assert.Nil(state.UpsertJob(999, j), "UpsertJob")

	// Upsert alloc triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertDeployment(3, d), "UpsertDeployment")
	})

	req := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
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

func TestDeploymentEndpoint_Allocations(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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

	assert.Nil(state.UpsertJob(998, j), "UpsertJob")
	assert.Nil(state.UpsertJobSummary(999, summary), "UpsertJobSummary")
	assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")
	assert.Nil(state.UpsertAllocs(1001, []*structs.Allocation{a}), "UpsertAllocs")

	// Lookup the allocations
	get := &structs.DeploymentSpecificRequest{
		DeploymentID: d.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.AllocListResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", get, &resp), "RPC")
	assert.EqualValues(1001, resp.Index, "Wrong Index")
	assert.Len(resp.Allocations, 1, "Allocations")
	assert.Equal(a.ID, resp.Allocations[0].ID, "Allocation ID")
}

func TestDeploymentEndpoint_Allocations_Blocking(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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

	assert.Nil(state.UpsertJob(1, j), "UpsertJob")
	assert.Nil(state.UpsertDeployment(2, d), "UpsertDeployment")
	assert.Nil(state.UpsertJobSummary(3, summary), "UpsertJobSummary")

	// Upsert alloc triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		assert.Nil(state.UpsertAllocs(4, []*structs.Allocation{a}), "UpsertAllocs")
	})

	req := &structs.DeploymentSpecificRequest{
		DeploymentID: d.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
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
		assert.Nil(state.UpdateAllocsFromClient(6, []*structs.Allocation{a2}), "bpdateAllocsFromClient")
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
