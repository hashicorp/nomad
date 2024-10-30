// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"reflect"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocEndpoint_List(t *testing.T) {
	ci.Parallel(t)

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

	// Lookup allocations with a filter
	get = &structs.AllocListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
			Filter:    "TaskGroup == web",
		},
	}

	var resp3 structs.AllocListResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp3))
	require.Equal(t, uint64(1000), resp3.Index)
	require.Len(t, resp3.Allocations, 1)
	require.Equal(t, alloc.ID, resp3.Allocations[0].ID)
}

func TestAllocEndpoint_List_PaginationFiltering(t *testing.T) {
	ci.Parallel(t)
	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// create a set of allocs and field values to filter on. these are in the order
	// that the state store will return them from the iterator (sorted by create
	// index), for ease of writing tests.
	mocks := []struct {
		ids       []string
		namespace string
		group     string
	}{
		{ids: []string{"aaaa1111-3350-4b4b-d185-0e1992ed43e9"}},                           // 0
		{ids: []string{"aaaaaa22-3350-4b4b-d185-0e1992ed43e9"}},                           // 1
		{ids: []string{"aaaaaa33-3350-4b4b-d185-0e1992ed43e9"}, namespace: "non-default"}, // 2
		{ids: []string{"aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"}, group: "bar"},             // 3
		{ids: []string{"aaaaaabb-3350-4b4b-d185-0e1992ed43e9"}, group: "goo"},             // 4
		{ids: []string{"aaaaaacc-3350-4b4b-d185-0e1992ed43e9"}},                           // 5
		{ids: []string{"aaaaaadd-3350-4b4b-d185-0e1992ed43e9"}, group: "bar"},             // 6
		{ids: []string{"aaaaaaee-3350-4b4b-d185-0e1992ed43e9"}, group: "goo"},             // 7
		{ids: []string{"aaaaaaff-3350-4b4b-d185-0e1992ed43e9"}, group: "bar"},             // 8
		{ids: []string{"00000111-3350-4b4b-d185-0e1992ed43e9"}},                           // 9
		{ids: []string{ // 10
			"00000222-3350-4b4b-d185-0e1992ed43e9",
			"00000333-3350-4b4b-d185-0e1992ed43e9",
		}},
		{}, // 11, index missing
		{ids: []string{"bbbb1111-3350-4b4b-d185-0e1992ed43e9"}}, // 12
	}

	state := s1.fsm.State()

	require.NoError(t, state.UpsertNamespaces(1099, []*structs.Namespace{
		{Name: "non-default"},
	}))

	var allocs []*structs.Allocation
	for i, m := range mocks {
		allocsInTx := []*structs.Allocation{}
		for _, id := range m.ids {
			alloc := mock.Alloc()
			alloc.ID = id
			if m.namespace != "" {
				alloc.Namespace = m.namespace
			}
			if m.group != "" {
				alloc.TaskGroup = m.group
			}
			allocs = append(allocs, alloc)
			allocsInTx = append(allocsInTx, alloc)
		}
		// other fields
		index := 1000 + uint64(i)
		require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, index, allocsInTx))
	}

	aclToken := mock.CreatePolicyAndToken(t,
		state, 1100, "test-valid-read",
		mock.NamespacePolicy("*", "read", nil),
	).SecretID

	cases := []struct {
		name         string
		namespace    string
		prefix       string
		nextToken    string
		pageSize     int32
		filter       string
		expIDs       []string
		expNextToken string
		expErr       string
	}{
		{
			name:     "test01 size-2 page-1 ns-default",
			pageSize: 2,
			expIDs: []string{ // first two items
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
			expNextToken: "1003.aaaaaaaa-3350-4b4b-d185-0e1992ed43e9", // next one in default ns
		},
		{
			name:     "test02 size-2 page-1 ns-default with-prefix",
			prefix:   "aaaa",
			pageSize: 2,
			expIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
			expNextToken: "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
		},
		{
			name:         "test03 size-2 page-2 ns-default",
			pageSize:     2,
			nextToken:    "1003.aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
			expNextToken: "1005.aaaaaacc-3350-4b4b-d185-0e1992ed43e9",
			expIDs: []string{
				"aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaabb-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:         "test04 size-2 page-2 ns-default with prefix",
			prefix:       "aaaa",
			pageSize:     2,
			nextToken:    "aaaaaabb-3350-4b4b-d185-0e1992ed43e9",
			expNextToken: "aaaaaadd-3350-4b4b-d185-0e1992ed43e9",
			expIDs: []string{
				"aaaaaabb-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaacc-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:      "test05 go-bexpr filter",
			filter:    `TaskGroup == "goo"`,
			nextToken: "",
			expIDs: []string{
				"aaaaaabb-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaaee-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:         "test06 go-bexpr filter with pagination",
			filter:       `TaskGroup == "bar"`,
			pageSize:     2,
			expNextToken: "1008.aaaaaaff-3350-4b4b-d185-0e1992ed43e9",
			expIDs: []string{
				"aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaadd-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:      "test07 go-bexpr filter namespace",
			namespace: "non-default",
			filter:    `ID contains "aaa"`,
			expIDs: []string{
				"aaaaaa33-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:      "test08 go-bexpr wrong namespace",
			namespace: "default",
			filter:    `Namespace == "non-default"`,
			expIDs:    []string(nil),
		},
		{
			name:   "test09 go-bexpr invalid expression",
			filter: `NotValid`,
			expErr: "failed to read filter expression",
		},
		{
			name:   "test10 go-bexpr invalid field",
			filter: `InvalidField == "value"`,
			expErr: "error finding value in datum",
		},
		{
			name:         "test11 non-lexicographic order",
			pageSize:     1,
			nextToken:    "1009.00000111-3350-4b4b-d185-0e1992ed43e9",
			expNextToken: "1010.00000222-3350-4b4b-d185-0e1992ed43e9",
			expIDs: []string{
				"00000111-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:         "test12 same index",
			pageSize:     1,
			nextToken:    "1010.00000222-3350-4b4b-d185-0e1992ed43e9",
			expNextToken: "1010.00000333-3350-4b4b-d185-0e1992ed43e9",
			expIDs: []string{
				"00000222-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:      "test13 missing index",
			pageSize:  1,
			nextToken: "1011.e9522802-0cd8-4b1d-9c9e-ab3d97938371",
			expIDs: []string{
				"bbbb1111-3350-4b4b-d185-0e1992ed43e9",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req = &structs.AllocListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Namespace: tc.namespace,
					Prefix:    tc.prefix,
					PerPage:   tc.pageSize,
					NextToken: tc.nextToken,
					Filter:    tc.filter,
				},
				Fields: &structs.AllocStubFields{
					Resources:  false,
					TaskStates: false,
				},
			}
			req.AuthToken = aclToken
			var resp structs.AllocListResponse
			err := msgpackrpc.CallWithCodec(codec, "Alloc.List", req, &resp)
			if tc.expErr == "" {
				require.NoError(t, err)
			} else {
				require.Contains(t, err, tc.expErr)
			}

			var gotIDs []string
			for _, alloc := range resp.Allocations {
				gotIDs = append(gotIDs, alloc.ID)
			}
			require.Equal(t, tc.expIDs, gotIDs)
			require.Equal(t, tc.expNextToken, resp.QueryMeta.NextToken)
		})
	}
}

func TestAllocEndpoint_List_order(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create register requests
	uuid1 := uuid.Generate()
	alloc1 := mock.Alloc()
	alloc1.ID = uuid1

	uuid2 := uuid.Generate()
	alloc2 := mock.Alloc()
	alloc2.ID = uuid2

	uuid3 := uuid.Generate()
	alloc3 := mock.Alloc()
	alloc3.ID = uuid3

	err := s1.fsm.State().UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1})
	require.NoError(t, err)

	err = s1.fsm.State().UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc2})
	require.NoError(t, err)

	err = s1.fsm.State().UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc3})
	require.NoError(t, err)

	// update alloc2 again so we can later assert create index order did not change
	err = s1.fsm.State().UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{alloc2})
	require.NoError(t, err)

	t.Run("default", func(t *testing.T) {
		// Lookup the allocations in the default order (oldest first)
		get := &structs.AllocListRequest{
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: "*",
			},
		}

		var resp structs.AllocListResponse
		err = msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp)
		require.NoError(t, err)
		require.Equal(t, uint64(1003), resp.Index)
		require.Len(t, resp.Allocations, 3)

		// Assert returned order is by CreateIndex (ascending)
		require.Equal(t, uint64(1000), resp.Allocations[0].CreateIndex)
		require.Equal(t, uuid1, resp.Allocations[0].ID)

		require.Equal(t, uint64(1001), resp.Allocations[1].CreateIndex)
		require.Equal(t, uuid2, resp.Allocations[1].ID)

		require.Equal(t, uint64(1002), resp.Allocations[2].CreateIndex)
		require.Equal(t, uuid3, resp.Allocations[2].ID)
	})

	t.Run("reverse", func(t *testing.T) {
		// Lookup the allocations in reverse order (newest first)
		get := &structs.AllocListRequest{
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: "*",
				Reverse:   true,
			},
		}

		var resp structs.AllocListResponse
		err = msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp)
		require.NoError(t, err)
		require.Equal(t, uint64(1003), resp.Index)
		require.Len(t, resp.Allocations, 3)

		// Assert returned order is by CreateIndex (descending)
		require.Equal(t, uint64(1002), resp.Allocations[0].CreateIndex)
		require.Equal(t, uuid3, resp.Allocations[0].ID)

		require.Equal(t, uint64(1001), resp.Allocations[1].CreateIndex)
		require.Equal(t, uuid2, resp.Allocations[1].ID)

		require.Equal(t, uint64(1000), resp.Allocations[2].CreateIndex)
		require.Equal(t, uuid1, resp.Allocations[2].ID)
	})
}

