// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-set"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestNodePoolEndpoint_List(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with some node pools.
	poolDev1 := &structs.NodePool{
		Name:        "dev-1",
		Description: "test node pool for dev-1",
		Meta: map[string]string{
			"env":   "dev",
			"index": "1",
		},
	}
	poolDev2 := &structs.NodePool{
		Name:        "dev-2",
		Description: "test node pool for dev-2",
		Meta: map[string]string{
			"env":   "dev",
			"index": "2",
		},
	}
	poolDevNoMeta := &structs.NodePool{
		Name:        "dev-no-meta",
		Description: "test node pool for dev without meta",
	}
	poolProd1 := &structs.NodePool{
		Name:        "prod-1",
		Description: "test node pool for prod-1",
		Meta: map[string]string{
			"env":   "prod",
			"index": "1",
		},
	}
	poolProd2 := &structs.NodePool{
		Name:        "prod-2",
		Description: "test node pool for prod-2",
		Meta: map[string]string{
			"env":   "prod",
			"index": "2",
		},
	}
	err := s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{
		poolDev1,
		poolDev2,
		poolDevNoMeta,
		poolProd1,
		poolProd2,
	})
	must.NoError(t, err)

	testCases := []struct {
		name              string
		req               *structs.NodePoolListRequest
		expectedErr       string
		expected          []string
		expectedNextToken string
	}{
		{
			name: "list all",
			req:  &structs.NodePoolListRequest{},
			expected: []string{
				"all", "default",
				"dev-1", "dev-2", "dev-no-meta",
				"prod-1", "prod-2",
			},
		},
		{
			name: "list all reverse",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					Reverse: true,
				},
			},
			expected: []string{
				"prod-2", "prod-1",
				"dev-no-meta", "dev-2", "dev-1",
				"default", "all",
			},
		},
		{
			name: "filter by prefix",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					Prefix: "prod-",
				},
			},
			expected: []string{"prod-1", "prod-2"},
		},
		{
			name: "filter by prefix reverse",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					Prefix:  "prod-",
					Reverse: true,
				},
			},
			expected: []string{"prod-2", "prod-1"},
		},
		{
			name: "filter by prefix no match",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					Prefix: "invalid-",
				},
			},
			expected: []string{},
		},
		{
			name: "filter by expression",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					Filter: `Meta.env == "dev"`,
				},
			},
			expected: []string{"dev-1", "dev-2"},
		},
		{
			name: "filter by expression reverse",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					Filter:  `Meta.env == "dev"`,
					Reverse: true,
				},
			},
			expected: []string{"dev-2", "dev-1"},
		},
		{
			name: "paginate per-page=2 page=1",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					PerPage: 2,
				},
			},
			expected:          []string{"all", "default"},
			expectedNextToken: "dev-1",
		},
		{
			name: "paginate per-page=2 page=2",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					PerPage:   2,
					NextToken: "dev-1",
				},
			},
			expected:          []string{"dev-1", "dev-2"},
			expectedNextToken: "dev-no-meta",
		},
		{
			name: "paginate per-page=2 page=last",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					PerPage:   2,
					NextToken: "prod-2",
				},
			},
			expected:          []string{"prod-2"},
			expectedNextToken: "",
		},
		{
			name: "paginate reverse per-page=2 page=2",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					PerPage:   2,
					NextToken: "dev-no-meta",
					Reverse:   true,
				},
			},
			expected:          []string{"dev-no-meta", "dev-2"},
			expectedNextToken: "dev-1",
		},
		{
			name: "paginate prefix per-page=1 page=2",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					PerPage:   1,
					NextToken: "dev-2",
					Prefix:    "dev",
				},
			},
			expected:          []string{"dev-2"},
			expectedNextToken: "dev-no-meta",
		},
		{
			name: "paginate filter per-page=1 page=2",
			req: &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					PerPage:   1,
					NextToken: "dev-2",
					Filter:    "Meta is not empty",
				},
			},
			expected:          []string{"dev-2"},
			expectedNextToken: "prod-1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Always send the request to the global region.
			tc.req.Region = "global"

			// Make node pool list request.
			var resp structs.NodePoolListResponse
			err := msgpackrpc.CallWithCodec(codec, "NodePool.List", tc.req, &resp)

			// Check response.
			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				got := make([]string, len(resp.NodePools))
				for i, pool := range resp.NodePools {
					got[i] = pool.Name
				}
				must.Eq(t, tc.expected, got)
				must.Eq(t, tc.expectedNextToken, resp.NextToken)
			}
		})
	}
}

