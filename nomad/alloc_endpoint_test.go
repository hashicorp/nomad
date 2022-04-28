package nomad

import (
	"reflect"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestAllocEndpoint_List(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc := mock.Alloc()
	summary := mock.JobSummary(alloc.JobID)
	state := s1.fsm.State()

	if err := state.UpsertJobSummary(999, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the allocations
	get := &structs.AllocListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp structs.AllocListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if len(resp.Allocations) != 1 {
		t.Fatalf("bad: %#v", resp.Allocations)
	}
	if resp.Allocations[0].ID != alloc.ID {
		t.Fatalf("bad: %#v", resp.Allocations[0])
	}

	// Lookup the allocations by prefix
	get = &structs.AllocListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
			Prefix:    alloc.ID[:4],
		},
	}

	var resp2 structs.AllocListResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp2))
	require.Equal(t, uint64(1000), resp2.Index)
	require.Len(t, resp2.Allocations, 1)
	require.Equal(t, alloc.ID, resp2.Allocations[0].ID)
}

func TestAllocEndpoint_List_Fields(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()

	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create a running alloc
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.TaskStates = map[string]*structs.TaskState{
		"web": {
			State:     structs.TaskStateRunning,
			StartedAt: time.Now(),
		},
	}
	summary := mock.JobSummary(alloc.JobID)
	state := s1.fsm.State()

	require.NoError(t, state.UpsertJobSummary(999, summary))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	cases := []struct {
		Name   string
		Fields *structs.AllocStubFields
		Assert func(t *testing.T, allocs []*structs.AllocListStub)
	}{
		{
			Name:   "None",
			Fields: nil,
			Assert: func(t *testing.T, allocs []*structs.AllocListStub) {
				require.Nil(t, allocs[0].AllocatedResources)
				require.Len(t, allocs[0].TaskStates, 1)
			},
		},
		{
			Name:   "Default",
			Fields: structs.NewAllocStubFields(),
			Assert: func(t *testing.T, allocs []*structs.AllocListStub) {
				require.Nil(t, allocs[0].AllocatedResources)
				require.Len(t, allocs[0].TaskStates, 1)
			},
		},
		{
			Name: "Resources",
			Fields: &structs.AllocStubFields{
				Resources:  true,
				TaskStates: false,
			},
			Assert: func(t *testing.T, allocs []*structs.AllocListStub) {
				require.NotNil(t, allocs[0].AllocatedResources)
				require.Len(t, allocs[0].TaskStates, 0)
			},
		},
		{
			Name: "NoTaskStates",
			Fields: &structs.AllocStubFields{
				Resources:  false,
				TaskStates: false,
			},
			Assert: func(t *testing.T, allocs []*structs.AllocListStub) {
				require.Nil(t, allocs[0].AllocatedResources)
				require.Len(t, allocs[0].TaskStates, 0)
			},
		},
		{
			Name: "Both",
			Fields: &structs.AllocStubFields{
				Resources:  true,
				TaskStates: true,
			},
			Assert: func(t *testing.T, allocs []*structs.AllocListStub) {
				require.NotNil(t, allocs[0].AllocatedResources)
				require.Len(t, allocs[0].TaskStates, 1)
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			get := &structs.AllocListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
				Fields: tc.Fields,
			}
			var resp structs.AllocListResponse
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp))
			require.Equal(t, uint64(1000), resp.Index)
			require.Len(t, resp.Allocations, 1)
			require.Equal(t, alloc.ID, resp.Allocations[0].ID)
			tc.Assert(t, resp.Allocations)
		})
	}

}

func TestAllocEndpoint_List_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the alloc
	alloc := mock.Alloc()
	allocs := []*structs.Allocation{alloc}
	summary := mock.JobSummary(alloc.JobID)
	state := s1.fsm.State()

	assert.Nil(state.UpsertJobSummary(999, summary), "UpsertJobSummary")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs), "UpsertAllocs")

	stubAllocs := []*structs.AllocListStub{alloc.Stub(nil)}
	stubAllocs[0].CreateIndex = 1000
	stubAllocs[0].ModifyIndex = 1000

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	// Lookup the allocs without a token and expect failure
	get := &structs.AllocListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}
	var resp structs.AllocListResponse
	assert.NotNil(msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp), "RPC")

	// Try with a valid token
	get.AuthToken = validToken.SecretID
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1000, "resp.Index")
	assert.Equal(stubAllocs, resp.Allocations, "Returned alloc list not equal")

	// Try with a invalid token
	get.AuthToken = invalidToken.SecretID
	err := msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp)
	assert.NotNil(err, "RPC")
	assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())

	// Try with a root token
	get.AuthToken = root.SecretID
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp), "RPC")
	assert.EqualValues(resp.Index, 1000, "resp.Index")
	assert.Equal(stubAllocs, resp.Allocations, "Returned alloc list not equal")
}