func TestAllocEndpoint_List_Fields(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	uuid1 := uuid.Generate()
	alloc1 := mock.Alloc()
	alloc1.ID = uuid1
	alloc1.Namespace = ns1.Name

	uuid2 := uuid.Generate()
	alloc2 := mock.Alloc()
	alloc2.ID = uuid2
	alloc2.Namespace = ns2.Name

	summary1 := mock.JobSummary(alloc1.JobID)
	summary2 := mock.JobSummary(alloc2.JobID)

	require.NoError(t, state.UpsertJobSummary(1000, summary1))
	require.NoError(t, state.UpsertJobSummary(1001, summary2))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc1}))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{alloc2}))

	t.Run("looking up all allocations", func(t *testing.T) {
		get := &structs.AllocListRequest{
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: "*",
			},
		}
		var resp structs.AllocListResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp))
		require.Equal(t, uint64(1003), resp.Index)
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
		require.Equal(t, uint64(1003), resp.Index)
		require.Len(t, resp.Allocations, 1)
		require.Equal(t, alloc1.ID, resp.Allocations[0].ID)
		require.Equal(t, alloc1.Namespace, resp.Allocations[0].Namespace)
	})

	t.Run("looking up allocations with mismatch prefix", func(t *testing.T) {
		get := &structs.AllocListRequest{
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: "*",
				Prefix:    "000000", // unlikely to match
			},
		}
		var resp structs.AllocListResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.List", get, &resp))
		require.Equal(t, uint64(1003), resp.Index)
		require.Empty(t, resp.Allocations)
	})
}