func TestNodePoolEndpoint_List_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with some node pools.
	poolDev1 := &structs.NodePool{
		Name:        "dev-1",
		Description: "test node pool for dev-1",
		Meta: map[string]string{
			"env":   "dev",
			"index": "1",
		},
	}
	poolDev2 := &structs.NodePool{
		Name:        "dev-2",
		Description: "test node pool for dev-2",
		Meta: map[string]string{
			"env":   "dev",
			"index": "2",
		},
	}
	poolProd1 := &structs.NodePool{
		Name:        "prod-1",
		Description: "test node pool for prod-1",
		Meta: map[string]string{
			"env":   "prod",
			"index": "1",
		},
	}
	err := s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{
		poolDev1,
		poolDev2,
		poolProd1,
	})
	must.NoError(t, err)

	// Create test ACL tokens.
	devToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1001, "dev-node-pools",
		mock.NodePoolPolicy("dev-*", "read", nil),
	)
	prodToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1003, "prod-node-pools",
		mock.NodePoolPolicy("prod-*", "read", nil),
	)
	noPolicyToken := mock.CreateToken(t, s.fsm.State(), 1005, nil)
	allPoolsToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1007, "all-node-pools",
		mock.NodePoolPolicy("*", "read", nil),
	)

	testCases := []struct {
		name     string
		token    string
		expected []string
	}{
		{
			name:  "management token lists all",
			token: root.SecretID,
			expected: []string{
				"all", "default",
				"dev-1", "dev-2", "prod-1",
			},
		},
		{
			name:     "dev token lists dev",
			token:    devToken.SecretID,
			expected: []string{"dev-1", "dev-2"},
		},
		{
			name:     "prod token lists prod",
			token:    prodToken.SecretID,
			expected: []string{"prod-1"},
		},
		{
			name:  "all pools token lists all",
			token: allPoolsToken.SecretID,
			expected: []string{
				"all", "default",
				"dev-1", "dev-2", "prod-1",
			},
		},
		{
			name:     "no policy token",
			token:    noPolicyToken.SecretID,
			expected: []string{},
		},
		{
			name:     "no token",
			token:    "",
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make node pool list request.
			req := &structs.NodePoolListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					AuthToken: tc.token,
				},
			}
			var resp structs.NodePoolListResponse
			err := msgpackrpc.CallWithCodec(codec, "NodePool.List", req, &resp)
			must.NoError(t, err)

			// Check response.
			got := make([]string, len(resp.NodePools))
			for i, pool := range resp.NodePools {
				got[i] = pool.Name
			}
			must.Eq(t, tc.expected, got)
		})
	}
}

func TestNodePoolEndpoint_List_BlockingQuery(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with some node pools.
	// Insert triggers watchers.
	pool := mock.NodePool()
	time.AfterFunc(100*time.Millisecond, func() {
		s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{pool})
	})

	req := &structs.NodePoolListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 999,
		},
	}
	var resp structs.NodePoolListResponse
	err := msgpackrpc.CallWithCodec(codec, "NodePool.List", req, &resp)
	must.NoError(t, err)
	must.Eq(t, 1000, resp.Index)

	// Delete triggers watchers.
	time.AfterFunc(100*time.Millisecond, func() {
		s.fsm.State().DeleteNodePools(structs.MsgTypeTestSetup, 1001, []string{pool.Name})
	})

	req.MinQueryIndex = 1000
	err = msgpackrpc.CallWithCodec(codec, "NodePool.List", req, &resp)
	must.NoError(t, err)
	must.Eq(t, 1001, resp.Index)
}