func TestAllocEndpoint_List_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the alloc
	alloc := mock.Alloc()

	summary := mock.JobSummary(alloc.JobID)
	if err := state.UpsertJobSummary(1, summary); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Upsert alloc triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 2, []*structs.Allocation{alloc}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.AllocListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			Namespace:     structs.DefaultNamespace,
			MinQueryIndex: 1,
		},
	}
	start := time.Now()
	var resp structs.AllocListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Alloc.List", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 2 {
		t.Fatalf("Bad index: %d %d", resp.Index, 2)
	}
	if len(resp.Allocations) != 1 || resp.Allocations[0].ID != alloc.ID {
		t.Fatalf("bad: %#v", resp.Allocations)
	}

	// Client updates trigger watches
	alloc2 := mock.Alloc()
	alloc2.ID = alloc.ID
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	time.AfterFunc(100*time.Millisecond, func() {
		state.UpsertJobSummary(3, mock.JobSummary(alloc2.JobID))
		if err := state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 4, []*structs.Allocation{alloc2}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 3
	start = time.Now()
	var resp2 structs.AllocListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Alloc.List", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 4 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 4)
	}
	if len(resp2.Allocations) != 1 || resp.Allocations[0].ID != alloc.ID ||
		resp2.Allocations[0].ClientStatus != structs.AllocClientStatusRunning {
		t.Fatalf("bad: %#v", resp2.Allocations)
	}
}

// TestAllocEndpoint_List_AllNamespaces_OSS asserts that server
// returns all allocations across namespaces.
func TestAllocEndpoint_List_AllNamespaces_OSS(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// two namespaces
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	require.NoError(t, state.UpsertNamespaces(900, []*structs.Namespace{ns1, ns2}))

	// Create the allocations
	alloc1 := mock.Alloc()
	alloc1.ID = "a" + alloc1.ID[1:]
	alloc1.Namespace = ns1.Name
	alloc2 := mock.Alloc()
	alloc2.ID = "b" + alloc2.ID[1:]
	alloc2.Namespace = ns2.Name
	summary1 := mock.JobSummary(alloc1.JobID)
	summary2 := mock.JobSummary(alloc2.JobID)

	require.NoError(t, state.UpsertJobSummary(999, summary1))
	require.NoError(t, state.UpsertJobSummary(999, summary2))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2}))

	t.Run("looking up all allocations", func(t *testing.T) {
		get := &structs.AllocListRequest{
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: "*",
			},
		}
		var resp structs.AllocListResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp))
		require.Equal(t, uint64(1000), resp.Index)
		require.Len(t, resp.Allocations, 2)
		require.ElementsMatch(t,
			[]string{resp.Allocations[0].ID, resp.Allocations[1].ID},
			[]string{alloc1.ID, alloc2.ID})
	})

	t.Run("looking up allocations with prefix", func(t *testing.T) {
		get := &structs.AllocListRequest{
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: "*",
				// allocations were constructed above to have non-matching prefix
				Prefix: alloc1.ID[:4],
			},
		}
		var resp structs.AllocListResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp))
		require.Equal(t, uint64(1000), resp.Index)
		require.Len(t, resp.Allocations, 1)
		require.Equal(t, alloc1.ID, resp.Allocations[0].ID)
		require.Equal(t, alloc1.Namespace, resp.Allocations[0].Namespace)
	})

	t.Run("looking up allocations with mismatch prefix", func(t *testing.T) {
		// allocations were constructed above to have prefix starting with "a" or "b"
		badPrefix := "cc"

		get := &structs.AllocListRequest{
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: "*",
				Prefix:    badPrefix,
			},
		}
		var resp structs.AllocListResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp))
		require.Equal(t, uint64(1000), resp.Index)
		require.Empty(t, resp.Allocations)
	})
}