func TestAllocEndpoint_GetAlloc(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	ci.Parallel(t)

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

	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1001, node)

	// Lookup the allocs
	get := &structs.AllocsGetRequest{
		AllocIDs: []string{alloc.ID, alloc2.ID},
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: node.SecretID,
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
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 50, node)

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
			AuthToken:     node.SecretID,
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
	ci.Parallel(t)
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
		Migrate: pointer.Of(true),
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
	ci.Parallel(t)
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
	ci.Parallel(t)

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
			Message:   structs.ErrPermissionDenied.Error(),
		},
		{
			Label:     "all namespaces with insufficient token",
			Namespace: "*",
			Token:     ns1tokenInsufficient.SecretID,
			Error:     true,
			Message:   structs.ErrPermissionDenied.Error(),
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
			Message:   structs.ErrPermissionDenied.Error(),
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

func TestAlloc_GetServiceRegistrations(t *testing.T) {
	ci.Parallel(t)

	// This function is a helper function to set up an allocation and service
	// which can be queried.
	correctSetupFn := func(s *Server) (error, string, *structs.ServiceRegistration) {
		// Generate an upsert an allocation.
		alloc := mock.Alloc()
		err := s.State().UpsertAllocs(structs.MsgTypeTestSetup, 10, []*structs.Allocation{alloc})
		if err != nil {
			return nil, "", nil
		}

		// Generate services. Set the allocation ID to the first, so it
		// matches the allocation. The alloc and first service both
		// reside in the default namespace.
		services := mock.ServiceRegistrations()
		services[0].AllocID = alloc.ID
		err = s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 20, services)

		return err, alloc.ID, services[0]
	}

	testCases := []struct {
		serverFn func(t *testing.T) (*Server, *structs.ACLToken, func())
		testFn   func(t *testing.T, s *Server, token *structs.ACLToken)
		name     string
	}{
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, allocID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Perform a lookup on the first service.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: allocID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.EqualValues(t, uint64(20), serviceRegResp.Index)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{service})
			},
			name: "ACLs disabled alloc found with regs",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate and upsert our services.
				services := mock.ServiceRegistrations()
				require.NoError(t, s.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 20, services))

				// Perform a lookup on the first service using the allocation
				// ID. This allocation does not exist within the Nomad state
				// meaning the service is orphaned or the caller used an
				// incorrect allocation ID.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: services[0].AllocID,
					QueryOptions: structs.QueryOptions{
						Namespace: services[0].Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err := msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.Nil(t, serviceRegResp.Services)
			},
			name: "ACLs disabled alloc not found",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, allocID, _ := correctSetupFn(s)
				require.NoError(t, err)

				// Perform a lookup on the first service using the allocation
				// ID but a random namespace. The namespace on the allocation
				// does therefore not match the request args.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: allocID,
					QueryOptions: structs.QueryOptions{
						Namespace: "platform",
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{})
			},
			name: "ACLs disabled alloc found in different namespace than request",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				server, cleanup := TestServer(t, nil)
				return server, nil, cleanup
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				// Generate an upsert an allocation.
				alloc := mock.Alloc()
				require.NoError(t, s.State().UpsertAllocs(
					structs.MsgTypeTestSetup, 10, []*structs.Allocation{alloc}))

				// Perform a lookup using the allocation information.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: alloc.ID,
					QueryOptions: structs.QueryOptions{
						Namespace: alloc.Namespace,
						Region:    s.Region(),
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err := msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{})
			},
			name: "ACLs disabled alloc found without regs",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, token *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, allocID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Perform a lookup using the allocation information.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: allocID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: token.SecretID,
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{service})
			},
			name: "ACLs enabled use management token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, allocID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Create and policy and grab the auth token.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-node-get-service-reg",
					mock.NamespacePolicy(service.Namespace, "", []string{acl.NamespaceCapabilityReadJob})).SecretID

				// Perform a lookup using the allocation information.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: allocID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: authToken,
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{service})
			},
			name: "ACLs enabled use read-job namespace capability token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, allocID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Create and policy and grab the auth token.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-node-get-service-reg",
					mock.NamespacePolicy(service.Namespace, "read", nil)).SecretID

				// Perform a lookup using the allocation information.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: allocID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: authToken,
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.NoError(t, err)
				require.ElementsMatch(t, serviceRegResp.Services, []*structs.ServiceRegistration{service})
			},
			name: "ACLs enabled use read namespace policy token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, allocID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Create and policy and grab the auth token.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-node-get-service-reg",
					mock.NamespacePolicy("ohno", "read", nil)).SecretID

				// Perform a lookup using the allocation information.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: allocID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: authToken,
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")
				require.Empty(t, serviceRegResp.Services)
			},
			name: "ACLs enabled use read incorrect namespace policy token",
		},
		{
			serverFn: func(t *testing.T) (*Server, *structs.ACLToken, func()) {
				return TestACLServer(t, nil)
			},
			testFn: func(t *testing.T, s *Server, _ *structs.ACLToken) {
				codec := rpcClient(t, s)
				testutil.WaitForLeader(t, s.RPC)

				err, allocID, service := correctSetupFn(s)
				require.NoError(t, err)

				// Create and policy and grab the auth token.
				authToken := mock.CreatePolicyAndToken(t, s.State(), 30, "test-node-get-service-reg",
					mock.NamespacePolicy(service.Namespace, "", []string{acl.NamespaceCapabilityReadScalingPolicy})).SecretID

				// Perform a lookup using the allocation information.
				serviceRegReq := &structs.AllocServiceRegistrationsRequest{
					AllocID: allocID,
					QueryOptions: structs.QueryOptions{
						Namespace: service.Namespace,
						Region:    s.Region(),
						AuthToken: authToken,
					},
				}
				var serviceRegResp structs.AllocServiceRegistrationsResponse
				err = msgpackrpc.CallWithCodec(codec, structs.AllocServiceRegistrationsRPCMethod, serviceRegReq, &serviceRegResp)
				require.Error(t, err)
				require.Contains(t, err.Error(), "Permission denied")
				require.Empty(t, serviceRegResp.Services)
			},
			name: "ACLs enabled use incorrect capability",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, aclToken, cleanup := tc.serverFn(t)
			defer cleanup()
			tc.testFn(t, server, aclToken)
		})
	}
}

