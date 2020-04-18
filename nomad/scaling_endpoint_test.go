package nomad

import (
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestScalingEndpoint_GetPolicy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	p1 := mock.ScalingPolicy()
	p2 := mock.ScalingPolicy()
	s1.fsm.State().UpsertScalingPolicies(1000, []*structs.ScalingPolicy{p1, p2})

	// Lookup the policy
	get := &structs.ScalingPolicySpecificRequest{
		ID: p1.ID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp structs.SingleScalingPolicyResponse
	err := msgpackrpc.CallWithCodec(codec, "Scaling.GetPolicy", get, &resp)
	require.NoError(err)
	require.EqualValues(1000, resp.Index)
	require.Equal(*p1, *resp.Policy)

	// Lookup non-existing policy
	get.ID = uuid.Generate()
	resp = structs.SingleScalingPolicyResponse{}
	err = msgpackrpc.CallWithCodec(codec, "Scaling.GetPolicy", get, &resp)
	require.NoError(err)
	require.EqualValues(1000, resp.Index)
	require.Nil(resp.Policy)
}

func TestScalingEndpoint_GetPolicy_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	p1 := mock.ScalingPolicy()
	p2 := mock.ScalingPolicy()
	state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{p1, p2})

	get := &structs.ScalingPolicySpecificRequest{
		ID: p1.ID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	// lookup without token should fail
	var resp structs.SingleScalingPolicyResponse
	err := msgpackrpc.CallWithCodec(codec, "Scaling.GetPolicy", get, &resp)
	require.Error(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListScalingPolicies}))
	get.AuthToken = invalidToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Scaling.GetPolicy", get, &resp)
	require.Error(err)
	require.Contains(err.Error(), "Permission denied")
	type testCase struct {
		authToken string
		name      string
	}
	cases := []testCase{
		{
			name:      "mgmt token should succeed",
			authToken: root.SecretID,
		},
		{
			name: "read disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-read",
				mock.NamespacePolicy(structs.DefaultNamespace, "read", nil)).SecretID,
		},
		{
			name: "write disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-write",
				mock.NamespacePolicy(structs.DefaultNamespace, "write", nil)).SecretID,
		},
		{
			name: "autoscaler disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-autoscaler",
				mock.NamespacePolicy(structs.DefaultNamespace, "scale", nil)).SecretID,
		},
		{
			name: "list-jobs+read-job capability should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-read-job-scaling",
				mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs, acl.NamespaceCapabilityReadJob})).SecretID,
		},
	}

	for _, tc := range cases {
		get.AuthToken = tc.authToken
		err = msgpackrpc.CallWithCodec(codec, "Scaling.GetPolicy", get, &resp)
		require.NoError(err, tc.name)
		require.EqualValues(1000, resp.Index)
		require.NotNil(resp.Policy)
	}

}

func TestScalingEndpoint_ListPolicies(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Lookup the policies
	get := &structs.ScalingPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp structs.ACLPolicyListResponse
	err := msgpackrpc.CallWithCodec(codec, "Scaling.ListPolicies", get, &resp)
	require.NoError(err)
	require.Empty(resp.Policies)

	p1 := mock.ScalingPolicy()
	p2 := mock.ScalingPolicy()
	s1.fsm.State().UpsertScalingPolicies(1000, []*structs.ScalingPolicy{p1, p2})

	err = msgpackrpc.CallWithCodec(codec, "Scaling.ListPolicies", get, &resp)
	require.NoError(err)
	require.EqualValues(1000, resp.Index)
	require.Len(resp.Policies, 2)
}

func TestScalingEndpoint_ListPolicies_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	p1 := mock.ScalingPolicy()
	p2 := mock.ScalingPolicy()
	state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{p1, p2})

	get := &structs.ScalingPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	// lookup without token should fail
	var resp structs.ACLPolicyListResponse
	err := msgpackrpc.CallWithCodec(codec, "Scaling.ListPolicies", get, &resp)
	require.Error(err)
	require.Contains(err.Error(), "Permission denied")

	// Expect failure for request with an invalid token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListScalingPolicies}))
	get.AuthToken = invalidToken.SecretID
	require.Error(err)
	require.Contains(err.Error(), "Permission denied")

	type testCase struct {
		authToken string
		name      string
	}
	cases := []testCase{
		{
			name:      "mgmt token should succeed",
			authToken: root.SecretID,
		},
		{
			name: "read disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-read",
				mock.NamespacePolicy(structs.DefaultNamespace, "read", nil)).SecretID,
		},
		{
			name: "write disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-write",
				mock.NamespacePolicy(structs.DefaultNamespace, "write", nil)).SecretID,
		},
		{
			name: "autoscaler disposition should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-autoscaler",
				mock.NamespacePolicy(structs.DefaultNamespace, "scale", nil)).SecretID,
		},
		{
			name: "list-scaling-policies capability should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-list-scaling-policies",
				mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListScalingPolicies})).SecretID,
		},
		{
			name: "list-jobs+read-job capability should succeed",
			authToken: mock.CreatePolicyAndToken(t, state, 1005, "test-valid-read-job-scaling",
				mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityListJobs, acl.NamespaceCapabilityReadJob})).SecretID,
		},
	}

	for _, tc := range cases {
		get.AuthToken = tc.authToken
		err = msgpackrpc.CallWithCodec(codec, "Scaling.ListPolicies", get, &resp)
		require.NoError(err, tc.name)
		require.EqualValues(1000, resp.Index)
		require.Len(resp.Policies, 2)
	}
}

func TestScalingEndpoint_ListPolicies_Blocking(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policies
	p1 := mock.ScalingPolicy()
	p2 := mock.ScalingPolicy()

	// First create an unrelated policy
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertScalingPolicies(100, []*structs.ScalingPolicy{p1})
		require.NoError(err)
	})

	// Upsert the policy we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertScalingPolicies(200, []*structs.ScalingPolicy{p2})
		require.NoError(err)
	})

	// Lookup the policy
	req := &structs.ScalingPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
		},
	}
	var resp structs.ScalingPolicyListResponse
	start := time.Now()
	err := msgpackrpc.CallWithCodec(codec, "Scaling.ListPolicies", req, &resp)
	require.NoError(err)

	require.True(time.Since(start) > 200*time.Millisecond, "should block: %#v", resp)
	require.EqualValues(200, resp.Index, "bad index")
	require.Len(resp.Policies, 2)
	require.ElementsMatch([]string{p1.ID, p2.ID}, []string{resp.Policies[0].ID, resp.Policies[1].ID})
}