func TestNodePoolEndpoint_GetNodePool(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with some node pools.
	pool := mock.NodePool()
	err := s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{pool})
	must.NoError(t, err)

	testCases := []struct {
		name     string
		pool     string
		expected *structs.NodePool
	}{
		{
			name:     "get pool",
			pool:     pool.Name,
			expected: pool,
		},
		{
			name:     "non-existing",
			pool:     "does-not-exist",
			expected: nil,
		},
		{
			name:     "empty",
			pool:     "",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make node pool fetch request.
			req := &structs.NodePoolSpecificRequest{
				QueryOptions: structs.QueryOptions{
					Region: "global",
				},
				Name: tc.pool,
			}
			var resp structs.SingleNodePoolResponse
			err := msgpackrpc.CallWithCodec(codec, "NodePool.GetNodePool", req, &resp)
			must.NoError(t, err)

			// Check response.
			must.Eq(t, tc.expected, resp.NodePool)
		})
	}
}

func TestNodePoolEndpoint_GetNodePool_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with some node pools.
	pool := mock.NodePool()
	err := s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{pool})
	must.NoError(t, err)

	// Create test ACL tokens.
	allowedToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1001, "allow",
		mock.NodePoolPolicy(pool.Name, "read", nil),
	)
	deniedToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1003, "deny",
		mock.NodePoolPolicy(pool.Name, "deny", nil),
	)
	noPolicyToken := mock.CreateToken(t, s.fsm.State(), 1005, nil)
	allPoolsToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1007, "all-node-pools",
		mock.NodePoolPolicy("*", "read", nil),
	)

	testCases := []struct {
		name        string
		token       string
		pool        string
		expectedErr string
		expected    string
	}{
		{
			name:     "management token",
			token:    root.SecretID,
			pool:     pool.Name,
			expected: pool.Name,
		},
		{
			name:     "allowed token",
			token:    allowedToken.SecretID,
			pool:     pool.Name,
			expected: pool.Name,
		},
		{
			name:     "all pools token",
			token:    allPoolsToken.SecretID,
			pool:     pool.Name,
			expected: pool.Name,
		},
		{
			name:        "denied token",
			token:       deniedToken.SecretID,
			pool:        pool.Name,
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no policy token",
			token:       noPolicyToken.SecretID,
			pool:        pool.Name,
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "invalid token",
			token:       "invalid",
			pool:        pool.Name,
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no token",
			token:       "",
			pool:        pool.Name,
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make node pool fetch request.
			req := &structs.NodePoolSpecificRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					AuthToken: tc.token,
				},
				Name: tc.pool,
			}
			var resp structs.SingleNodePoolResponse
			err := msgpackrpc.CallWithCodec(codec, "NodePool.GetNodePool", req, &resp)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
				must.Nil(t, resp.NodePool)
			} else {
				must.NoError(t, err)
				must.Eq(t, tc.expected, resp.NodePool.Name)
			}
		})
	}
}

func TestNodePoolEndpoint_GetNodePool_BlockingQuery(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Upsert triggers watchers.
	// Populate state with a node pools.
	pool1 := mock.NodePool()
	time.AfterFunc(100*time.Millisecond, func() {
		s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{pool1})
	})

	// Insert node pool that should not trigger watcher.
	pool2 := mock.NodePool()
	time.AfterFunc(200*time.Millisecond, func() {
		s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1001, []*structs.NodePool{pool2})
	})

	// Update first node pool to trigger watcher.
	pool1 = pool1.Copy()
	pool1.Meta["updated"] = "true"
	time.AfterFunc(300*time.Millisecond, func() {
		s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1002, []*structs.NodePool{pool1})
	})

	req := &structs.NodePoolSpecificRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1000,
		},
		Name: pool1.Name,
	}
	var resp structs.SingleNodePoolResponse
	err := msgpackrpc.CallWithCodec(codec, "NodePool.GetNodePool", req, &resp)
	must.NoError(t, err)
	must.Eq(t, 1002, resp.Index)

	// Delete triggers watchers.
	// Delete pool that is not being watched.
	time.AfterFunc(100*time.Millisecond, func() {
		s.fsm.State().DeleteNodePools(structs.MsgTypeTestSetup, 1003, []string{pool2.Name})
	})

	// Delete pool that is being watched.
	time.AfterFunc(200*time.Millisecond, func() {
		s.fsm.State().DeleteNodePools(structs.MsgTypeTestSetup, 1004, []string{pool1.Name})
	})

	req.MinQueryIndex = 1002
	err = msgpackrpc.CallWithCodec(codec, "NodePool.GetNodePool", req, &resp)
	must.NoError(t, err)
	must.Eq(t, 1004, resp.Index)
}

