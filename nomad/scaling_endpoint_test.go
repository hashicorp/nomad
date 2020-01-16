package nomad

import (
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

// func TestACLEndpoint_GetPolicy(t *testing.T) {
// 	t.Parallel()
//
// 	s1, root, cleanupS1 := TestACLServer(t, nil)
// 	defer cleanupS1()
// 	codec := rpcClient(t, s1)
// 	testutil.WaitForLeader(t, s1.RPC)
//
// 	// Create the register request
// 	policy := mock.ACLPolicy()
// 	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{policy})
//
// 	anonymousPolicy := mock.ACLPolicy()
// 	anonymousPolicy.Name = "anonymous"
// 	s1.fsm.State().UpsertACLPolicies(1001, []*structs.ACLPolicy{anonymousPolicy})
//
// 	// Create a token with one the policy
// 	token := mock.ACLToken()
// 	token.Policies = []string{policy.Name}
// 	s1.fsm.State().UpsertACLTokens(1002, []*structs.ACLToken{token})
//
// 	// Lookup the policy
// 	get := &structs.ACLPolicySpecificRequest{
// 		Name: policy.Name,
// 		QueryOptions: structs.QueryOptions{
// 			Region:    "global",
// 			AuthToken: root.SecretID,
// 		},
// 	}
// 	var resp structs.SingleACLPolicyResponse
// 	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp); err != nil {
// 		t.Fatalf("err: %v", err)
// 	}
// 	assert.Equal(t, uint64(1000), resp.Index)
// 	assert.Equal(t, policy, resp.Policy)
//
// 	// Lookup non-existing policy
// 	get.Name = uuid.Generate()
// 	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp); err != nil {
// 		t.Fatalf("err: %v", err)
// 	}
// 	assert.Equal(t, uint64(1001), resp.Index)
// 	assert.Nil(t, resp.Policy)
//
// 	// Lookup the policy with the token
// 	get = &structs.ACLPolicySpecificRequest{
// 		Name: policy.Name,
// 		QueryOptions: structs.QueryOptions{
// 			Region:    "global",
// 			AuthToken: token.SecretID,
// 		},
// 	}
// 	var resp2 structs.SingleACLPolicyResponse
// 	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp2); err != nil {
// 		t.Fatalf("err: %v", err)
// 	}
// 	assert.EqualValues(t, 1000, resp2.Index)
// 	assert.Equal(t, policy, resp2.Policy)
//
// 	// Lookup the anonymous policy with no token
// 	get = &structs.ACLPolicySpecificRequest{
// 		Name: anonymousPolicy.Name,
// 		QueryOptions: structs.QueryOptions{
// 			Region: "global",
// 		},
// 	}
// 	var resp3 structs.SingleACLPolicyResponse
// 	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp3); err != nil {
// 		require.NoError(t, err)
// 	}
// 	assert.EqualValues(t, 1001, resp3.Index)
// 	assert.Equal(t, anonymousPolicy, resp3.Policy)
//
// 	// Lookup non-anonoymous policy with no token
// 	get = &structs.ACLPolicySpecificRequest{
// 		Name: policy.Name,
// 		QueryOptions: structs.QueryOptions{
// 			Region: "global",
// 		},
// 	}
// 	var resp4 structs.SingleACLPolicyResponse
// 	err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp4)
// 	require.Error(t, err)
// 	require.Contains(t, err.Error(), structs.ErrPermissionDenied.Error())
// }

// func TestACLEndpoint_GetPolicy_Blocking(t *testing.T) {
// 	t.Parallel()
//
// 	s1, root, cleanupS1 := TestACLServer(t, nil)
// 	defer cleanupS1()
// 	state := s1.fsm.State()
// 	codec := rpcClient(t, s1)
// 	testutil.WaitForLeader(t, s1.RPC)
//
// 	// Create the policies
// 	p1 := mock.ACLPolicy()
// 	p2 := mock.ACLPolicy()
//
// 	// First create an unrelated policy
// 	time.AfterFunc(100*time.Millisecond, func() {
// 		err := state.UpsertACLPolicies(100, []*structs.ACLPolicy{p1})
// 		if err != nil {
// 			t.Fatalf("err: %v", err)
// 		}
// 	})
//
// 	// Upsert the policy we are watching later
// 	time.AfterFunc(200*time.Millisecond, func() {
// 		err := state.UpsertACLPolicies(200, []*structs.ACLPolicy{p2})
// 		if err != nil {
// 			t.Fatalf("err: %v", err)
// 		}
// 	})
//
// 	// Lookup the policy
// 	req := &structs.ACLPolicySpecificRequest{
// 		Name: p2.Name,
// 		QueryOptions: structs.QueryOptions{
// 			Region:        "global",
// 			MinQueryIndex: 150,
// 			AuthToken:     root.SecretID,
// 		},
// 	}
// 	var resp structs.SingleACLPolicyResponse
// 	start := time.Now()
// 	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", req, &resp); err != nil {
// 		t.Fatalf("err: %v", err)
// 	}
//
// 	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
// 		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
// 	}
// 	if resp.Index != 200 {
// 		t.Fatalf("Bad index: %d %d", resp.Index, 200)
// 	}
// 	if resp.Policy == nil || resp.Policy.Name != p2.Name {
// 		t.Fatalf("bad: %#v", resp.Policy)
// 	}
//
// 	// Eval delete triggers watches
// 	time.AfterFunc(100*time.Millisecond, func() {
// 		err := state.DeleteACLPolicies(300, []string{p2.Name})
// 		if err != nil {
// 			t.Fatalf("err: %v", err)
// 		}
// 	})
//
// 	req.QueryOptions.MinQueryIndex = 250
// 	var resp2 structs.SingleACLPolicyResponse
// 	start = time.Now()
// 	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", req, &resp2); err != nil {
// 		t.Fatalf("err: %v", err)
// 	}
//
// 	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
// 		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
// 	}
// 	if resp2.Index != 300 {
// 		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
// 	}
// 	if resp2.Policy != nil {
// 		t.Fatalf("bad: %#v", resp2.Policy)
// 	}
// }

func TestScalingEndpoint_GetPolicy(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
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
	require.Equal(uint64(1000), resp.Index)
	require.Equal(*p1, *resp.Policy)

	// Lookup non-existing policy
	get.ID = uuid.Generate()
	resp = structs.SingleScalingPolicyResponse{}
	err = msgpackrpc.CallWithCodec(codec, "Scaling.GetPolicy", get, &resp)
	require.NoError(err)
	require.Equal(uint64(1000), resp.Index)
	require.Nil(resp.Policy)
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
	require.Equal(uint64(200), resp.Index, "bad index")
	require.Len(resp.Policies, 2)
	require.ElementsMatch([]string{p1.ID, p2.ID}, []string{resp.Policies[0].ID, resp.Policies[1].ID})
}

func TestScalingEndpoint_ListPolicies(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ScalingPolicy()
	p2 := mock.ScalingPolicy()

	s1.fsm.State().UpsertScalingPolicies(1000, []*structs.ScalingPolicy{p1, p2})

	// Lookup the policies
	get := &structs.ScalingPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Scaling.ListPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.EqualValues(1000, resp.Index)
	assert.Len(resp.Policies, 2)
}

// TestACLEndpoint_ListPolicies_Unauthenticated asserts that
// unauthenticated ListPolicies returns anonymous policy if one
// exists, otherwise, empty
// func TestACLEndpoint_ListPolicies_Unauthenticated(t *testing.T) {
// 	t.Parallel()
//
// 	s1, _, cleanupS1 := TestACLServer(t, nil)
// 	defer cleanupS1()
// 	codec := rpcClient(t, s1)
// 	testutil.WaitForLeader(t, s1.RPC)
//
// 	listPolicies := func() (*structs.ACLPolicyListResponse, error) {
// 		// Lookup the policies
// 		get := &structs.ACLPolicyListRequest{
// 			QueryOptions: structs.QueryOptions{
// 				Region: "global",
// 			},
// 		}
//
// 		var resp structs.ACLPolicyListResponse
// 		err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return &resp, nil
// 	}
//
// 	p1 := mock.ACLPolicy()
// 	p1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
// 	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{p1})
//
// 	t.Run("no anonymous policy", func(t *testing.T) {
// 		resp, err := listPolicies()
// 		require.NoError(t, err)
// 		require.Empty(t, resp.Policies)
// 		require.Equal(t, uint64(1000), resp.Index)
// 	})
//
// 	// now try with anonymous policy
// 	p2 := mock.ACLPolicy()
// 	p2.Name = "anonymous"
// 	s1.fsm.State().UpsertACLPolicies(1001, []*structs.ACLPolicy{p2})
//
// 	t.Run("with anonymous policy", func(t *testing.T) {
// 		resp, err := listPolicies()
// 		require.NoError(t, err)
// 		require.Len(t, resp.Policies, 1)
// 		require.Equal(t, "anonymous", resp.Policies[0].Name)
// 		require.Equal(t, uint64(1001), resp.Index)
// 	})
// }