func TestAllocEndpoint_GetAlloc(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	prevAllocID := uuid.Generate()
	alloc := mock.Alloc()
	alloc.RescheduleTracker = &structs.RescheduleTracker{
		Events: []*structs.RescheduleEvent{
			{RescheduleTime: time.Now().UTC().UnixNano(), PrevNodeID: "boom", PrevAllocID: prevAllocID},
		},
	}
	state := s1.fsm.State()
	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the alloc
	get := &structs.AllocSpecificRequest{
		AllocID:      alloc.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.SingleAllocResponse
	if err := msgpackrpc.CallWithCodec(codec, "Alloc.GetAlloc", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if !reflect.DeepEqual(alloc, resp.Alloc) {
		t.Fatalf("bad: %#v", resp.Alloc)
	}
}

func TestAllocEndpoint_GetAlloc_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the alloc
	alloc := mock.Alloc()
	allocs := []*structs.Allocation{alloc}
	summary := mock.JobSummary(alloc.JobID)
	state := s1.fsm.State()

	assert.Nil(state.UpsertJobSummary(999, summary), "UpsertJobSummary")
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs), "UpsertAllocs")

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))

	getReq := func() *structs.AllocSpecificRequest {
		return &structs.AllocSpecificRequest{
			AllocID: alloc.ID,
			QueryOptions: structs.QueryOptions{
				Region: "global",
			},
		}
	}

	cases := []struct {
		Name string
		F    func(t *testing.T)
	}{
		// Lookup the alloc without a token and expect failure
		{
			Name: "no-token",
			F: func(t *testing.T) {
				var resp structs.SingleAllocResponse
				err := msgpackrpc.CallWithCodec(codec, "Alloc.GetAlloc", getReq(), &resp)
				require.True(t, structs.IsErrUnknownAllocation(err), "expected unknown alloc but found: %v", err)
			},
		},

		// Try with a valid ACL token
		{
			Name: "valid-token",
			F: func(t *testing.T) {
				get := getReq()
				get.AuthToken = validToken.SecretID
				get.AllocID = alloc.ID
				var resp structs.SingleAllocResponse
				require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.GetAlloc", get, &resp), "RPC")
				require.EqualValues(t, resp.Index, 1000, "resp.Index")
				require.Equal(t, alloc, resp.Alloc, "Returned alloc not equal")
			},
		},

		// Try with a valid Node.SecretID
		{
			Name: "valid-node-secret",
			F: func(t *testing.T) {
				node := mock.Node()
				assert.Nil(state.UpsertNode(structs.MsgTypeTestSetup, 1005, node))
				get := getReq()
				get.AuthToken = node.SecretID
				get.AllocID = alloc.ID
				var resp structs.SingleAllocResponse
				require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.GetAlloc", get, &resp), "RPC")
				require.EqualValues(t, resp.Index, 1000, "resp.Index")
				require.Equal(t, alloc, resp.Alloc, "Returned alloc not equal")
			},
		},

		// Try with a invalid token
		{
			Name: "invalid-token",
			F: func(t *testing.T) {
				get := getReq()
				get.AuthToken = invalidToken.SecretID
				get.AllocID = alloc.ID
				var resp structs.SingleAllocResponse
				err := msgpackrpc.CallWithCodec(codec, "Alloc.GetAlloc", get, &resp)
				require.NotNil(t, err, "RPC")
				require.True(t, structs.IsErrUnknownAllocation(err), "expected unknown alloc but found: %v", err)
			},
		},

		// Try with a root token
		{
			Name: "root-token",
			F: func(t *testing.T) {
				get := getReq()
				get.AuthToken = root.SecretID
				get.AllocID = alloc.ID
				var resp structs.SingleAllocResponse
				require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.GetAlloc", get, &resp), "RPC")
				require.EqualValues(t, resp.Index, 1000, "resp.Index")
				require.Equal(t, alloc, resp.Alloc, "Returned alloc not equal")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, tc.F)
	}
}