func TestNodePoolEndpoint_UpsertNodePools(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Insert a node pool that we can update.
	existing := mock.NodePool()
	err := s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{existing})
	must.NoError(t, err)

	testCases := []struct {
		name        string
		pools       []*structs.NodePool
		expectedErr string
	}{
		{
			name: "insert new pool",
			pools: []*structs.NodePool{
				mock.NodePool(),
			},
		},
		{
			name: "insert multiple pools",
			pools: []*structs.NodePool{
				mock.NodePool(),
				mock.NodePool(),
			},
		},
		{
			name: "update pool",
			pools: []*structs.NodePool{
				{
					Name:        existing.Name,
					Description: "updated pool",
					Meta: map[string]string{
						"updated": "true",
					},
				},
			},
		},
		{
			name: "invalid pool name",
			pools: []*structs.NodePool{
				{
					Name: "%invalid%",
				},
			},
			expectedErr: "invalid node pool",
		},
		{
			name: "missing pool name",
			pools: []*structs.NodePool{
				{
					Name:        "",
					Description: "no name",
				},
			},
			expectedErr: "invalid node pool",
		},
		{
			name:        "empty request",
			pools:       []*structs.NodePool{},
			expectedErr: "must specify at least one node pool",
		},
		{
			name: "fail to update built-in pool all",
			pools: []*structs.NodePool{
				{
					Name:        structs.NodePoolAll,
					Description: "trying to update built-in pool",
				},
			},
			expectedErr: "not allowed",
		},
		{
			name: "fail to update built-in pool default",
			pools: []*structs.NodePool{
				{
					Name:        structs.NodePoolDefault,
					Description: "trying to update built-in pool",
				},
			},
			expectedErr: "not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.NodePoolUpsertRequest{
				WriteRequest: structs.WriteRequest{
					Region: "global",
				},
				NodePools: tc.pools,
			}
			var resp structs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "NodePool.UpsertNodePools", req, &resp)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				for _, pool := range tc.pools {
					ws := memdb.NewWatchSet()
					got, err := s.fsm.State().NodePoolByName(ws, pool.Name)
					must.NoError(t, err)
					must.Eq(t, pool, got, must.Cmp(cmpopts.IgnoreFields(
						structs.NodePool{},
						"CreateIndex",
						"ModifyIndex",
					)))
				}
			}
		})
	}
}

func TestNodePoolEndpoint_UpsertNodePool_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create test ACL tokens.
	devToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1001, "dev-node-pools",
		mock.NodePoolPolicy("dev-*", "write", nil),
	)
	devSpecificToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1003, "dev-1-node-pools",
		mock.NodePoolPolicy("dev-1", "write", nil),
	)
	prodToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1005, "prod-node-pools",
		mock.NodePoolPolicy("prod-*", "", []string{"write"}),
	)
	noPolicyToken := mock.CreateToken(t, s.fsm.State(), 1007, nil)
	readOnlyToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1009, "node-pools-read-only",
		mock.NodePoolPolicy("*", "read", nil),
	)

	testCases := []struct {
		name        string
		token       string
		pools       []*structs.NodePool
		expectedErr string
	}{
		{
			name:  "management token has full access",
			token: root.SecretID,
			pools: []*structs.NodePool{
				{Name: "dev-1"},
				{Name: "prod-1"},
				{Name: "qa-1"},
			},
		},
		{
			name:  "allowed by policy",
			token: devToken.SecretID,
			pools: []*structs.NodePool{
				{Name: "dev-1"},
			},
		},
		{
			name:  "allowed by capability",
			token: prodToken.SecretID,
			pools: []*structs.NodePool{
				{Name: "prod-1"},
			},
		},
		{
			name:  "allowed by exact match",
			token: devSpecificToken.SecretID,
			pools: []*structs.NodePool{
				{Name: "dev-1"},
			},
		},
		{
			name:  "token restricted to wildcard",
			token: devToken.SecretID,
			pools: []*structs.NodePool{
				{Name: "dev-1"},  // ok
				{Name: "prod-1"}, // not ok
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:  "token restricted if not exact match",
			token: devSpecificToken.SecretID,
			pools: []*structs.NodePool{
				{Name: "dev-2"},
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:  "no token",
			token: "",
			pools: []*structs.NodePool{
				{Name: "dev-2"},
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:  "no policy",
			token: noPolicyToken.SecretID,
			pools: []*structs.NodePool{
				{Name: "dev-1"},
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:  "no write",
			token: readOnlyToken.SecretID,
			pools: []*structs.NodePool{
				{Name: "dev-1"},
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.NodePoolUpsertRequest{
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					AuthToken: tc.token,
				},
				NodePools: tc.pools,
			}
			var resp structs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "NodePool.UpsertNodePools", req, &resp)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				for _, pool := range tc.pools {
					ws := memdb.NewWatchSet()
					got, err := s.fsm.State().NodePoolByName(ws, pool.Name)
					must.NoError(t, err)
					must.Eq(t, pool, got, must.Cmp(cmpopts.IgnoreFields(
						structs.NodePool{},
						"CreateIndex",
						"ModifyIndex",
					)))
				}
			}
		})
	}
}