func TestAlloc_SignIdentities_Bad(t *testing.T) {
	ci.Parallel(t)

	// Use non-ACL server because auth should always be enforced on this endpoint
	s1, cleanupS1 := TestServer(t, nil)
	t.Cleanup(cleanupS1)
	codec := rpcClient(t, s1)
	testutil.WaitForKeyring(t, s1.RPC, s1.Region())

	node := mock.Node()
	must.NoError(t, s1.fsm.State().UpsertNode(structs.MsgTypeTestSetup, 100, node))

	req := &structs.AllocIdentitiesRequest{
		QueryOptions: structs.QueryOptions{
			Region:     "global",
			Namespace:  structs.DefaultNamespace,
			AllowStale: true,
			AuthToken:  node.SecretID,
		},
	}
	var resp structs.AllocIdentitiesResponse

	// Not including identities results in an error to catch bad client
	// implementations
	must.EqError(t, msgpackrpc.CallWithCodec(codec, "Alloc.SignIdentities", &req, &resp), "no identities requested")

	// Making up an alloc returns a rejection.
	req.Identities = []*structs.WorkloadIdentityRequest{{
		AllocID: uuid.Generate(),
		WIHandle: structs.WIHandle{
			WorkloadIdentifier: "foo",
			IdentityName:       "bar",
		},
	}}
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.SignIdentities", &req, &resp))
	must.Len(t, 1, resp.Rejections)
	must.Eq(t, *req.Identities[0], resp.Rejections[0].WorkloadIdentityRequest)
	must.Eq(t, structs.WIRejectionReasonMissingAlloc, resp.Rejections[0].Reason)

	// Insert an alloc with an alternate identity
	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Tasks[0].Identities = []*structs.WorkloadIdentity{
		{
			Name:     "alt",
			Audience: []string{"test"},
		},
	}
	summary := mock.JobSummary(alloc.JobID)
	state := s1.fsm.State()
	must.NoError(t, state.UpsertJobSummary(100, summary))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 101, []*structs.Allocation{alloc}))

	// A valid alloc and invalid TaskName is an error
	req.Identities[0].AllocID = alloc.ID
	req.Identities[0].WorkloadIdentifier = "invalid"
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.SignIdentities", &req, &resp))
	must.Len(t, 1, resp.Rejections)
	must.Eq(t, *req.Identities[0], resp.Rejections[0].WorkloadIdentityRequest)
	must.Eq(t, structs.WIRejectionReasonMissingTask, resp.Rejections[0].Reason)

	// A valid alloc+task name still errors if the identity doesn't exist
	req.Identities[0].WorkloadIdentifier = "web"
	req.Identities[0].IdentityName = "invalid"
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.SignIdentities", &req, &resp))
	must.Len(t, 1, resp.Rejections)
	must.Eq(t, *req.Identities[0], resp.Rejections[0].WorkloadIdentityRequest)
	must.Eq(t, structs.WIRejectionReasonMissingIdentity, resp.Rejections[0].Reason)

	// I know the test is named "Bad" but let's make sure it does actually work
	req.Identities[0].IdentityName = "alt"
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.SignIdentities", &req, &resp))
	must.Len(t, 0, resp.Rejections)
	must.Len(t, 1, resp.SignedIdentities)

	// Looking for a missing alloc should return a rejection and a signed id
	req.Identities = append(req.Identities, &structs.WorkloadIdentityRequest{
		AllocID: uuid.Generate(),
		WIHandle: structs.WIHandle{
			WorkloadIdentifier: "foo",
			IdentityName:       "bar",
		},
	})
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Alloc.SignIdentities", &req, &resp))
	must.Len(t, 1, resp.Rejections)
	must.Eq(t, *req.Identities[1], resp.Rejections[0].WorkloadIdentityRequest)
	must.Eq(t, structs.WIRejectionReasonMissingAlloc, resp.Rejections[0].Reason)
	must.Len(t, 1, resp.SignedIdentities)
}