func TestAllocEndpoint_GetAlloc_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the allocs
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()

	// First create an unrelated alloc
	time.AfterFunc(100*time.Millisecond, func() {
		state.UpsertJobSummary(99, mock.JobSummary(alloc1.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 100, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Create the alloc we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		state.UpsertJobSummary(199, mock.JobSummary(alloc2.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the allocs
	get := &structs.AllocSpecificRequest{
		AllocID: alloc2.ID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.SingleAllocResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Alloc.GetAlloc", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Alloc == nil || resp.Alloc.ID != alloc2.ID {
		t.Fatalf("bad: %#v", resp.Alloc)
	}
}

func TestAllocEndpoint_GetAllocs(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc := mock.Alloc()
	alloc2 := mock.Alloc()
	state := s1.fsm.State()
	state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID))
	state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the allocs
	get := &structs.AllocsGetRequest{
		AllocIDs: []string{alloc.ID, alloc2.ID},
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp structs.AllocsGetResponse
	if err := msgpackrpc.CallWithCodec(codec, "Alloc.GetAllocs", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index != 1000 {
		t.Fatalf("Bad index: %d %d", resp.Index, 1000)
	}

	if len(resp.Allocs) != 2 {
		t.Fatalf("bad: %#v", resp.Allocs)
	}

	// Lookup nonexistent allocs.
	get = &structs.AllocsGetRequest{
		AllocIDs:     []string{"foo"},
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Alloc.GetAllocs", get, &resp); err == nil {
		t.Fatalf("expect error")
	}
}

func TestAllocEndpoint_GetAllocs_Blocking(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the allocs
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()

	// First create an unrelated alloc
	time.AfterFunc(100*time.Millisecond, func() {
		state.UpsertJobSummary(99, mock.JobSummary(alloc1.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 100, []*structs.Allocation{alloc1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Create the alloc we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		state.UpsertJobSummary(199, mock.JobSummary(alloc2.JobID))
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the allocs
	get := &structs.AllocsGetRequest{
		AllocIDs: []string{alloc1.ID, alloc2.ID},
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.AllocsGetResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Alloc.GetAllocs", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Allocs) != 2 {
		t.Fatalf("bad: %#v", resp.Allocs)
	}
}

func TestAllocEndpoint_UpdateDesiredTransition(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc := mock.Alloc()
	alloc2 := mock.Alloc()
	state := s1.fsm.State()
	require.Nil(state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID)))
	require.Nil(state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID)))
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc, alloc2}))

	t1 := &structs.DesiredTransition{
		Migrate: helper.BoolToPtr(true),
	}

	// Update the allocs desired status
	get := &structs.AllocUpdateDesiredTransitionRequest{
		Allocs: map[string]*structs.DesiredTransition{
			alloc.ID:  t1,
			alloc2.ID: t1,
		},
		Evals: []*structs.Evaluation{
			{
				ID:             uuid.Generate(),
				Namespace:      alloc.Namespace,
				Priority:       alloc.Job.Priority,
				Type:           alloc.Job.Type,
				TriggeredBy:    structs.EvalTriggerNodeDrain,
				JobID:          alloc.Job.ID,
				JobModifyIndex: alloc.Job.ModifyIndex,
				Status:         structs.EvalStatusPending,
			},
			{
				ID:             uuid.Generate(),
				Namespace:      alloc2.Namespace,
				Priority:       alloc2.Job.Priority,
				Type:           alloc2.Job.Type,
				TriggeredBy:    structs.EvalTriggerNodeDrain,
				JobID:          alloc2.Job.ID,
				JobModifyIndex: alloc2.Job.ModifyIndex,
				Status:         structs.EvalStatusPending,
			},
		},
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}

	// Try without permissions
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Alloc.UpdateDesiredTransition", get, &resp)
	require.NotNil(err)
	require.True(structs.IsErrPermissionDenied(err))

	// Try with permissions
	get.WriteRequest.AuthToken = s1.getLeaderAcl()
	var resp2 structs.GenericResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Alloc.UpdateDesiredTransition", get, &resp2))
	require.NotZero(resp2.Index)

	// Look up the allocations
	out1, err := state.AllocByID(nil, alloc.ID)
	require.Nil(err)
	out2, err := state.AllocByID(nil, alloc.ID)
	require.Nil(err)
	e1, err := state.EvalByID(nil, get.Evals[0].ID)
	require.Nil(err)
	e2, err := state.EvalByID(nil, get.Evals[1].ID)
	require.Nil(err)

	require.NotNil(out1.DesiredTransition.Migrate)
	require.NotNil(out2.DesiredTransition.Migrate)
	require.NotNil(e1)
	require.NotNil(e2)
	require.True(*out1.DesiredTransition.Migrate)
	require.True(*out2.DesiredTransition.Migrate)
}

func TestAllocEndpoint_Stop_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	alloc := mock.Alloc()
	alloc2 := mock.Alloc()
	state := s1.fsm.State()
	require.Nil(state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID)))
	require.Nil(state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID)))
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc, alloc2}))

	req := &structs.AllocStopRequest{
		AllocID: alloc.ID,
	}
	req.Namespace = structs.DefaultNamespace
	req.Region = alloc.Job.Region

	// Try without permissions
	var resp structs.AllocStopResponse
	err := msgpackrpc.CallWithCodec(codec, "Alloc.Stop", req, &resp)
	require.True(structs.IsErrPermissionDenied(err), "expected permissions error, got: %v", err)

	// Try with management permissions
	req.WriteRequest.AuthToken = s1.getLeaderAcl()
	var resp2 structs.AllocStopResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Alloc.Stop", req, &resp2))
	require.NotZero(resp2.Index)

	// Try with alloc-lifecycle permissions
	validToken := mock.CreatePolicyAndToken(t, state, 1002, "valid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityAllocLifecycle}))
	req.WriteRequest.AuthToken = validToken.SecretID
	req.AllocID = alloc2.ID

	var resp3 structs.AllocStopResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Alloc.Stop", req, &resp3))
	require.NotZero(resp3.Index)

	// Look up the allocations
	out1, err := state.AllocByID(nil, alloc.ID)
	require.Nil(err)
	out2, err := state.AllocByID(nil, alloc2.ID)
	require.Nil(err)
	e1, err := state.EvalByID(nil, resp2.EvalID)
	require.Nil(err)
	e2, err := state.EvalByID(nil, resp3.EvalID)
	require.Nil(err)

	require.NotNil(out1.DesiredTransition.Migrate)
	require.NotNil(out2.DesiredTransition.Migrate)
	require.NotNil(e1)
	require.NotNil(e2)
	require.True(*out1.DesiredTransition.Migrate)
	require.True(*out2.DesiredTransition.Migrate)
}

