// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

const jobIndex = 1000

func registerMockJob(s *Server, t *testing.T, prefix string, counter int) *structs.Job {
	job := mock.Job()
	job.ID = prefix + strconv.Itoa(counter)
	registerJob(s, t, job)
	return job
}

func registerJob(s *Server, t *testing.T, job *structs.Job) {
	fsmState := s.fsm.State()
	require.NoError(t, fsmState.UpsertJob(structs.MsgTypeTestSetup, jobIndex, nil, job))
}

func mockAlloc() *structs.Allocation {
	a := mock.Alloc()
	a.Name = fmt.Sprintf("%s.%s[%d]", a.Job.Name, "web", 0)
	return a
}

func TestSearch_PrefixSearch_Job(t *testing.T) {
	ci.Parallel(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerMockJob(s, t, prefix, 0)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	require.Len(t, resp.Matches[structs.Jobs], 1)
	require.Equal(t, job.ID, resp.Matches[structs.Jobs][0])
	require.Equal(t, uint64(jobIndex), resp.Index)
}

func TestSearch_PrefixSearch_ACL(t *testing.T) {
	ci.Parallel(t)

	jobID := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	s, root, cleanupS := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	store := s.fsm.State()

	ns := mock.Namespace()
	ns.Name = "not-allowed"
	must.NoError(t, store.UpsertNamespaces(10, []*structs.Namespace{ns}))

	job := registerMockJob(s, t, jobID, 0)

	variable := mock.VariableEncrypted()
	resp := store.VarSet(1001, &structs.VarApplyStateRequest{
		Op:  structs.VarOpSet,
		Var: variable,
	})
	must.NoError(t, resp.Error)

	plugin := mock.CSIPlugin()
	must.NoError(t, store.UpsertCSIPlugin(1002, plugin))

	node := mock.Node()
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1003, node))

	disallowedVariable := mock.VariableEncrypted()
	disallowedVariable.Namespace = "not-allowed"
	resp = store.VarSet(2001, &structs.VarApplyStateRequest{
		Op:  structs.VarOpSet,
		Var: disallowedVariable,
	})
	must.NoError(t, resp.Error)

	req := &structs.SearchRequest{
		Prefix:  "",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Try without a token and expect failure
	{
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect failure
	{
		invalidToken := mock.CreatePolicyAndToken(t, store, 1003, "test-invalid",
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
		req.AuthToken = invalidToken.SecretID
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect failure due to Jobs being the context
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1005, "test-invalid2", mock.NodePolicy(acl.PolicyRead))
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect success due to All context
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1007, "test-valid", mock.NodePolicy(acl.PolicyRead))
		req.Context = structs.All
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.Eq(t, []string{node.ID}, resp.Matches[structs.Nodes])

		// Jobs, Plugins, and Variables filtered out since token only has access
		// to node:read
		must.SliceEmpty(t, resp.Matches[structs.Jobs])
		must.SliceEmpty(t, resp.Matches[structs.Plugins])
		must.SliceEmpty(t, resp.Matches[structs.Variables])

		must.Eq(t, uint64(1003), resp.Index) // index of node
	}

	// Try with a valid token for namespace:read-job
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1009, "test-valid2",
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.Eq(t, []string{job.ID}, resp.Matches[structs.Jobs])

		// Nodes, Plugins, and Variables filtered out since token only has
		// access to namespace:read-job
		must.SliceEmpty(t, resp.Matches[structs.Nodes])
		must.SliceEmpty(t, resp.Matches[structs.Plugins])
		must.SliceEmpty(t, resp.Matches[structs.Variables])

		// Index of job because all other contexts are filtered out
		must.Eq(t, uint64(1000), resp.Index)
	}

	// Try with a valid token for node:read and namespace:read-job
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1011, "test-valid3", strings.Join([]string{
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			mock.NodePolicy(acl.PolicyRead),
		}, "\n"))
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.Eq(t, []string{job.ID}, resp.Matches[structs.Jobs])
		must.SliceEmpty(t, resp.Matches[structs.Plugins])
		must.SliceEmpty(t, resp.Matches[structs.Variables])
		must.Eq(t, []string{node.ID}, resp.Matches[structs.Nodes])
		must.Eq(t, uint64(1003), resp.Index) // index of node
	}

	// Try with a valid token for node:read and namespace:variable:read
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1012, "test-valid4", strings.Join([]string{
			mock.NamespacePolicyWithVariables(structs.DefaultNamespace, "", []string{},
				map[string][]string{"*": []string{"list"}}),
			mock.NodePolicy(acl.PolicyRead),
		}, "\n"))
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.SliceEmpty(t, resp.Matches[structs.Jobs])
		must.SliceEmpty(t, resp.Matches[structs.Plugins])
		must.Eq(t, []string{variable.Path}, resp.Matches[structs.Variables])
		must.Eq(t, []string{node.ID}, resp.Matches[structs.Nodes])
		must.Eq(t, uint64(2001), resp.Index) // index of variables
	}

	// Try with a valid token for node:read and namespace:variable:read, wildcard ns
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1012, "test-valid4", strings.Join([]string{
			mock.NamespacePolicyWithVariables(structs.DefaultNamespace, "", []string{},
				map[string][]string{"*": []string{"list"}}),
			mock.NodePolicy(acl.PolicyRead),
		}, "\n"))
		req.AuthToken = validToken.SecretID
		req.Namespace = structs.AllNamespacesSentinel
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.SliceEmpty(t, resp.Matches[structs.Jobs])
		must.SliceEmpty(t, resp.Matches[structs.Plugins])
		must.Eq(t, []string{variable.Path}, resp.Matches[structs.Variables])
		must.Eq(t, []string{node.ID}, resp.Matches[structs.Nodes])
		must.Eq(t, uint64(2001), resp.Index) // index of variables
	}

	// Try with a management token
	{
		req.AuthToken = root.SecretID
		req.Namespace = structs.DefaultNamespace
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.Eq(t, []string{job.ID}, resp.Matches[structs.Jobs])
		must.Eq(t, []string{plugin.ID}, resp.Matches[structs.Plugins])

		expectVars := []string{variable.Path, disallowedVariable.Path}
		slices.Sort(expectVars)
		slices.Sort(resp.Matches[structs.Variables])
		must.Eq(t, expectVars, resp.Matches[structs.Variables])
		must.Eq(t, []string{node.ID}, resp.Matches[structs.Nodes])
		must.Eq(t, uint64(2001), resp.Index) // highest index
	}
}

func TestSearch_PrefixSearch_All_JobWithHyphen(t *testing.T) {
	ci.Parallel(t)

	prefix := "example-test-------" // Assert that a job with more than 4 hyphens works

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Register a job and an allocation
	job := registerMockJob(s, t, prefix, 0)
	alloc := mockAlloc()
	alloc.JobID = job.ID
	alloc.Namespace = job.Namespace
	summary := mock.JobSummary(alloc.JobID)
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertJobSummary(999, summary))
	require.NoError(t, fsmState.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	req := &structs.SearchRequest{
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// req.Prefix = "example-te": 9
	for i := 1; i < len(prefix); i++ {
		req.Prefix = prefix[:i]
		var resp structs.SearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		require.Equal(t, 1, len(resp.Matches[structs.Jobs]))
		require.Equal(t, job.ID, resp.Matches[structs.Jobs][0])
		require.EqualValues(t, jobIndex, resp.Index)
	}
}

func TestSearch_PrefixSearch_All_LongJob(t *testing.T) {
	ci.Parallel(t)

	prefix := strings.Repeat("a", 100)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Register a job and an allocation
	job := registerMockJob(s, t, prefix, 0)
	alloc := mockAlloc()
	alloc.JobID = job.ID
	summary := mock.JobSummary(alloc.JobID)
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertJobSummary(999, summary))
	require.NoError(t, fsmState.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Jobs], 1)
	require.Equal(t, job.ID, resp.Matches[structs.Jobs][0])
	require.EqualValues(t, jobIndex, resp.Index)
}