func TestNodePoolEndpoint_DeleteNodePools(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Insert a few node pools that we can delete.
	var pools []*structs.NodePool
	for i := 0; i < 10; i++ {
		pools = append(pools, mock.NodePool())
	}

	testCases := []struct {
		name        string
		pools       []string
		expectedErr string
	}{
		{
			name:  "delete existing pool",
			pools: []string{pools[0].Name},
		},
		{
			name: "delete multiple pools",
			pools: []string{
				pools[1].Name,
				pools[2].Name,
			},
		},
		{
			name:        "pool doesn't exist",
			pools:       []string{"doesnt-exist"},
			expectedErr: "not found",
		},
		{
			name:        "empty request",
			pools:       []string{},
			expectedErr: "must specify at least one node pool to delete",
		},
		{
			name:        "empty name",
			pools:       []string{""},
			expectedErr: "node pool name is empty",
		},
		{
			name:        "can't delete built-in pool all",
			pools:       []string{"all"},
			expectedErr: "not allowed",
		},
		{
			name:        "can't delete built-in pool default",
			pools:       []string{"default"},
			expectedErr: "not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, pools)
			must.NoError(t, err)

			req := &structs.NodePoolDeleteRequest{
				WriteRequest: structs.WriteRequest{
					Region: "global",
				},
				Names: tc.pools,
			}
			var resp structs.GenericResponse
			err = msgpackrpc.CallWithCodec(codec, "NodePool.DeleteNodePools", req, &resp)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				for _, pool := range tc.pools {
					ws := memdb.NewWatchSet()
					got, err := s.fsm.State().NodePoolByName(ws, pool)
					must.NoError(t, err)
					must.Nil(t, got)
				}
			}
		})
	}
}

func TestNodePoolEndpoint_DeleteNodePools_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create test ACL tokens.
	devToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1001, "dev-node-pools",
		mock.NodePoolPolicy("dev-*", "write", nil),
	)
	devSpecificToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1003, "dev-1-node-pools",
		mock.NodePoolPolicy("dev-1", "write", nil),
	)
	prodToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1005, "prod-node-pools",
		mock.NodePoolPolicy("prod-*", "", []string{"delete"}),
	)
	noPolicyToken := mock.CreateToken(t, s.fsm.State(), 1007, nil)
	noDeleteToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1009, "node-pools-no-delete",
		mock.NodePoolPolicy("*", "", []string{"read", "write"}),
	)

	// Insert a few node pools that we can delete.
	var pools []*structs.NodePool
	for i := 0; i < 5; i++ {
		devPool := mock.NodePool()
		devPool.Name = fmt.Sprintf("dev-%d", i)
		pools = append(pools, devPool)

		prodPool := mock.NodePool()
		prodPool.Name = fmt.Sprintf("prod-%d", i)
		pools = append(pools, prodPool)

		qaPool := mock.NodePool()
		qaPool.Name = fmt.Sprintf("qa-%d", i)
		pools = append(pools, qaPool)
	}

	testCases := []struct {
		name        string
		token       string
		pools       []string
		expectedErr string
	}{
		{
			name:  "management token has full access",
			token: root.SecretID,
			pools: []string{
				"dev-1",
				"prod-1",
				"qa-1",
			},
		},
		{
			name:  "allowed by write policy",
			token: devToken.SecretID,
			pools: []string{"dev-1"},
		},
		{
			name:  "allowed by delete capability",
			token: prodToken.SecretID,
			pools: []string{"prod-1"},
		},
		{
			name:  "allowed by exact match",
			token: devSpecificToken.SecretID,
			pools: []string{"dev-1"},
		},
		{
			name:  "restricted by wildcard",
			token: devToken.SecretID,
			pools: []string{
				"dev-1",  // ok
				"prod-1", // not ok
			},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "restricted if not exact match",
			token:       devSpecificToken.SecretID,
			pools:       []string{"dev-2"},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no token",
			token:       "",
			pools:       []string{"dev-1"},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no policy",
			token:       noPolicyToken.SecretID,
			pools:       []string{"dev-1"},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no delete",
			token:       noDeleteToken.SecretID,
			pools:       []string{"dev-1"},
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.fsm.State().UpsertNodePools(structs.MsgTypeTestSetup, 1000, pools)
			must.NoError(t, err)

			req := &structs.NodePoolDeleteRequest{
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					AuthToken: tc.token,
				},
				Names: tc.pools,
			}
			var resp structs.GenericResponse
			err = msgpackrpc.CallWithCodec(codec, "NodePool.DeleteNodePools", req, &resp)

			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				for _, pool := range tc.pools {
					ws := memdb.NewWatchSet()
					got, err := s.fsm.State().NodePoolByName(ws, pool)
					must.NoError(t, err)
					must.Nil(t, got)
				}
			}
		})
	}
}