func TestAllocEndpoint_List_AllNamespaces_ACL_OSS(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// two namespaces
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	require.NoError(t, state.UpsertNamespaces(900, []*structs.Namespace{ns1, ns2}))

	// Create the allocations
	alloc1 := mock.Alloc()
	alloc1.ID = "a" + alloc1.ID[1:]
	alloc1.Namespace = ns1.Name
	alloc2 := mock.Alloc()
	alloc2.ID = "b" + alloc2.ID[1:]
	alloc2.Namespace = ns2.Name
	summary1 := mock.JobSummary(alloc1.JobID)
	summary2 := mock.JobSummary(alloc2.JobID)

	require.NoError(t, state.UpsertJobSummary(999, summary1))
	require.NoError(t, state.UpsertJobSummary(999, summary2))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2}))
	alloc1.CreateIndex = 1000
	alloc1.ModifyIndex = 1000
	alloc2.CreateIndex = 1000
	alloc2.ModifyIndex = 1000

	everythingButReadJob := []string{
		acl.NamespaceCapabilityDeny,
		acl.NamespaceCapabilityListJobs,
		// acl.NamespaceCapabilityReadJob,
		acl.NamespaceCapabilitySubmitJob,
		acl.NamespaceCapabilityDispatchJob,
		acl.NamespaceCapabilityReadLogs,
		acl.NamespaceCapabilityReadFS,
		acl.NamespaceCapabilityAllocExec,
		acl.NamespaceCapabilityAllocNodeExec,
		acl.NamespaceCapabilityAllocLifecycle,
		acl.NamespaceCapabilitySentinelOverride,
		acl.NamespaceCapabilityCSIRegisterPlugin,
		acl.NamespaceCapabilityCSIWriteVolume,
		acl.NamespaceCapabilityCSIReadVolume,
		acl.NamespaceCapabilityCSIListVolume,
		acl.NamespaceCapabilityCSIMountVolume,
		acl.NamespaceCapabilityListScalingPolicies,
		acl.NamespaceCapabilityReadScalingPolicy,
		acl.NamespaceCapabilityReadJobScaling,
		acl.NamespaceCapabilityScaleJob,
		acl.NamespaceCapabilitySubmitRecommendation,
	}

	ns1token := mock.CreatePolicyAndToken(t, state, 1001, "ns1",
		mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob}))
	ns1tokenInsufficient := mock.CreatePolicyAndToken(t, state, 1001, "ns1-insufficient",
		mock.NamespacePolicy(ns1.Name, "", everythingButReadJob))
	ns2token := mock.CreatePolicyAndToken(t, state, 1001, "ns2",
		mock.NamespacePolicy(ns2.Name, "", []string{acl.NamespaceCapabilityReadJob}))
	bothToken := mock.CreatePolicyAndToken(t, state, 1001, "nsBoth",
		mock.NamespacePolicy(ns1.Name, "", []string{acl.NamespaceCapabilityReadJob})+
			mock.NamespacePolicy(ns2.Name, "", []string{acl.NamespaceCapabilityReadJob}))

	cases := []struct {
		Label     string
		Namespace string
		Token     string
		Allocs    []*structs.Allocation
		Error     bool
		Message   string
		Prefix    string
	}{
		{
			Label:     "all namespaces with sufficient token",
			Namespace: "*",
			Token:     bothToken.SecretID,
			Allocs:    []*structs.Allocation{alloc1, alloc2},
		},
		{
			Label:     "all namespaces with root token",
			Namespace: "*",
			Token:     root.SecretID,
			Allocs:    []*structs.Allocation{alloc1, alloc2},
		},
		{
			Label:     "all namespaces with ns1 token",
			Namespace: "*",
			Token:     ns1token.SecretID,
			Allocs:    []*structs.Allocation{alloc1},
		},
		{
			Label:     "all namespaces with ns2 token",
			Namespace: "*",
			Token:     ns2token.SecretID,
			Allocs:    []*structs.Allocation{alloc2},
		},
		{
			Label:     "all namespaces with bad token",
			Namespace: "*",
			Token:     uuid.Generate(),
			Error:     true,
			Message:   structs.ErrTokenNotFound.Error(),
		},
		{
			Label:     "all namespaces with insufficient token",
			Namespace: "*",
			Allocs:    []*structs.Allocation{},
			Token:     ns1tokenInsufficient.SecretID,
		},
		{
			Label:     "ns1 with ns1 token",
			Namespace: ns1.Name,
			Token:     ns1token.SecretID,
			Allocs:    []*structs.Allocation{alloc1},
		},
		{
			Label:     "ns1 with root token",
			Namespace: ns1.Name,
			Token:     root.SecretID,
			Allocs:    []*structs.Allocation{alloc1},
		},
		{
			Label:     "ns1 with ns2 token",
			Namespace: ns1.Name,
			Token:     ns2token.SecretID,
			Error:     true,
		},
		{
			Label:     "ns1 with invalid token",
			Namespace: ns1.Name,
			Token:     uuid.Generate(),
			Error:     true,
			Message:   structs.ErrTokenNotFound.Error(),
		},
		{
			Label:     "bad namespace with root token",
			Namespace: uuid.Generate(),
			Token:     root.SecretID,
			Allocs:    []*structs.Allocation{},
		},
		{
			Label:     "all namespaces with prefix",
			Namespace: "*",
			Prefix:    alloc1.ID[:2],
			Token:     root.SecretID,
			Allocs:    []*structs.Allocation{alloc1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Label, func(t *testing.T) {

			get := &structs.AllocListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Namespace: tc.Namespace,
					Prefix:    tc.Prefix,
					AuthToken: tc.Token,
				},
			}
			var resp structs.AllocListResponse
			err := msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp)
			if tc.Error {
				require.Error(t, err)
				if tc.Message != "" {
					require.Equal(t, err.Error(), tc.Message)
				} else {
					require.Equal(t, err.Error(), structs.ErrPermissionDenied.Error())
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, uint64(1000), resp.Index)
				exp := make([]*structs.AllocListStub, len(tc.Allocs))
				for i, a := range tc.Allocs {
					exp[i] = a.Stub(nil)
				}
				require.ElementsMatch(t, exp, resp.Allocations)
			}
		})
	}

}