// TestAlloc_SignIdentities_Blocking asserts that if a server is behind the
// desired index the signing request will block until the index is reached.
func TestAlloc_SignIdentities_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	t.Cleanup(cleanupS1)
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	node := mock.Node()
	must.NoError(t, s1.fsm.State().UpsertNode(structs.MsgTypeTestSetup, 100, node))

	// Create the alloc we're going to query for, but don't insert it yet. This
	// simulates querying a slow follower or a restoring server.
	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Tasks[0].Identities = []*structs.WorkloadIdentity{
		{
			Name:     "alt",
			Audience: []string{"test"},
		},
	}
	summary := mock.JobSummary(alloc.JobID)

	// Write a different alloc so the index is known but won't match our request
	otherAlloc := mock.Alloc()
	otherSummary := mock.JobSummary(otherAlloc.JobID)
	must.NoError(t, state.UpsertJobSummary(999, otherSummary))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{otherAlloc}))

	type resultT struct {
		Err   error
		Reply structs.AllocIdentitiesResponse
	}
	resultCh := make(chan resultT, 1)

	go func() {
		req := &structs.AllocIdentitiesRequest{
			Identities: []*structs.WorkloadIdentityRequest{
				{
					AllocID: alloc.ID,
					WIHandle: structs.WIHandle{
						WorkloadIdentifier: "web",
						IdentityName:       "alt",
					},
				},
			},
			QueryOptions: structs.QueryOptions{
				Region:        "global",
				Namespace:     structs.DefaultNamespace,
				AllowStale:    true,
				MinQueryIndex: 1999,
				MaxQueryTime:  10 * time.Second,
				AuthToken:     node.SecretID,
			},
		}
		var resp structs.AllocIdentitiesResponse

		err := msgpackrpc.CallWithCodec(codec, "Alloc.SignIdentities", &req, &resp)
		resultCh <- resultT{
			Err:   err,
			Reply: resp,
		}
	}()

	select {
	case result := <-resultCh:
		t.Fatalf("1. result returned when RPC should have blocked.\n >> err=%s\n >> rejections=%v", result.Err, result.Reply.Rejections)
	case <-time.After(100 * time.Millisecond):
	}

	// Add another alloc to bump the index but not to the MinQueryIndex
	otherAlloc = mock.Alloc()
	otherSummary = mock.JobSummary(otherAlloc.JobID)
	must.NoError(t, state.UpsertJobSummary(1997, otherSummary))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1998, []*structs.Allocation{otherAlloc}))

	select {
	case result := <-resultCh:
		t.Fatalf("2. result returned when RPC should have blocked.\n >> err=%s\n >> rejections=%v", result.Err, result.Reply.Rejections)
	case <-time.After(100 * time.Millisecond):
	}

	// Finally add the alloc we're waiting for
	must.NoError(t, state.UpsertJobSummary(1999, summary))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 2000, []*structs.Allocation{alloc}))

	select {
	case result := <-resultCh:
		must.NoError(t, result.Err)
		must.Eq(t, 2000, result.Reply.Index)
		must.Len(t, 0, result.Reply.Rejections)
		must.Len(t, 1, result.Reply.SignedIdentities)
		sid := result.Reply.SignedIdentities[0]
		must.Eq(t, alloc.ID, sid.AllocID)
		must.Eq(t, "web", sid.WorkloadIdentifier)
		must.Eq(t, "alt", sid.IdentityName)
	case <-time.After(5 * time.Second):
		t.Fatalf("result not returned when expected")
	}
}