func TestNodePoolEndpoint_ListJobs_ACLs(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	store := s1.fsm.State()
	index := uint64(1000)

	var err error

	// Populate state with some node pools.
	poolDev := &structs.NodePool{
		Name:        "dev-1",
		Description: "test node pool for dev-1",
	}
	poolProd := &structs.NodePool{
		Name:        "prod-1",
		Description: "test node pool for prod-1",
	}
	err = store.UpsertNodePools(structs.MsgTypeTestSetup, index, []*structs.NodePool{
		poolDev,
		poolProd,
	})
	must.NoError(t, err)

	// for refering to the jobs in assertions
	jobIDs := map[string]string{}

	// register jobs in all pools and all namespaces
	for _, ns := range []string{"engineering", "system", "default"} {
		index++
		must.NoError(t, store.UpsertNamespaces(index, []*structs.Namespace{{Name: ns}}))

		for _, pool := range []string{"dev-1", "prod-1", "default"} {
			job := mock.MinJob()
			job.Namespace = ns
			job.NodePool = pool
			jobIDs[ns+"+"+pool] = job.ID
			index++
			must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))
		}
	}

	req := &structs.NodePoolJobsRequest{
		Name: "dev-1",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.AllNamespacesSentinel},
	}

	// Expect failure for request without a token
	var resp structs.NodePoolJobsResponse
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
	must.EqError(t, err, structs.ErrPermissionDenied.Error())

	// Management token can read any namespace / any pool
	//	var mgmtResp structs.NodePoolJobsResponse
	req.AuthToken = root.SecretID
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
	must.NoError(t, err)
	must.Len(t, 3, resp.Jobs)
	must.SliceContainsAll(t,
		helper.ConvertSlice(resp.Jobs, func(j *structs.JobListStub) string { return j.ID }),
		[]string{jobIDs["engineering+dev-1"], jobIDs["system+dev-1"], jobIDs["default+dev-1"]})

	// Policy that allows access to any pool but one namespace
	index++
	devToken := mock.CreatePolicyAndToken(t, store, index, "dev-node-pools",
		fmt.Sprintf("%s\n%s\n%s\n",
			mock.NodePoolPolicy("dev-*", "read", nil),
			mock.NodePoolPolicy("default", "read", nil),
			mock.NamespacePolicy("engineering", "read", nil)),
	)
	req.AuthToken = devToken.SecretID

	// with wildcard namespace
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
	must.NoError(t, err)
	must.Len(t, 1, resp.Jobs)
	must.Eq(t, jobIDs["engineering+dev-1"], resp.Jobs[0].ID)

	// with specific allowed namespaces
	req.Namespace = "engineering"
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
	must.NoError(t, err)
	must.Len(t, 1, resp.Jobs)
	must.Eq(t, jobIDs["engineering+dev-1"], resp.Jobs[0].ID)

	// with disallowed namespace
	req.Namespace = "system"
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
	must.NoError(t, err)
	must.Len(t, 0, resp.Jobs)

	// with disallowed pool but allowed namespace
	req.Namespace = "engineering"
	req.Name = "prod-1"
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
	must.EqError(t, err, structs.ErrPermissionDenied.Error())
}