// truncate should limit results to 20
func TestSearch_PrefixSearch_Truncate(t *testing.T) {
	ci.Parallel(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	for counter := 0; counter < 25; counter++ {
		registerMockJob(s, t, prefix, counter)
	}

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: "default",
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Jobs], 20)
	require.True(t, resp.Truncations[structs.Jobs])
	require.Equal(t, uint64(jobIndex), resp.Index)
}

func TestSearch_PrefixSearch_AllWithJob(t *testing.T) {
	ci.Parallel(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerMockJob(s, t, prefix, 0)
	eval1 := mock.Eval()
	eval1.ID = job.ID
	require.NoError(t, s.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 2000, []*structs.Evaluation{eval1}))

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Jobs], 1)
	require.Equal(t, job.ID, resp.Matches[structs.Jobs][0])
	require.Len(t, resp.Matches[structs.Evals], 1)
	require.Equal(t, eval1.ID, resp.Matches[structs.Evals][0])
}

func TestSearch_PrefixSearch_Evals(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	eval1 := mock.Eval()
	require.NoError(t, s.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 2000, []*structs.Evaluation{eval1}))

	prefix := eval1.ID[:len(eval1.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Evals,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Evals], 1)
	require.Equal(t, eval1.ID, resp.Matches[structs.Evals][0])
	require.False(t, resp.Truncations[structs.Evals])
	require.Equal(t, uint64(2000), resp.Index)
}

func TestSearch_PrefixSearch_Allocation(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	alloc := mockAlloc()
	summary := mock.JobSummary(alloc.JobID)
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertJobSummary(999, summary))
	require.NoError(t, fsmState.UpsertAllocs(structs.MsgTypeTestSetup, 90, []*structs.Allocation{alloc}))

	prefix := alloc.ID[:len(alloc.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Allocs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: alloc.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Allocs], 1)
	require.Equal(t, alloc.ID, resp.Matches[structs.Allocs][0])
	require.False(t, resp.Truncations[structs.Allocs])
	require.Equal(t, uint64(90), resp.Index)
}

func TestSearch_PrefixSearch_All_UUID(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	alloc := mockAlloc()
	summary := mock.JobSummary(alloc.JobID)
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertJobSummary(999, summary))
	require.NoError(t, fsmState.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	node := mock.Node()
	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 1001, node))

	eval1 := mock.Eval()
	eval1.ID = node.ID
	require.NoError(t, fsmState.UpsertEvals(structs.MsgTypeTestSetup, 1002, []*structs.Evaluation{eval1}))

	req := &structs.SearchRequest{
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	for i := 1; i < len(alloc.ID); i++ {
		req.Prefix = alloc.ID[:i]
		var resp structs.SearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		require.Len(t, resp.Matches[structs.Allocs], 1)
		require.Equal(t, alloc.ID, resp.Matches[structs.Allocs][0])
		require.False(t, resp.Truncations[structs.Allocs])
		require.EqualValues(t, 1002, resp.Index)
	}
}

func TestSearch_PrefixSearch_Node(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	fsmState := s.fsm.State()
	node := mock.Node()

	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 100, node))

	prefix := node.ID[:len(node.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Nodes,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	require.Len(t, resp.Matches[structs.Nodes], 1)
	require.Equal(t, node.ID, resp.Matches[structs.Nodes][0])
	require.False(t, resp.Truncations[structs.Nodes])
	require.Equal(t, uint64(100), resp.Index)
}

func TestSearch_PrefixSearch_NodePool(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with test node pools.
	fsmState := s.fsm.State()
	dev1 := &structs.NodePool{Name: "dev-1"}
	dev2 := &structs.NodePool{Name: "dev-2"}
	prod := &structs.NodePool{Name: "prod"}

	err := fsmState.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{dev1, dev2, prod})
	must.NoError(t, err)

	// Run test cases.
	testCases := []struct {
		name     string
		prefix   string
		context  structs.Context
		expected []string
	}{
		{
			name:     "prefix match",
			prefix:   "dev",
			context:  structs.NodePools,
			expected: []string{dev1.Name, dev2.Name},
		},
		{
			name:     "prefix match - all",
			prefix:   "dev",
			context:  structs.All,
			expected: []string{dev1.Name, dev2.Name},
		},
		{
			name:    "empty prefix",
			prefix:  "",
			context: structs.NodePools,
			expected: []string{
				structs.NodePoolAll, structs.NodePoolDefault,
				dev1.Name, dev2.Name, prod.Name,
			},
		},
		{
			name:     "other context",
			prefix:   "dev",
			context:  structs.Jobs,
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.SearchRequest{
				Prefix:  tc.prefix,
				Context: tc.context,
				QueryOptions: structs.QueryOptions{
					Region: "global",
				},
			}
			var resp structs.SearchResponse
			err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
			must.NoError(t, err)
			must.Len(t, len(tc.expected), resp.Matches[structs.NodePools])

			for k, v := range resp.Matches {
				switch k {
				case structs.NodePools:
					must.SliceContainsAll(t, v, tc.expected)
				default:
					must.Len(t, 0, v, must.Sprintf("found %d results in %v: %v", len(v), k, v))
				}
			}
		})
	}
}

func TestSearch_PrefixSearch_NodePool_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start test server with ACL.
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with test node pools and ACL policies.
	fsmState := s.fsm.State()

	dev1 := &structs.NodePool{Name: "dev-1"}
	dev2 := &structs.NodePool{Name: "dev-2"}
	prod := &structs.NodePool{Name: "prod"}
	err := fsmState.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{dev1, dev2, prod})
	must.NoError(t, err)

	devToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1001, "dev-node-pools",
		mock.NodePoolPolicy("dev-*", "read", nil),
	)
	noPolicyToken := mock.CreateToken(t, s.fsm.State(), 1003, nil)
	allPoolsToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1005, "all-node-pools",
		mock.NodePoolPolicy("*", "read", nil),
	)
	denyDevToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1007, "deny-dev-node-pools",
		mock.NodePoolPolicy("dev-*", "deny", nil),
	)

	// Run test cases.
	testCases := []struct {
		name        string
		token       string
		prefix      string
		expected    []string
		expectedErr string
	}{
		{
			name:   "management token has access to all",
			token:  root.SecretID,
			prefix: "",
			expected: []string{
				structs.NodePoolAll, structs.NodePoolDefault,
				dev1.Name, dev2.Name, prod.Name,
			},
		},
		{
			name:   "all pools access",
			token:  allPoolsToken.SecretID,
			prefix: "",
			expected: []string{
				structs.NodePoolAll, structs.NodePoolDefault,
				dev1.Name, dev2.Name, prod.Name,
			},
		},
		{
			name:     "only return what token has access",
			token:    devToken.SecretID,
			prefix:   "dev",
			expected: []string{dev1.Name, dev2.Name},
		},
		{
			name:     "no results if token doesn't have access",
			token:    devToken.SecretID,
			prefix:   "prod",
			expected: []string{},
		},
		{
			name:        "no results if token is denied",
			token:       denyDevToken.SecretID,
			prefix:      "dev",
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no policy",
			token:       noPolicyToken.SecretID,
			prefix:      "",
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no token",
			token:       "",
			prefix:      "",
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.SearchRequest{
				Prefix:  tc.prefix,
				Context: structs.NodePools,
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					AuthToken: tc.token,
				},
			}
			var resp structs.SearchResponse
			err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
				return
			}

			must.NoError(t, err)
			must.Len(t, len(tc.expected), resp.Matches[structs.NodePools])

			for k, v := range resp.Matches {
				switch k {
				case structs.NodePools:
					must.SliceContainsAll(t, v, tc.expected)
				default:
					must.Len(t, 0, v, must.Sprintf("found %d results in %v: %v", len(v), k, v))
				}
			}
		})
	}
}

