package nomad

import (
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestDeploymentEndpoint_List(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	deployment := mock.Deployment()
	state := s1.fsm.State()

	if err := state.UpsertDeployment(1000, deployment, false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the deployments
	get := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.DeploymentListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if len(resp.Deployments) != 1 {
		t.Fatalf("bad: %#v", resp.Deployments)
	}
	if resp.Deployments[0].ID != deployment.ID {
		t.Fatalf("bad: %#v", resp.Deployments[0])
	}

	// Lookup the deploys by prefix
	get = &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{Region: "global", Prefix: deployment.ID[:4]},
	}

	var resp2 structs.DeploymentListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Deployment.List", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp2.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 1000)
	}

	if len(resp2.Deployments) != 1 {
		t.Fatalf("bad: %#v", resp2.Deployments)
	}
	if resp2.Deployments[0].ID != deployment.ID {
		t.Fatalf("bad: %#v", resp2.Deployments[0])
	}
}

func TestDeploymentEndpoint_List_Blocking(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the deployment
	deployment := mock.Deployment()

	// Upsert alloc triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertDeployment(3, deployment, false); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.DeploymentListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.DeploymentListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Deployment.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 3 {
		t.Fatalf("Bad index: %d %d", resp.Index, 3)
	}
	if len(resp.Deployments) != 1 || resp.Deployments[0].ID != deployment.ID {
		t.Fatalf("bad: %#v", resp.Deployments)
	}

	// Deployment updates trigger watches
	deployment2 := deployment.Copy()
	deployment2.Status = structs.DeploymentStatusPaused
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertDeployment(5, deployment2, false); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 3
	start = time.Now()
	var resp2 structs.DeploymentListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Deployment.List", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 5 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 5)
	}
	if len(resp2.Deployments) != 1 || resp.Deployments[0].ID != deployment2.ID ||
		resp2.Deployments[0].Status != structs.DeploymentStatusPaused {
		t.Fatalf("bad: %#v", resp2.Deployments)
	}
}

func TestDeploymentEndpoint_Allocations(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	deployment := mock.Deployment()
	alloc := mock.Alloc()
	alloc.DeploymentID = deployment.ID
	summary := mock.JobSummary(alloc.JobID)
	state := s1.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertDeployment(1000, deployment, false); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(1001, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the allocations
	get := &structs.DeploymentSpecificRequest{
		DeploymentID: deployment.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.AllocListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1001 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1001)
	}

	if len(resp.Allocations) != 1 {
		t.Fatalf("bad: %#v", resp.Allocations)
	}
	if resp.Allocations[0].ID != alloc.ID {
		t.Fatalf("bad: %#v", resp.Allocations[0])
	}
}

func TestDeploymentEndpoint_Allocations_Blocking(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the alloc
	deployment := mock.Deployment()
	alloc := mock.Alloc()
	alloc.DeploymentID = deployment.ID
	summary := mock.JobSummary(alloc.JobID)

	if err := state.UpsertDeployment(1, deployment, false); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertJobSummary(2, summary); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Upsert alloc triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertAllocs(3, []*structs.Allocation{alloc}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.DeploymentSpecificRequest{
		DeploymentID: deployment.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.AllocListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 3 {
		t.Fatalf("Bad index: %d %d", resp.Index, 3)
	}
	if len(resp.Allocations) != 1 || resp.Allocations[0].ID != alloc.ID {
		t.Fatalf("bad: %#v", resp.Allocations)
	}

	// Client updates trigger watches
	alloc2 := mock.Alloc()
	alloc2.ID = alloc.ID
	alloc2.DeploymentID = alloc.DeploymentID
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	time.AfterFunc(100*time.Millisecond, func() {
		state.UpsertJobSummary(4, mock.JobSummary(alloc2.JobID))
		if err := state.UpdateAllocsFromClient(5, []*structs.Allocation{alloc2}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 3
	start = time.Now()
	var resp2 structs.AllocListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Deployment.Allocations", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 5 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 5)
	}
	if len(resp2.Allocations) != 1 || resp.Allocations[0].ID != alloc.ID ||
		resp2.Allocations[0].ClientStatus != structs.AllocClientStatusRunning {
		t.Fatalf("bad: %#v", resp2.Allocations)
	}
}