func TestNodePoolEndpoint_ListJobs_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	store := s1.fsm.State()
	index := uint64(1000)

	var err error

	// Populate state with a node pool and a job in the default pool
	poolDev := &structs.NodePool{
		Name:        "dev-1",
		Description: "test node pool for dev-1",
	}
	err = store.UpsertNodePools(structs.MsgTypeTestSetup, index, []*structs.NodePool{poolDev})
	must.NoError(t, err)

	job := mock.MinJob()
	index++
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))

	req := &structs.NodePoolJobsRequest{
		Name: "default",
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.AllNamespacesSentinel},
	}

	// List the job and get the index
	var resp structs.NodePoolJobsResponse
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
	must.Len(t, 1, resp.Jobs)
	must.Eq(t, index, resp.Index)
	must.Eq(t, "default", resp.Jobs[0].NodePool)

	// Moving a job into a pool we're watching should trigger a watch
	index++
	time.AfterFunc(100*time.Millisecond, func() {
		job = job.Copy()
		job.NodePool = "dev-1"
		must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))
	})

	req.Name = "dev-1"
	req.MinQueryIndex = index
	req.MaxQueryTime = 500 * time.Millisecond
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
	must.Len(t, 1, resp.Jobs)
	must.Eq(t, index, resp.Index)

	// Moving a job out of a pool we're watching should trigger a watch
	index++
	time.AfterFunc(100*time.Millisecond, func() {
		job = job.Copy()
		job.NodePool = "default"
		must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))
	})

	req.Name = "dev-1"
	req.MinQueryIndex = index
	req.MaxQueryTime = 500 * time.Millisecond
	err = msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)

	must.Len(t, 0, resp.Jobs)
	must.Eq(t, index, resp.Index)
}