func TestSearch_PrefixSearch_Deployment(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	deployment := mock.Deployment()
	require.NoError(t, s.fsm.State().UpsertDeployment(2000, deployment))

	prefix := deployment.ID[:len(deployment.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Deployments,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: deployment.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
	require.Len(t, resp.Matches[structs.Deployments], 1)
	require.Equal(t, deployment.ID, resp.Matches[structs.Deployments][0])
	require.False(t, resp.Truncations[structs.Deployments])
	require.Equal(t, uint64(2000), resp.Index)
}

func TestSearch_PrefixSearch_AllContext(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	fsmState := s.fsm.State()
	node := mock.Node()

	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 100, node))

	eval1 := mock.Eval()
	eval1.ID = node.ID
	require.NoError(t, fsmState.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1}))

	prefix := node.ID[:len(node.ID)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.All,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Nodes], 1)
	require.Len(t, resp.Matches[structs.Evals], 1)
	require.Equal(t, node.ID, resp.Matches[structs.Nodes][0])
	require.Equal(t, eval1.ID, resp.Matches[structs.Evals][0])
	require.Equal(t, uint64(1000), resp.Index)
}

// Tests that the top 20 matches are returned when no prefix is set
func TestSearch_PrefixSearch_NoPrefix(t *testing.T) {
	ci.Parallel(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerMockJob(s, t, prefix, 0)

	req := &structs.SearchRequest{
		Prefix:  "",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
	require.Len(t, resp.Matches[structs.Jobs], 1)
	require.Equal(t, job.ID, resp.Matches[structs.Jobs][0])
	require.Equal(t, uint64(jobIndex), resp.Index)
}

// Tests that the zero matches are returned when a prefix has no matching
// results
func TestSearch_PrefixSearch_NoMatches(t *testing.T) {
	ci.Parallel(t)

	prefix := "aaaaaaaa-e8f7-fd38-c855-ab94ceb8970"

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
	require.Empty(t, resp.Matches[structs.Jobs])
	require.Equal(t, uint64(0), resp.Index)
}

// Prefixes can only be looked up if their length is a power of two. For
// prefixes which are an odd length, use the length-1 characters.
func TestSearch_PrefixSearch_RoundDownToEven(t *testing.T) {
	ci.Parallel(t)

	id1 := "aaafaaaa-e8f7-fd38-c855-ab94ceb89"
	id2 := "aaafeaaa-e8f7-fd38-c855-ab94ceb89"
	prefix := "aaafa"

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job := registerMockJob(s, t, id1, 0)
	registerMockJob(s, t, id2, 50)

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
	require.Len(t, resp.Matches[structs.Jobs], 1)
	require.Equal(t, job.ID, resp.Matches[structs.Jobs][0])
}

func TestSearch_PrefixSearch_MultiRegion(t *testing.T) {
	ci.Parallel(t)

	jobName := "exampleexample"

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.Region = "foo"
	})
	defer cleanupS1()

	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.Region = "bar"
	})
	defer cleanupS2()

	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)

	job := registerMockJob(s1, t, jobName, 0)

	req := &structs.SearchRequest{
		Prefix:  "",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "foo",
			Namespace: job.Namespace,
		},
	}

	codec := rpcClient(t, s2)

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Jobs], 1)
	require.Equal(t, job.ID, resp.Matches[structs.Jobs][0])
	require.Equal(t, uint64(jobIndex), resp.Index)
}

func TestSearch_PrefixSearch_CSIPlugin(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	id := uuid.Generate()
	state.CreateTestCSIPlugin(s.fsm.State(), id)

	prefix := id[:len(id)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Plugins,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Plugins], 1)
	require.Equal(t, id, resp.Matches[structs.Plugins][0])
	require.False(t, resp.Truncations[structs.Plugins])
}

func TestSearch_PrefixSearch_CSIVolume(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	id := uuid.Generate()
	err := s.fsm.State().UpsertCSIVolume(1000, []*structs.CSIVolume{{
		ID:        id,
		Namespace: structs.DefaultNamespace,
		PluginID:  "glade",
	}})
	require.NoError(t, err)

	prefix := id[:len(id)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Volumes,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Volumes], 1)
	require.Equal(t, id, resp.Matches[structs.Volumes][0])
	require.False(t, resp.Truncations[structs.Volumes])
}

func TestSearch_PrefixSearch_Namespace(t *testing.T) {
	ci.Parallel(t)

	s, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	ns := mock.Namespace()
	require.NoError(t, s.fsm.State().UpsertNamespaces(2000, []*structs.Namespace{ns}))

	prefix := ns.Name[:len(ns.Name)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Namespaces,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

	require.Len(t, resp.Matches[structs.Namespaces], 1)
	require.Equal(t, ns.Name, resp.Matches[structs.Namespaces][0])
	require.False(t, resp.Truncations[structs.Namespaces])
	require.Equal(t, uint64(2000), resp.Index)
}

func TestSearch_PrefixSearch_Namespace_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanup := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanup()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	store := s.fsm.State()

	ns := mock.Namespace()
	must.NoError(t, store.UpsertNamespaces(500, []*structs.Namespace{ns}))

	job1 := mock.Job()
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 502, nil, job1))

	job2 := mock.Job()
	job2.Namespace = ns.Name
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 504, nil, job2))

	node := mock.Node()
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1001, node))

	req := &structs.SearchRequest{
		Prefix:  "",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job1.Namespace,
		},
	}

	// Try without a token and expect failure
	{
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect failure
	{
		invalidToken := mock.CreatePolicyAndToken(t, store, 1003, "test-invalid",
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
		req.AuthToken = invalidToken.SecretID
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect failure due to Namespaces being the context
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1005, "test-invalid2", mock.NodePolicy(acl.PolicyRead))
		req.Context = structs.Namespaces
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect success due to All context
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1007, "test-valid", mock.NodePolicy(acl.PolicyRead))
		req.Context = structs.All
		req.AuthToken = validToken.SecretID
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
		must.Eq(t, uint64(1001), resp.Index)
		must.Eq(t, []string{node.ID}, resp.Matches[structs.Nodes])

		// Jobs filtered out since token only has access to node:read
		must.SliceEmpty(t, resp.Matches[structs.Jobs])
	}

	// Try with a valid token for non-default namespace:read-job
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1009, "test-valid2",
			mock.NamespacePolicy(job2.Namespace, "", []string{acl.NamespaceCapabilityReadJob}))
		req.Context = structs.All
		req.AuthToken = validToken.SecretID
		req.Namespace = job2.Namespace
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.Eq(t, []string{job2.ID}, resp.Matches[structs.Jobs])
		must.Eq(t, []string{ns.Name}, resp.Matches[structs.Namespaces])

		// Index of job - not node - because node context is filtered out
		must.Eq(t, uint64(504), resp.Index)

		// Nodes filtered out since token only has access to namespace:read-job
		must.SliceEmpty(t, resp.Matches[structs.Nodes])
	}

	// Try with a valid token for node:read and default namespace:read-job
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1011, "test-valid3", strings.Join([]string{
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}),
			mock.NodePolicy(acl.PolicyRead),
		}, "\n"))
		req.Context = structs.All
		req.AuthToken = validToken.SecretID
		req.Namespace = structs.DefaultNamespace
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.Eq(t, []string{job1.ID}, resp.Matches[structs.Jobs])
		must.Eq(t, []string{node.ID}, resp.Matches[structs.Nodes])
		must.Eq(t, []string{"default"}, resp.Matches[structs.Namespaces])

		must.Eq(t, uint64(1001), resp.Index)

	}

	// Try with a management token
	{
		req.Context = structs.All
		req.AuthToken = root.SecretID
		req.Namespace = structs.DefaultNamespace
		var resp structs.SearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))

		must.Eq(t, []string{job1.ID}, resp.Matches[structs.Jobs])
		must.Eq(t, []string{node.ID}, resp.Matches[structs.Nodes])
		must.Eq(t, []string{"default", ns.Name}, resp.Matches[structs.Namespaces])

		must.Eq(t, uint64(1001), resp.Index)
	}
}