func TestNodePoolEndpoint_ListJobs_PaginationFiltering(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	store := s1.fsm.State()
	index := uint64(1000)

	var err error

	// Populate state with some node pools.
	poolDev := &structs.NodePool{
		Name:        "dev-1",
		Description: "test node pool for dev-1",
	}
	poolProd := &structs.NodePool{
		Name:        "prod-1",
		Description: "test node pool for prod-1",
	}
	err = store.UpsertNodePools(structs.MsgTypeTestSetup, index, []*structs.NodePool{
		poolDev,
		poolProd,
	})
	must.NoError(t, err)

	index++
	must.NoError(t, store.UpsertNamespaces(index,
		[]*structs.Namespace{{Name: "non-default"}, {Name: "other"}}))

	// create a set of jobs. these are in the order that the state store will
	// return them from the iterator (sorted by key) for ease of writing tests
	mocks := []struct {
		name      string
		pool      string
		namespace string
		status    string
	}{
		{name: "job-00", pool: "dev-1", namespace: "default", status: structs.JobStatusPending},
		{name: "job-01", pool: "dev-1", namespace: "default", status: structs.JobStatusPending},
		{name: "job-02", pool: "default", namespace: "default", status: structs.JobStatusPending},
		{name: "job-03", pool: "dev-1", namespace: "non-default", status: structs.JobStatusPending},
		{name: "job-04", pool: "dev-1", namespace: "default", status: structs.JobStatusRunning},
		{name: "job-05", pool: "dev-1", namespace: "default", status: structs.JobStatusRunning},
		{name: "job-06", pool: "dev-1", namespace: "other", status: structs.JobStatusPending},
		// job-07 is missing for missing index assertion
		{name: "job-08", pool: "prod-1", namespace: "default", status: structs.JobStatusRunning},
		{name: "job-09", pool: "prod-1", namespace: "non-default", status: structs.JobStatusPending},
		{name: "job-10", pool: "dev-1", namespace: "default", status: structs.JobStatusPending},
	}
	for _, m := range mocks {
		job := mock.MinJob()
		job.ID = m.name
		job.Name = m.name
		job.NodePool = m.pool
		job.Status = m.status
		job.Namespace = m.namespace
		index++
		job.CreateIndex = index
		must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))
	}

	// Policy that allows access to 2 pools and any namespace
	index++
	devToken := mock.CreatePolicyAndToken(t, store, index, "dev-node-pools",
		fmt.Sprintf("%s\n%s\n%s\n",
			mock.NodePoolPolicy("dev-*", "read", nil),
			mock.NodePoolPolicy("default", "read", nil),
			mock.NamespacePolicy("*", "read", nil)),
	)

	cases := []struct {
		name              string
		pool              string
		namespace         string
		filter            string
		nextToken         string
		pageSize          int32
		expectedNextToken string
		expectedIDs       []string
		expectedError     string
	}{
		{
			name:        "test00 all dev pool default NS",
			pool:        "dev-1",
			expectedIDs: []string{"job-00", "job-01", "job-04", "job-05", "job-10"},
		},
		{
			name:              "test01 size-2 page-1 dev pool default NS",
			pool:              "dev-1",
			pageSize:          2,
			expectedNextToken: "default.job-04",
			expectedIDs:       []string{"job-00", "job-01"},
		},
		{
			name:              "test02 size-2 page-1 dev pool wildcard NS",
			pool:              "dev-1",
			namespace:         "*",
			pageSize:          2,
			expectedNextToken: "default.job-04",
			expectedIDs:       []string{"job-00", "job-01"},
		},
		{
			name:              "test03 size-2 page-2 dev pool default NS",
			pool:              "dev-1",
			pageSize:          2,
			nextToken:         "default.job-04",
			expectedNextToken: "default.job-10",
			expectedIDs:       []string{"job-04", "job-05"},
		},
		{
			name:              "test04 size-2 page-2 wildcard NS",
			pool:              "dev-1",
			namespace:         "*",
			pageSize:          2,
			nextToken:         "default.job-04",
			expectedNextToken: "default.job-10",
			expectedIDs:       []string{"job-04", "job-05"},
		},
		{
			name:        "test05 no valid results with filters",
			pool:        "dev-1",
			pageSize:    2,
			nextToken:   "",
			filter:      `Name matches "not-job"`,
			expectedIDs: []string{},
		},
		{
			name:        "test06 go-bexpr filter across namespaces",
			pool:        "dev-1",
			namespace:   "*",
			filter:      `Name matches "job-0[12345]"`,
			expectedIDs: []string{"job-01", "job-04", "job-05", "job-03"},
		},
		{
			name:              "test07 go-bexpr filter with pagination",
			pool:              "dev-1",
			namespace:         "*",
			filter:            `Name matches "job-0[12345]"`,
			pageSize:          3,
			expectedNextToken: "non-default.job-03",
			expectedIDs:       []string{"job-01", "job-04", "job-05"},
		},
		{
			name:        "test08 go-bexpr filter in namespace",
			pool:        "dev-1",
			namespace:   "non-default",
			filter:      `Status == "pending"`,
			expectedIDs: []string{"job-03"},
		},
		{
			name:          "test09 go-bexpr invalid expression",
			pool:          "dev-1",
			filter:        `NotValid`,
			expectedError: "failed to read filter expression",
		},
		{
			name:          "test10 go-bexpr invalid field",
			pool:          "dev-1",
			filter:        `InvalidField == "value"`,
			expectedError: "error finding value in datum",
		},
		{
			name:        "test11 missing index",
			pool:        "dev-1",
			pageSize:    1,
			nextToken:   "default.job-07",
			expectedIDs: []string{"job-10"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.NodePoolJobsRequest{
				Name: tc.pool,
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Namespace: tc.namespace,
					Filter:    tc.filter,
					PerPage:   tc.pageSize,
					NextToken: tc.nextToken,
				},
			}
			req.AuthToken = devToken.SecretID

			var resp structs.NodePoolJobsResponse
			err := msgpackrpc.CallWithCodec(codec, "NodePool.ListJobs", req, &resp)
			if tc.expectedError == "" {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
				must.ErrorContains(t, err, tc.expectedError)
				return
			}

			got := set.FromFunc(resp.Jobs,
				func(j *structs.JobListStub) string { return j.ID })
			must.True(t, got.ContainsSlice(tc.expectedIDs),
				must.Sprintf("unexpected page of jobs: %v", got))

			must.Eq(t, tc.expectedNextToken, resp.QueryMeta.NextToken,
				must.Sprint("unexpected NextToken"))
		})
	}

}