func TestSearch_PrefixSearch_ScalingPolicy(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job, policy := mock.JobWithScalingPolicy()
	prefix := policy.ID
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertJob(structs.MsgTypeTestSetup, jobIndex, nil, job))

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.ScalingPolicies,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.SearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
	require.Len(t, resp.Matches[structs.ScalingPolicies], 1)
	require.Equal(t, policy.ID, resp.Matches[structs.ScalingPolicies][0])
	require.Equal(t, uint64(jobIndex), resp.Index)

	req.Context = structs.All
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp))
	require.Len(t, resp.Matches[structs.ScalingPolicies], 1)
	require.Equal(t, policy.ID, resp.Matches[structs.ScalingPolicies][0])
	require.Equal(t, uint64(jobIndex), resp.Index)
}

func TestSearch_FuzzySearch_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanupS := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.SearchConfig.MinTermLength = 1
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	store := s.fsm.State()

	ns := mock.Namespace()
	ns.Name = "not-allowed"
	must.NoError(t, store.UpsertNamespaces(10, []*structs.Namespace{ns}))

	job := mock.Job()
	registerJob(s, t, job)

	variable := mock.VariableEncrypted()
	variable.Path = "test-path/o"
	resp := store.VarSet(1001, &structs.VarApplyStateRequest{
		Op:  structs.VarOpSet,
		Var: variable,
	})
	must.NoError(t, resp.Error)

	plugin := mock.CSIPlugin()
	plugin.ID = "mock.hashicorp.com"
	must.NoError(t, store.UpsertCSIPlugin(1002, plugin))

	node := mock.Node()
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1003, node))

	disallowedVariable := mock.VariableEncrypted()
	disallowedVariable.Namespace = "not-allowed"
	resp = store.VarSet(2001, &structs.VarApplyStateRequest{
		Op:  structs.VarOpSet,
		Var: disallowedVariable,
	})
	must.NoError(t, resp.Error)

	req := &structs.FuzzySearchRequest{
		Text:         "set-this-in-test",
		Context:      structs.Jobs,
		QueryOptions: structs.QueryOptions{Region: "global", Namespace: job.Namespace},
	}

	// Try without a token and expect failure
	{
		var resp structs.FuzzySearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect failure
	{
		invalidToken := mock.CreatePolicyAndToken(t, store, 1003, "test-invalid",
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
		req.AuthToken = invalidToken.SecretID
		var resp structs.FuzzySearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect failure due to Jobs being the context
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1005, "test-invalid2", mock.NodePolicy(acl.PolicyRead))
		req.AuthToken = validToken.SecretID
		var resp structs.FuzzySearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect success due to All context
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1007, "test-valid", mock.NodePolicy(acl.PolicyRead))
		req.Context = structs.All
		req.AuthToken = validToken.SecretID
		req.Text = "oo" // mock node ID is foobar
		var resp structs.FuzzySearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

		must.Eq(t, []structs.FuzzyMatch{{ID: node.Name, Scope: []string{node.ID}}},
			resp.Matches[structs.Nodes])

		// Jobs, Plugins, Variables filtered out since token only has access to
		// node:read
		must.SliceEmpty(t, resp.Matches[structs.Jobs])
		must.SliceEmpty(t, resp.Matches[structs.Plugins])
		must.SliceEmpty(t, resp.Matches[structs.Variables])

		must.Eq(t, uint64(1003), resp.Index) // index of node
	}

	// Try with a valid token for namespace:read-job
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1009, "test-valid2",
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
		req.AuthToken = validToken.SecretID
		req.Text = "jo" // mock job Name is my-job
		var resp structs.FuzzySearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Len(t, resp.Matches[structs.Jobs], 1)
		must.Eq(t, structs.FuzzyMatch{
			ID:    "my-job",
			Scope: []string{"default", job.ID},
		}, resp.Matches[structs.Jobs][0])

		// Index of job - not node - because node context is filtered out
		must.Eq(t, uint64(1000), resp.Index)

		// Nodes filtered out since token only has access to namespace:read-job
		must.SliceEmpty(t, resp.Matches[structs.Nodes])
	}

	// Try with a valid token for node:read and namespace:variable:read
	{
		validToken := mock.CreatePolicyAndToken(t, store, 1012, "test-valid4", strings.Join([]string{
			mock.NamespacePolicyWithVariables(structs.DefaultNamespace, "", []string{},
				map[string][]string{"*": []string{"list"}}),
			mock.NodePolicy(acl.PolicyRead),
		}, "\n"))
		req.Text = "o" // matches Job:my-job, Node:foobar, Plugin, and Variables
		req.AuthToken = validToken.SecretID
		var resp structs.FuzzySearchResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

		must.SliceEmpty(t, resp.Matches[structs.Jobs])
		must.SliceEmpty(t, resp.Matches[structs.Plugins])

		must.Eq(t, []structs.FuzzyMatch{
			{ID: node.Name, Scope: []string{node.ID}}},
			resp.Matches[structs.Nodes])

		must.Eq(t, []structs.FuzzyMatch{{
			ID:    variable.Path,
			Scope: []string{structs.DefaultNamespace, variable.Path}}},
			resp.Matches[structs.Variables])

		must.Eq(t, uint64(2001), resp.Index) // index of variables
	}

	// Try with a management token
	{
		req.AuthToken = root.SecretID
		var resp structs.FuzzySearchResponse
		req.Text = "o" // matches Job:my-job, Node:foobar, Plugin, and Variables
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

		must.Eq(t, []structs.FuzzyMatch{
			{ID: job.Name, Scope: []string{"default", job.ID}}},
			resp.Matches[structs.Jobs])

		must.Eq(t, []structs.FuzzyMatch{
			{ID: node.Name, Scope: []string{node.ID}}},
			resp.Matches[structs.Nodes])

		must.Eq(t, []structs.FuzzyMatch{{ID: plugin.ID}},
			resp.Matches[structs.Plugins])

		must.Eq(t, []structs.FuzzyMatch{{
			ID:    variable.Path,
			Scope: []string{structs.DefaultNamespace, variable.Path}}},
			resp.Matches[structs.Variables])

		must.Eq(t, uint64(2001), resp.Index) // index of variables
	}
}

func TestSearch_FuzzySearch_NotEnabled(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.SearchConfig.FuzzyEnabled = false
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	fsmState := s.fsm.State()

	job := mock.Job()
	registerJob(s, t, job)

	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 1001, mock.Node()))

	req := &structs.FuzzySearchRequest{
		Text:         "foo", // min set to 5
		Context:      structs.Jobs,
		QueryOptions: structs.QueryOptions{Region: "global", Namespace: job.Namespace},
	}

	var resp structs.FuzzySearchResponse
	require.EqualError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp),
		"fuzzy search is not enabled")
}

func TestSearch_FuzzySearch_ShortText(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.SearchConfig.MinTermLength = 5
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	fsmState := s.fsm.State()

	job := mock.Job()
	registerJob(s, t, job)

	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 1001, mock.Node()))

	req := &structs.FuzzySearchRequest{
		Text:         "foo", // min set to 5
		Context:      structs.Jobs,
		QueryOptions: structs.QueryOptions{Region: "global", Namespace: job.Namespace},
	}

	var resp structs.FuzzySearchResponse
	require.EqualError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp),
		"fuzzy search query must be at least 5 characters, got 3")
}

func TestSearch_FuzzySearch_TruncateLimitQuery(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 1001, mock.Node()))

	req := &structs.FuzzySearchRequest{
		Text:         "job",
		Context:      structs.Jobs,
		QueryOptions: structs.QueryOptions{Region: "global", Namespace: "default"},
	}

	for i := 0; i < 25; i++ {
		job := mock.Job()
		job.Name = fmt.Sprintf("my-job-%d", i)
		registerJob(s, t, job)
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

	require.Len(t, resp.Matches[structs.Jobs], 20)
	require.True(t, resp.Truncations[structs.Jobs])
	require.Equal(t, uint64(jobIndex), resp.Index)
}

func TestSearch_FuzzySearch_TruncateLimitResults(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
		c.SearchConfig.LimitQuery = 10000
		c.SearchConfig.LimitResults = 5
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 1001, mock.Node()))

	req := &structs.FuzzySearchRequest{
		Text:         "job",
		Context:      structs.Jobs,
		QueryOptions: structs.QueryOptions{Region: "global", Namespace: "default"},
	}

	for i := 0; i < 25; i++ {
		job := mock.Job()
		job.Name = fmt.Sprintf("my-job-%d", i)
		registerJob(s, t, job)
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

	require.Len(t, resp.Matches[structs.Jobs], 5)
	require.True(t, resp.Truncations[structs.Jobs])
	require.Equal(t, uint64(jobIndex), resp.Index)
}

func TestSearch_FuzzySearch_Evals(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	eval1 := mock.Eval()
	eval1.ID = "f7dee5a1-d2b0-2f6a-2e75-6c8e467a4b99"
	require.NoError(t, s.fsm.State().UpsertEvals(structs.MsgTypeTestSetup, 2000, []*structs.Evaluation{eval1}))

	req := &structs.FuzzySearchRequest{
		Text:    "f7dee", // evals are prefix searched
		Context: structs.Evals,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: eval1.Namespace,
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

	require.Len(t, resp.Matches[structs.Evals], 1)
	require.Equal(t, eval1.ID, resp.Matches[structs.Evals][0].ID)
	require.False(t, resp.Truncations[structs.Evals])
	require.Equal(t, uint64(2000), resp.Index)
}

func TestSearch_FuzzySearch_Allocation(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	alloc := mockAlloc()
	summary := mock.JobSummary(alloc.JobID)
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertJobSummary(999, summary))
	require.NoError(t, fsmState.UpsertAllocs(structs.MsgTypeTestSetup, 90, []*structs.Allocation{alloc}))

	req := &structs.FuzzySearchRequest{
		Text:    "web",
		Context: structs.Allocs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: alloc.Namespace,
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

	require.Len(t, resp.Matches[structs.Allocs], 1)
	require.Equal(t, alloc.Name, resp.Matches[structs.Allocs][0].ID)
	require.False(t, resp.Truncations[structs.Allocs])
	require.Equal(t, uint64(90), resp.Index)
}

func TestSearch_FuzzySearch_Node(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	fsmState := s.fsm.State()
	node := mock.Node()

	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 100, node))

	req := &structs.FuzzySearchRequest{
		Text:    "oo",
		Context: structs.Nodes,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
	require.Len(t, resp.Matches[structs.Nodes], 1)
	require.Equal(t, node.Name, resp.Matches[structs.Nodes][0].ID)
	require.False(t, resp.Truncations[structs.Nodes])
	require.Equal(t, uint64(100), resp.Index)
}

func TestSearch_FuzzySearch_NodePool(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with test node pools.
	fsmState := s.fsm.State()
	devEng := &structs.NodePool{Name: "dev-eng"}
	devInfra := &structs.NodePool{Name: "dev-infra"}
	prodEng := &structs.NodePool{Name: "prod-eng"}

	err := fsmState.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{devEng, devInfra, prodEng})
	must.NoError(t, err)

	// Run test cases.
	testCases := []struct {
		name        string
		text        string
		context     structs.Context
		expected    []string
		expectedErr string
	}{
		{
			name:     "fuzzy match",
			text:     "eng",
			context:  structs.NodePools,
			expected: []string{devEng.Name, prodEng.Name},
		},
		{
			name:     "fuzzy match - all",
			text:     "eng",
			context:  structs.All,
			expected: []string{devEng.Name, prodEng.Name},
		},
		{
			name:        "empty prefix",
			text:        "",
			context:     structs.NodePools,
			expectedErr: "search query must be at least 2 characters",
		},
		{
			name:     "other context",
			text:     "eng",
			context:  structs.Jobs,
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.FuzzySearchRequest{
				Text:    tc.text,
				Context: tc.context,
				QueryOptions: structs.QueryOptions{
					Region: "global",
				},
			}
			var resp structs.FuzzySearchResponse
			err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
				return
			}
			must.NoError(t, err)
			must.Len(t, len(tc.expected), resp.Matches[structs.NodePools])

			for k, v := range resp.Matches {
				switch k {
				case structs.NodePools:
					got := make([]string, len(v))
					for i, m := range v {
						got[i] = m.ID
					}
					must.SliceContainsAll(t, got, tc.expected)
				default:
					must.Len(t, 0, v, must.Sprintf("found %d results in %v: %v", len(v), k, v))
				}
			}
		})
	}
}

func TestSearch_FuzzySearch_NodePool_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start test server with ACL.
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Populate state with test node pools and ACL policies.
	fsmState := s.fsm.State()

	devEng := &structs.NodePool{Name: "dev-eng"}
	devInfra := &structs.NodePool{Name: "dev-infra"}
	prodEng := &structs.NodePool{Name: "prod-eng"}

	err := fsmState.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{devEng, devInfra, prodEng})
	must.NoError(t, err)

	engToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1001, "eng-node-pools",
		mock.NodePoolPolicy("*eng", "read", nil),
	)
	noPolicyToken := mock.CreateToken(t, s.fsm.State(), 1003, nil)
	allPoolsToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1005, "all-node-pools",
		mock.NodePoolPolicy("*", "read", nil),
	)
	denyEngToken := mock.CreatePolicyAndToken(t, s.fsm.State(), 1007, "deny-eng-node-pools",
		mock.NodePoolPolicy("*eng", "deny", nil),
	)

	// Run test cases.
	testCases := []struct {
		name        string
		token       string
		text        string
		expected    []string
		expectedErr string
	}{
		{
			name:     "management token has access to all",
			token:    root.SecretID,
			text:     "dev",
			expected: []string{devEng.Name, devInfra.Name},
		},
		{
			name:     "all pools access",
			token:    allPoolsToken.SecretID,
			text:     "dev",
			expected: []string{devEng.Name, devInfra.Name},
		},
		{
			name:     "only return what token has access",
			token:    engToken.SecretID,
			text:     "eng",
			expected: []string{devEng.Name, prodEng.Name},
		},
		{
			name:     "no results if token doesn't have access",
			token:    engToken.SecretID,
			text:     "infra",
			expected: []string{},
		},
		{
			name:        "no results if token is denied",
			token:       denyEngToken.SecretID,
			text:        "eng",
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no policy",
			token:       noPolicyToken.SecretID,
			text:        "dev",
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
		{
			name:        "no token",
			token:       "",
			text:        "dev",
			expectedErr: structs.ErrPermissionDenied.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.FuzzySearchRequest{
				Text:    tc.text,
				Context: structs.NodePools,
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					AuthToken: tc.token,
				},
			}
			var resp structs.FuzzySearchResponse
			err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
				return
			}

			must.NoError(t, err)
			must.Len(t, len(tc.expected), resp.Matches[structs.NodePools])

			for k, v := range resp.Matches {
				switch k {
				case structs.NodePools:
					got := make([]string, len(v))
					for i, m := range v {
						got[i] = m.ID
					}
					must.SliceContainsAll(t, got, tc.expected)
				default:
					must.Len(t, 0, v, must.Sprintf("found %d results in %v: %v", len(v), k, v))
				}
			}
		})
	}
}

func TestSearch_FuzzySearch_Deployment(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	deployment := mock.Deployment()
	require.NoError(t, s.fsm.State().UpsertDeployment(2000, deployment))

	req := &structs.FuzzySearchRequest{
		Text:    deployment.ID[0:3], // deployments are prefix searched
		Context: structs.Deployments,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: deployment.Namespace,
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
	require.Len(t, resp.Matches[structs.Deployments], 1)
	require.Equal(t, deployment.ID, resp.Matches[structs.Deployments][0].ID)
	require.False(t, resp.Truncations[structs.Deployments])
	require.Equal(t, uint64(2000), resp.Index)
}

func TestSearch_FuzzySearch_CSIPlugin(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	state.CreateTestCSIPlugin(s.fsm.State(), "my-plugin")

	req := &structs.FuzzySearchRequest{
		Text:    "lug",
		Context: structs.Plugins,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

	require.Len(t, resp.Matches[structs.Plugins], 1)
	require.Equal(t, "my-plugin", resp.Matches[structs.Plugins][0].ID)
	require.False(t, resp.Truncations[structs.Plugins])
}

func TestSearch_FuzzySearch_CSIVolume(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	id := uuid.Generate()
	err := s.fsm.State().UpsertCSIVolume(1000, []*structs.CSIVolume{{
		ID:        id,
		Namespace: structs.DefaultNamespace,
		PluginID:  "glade",
	}})
	require.NoError(t, err)

	req := &structs.FuzzySearchRequest{
		Text:    id[0:3], // volumes are prefix searched
		Context: structs.Volumes,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

	require.Len(t, resp.Matches[structs.Volumes], 1)
	require.Equal(t, id, resp.Matches[structs.Volumes][0].ID)
	require.False(t, resp.Truncations[structs.Volumes])
}

func TestSearch_FuzzySearch_Namespace(t *testing.T) {
	ci.Parallel(t)

	s, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	ns := mock.Namespace()
	require.NoError(t, s.fsm.State().UpsertNamespaces(2000, []*structs.Namespace{ns}))

	req := &structs.FuzzySearchRequest{
		Text:    "am", // mock is team-<uuid>
		Context: structs.Namespaces,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

	require.Len(t, resp.Matches[structs.Namespaces], 1)
	require.Equal(t, ns.Name, resp.Matches[structs.Namespaces][0].ID)
	require.False(t, resp.Truncations[structs.Namespaces])
	require.Equal(t, uint64(2000), resp.Index)
}

func TestSearch_FuzzySearch_Namespace_caseInsensitive(t *testing.T) {
	ci.Parallel(t)

	s, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	ns := mock.Namespace()
	ns.Name = "TheFooNamespace"
	require.NoError(t, s.fsm.State().UpsertNamespaces(2000, []*structs.Namespace{ns}))

	req := &structs.FuzzySearchRequest{
		Text:    "foon",
		Context: structs.Namespaces,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

	require.Len(t, resp.Matches[structs.Namespaces], 1)
	require.Equal(t, ns.Name, resp.Matches[structs.Namespaces][0].ID)
	require.False(t, resp.Truncations[structs.Namespaces])
	require.Equal(t, uint64(2000), resp.Index)
}

func TestSearch_FuzzySearch_ScalingPolicy(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	job, policy := mock.JobWithScalingPolicy()
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertJob(structs.MsgTypeTestSetup, jobIndex, nil, job))

	req := &structs.FuzzySearchRequest{
		Text:    policy.ID[0:3], // scaling policies are prefix searched
		Context: structs.ScalingPolicies,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	var resp structs.FuzzySearchResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
	require.Len(t, resp.Matches[structs.ScalingPolicies], 1)
	require.Equal(t, policy.ID, resp.Matches[structs.ScalingPolicies][0].ID)
	require.Equal(t, uint64(jobIndex), resp.Index)

	req.Context = structs.All
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
	require.Len(t, resp.Matches[structs.ScalingPolicies], 1)
	require.Equal(t, policy.ID, resp.Matches[structs.ScalingPolicies][0].ID)
	require.Equal(t, uint64(jobIndex), resp.Index)
}

func TestSearch_FuzzySearch_Namespace_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanup := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanup()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	fsmState := s.fsm.State()

	ns := mock.Namespace()
	ns.Name = "team-job-app"
	require.NoError(t, fsmState.UpsertNamespaces(500, []*structs.Namespace{ns}))

	job1 := mock.Job()
	require.NoError(t, fsmState.UpsertJob(structs.MsgTypeTestSetup, 502, nil, job1))

	job2 := mock.Job()
	job2.Namespace = ns.Name
	require.NoError(t, fsmState.UpsertJob(structs.MsgTypeTestSetup, 504, nil, job2))

	node := mock.Node()
	node.Name = "run-jobs"
	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 1001, node))

	req := &structs.FuzzySearchRequest{
		Text:    "set-text-in-test",
		Context: structs.Jobs,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: job1.Namespace,
		},
	}

	// Try without a token and expect failure
	{
		var resp structs.FuzzySearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect failure
	{
		invalidToken := mock.CreatePolicyAndToken(t, fsmState, 1003, "test-invalid",
			mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs}))
		req.AuthToken = invalidToken.SecretID
		var resp structs.FuzzySearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect failure due to Namespaces being the context
	{
		validToken := mock.CreatePolicyAndToken(t, fsmState, 1005, "test-invalid2", mock.NodePolicy(acl.PolicyRead))
		req.Context = structs.Namespaces
		req.AuthToken = validToken.SecretID
		var resp structs.FuzzySearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	}

	// Try with a node:read token and expect success due to All context
	{
		validToken := mock.CreatePolicyAndToken(t, fsmState, 1007, "test-valid", mock.NodePolicy(acl.PolicyRead))
		req.Text = "job"
		req.Context = structs.All
		req.AuthToken = validToken.SecretID
		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Equal(t, uint64(1001), resp.Index)
		require.Len(t, resp.Matches[structs.Nodes], 1)

		// Jobs filtered out since token only has access to node:read
		require.Len(t, resp.Matches[structs.Jobs], 0)
	}

	// Try with a valid token for non-default namespace:read-job
	{
		validToken := mock.CreatePolicyAndToken(t, fsmState, 1009, "test-valid2",
			mock.NamespacePolicy(job2.Namespace, "", []string{acl.NamespaceCapabilityReadJob}))
		req.Text = "job"
		req.Context = structs.All
		req.AuthToken = validToken.SecretID
		req.Namespace = job2.Namespace
		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Len(t, resp.Matches[structs.Jobs], 1)
		require.Equal(t, job2.Name, resp.Matches[structs.Jobs][0].ID)

		// Index of job - not node - because node context is filtered out
		require.Equal(t, uint64(504), resp.Index)

		// Nodes filtered out since token only has access to namespace:read-job
		require.Len(t, resp.Matches[structs.Nodes], 0)
	}

	// Try with a management token
	{
		req.Text = "job"
		req.Context = structs.All
		req.AuthToken = root.SecretID
		req.Namespace = job1.Namespace
		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Equal(t, uint64(1001), resp.Index)
		require.Len(t, resp.Matches[structs.Jobs], 1)
		require.Equal(t, job1.Name, resp.Matches[structs.Jobs][0].ID)
		require.Len(t, resp.Matches[structs.Nodes], 1)
		require.Len(t, resp.Matches[structs.Namespaces], 1) // matches "team-job-app"
	}
}

func TestSearch_FuzzySearch_MultiNamespace_ACL(t *testing.T) {
	ci.Parallel(t)

	s, root, cleanupS := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()

	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	fsmState := s.fsm.State()

	require.NoError(t, fsmState.UpsertNamespaces(500, []*structs.Namespace{{
		Name:        "teamA",
		Description: "first namespace",
		CreateIndex: 100,
		ModifyIndex: 200,
	}, {
		Name:        "teamB",
		Description: "second namespace",
		CreateIndex: 101,
		ModifyIndex: 201,
	}, {
		Name:        "teamC",
		Description: "third namespace",
		CreateIndex: 102,
		ModifyIndex: 202,
	}}))

	// Closure to simplify fsm indexing
	index := uint64(1000)
	inc := func() uint64 {
		index++
		return index
	}

	// Upsert 3 jobs each in separate namespace
	job1 := mock.Job()
	job1.Name = "teamA-job1"
	job1.ID = "job1"
	job1.Namespace = "teamA"
	require.NoError(t, fsmState.UpsertJob(structs.MsgTypeTestSetup, inc(), nil, job1))

	job2 := mock.Job()
	job2.Name = "teamB-job2"
	job2.ID = "job2"
	job2.Namespace = "teamB"
	require.NoError(t, fsmState.UpsertJob(structs.MsgTypeTestSetup, inc(), nil, job2))

	job3 := mock.Job()
	job3.Name = "teamC-job3"
	job3.ID = "job3"
	job3.Namespace = "teamC"
	require.NoError(t, fsmState.UpsertJob(structs.MsgTypeTestSetup, inc(), nil, job3))

	// Upsert a node
	node := mock.Node()
	node.Name = "node-for-teams"
	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, inc(), node))

	// Upsert a node that will not be matched
	node2 := mock.Node()
	node2.Name = "node-for-ops"
	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, inc(), node2))

	// Create parameterized requests
	request := func(text, namespace, token string, context structs.Context) *structs.FuzzySearchRequest {
		return &structs.FuzzySearchRequest{
			Text:    text,
			Context: context,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: namespace,
				AuthToken: token,
			},
		}
	}

	t.Run("without a token expect failure", func(t *testing.T) {
		var resp structs.FuzzySearchResponse
		req := request("anything", job1.Namespace, "", structs.Jobs)
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	})

	t.Run("with an invalid token expect failure", func(t *testing.T) {
		policy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs})
		invalidToken := mock.CreatePolicyAndToken(t, fsmState, inc(), "test-invalid", policy)
		req := request("anything", job1.Namespace, invalidToken.SecretID, structs.Jobs)

		var resp structs.FuzzySearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	})

	t.Run("with node:read token search namespaces expect failure", func(t *testing.T) {
		validToken := mock.CreatePolicyAndToken(t, fsmState, inc(), "test-invalid2", mock.NodePolicy(acl.PolicyRead))
		req := request("team", job1.Namespace, validToken.SecretID, structs.Namespaces)

		var resp structs.FuzzySearchResponse
		err := msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp)
		require.EqualError(t, err, structs.ErrPermissionDenied.Error())
	})

	t.Run("with node:read token search all expect success", func(t *testing.T) {
		validToken := mock.CreatePolicyAndToken(t, fsmState, inc(), "test-valid", mock.NodePolicy(acl.PolicyRead))
		req := request("team", job1.Namespace, validToken.SecretID, structs.All)

		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

		// One matching node
		require.Len(t, resp.Matches[structs.Nodes], 1)

		// Jobs filtered out since token only has access to node:read
		require.Len(t, resp.Matches[structs.Jobs], 0)
	})

	t.Run("with a teamB/job:read token search all expect 1 job", func(t *testing.T) {
		policy := mock.NamespacePolicy(job2.Namespace, "", []string{acl.NamespaceCapabilityReadJob})
		token := mock.CreatePolicyAndToken(t, fsmState, inc(), "test-valid2", policy)
		req := request("team", job2.Namespace, token.SecretID, structs.All)

		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Len(t, resp.Matches[structs.Jobs], 1)
		require.Equal(t, job2.Name, resp.Matches[structs.Jobs][0].ID)

		// Nodes filtered out since token only has access to namespace:read-job
		require.Len(t, resp.Matches[structs.Nodes], 0)
	})

	// Using a token that can read jobs in 2 namespaces, we should get job results from
	// both those namespaces (using wildcard namespace in the query) but not the
	// third (and from no other contexts).
	t.Run("with a multi-ns job:read token search all expect 2 jobs", func(t *testing.T) {
		policyB := mock.NamespacePolicy(job2.Namespace, "", []string{acl.NamespaceCapabilityReadJob})
		mock.CreatePolicy(t, fsmState, inc(), "policyB", policyB)

		policyC := mock.NamespacePolicy(job3.Namespace, "", []string{acl.NamespaceCapabilityReadJob})
		mock.CreatePolicy(t, fsmState, inc(), "policyC", policyC)

		token := mock.CreateToken(t, fsmState, inc(), []string{"policyB", "policyC"})
		req := request("team", structs.AllNamespacesSentinel, token.SecretID, structs.Jobs)

		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Len(t, resp.Matches[structs.Jobs], 2)
		require.Equal(t, job2.Name, resp.Matches[structs.Jobs][0].ID)
		require.Equal(t, job3.Name, resp.Matches[structs.Jobs][1].ID)
	})

	// Using a management token, we should get job results from all three namespaces
	// (using wildcard namespace in the query).
	t.Run("with a management token search all expect 3 jobs", func(t *testing.T) {
		req := request("team", structs.AllNamespacesSentinel, root.SecretID, structs.Jobs)

		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Len(t, resp.Matches[structs.Jobs], 3)
		require.Equal(t, job1.Name, resp.Matches[structs.Jobs][0].ID)
		require.Equal(t, job2.Name, resp.Matches[structs.Jobs][1].ID)
		require.Equal(t, job3.Name, resp.Matches[structs.Jobs][2].ID)
	})

	// Using a token that can read nodes, we should get our 1 matching node when
	// searching the nodes context.
	t.Run("with node:read token read nodes", func(t *testing.T) {
		policy := mock.NodePolicy("read")
		mock.CreatePolicy(t, fsmState, inc(), "node-read-policy", policy)

		token := mock.CreateToken(t, fsmState, inc(), []string{"node-read-policy"})
		req := request("team", structs.AllNamespacesSentinel, token.SecretID, structs.Nodes)

		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Len(t, resp.Matches[structs.Nodes], 1)
		require.Equal(t, "node-for-teams", resp.Matches[structs.Nodes][0].ID)
	})

	// Using a token that cannot read nodes, we should get no matching nodes when
	// searching the nodes context.
	t.Run("with a job:read token read nodes", func(t *testing.T) {
		policy := mock.AgentPolicy("read")
		mock.CreatePolicy(t, fsmState, inc(), "agent-read-policy", policy)

		token := mock.CreateToken(t, fsmState, inc(), []string{"agent-read-policy"})
		req := request("team", structs.AllNamespacesSentinel, token.SecretID, structs.Nodes)
		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Empty(t, resp.Matches[structs.Nodes])
	})

	// Using a token that can read only job:read one namespace, but with wildcard
	// namespace should return only the one alloc the token can access.
	t.Run("with job:read token read allocs", func(t *testing.T) {
		policyD := mock.NamespacePolicy(job2.Namespace, "", []string{acl.NamespaceCapabilityReadJob})
		mock.CreatePolicy(t, fsmState, inc(), "policyD", policyD)

		// Create an alloc for each of the 3 jobs
		alloc1 := mockAlloc()
		alloc1.JobID = job1.ID
		alloc1.Name = job1.Name + ".task[0]"
		alloc1.Namespace = job1.Namespace
		summary1 := mock.JobSummary(alloc1.JobID)
		require.NoError(t, fsmState.UpsertJobSummary(inc(), summary1))

		alloc2 := mockAlloc()
		alloc2.JobID = job2.ID
		alloc2.Name = job2.Name + ".task[0]"
		alloc2.Namespace = job2.Namespace
		summary2 := mock.JobSummary(alloc2.JobID)
		require.NoError(t, fsmState.UpsertJobSummary(inc(), summary2))

		alloc3 := mockAlloc()
		alloc3.JobID = job3.ID
		alloc3.Name = job3.Name + ".task[0]"
		alloc3.Namespace = job3.Namespace
		summary3 := mock.JobSummary(alloc3.JobID)
		require.NoError(t, fsmState.UpsertJobSummary(inc(), summary3))

		// Upsert the allocs
		require.NoError(t, fsmState.UpsertAllocs(structs.MsgTypeTestSetup, inc(), []*structs.Allocation{alloc1, alloc2, alloc3}))

		token := mock.CreateToken(t, fsmState, inc(), []string{"policyD"})
		req := request("team", structs.AllNamespacesSentinel, token.SecretID, structs.Allocs)
		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Len(t, resp.Matches[structs.Allocs], 1)
		require.Equal(t, "teamB-job2.task[0]", resp.Matches[structs.Allocs][0].ID)
	})

	// Using a management token should return allocs from all the jobs.
	t.Run("with job:read token read allocs", func(t *testing.T) {
		// Create an alloc for each of the 3 jobs
		alloc1 := mockAlloc()
		alloc1.ID = uuid.Generate()
		alloc1.JobID = job1.ID
		alloc1.Name = "test-alloc.one[0]"
		alloc1.Namespace = job1.Namespace
		summary1 := mock.JobSummary(alloc1.JobID)
		require.NoError(t, fsmState.UpsertJobSummary(inc(), summary1))

		alloc2 := mockAlloc()
		alloc2.ID = uuid.Generate()
		alloc2.JobID = job2.ID
		alloc2.Name = "test-alloc.two[0]"
		alloc2.Namespace = job2.Namespace
		summary2 := mock.JobSummary(alloc2.JobID)
		require.NoError(t, fsmState.UpsertJobSummary(inc(), summary2))

		alloc3 := mockAlloc()
		alloc3.ID = uuid.Generate()
		alloc3.JobID = job3.ID
		alloc3.Name = "test-alloc.three[0]"
		alloc3.Namespace = job3.Namespace
		summary3 := mock.JobSummary(alloc3.JobID)
		require.NoError(t, fsmState.UpsertJobSummary(inc(), summary3))

		// Upsert the allocs
		require.NoError(t, fsmState.UpsertAllocs(structs.MsgTypeTestSetup, inc(), []*structs.Allocation{alloc1, alloc2, alloc3}))

		req := request("alloc", structs.AllNamespacesSentinel, root.SecretID, structs.Allocs)
		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		require.Len(t, resp.Matches[structs.Allocs], 3)
		require.Equal(t, alloc1.Name, resp.Matches[structs.Allocs][0].ID)
		require.Equal(t, []string{"teamA", alloc1.ID}, resp.Matches[structs.Allocs][0].Scope)
		require.Equal(t, alloc2.Name, resp.Matches[structs.Allocs][1].ID)
		require.Equal(t, []string{"teamB", alloc2.ID}, resp.Matches[structs.Allocs][1].Scope)
		require.Equal(t, alloc3.Name, resp.Matches[structs.Allocs][2].ID)
		require.Equal(t, []string{"teamC", alloc3.ID}, resp.Matches[structs.Allocs][2].Scope)
	})

	// Allow plugin read and wildcard namespace
	t.Run("with plugin:read", func(t *testing.T) {
		policy := mock.PluginPolicy("read")
		mock.CreatePolicy(t, fsmState, inc(), "plugin-read-policy", policy)

		token := mock.CreateToken(t, fsmState, inc(), []string{"plugin-read-policy"})

		// Create CSI plugins
		state.CreateTestCSIPlugin(s.fsm.State(), "plugin-for-teams")
		state.CreateTestCSIPlugin(s.fsm.State(), "plugin-for-ops")

		req := request("teams", structs.AllNamespacesSentinel, token.SecretID, structs.Plugins)
		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))

		require.Len(t, resp.Matches[structs.Plugins], 1)
		require.Empty(t, resp.Matches[structs.Plugins][0].Scope) // no scope
	})
}

func TestSearch_FuzzySearch_Job(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	fsmState := s.fsm.State()

	job := mock.Job()
	job.Name = "demo-sleep"
	job.Namespace = "team-sleepy"
	job.TaskGroups = []*structs.TaskGroup{{
		Name: "qa-sleeper-group-one",
		Services: []*structs.Service{{
			Name: "qa-group-sleep-svc-one",
		}},
		Tasks: []*structs.Task{{
			Name: "qa-sleep-task-one",
			Services: []*structs.Service{{
				Name: "some-sleepy-task-svc-one",
			}},
			Driver: "docker",
			Config: map[string]interface{}{
				"image": "sleeper:latest",
			},
		}},
	}, {
		Name: "prod-sleeper-group-one",
		Tasks: []*structs.Task{{
			Name:   "prod-sleep-task-one",
			Driver: "exec",
			Config: map[string]interface{}{
				"command": "/bin/sleep",
			},
		}, {
			Name:   "prod-task-two",
			Driver: "raw_exec",
			Config: map[string]interface{}{
				"command": "/usr/sbin/sleep",
			},
			Services: []*structs.Service{{
				Name: "some-sleepy-task-svc-two",
			}},
		}},
	}, {
		Name: "sleep-in-java",
		Tasks: []*structs.Task{{
			Name:   "prod-java-sleep",
			Driver: "java",
			Config: map[string]interface{}{
				"class": "sleep.class",
			},
		}},
	}}

	ns := mock.Namespace()
	ns.Name = job.Namespace
	require.NoError(t, fsmState.UpsertNamespaces(2000, []*structs.Namespace{ns}))
	registerJob(s, t, job)
	require.NoError(t, fsmState.UpsertNode(structs.MsgTypeTestSetup, 1003, mock.Node()))

	t.Run("sleep", func(t *testing.T) {
		req := &structs.FuzzySearchRequest{
			Text:    "sleep",
			Context: structs.Jobs,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}
		var resp structs.FuzzySearchResponse
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "Search.FuzzySearch", req, &resp))
		m := resp.Matches
		require.Equal(t, uint64(1000), resp.Index) // job is explicit search context, has id=1000

		// just the one job
		require.Len(t, m[structs.Jobs], 1)

		// 3 services (1 group, 2 task)
		require.Len(t, m[structs.Services], 3)
		require.Equal(t, []structs.FuzzyMatch{{
			ID:    "some-sleepy-task-svc-one",
			Scope: []string{"team-sleepy", job.ID, "qa-sleeper-group-one", "qa-sleep-task-one"},
		}, {
			ID:    "some-sleepy-task-svc-two",
			Scope: []string{"team-sleepy", job.ID, "prod-sleeper-group-one", "prod-task-two"},
		}, {
			ID:    "qa-group-sleep-svc-one",
			Scope: []string{"team-sleepy", job.ID, "qa-sleeper-group-one"},
		}}, m[structs.Services])

		// 3 groups
		require.Len(t, m[structs.Groups], 3)
		require.Equal(t, []structs.FuzzyMatch{{
			ID:    "sleep-in-java",
			Scope: []string{"team-sleepy", job.ID},
		}, {
			ID:    "qa-sleeper-group-one",
			Scope: []string{"team-sleepy", job.ID},
		}, {
			ID:    "prod-sleeper-group-one",
			Scope: []string{"team-sleepy", job.ID},
		}}, m[structs.Groups])

		// 3 tasks (1 does not match)
		require.Len(t, m[structs.Tasks], 3)
		require.Equal(t, []structs.FuzzyMatch{{
			ID:    "qa-sleep-task-one",
			Scope: []string{"team-sleepy", job.ID, "qa-sleeper-group-one"},
		}, {
			ID:    "prod-sleep-task-one",
			Scope: []string{"team-sleepy", job.ID, "prod-sleeper-group-one"},
		}, {
			ID:    "prod-java-sleep",
			Scope: []string{"team-sleepy", job.ID, "sleep-in-java"},
		}}, m[structs.Tasks])

		// 2 tasks with command
		require.Len(t, m[structs.Commands], 2)
		require.Equal(t, []structs.FuzzyMatch{{
			ID:    "/bin/sleep",
			Scope: []string{"team-sleepy", job.ID, "prod-sleeper-group-one", "prod-sleep-task-one"},
		}, {
			ID:    "/usr/sbin/sleep",
			Scope: []string{"team-sleepy", job.ID, "prod-sleeper-group-one", "prod-task-two"},
		}}, m[structs.Commands])

		// 1 task with image
		require.Len(t, m[structs.Images], 1)
		require.Equal(t, []structs.FuzzyMatch{{
			ID:    "sleeper:latest",
			Scope: []string{"team-sleepy", job.ID, "qa-sleeper-group-one", "qa-sleep-task-one"},
		}}, m[structs.Images])

		// 1 task with class
		require.Len(t, m[structs.Classes], 1)
		require.Equal(t, []structs.FuzzyMatch{{
			ID:    "sleep.class",
			Scope: []string{"team-sleepy", job.ID, "sleep-in-java", "prod-java-sleep"},
		}}, m[structs.Classes])
	})
}

func TestSearch_FuzzySearch_fuzzyIndex(t *testing.T) {
	ci.Parallel(t)

	for _, tc := range []struct {
		name, text string
		exp        int
	}{
		{name: "foo-bar-baz", text: "bar", exp: 4},
		{name: "Foo-Bar-Baz", text: "bar", exp: 4},
		{name: "foo-bar-baz", text: "zap", exp: -1},
	} {
		result := fuzzyIndex(tc.name, tc.text)
		require.Equal(t, tc.exp, result, "name: %s, text: %s, exp: %d, got: %d", tc.name, tc.text, tc.exp, result)
	}
}
