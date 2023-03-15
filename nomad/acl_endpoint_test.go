package nomad

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACLEndpoint_GetPolicy(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy})

	anonymousPolicy := mock.ACLPolicy()
	anonymousPolicy.Name = "anonymous"
	s1.fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1001, []*structs.ACLPolicy{anonymousPolicy})

	// Create a token with one the policy
	token := mock.ACLToken()
	token.Policies = []string{policy.Name}
	s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1002, []*structs.ACLToken{token})

	// Lookup the policy
	get := &structs.ACLPolicySpecificRequest{
		Name: policy.Name,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.SingleACLPolicyResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, policy, resp.Policy)

	// Lookup non-existing policy
	get.Name = uuid.Generate()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1001), resp.Index)
	assert.Nil(t, resp.Policy)

	// Lookup the policy with the token
	get = &structs.ACLPolicySpecificRequest{
		Name: policy.Name,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: token.SecretID,
		},
	}
	var resp2 structs.SingleACLPolicyResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.EqualValues(t, 1000, resp2.Index)
	assert.Equal(t, policy, resp2.Policy)

	// Lookup the anonymous policy with no token
	get = &structs.ACLPolicySpecificRequest{
		Name: anonymousPolicy.Name,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp3 structs.SingleACLPolicyResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp3); err != nil {
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1001, resp3.Index)
	assert.Equal(t, anonymousPolicy, resp3.Policy)

	// Lookup non-anonoymous policy with no token
	get = &structs.ACLPolicySpecificRequest{
		Name: policy.Name,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var resp4 structs.SingleACLPolicyResponse
	err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp4)
	require.Error(t, err)
	require.Contains(t, err.Error(), structs.ErrPermissionDenied.Error())

	// Generate and upsert an ACL role which links to the previously created
	// policy.
	mockACLRole := mock.ACLRole()
	mockACLRole.Policies = []*structs.ACLRolePolicyLink{{Name: policy.Name}}
	must.NoError(t, s1.fsm.State().UpsertACLRoles(
		structs.MsgTypeTestSetup, 1010, []*structs.ACLRole{mockACLRole}, false))

	// Generate and upsert an ACL token which only has ACL role links.
	mockTokenWithRole := mock.ACLToken()
	mockTokenWithRole.Policies = []string{}
	mockTokenWithRole.Roles = []*structs.ACLTokenRoleLink{{ID: mockACLRole.ID}}
	must.NoError(t, s1.fsm.State().UpsertACLTokens(
		structs.MsgTypeTestSetup, 1020, []*structs.ACLToken{mockTokenWithRole}))

	// Use the newly created token to attempt to read the policy which is
	// linked via a role, and not directly referenced within the policy array.
	req5 := &structs.ACLPolicySpecificRequest{
		Name: policy.Name,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: mockTokenWithRole.SecretID,
		},
	}

	var resp5 structs.SingleACLPolicyResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", req5, &resp5))
	must.Eq(t, 1000, resp5.Index)
	must.Eq(t, policy, resp5.Policy)
}

func TestACLEndpoint_GetPolicy_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policies
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()

	// First create an unrelated policy
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the policy we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 200, []*structs.ACLPolicy{p2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the policy
	req := &structs.ACLPolicySpecificRequest{
		Name: p2.Name,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
			AuthToken:     root.SecretID,
		},
	}
	var resp structs.SingleACLPolicyResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Policy == nil || resp.Policy.Name != p2.Name {
		t.Fatalf("bad: %#v", resp.Policy)
	}

	// Eval delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.DeleteACLPolicies(structs.MsgTypeTestSetup, 300, []string{p2.Name})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleACLPolicyResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Policy != nil {
		t.Fatalf("bad: %#v", resp2.Policy)
	}
}

func TestACLEndpoint_GetPolicies(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy, policy2})

	// Lookup the policy
	get := &structs.ACLPolicySetRequest{
		Names: []string{policy.Name, policy2.Name},
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.ACLPolicySetResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 2, len(resp.Policies))
	assert.Equal(t, policy, resp.Policies[policy.Name])
	assert.Equal(t, policy2, resp.Policies[policy2.Name])

	// Lookup non-existing policy
	get.Names = []string{uuid.Generate()}
	resp = structs.ACLPolicySetResponse{}
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 0, len(resp.Policies))
}

func TestACLEndpoint_GetPolicies_TokenSubset(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy, policy2})

	token := mock.ACLToken()
	token.Policies = []string{policy.Name}
	s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{token})

	// Lookup the policy which is a subset of our tokens
	get := &structs.ACLPolicySetRequest{
		Names: []string{policy.Name},
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: token.SecretID,
		},
	}
	var resp structs.ACLPolicySetResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 1, len(resp.Policies))
	assert.Equal(t, policy, resp.Policies[policy.Name])

	// Lookup non-associated policy
	get.Names = []string{policy2.Name}
	resp = structs.ACLPolicySetResponse{}
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", get, &resp); err == nil {
		t.Fatalf("expected error")
	}

	// Generate and upsert an ACL role which links to the previously created
	// policy.
	mockACLRole := mock.ACLRole()
	mockACLRole.Policies = []*structs.ACLRolePolicyLink{{Name: policy.Name}}
	must.NoError(t, s1.fsm.State().UpsertACLRoles(
		structs.MsgTypeTestSetup, 1010, []*structs.ACLRole{mockACLRole}, false))

	// Generate and upsert an ACL token which only has ACL role links.
	mockTokenWithRole := mock.ACLToken()
	mockTokenWithRole.Policies = []string{}
	mockTokenWithRole.Roles = []*structs.ACLTokenRoleLink{{ID: mockACLRole.ID}}
	must.NoError(t, s1.fsm.State().UpsertACLTokens(
		structs.MsgTypeTestSetup, 1020, []*structs.ACLToken{mockTokenWithRole}))

	// Use the newly created token to attempt to read the policy which is
	// linked via a role, and not directly referenced within the policy array.
	req1 := &structs.ACLPolicySetRequest{
		Names: []string{policy.Name},
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: mockTokenWithRole.SecretID,
		},
	}

	var resp1 structs.ACLPolicySetResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", req1, &resp1))
	must.Eq(t, 1000, resp1.Index)
	must.Eq(t, 1, len(resp1.Policies))
	must.Eq(t, policy, resp1.Policies[policy.Name])

	// Generate and upsert an ACL token which only has both direct policy links
	// and ACL role links.
	mockTokenWithRolePolicy := mock.ACLToken()
	mockTokenWithRolePolicy.Policies = []string{policy2.Name}
	mockTokenWithRolePolicy.Roles = []*structs.ACLTokenRoleLink{{ID: mockACLRole.ID}}
	must.NoError(t, s1.fsm.State().UpsertACLTokens(
		structs.MsgTypeTestSetup, 1030, []*structs.ACLToken{mockTokenWithRolePolicy}))

	// Use the newly created token to attempt to read the policies which are
	// linked directly, and by ACL roles.
	req2 := &structs.ACLPolicySetRequest{
		Names: []string{policy.Name, policy2.Name},
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: mockTokenWithRolePolicy.SecretID,
		},
	}

	var resp2 structs.ACLPolicySetResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", req2, &resp2))
	must.Eq(t, 1000, resp2.Index)
	must.Eq(t, 2, len(resp2.Policies))
}

func TestACLEndpoint_GetPolicies_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policies
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()

	// First create an unrelated policy
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the policy we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 200, []*structs.ACLPolicy{p2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the policy
	req := &structs.ACLPolicySetRequest{
		Names: []string{p2.Name},
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
			AuthToken:     root.SecretID,
		},
	}
	var resp structs.ACLPolicySetResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Policies) == 0 || resp.Policies[p2.Name] == nil {
		t.Fatalf("bad: %#v", resp.Policies)
	}

	// Eval delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.DeleteACLPolicies(structs.MsgTypeTestSetup, 300, []string{p2.Name})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.ACLPolicySetResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if len(resp2.Policies) != 0 {
		t.Fatalf("bad: %#v", resp2.Policies)
	}
}

func TestACLEndpoint_ListPolicies(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()

	p1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	p2.Name = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{p1, p2})

	// Create a token with one of those policies
	token := mock.ACLToken()
	token.Policies = []string{p1.Name}
	s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1001, []*structs.ACLToken{token})

	// Lookup the policies
	get := &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.EqualValues(1000, resp.Index)
	assert.Len(resp.Policies, 2)

	// Lookup the policies by prefix
	get = &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Prefix:    "aaaabb",
			AuthToken: root.SecretID,
		},
	}
	var resp2 structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.EqualValues(1000, resp2.Index)
	assert.Len(resp2.Policies, 1)

	// List policies using the created token
	get = &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: token.SecretID,
		},
	}
	var resp3 structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.EqualValues(1000, resp3.Index)
	if assert.Len(resp3.Policies, 1) {
		assert.Equal(resp3.Policies[0].Name, p1.Name)
	}

	// Generate and upsert an ACL role which links to the previously created
	// policy.
	mockACLRole := mock.ACLRole()
	mockACLRole.Policies = []*structs.ACLRolePolicyLink{{Name: p1.Name}}
	must.NoError(t, s1.fsm.State().UpsertACLRoles(
		structs.MsgTypeTestSetup, 1010, []*structs.ACLRole{mockACLRole}, false))

	// Generate and upsert an ACL token which only has ACL role links.
	mockTokenWithRole := mock.ACLToken()
	mockTokenWithRole.Policies = []string{}
	mockTokenWithRole.Roles = []*structs.ACLTokenRoleLink{{ID: mockACLRole.ID}}
	must.NoError(t, s1.fsm.State().UpsertACLTokens(
		structs.MsgTypeTestSetup, 1020, []*structs.ACLToken{mockTokenWithRole}))

	// Use the newly created token to attempt to list the policies. We should
	// get the single policy linked by the ACL role.
	req4 := &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: mockTokenWithRole.SecretID,
		},
	}

	var resp4 structs.ACLPolicyListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", req4, &resp4))
	must.Eq(t, 1000, resp4.Index)
	must.Len(t, 1, resp4.Policies)
	must.Eq(t, p1.Name, resp4.Policies[0].Name)
	must.Eq(t, p1.Hash, resp4.Policies[0].Hash)
}

// TestACLEndpoint_ListPolicies_Unauthenticated asserts that
// unauthenticated ListPolicies returns anonymous policy if one
// exists, otherwise, empty
func TestACLEndpoint_ListPolicies_Unauthenticated(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	listPolicies := func() (*structs.ACLPolicyListResponse, error) {
		// Lookup the policies
		get := &structs.ACLPolicyListRequest{
			QueryOptions: structs.QueryOptions{
				Region: "global",
			},
		}

		var resp structs.ACLPolicyListResponse
		err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp)
		if err != nil {
			return nil, err
		}
		return &resp, nil
	}

	p1 := mock.ACLPolicy()
	p1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{p1})

	t.Run("no anonymous policy", func(t *testing.T) {
		resp, err := listPolicies()
		require.NoError(t, err)
		require.Empty(t, resp.Policies)
		require.Equal(t, uint64(1000), resp.Index)
	})

	// now try with anonymous policy
	p2 := mock.ACLPolicy()
	p2.Name = "anonymous"
	s1.fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1001, []*structs.ACLPolicy{p2})

	t.Run("with anonymous policy", func(t *testing.T) {
		resp, err := listPolicies()
		require.NoError(t, err)
		require.Len(t, resp.Policies, 1)
		require.Equal(t, "anonymous", resp.Policies[0].Name)
		require.Equal(t, uint64(1001), resp.Index)
	})
}

func TestACLEndpoint_ListPolicies_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policy
	policy := mock.ACLPolicy()

	// Upsert eval triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 2, []*structs.ACLPolicy{policy}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1,
			AuthToken:     root.SecretID,
		},
	}
	start := time.Now()
	var resp structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	assert.Equal(t, uint64(2), resp.Index)
	if len(resp.Policies) != 1 || resp.Policies[0].Name != policy.Name {
		t.Fatalf("bad: %#v", resp.Policies)
	}

	// Eval deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteACLPolicies(structs.MsgTypeTestSetup, 3, []string{policy.Name}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 2
	start = time.Now()
	var resp2 structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	assert.Equal(t, uint64(3), resp2.Index)
	assert.Equal(t, 0, len(resp2.Policies))
}

func TestACLEndpoint_DeletePolicies(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{p1})

	// Lookup the policies
	req := &structs.ACLPolicyDeleteRequest{
		Names: []string{p1.Name},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.DeletePolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
}

func TestACLEndpoint_UpsertPolicies(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()

	// Lookup the policies
	req := &structs.ACLPolicyUpsertRequest{
		Policies: []*structs.ACLPolicy{p1},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertPolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)

	// Check we created the policy
	out, err := s1.fsm.State().ACLPolicyByName(nil, p1.Name)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestACLEndpoint_UpsertPolicies_Invalid(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	p1.Rules = "blah blah invalid"

	// Lookup the policies
	req := &structs.ACLPolicyUpsertRequest{
		Policies: []*structs.ACLPolicy{p1},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertPolicies", req, &resp)
	must.ErrorContains(t, err, "400")
	must.ErrorContains(t, err, "failed to parse")
}

func TestACLEndpoint_GetToken(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	token := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{token})

	// Lookup the token
	get := &structs.ACLTokenSpecificRequest{
		AccessorID: token.AccessorID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.SingleACLTokenResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, token, resp.Token)

	// Lookup non-existing token
	get.AccessorID = uuid.Generate()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Nil(t, resp.Token)

	// Lookup the token by accessor id using the tokens secret ID
	get.AccessorID = token.AccessorID
	get.AuthToken = token.SecretID
	var resp2 structs.SingleACLTokenResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp2.Index)
	assert.Equal(t, token, resp2.Token)
}

func TestACLEndpoint_GetToken_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the tokens
	p1 := mock.ACLToken()
	p2 := mock.ACLToken()

	// First create an unrelated token
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 100, []*structs.ACLToken{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the token we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 200, []*structs.ACLToken{p2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the token
	req := &structs.ACLTokenSpecificRequest{
		AccessorID: p2.AccessorID,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
			AuthToken:     root.SecretID,
		},
	}
	var resp structs.SingleACLTokenResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if resp.Token == nil || resp.Token.AccessorID != p2.AccessorID {
		t.Fatalf("bad: %#v", resp.Token)
	}

	// Eval delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.DeleteACLTokens(structs.MsgTypeTestSetup, 300, []string{p2.AccessorID})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleACLTokenResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if resp2.Token != nil {
		t.Fatalf("bad: %#v", resp2.Token)
	}
}

func TestACLEndpoint_GetTokens(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	token := mock.ACLToken()
	token2 := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{token, token2})

	// Lookup the token
	get := &structs.ACLTokenSetRequest{
		AccessorIDS: []string{token.AccessorID, token2.AccessorID},
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.ACLTokenSetResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetTokens", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 2, len(resp.Tokens))
	assert.Equal(t, token, resp.Tokens[token.AccessorID])

	// Lookup non-existing token
	get.AccessorIDS = []string{uuid.Generate()}
	resp = structs.ACLTokenSetResponse{}
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetTokens", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 0, len(resp.Tokens))
}

func TestACLEndpoint_GetTokens_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the tokens
	p1 := mock.ACLToken()
	p2 := mock.ACLToken()

	// First create an unrelated token
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 100, []*structs.ACLToken{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the token we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 200, []*structs.ACLToken{p2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the token
	req := &structs.ACLTokenSetRequest{
		AccessorIDS: []string{p2.AccessorID},
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
			AuthToken:     root.SecretID,
		},
	}
	var resp structs.ACLTokenSetResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	if resp.Index != 200 {
		t.Fatalf("Bad index: %d %d", resp.Index, 200)
	}
	if len(resp.Tokens) == 0 || resp.Tokens[p2.AccessorID] == nil {
		t.Fatalf("bad: %#v", resp.Tokens)
	}

	// Eval delete triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.DeleteACLTokens(structs.MsgTypeTestSetup, 300, []string{p2.AccessorID})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.ACLTokenSetResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetTokens", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	if resp2.Index != 300 {
		t.Fatalf("Bad index: %d %d", resp2.Index, 300)
	}
	if len(resp2.Tokens) != 0 {
		t.Fatalf("bad: %#v", resp2.Tokens)
	}
}

func TestACLEndpoint_ListTokens(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	p2 := mock.ACLToken()
	p2.Global = true

	p1.AccessorID = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	p2.AccessorID = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{p1, p2})

	// Lookup the tokens
	get := &structs.ACLTokenListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 3, len(resp.Tokens))

	// Lookup the tokens by prefix
	get = &structs.ACLTokenListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Prefix:    "aaaabb",
			AuthToken: root.SecretID,
		},
	}
	var resp2 structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp2.Index)
	assert.Equal(t, 1, len(resp2.Tokens))

	// Lookup the global tokens
	get = &structs.ACLTokenListRequest{
		GlobalOnly: true,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp3 structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", get, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp3.Index)
	assert.Equal(t, 2, len(resp3.Tokens))
}

func TestACLEndpoint_ListTokens_PaginationFiltering(t *testing.T) {
	ci.Parallel(t)
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// create a set of ACL tokens. these are in the order that the state store
	// will return them from the iterator (sorted by key) for ease of writing
	// tests
	mocks := []struct {
		ids []string
		typ string
	}{
		{ids: []string{"aaaa1111-3350-4b4b-d185-0e1992ed43e9"}, typ: "management"}, // 0
		{ids: []string{"aaaaaa22-3350-4b4b-d185-0e1992ed43e9"}},                    // 1
		{ids: []string{"aaaaaa33-3350-4b4b-d185-0e1992ed43e9"}},                    // 2
		{ids: []string{"aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"}},                    // 3
		{ids: []string{"aaaaaabb-3350-4b4b-d185-0e1992ed43e9"}},                    // 4
		{ids: []string{"aaaaaacc-3350-4b4b-d185-0e1992ed43e9"}},                    // 5
		{ids: []string{"aaaaaadd-3350-4b4b-d185-0e1992ed43e9"}},                    // 6
		{ids: []string{"00000111-3350-4b4b-d185-0e1992ed43e9"}},                    // 7
		{ids: []string{ // 8
			"00000222-3350-4b4b-d185-0e1992ed43e9",
			"00000333-3350-4b4b-d185-0e1992ed43e9",
		}},
		{}, // 9, index missing
		{ids: []string{"bbbb1111-3350-4b4b-d185-0e1992ed43e9"}}, // 10
	}

	state := s1.fsm.State()

	var bootstrapToken string
	for i, m := range mocks {
		tokensInTx := []*structs.ACLToken{}
		for _, id := range m.ids {
			token := mock.ACLToken()
			token.AccessorID = id
			token.Type = m.typ
			tokensInTx = append(tokensInTx, token)
		}
		index := 1000 + uint64(i)

		// bootstrap cluster with the first token
		if i == 0 {
			token := tokensInTx[0]
			bootstrapToken = token.SecretID
			err := s1.State().BootstrapACLTokens(structs.MsgTypeTestSetup, index, 0, token)
			require.NoError(t, err)

			err = state.UpsertACLTokens(structs.MsgTypeTestSetup, index, tokensInTx[1:])
			require.NoError(t, err)
		} else {
			err := state.UpsertACLTokens(structs.MsgTypeTestSetup, index, tokensInTx)
			require.NoError(t, err)
		}
	}

	cases := []struct {
		name              string
		prefix            string
		filter            string
		nextToken         string
		pageSize          int32
		expectedNextToken string
		expectedIDs       []string
		expectedError     string
	}{
		{
			name:              "test01 size-2 page-1",
			pageSize:          2,
			expectedNextToken: "1002.aaaaaa33-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test02 size-2 page-1 with prefix",
			prefix:            "aaaa",
			pageSize:          2,
			expectedNextToken: "aaaaaa33-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test03 size-2 page-2 default NS",
			pageSize:          2,
			nextToken:         "1002.aaaaaa33-3350-4b4b-d185-0e1992ed43e9",
			expectedNextToken: "1004.aaaaaabb-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaaaa33-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaaaa-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:   "test04 go-bexpr filter",
			filter: `AccessorID matches "^a+[123]"`,
			expectedIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa33-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test05 go-bexpr filter with pagination",
			filter:            `AccessorID matches "^a+[123]"`,
			pageSize:          2,
			expectedNextToken: "1002.aaaaaa33-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"aaaa1111-3350-4b4b-d185-0e1992ed43e9",
				"aaaaaa22-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:          "test06 go-bexpr invalid expression",
			filter:        `NotValid`,
			expectedError: "failed to read filter expression",
		},
		{
			name:          "test07 go-bexpr invalid field",
			filter:        `InvalidField == "value"`,
			expectedError: "error finding value in datum",
		},
		{
			name:              "test08 non-lexicographic order",
			pageSize:          1,
			nextToken:         "1007.00000111-3350-4b4b-d185-0e1992ed43e9",
			expectedNextToken: "1008.00000222-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"00000111-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:              "test09 same index",
			pageSize:          1,
			nextToken:         "1008.00000222-3350-4b4b-d185-0e1992ed43e9",
			expectedNextToken: "1008.00000333-3350-4b4b-d185-0e1992ed43e9",
			expectedIDs: []string{
				"00000222-3350-4b4b-d185-0e1992ed43e9",
			},
		},
		{
			name:      "test10 missing index",
			pageSize:  1,
			nextToken: "1009.e9522802-0cd8-4b1d-9c9e-ab3d97938371",
			expectedIDs: []string{
				"bbbb1111-3350-4b4b-d185-0e1992ed43e9",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.ACLTokenListRequest{
				QueryOptions: structs.QueryOptions{
					Region:    "global",
					Prefix:    tc.prefix,
					Filter:    tc.filter,
					PerPage:   tc.pageSize,
					NextToken: tc.nextToken,
				},
			}
			req.AuthToken = bootstrapToken
			var resp structs.ACLTokenListResponse
			err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", req, &resp)
			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
				return
			}

			gotIDs := []string{}
			for _, token := range resp.Tokens {
				gotIDs = append(gotIDs, token.AccessorID)
			}
			require.Equal(t, tc.expectedIDs, gotIDs, "unexpected page of tokens")
			require.Equal(t, tc.expectedNextToken, resp.QueryMeta.NextToken, "unexpected NextToken")
		})
	}
}

func TestACLEndpoint_ListTokens_Order(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create register requests
	uuid1 := uuid.Generate()
	token1 := mock.ACLManagementToken()
	token1.AccessorID = uuid1

	uuid2 := uuid.Generate()
	token2 := mock.ACLToken()
	token2.AccessorID = uuid2

	uuid3 := uuid.Generate()
	token3 := mock.ACLToken()
	token3.AccessorID = uuid3

	// bootstrap cluster with the first token
	bootstrapToken := token1.SecretID
	err := s1.State().BootstrapACLTokens(structs.MsgTypeTestSetup, 1000, 0, token1)
	require.NoError(t, err)

	err = s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1001, []*structs.ACLToken{token2})
	require.NoError(t, err)

	err = s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1002, []*structs.ACLToken{token3})
	require.NoError(t, err)

	// update token2 again so we can later assert create index order did not change
	err = s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1003, []*structs.ACLToken{token2})
	require.NoError(t, err)

	t.Run("default", func(t *testing.T) {
		// Lookup the tokens in the default order (oldest first)
		get := &structs.ACLTokenListRequest{
			QueryOptions: structs.QueryOptions{
				Region: "global",
			},
		}
		get.AuthToken = bootstrapToken

		var resp structs.ACLTokenListResponse
		err = msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", get, &resp)
		require.NoError(t, err)
		require.Equal(t, uint64(1003), resp.Index)
		require.Len(t, resp.Tokens, 3)

		// Assert returned order is by CreateIndex (ascending)
		require.Equal(t, uint64(1000), resp.Tokens[0].CreateIndex)
		require.Equal(t, uuid1, resp.Tokens[0].AccessorID)

		require.Equal(t, uint64(1001), resp.Tokens[1].CreateIndex)
		require.Equal(t, uuid2, resp.Tokens[1].AccessorID)

		require.Equal(t, uint64(1002), resp.Tokens[2].CreateIndex)
		require.Equal(t, uuid3, resp.Tokens[2].AccessorID)
	})

	t.Run("reverse", func(t *testing.T) {
		// Lookup the tokens in reverse order (newest first)
		get := &structs.ACLTokenListRequest{
			QueryOptions: structs.QueryOptions{
				Region:  "global",
				Reverse: true,
			},
		}
		get.AuthToken = bootstrapToken

		var resp structs.ACLTokenListResponse
		err = msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", get, &resp)
		require.NoError(t, err)
		require.Equal(t, uint64(1003), resp.Index)
		require.Len(t, resp.Tokens, 3)

		// Assert returned order is by CreateIndex (descending)
		require.Equal(t, uint64(1002), resp.Tokens[0].CreateIndex)
		require.Equal(t, uuid3, resp.Tokens[0].AccessorID)

		require.Equal(t, uint64(1001), resp.Tokens[1].CreateIndex)
		require.Equal(t, uuid2, resp.Tokens[1].AccessorID)

		require.Equal(t, uint64(1000), resp.Tokens[2].CreateIndex)
		require.Equal(t, uuid1, resp.Tokens[2].AccessorID)
	})
}

func TestACLEndpoint_ListTokens_Blocking(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the token
	token := mock.ACLToken()

	// Upsert eval triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 3, []*structs.ACLToken{token}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.ACLTokenListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 2,
			AuthToken:     root.SecretID,
		},
	}
	start := time.Now()
	var resp structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp)
	}
	assert.Equal(t, uint64(3), resp.Index)
	if len(resp.Tokens) != 2 {
		t.Fatalf("bad: %#v", resp.Tokens)
	}

	// Eval deletion triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.DeleteACLTokens(structs.MsgTypeTestSetup, 4, []string{token.AccessorID}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 3
	start = time.Now()
	var resp2 structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	assert.Equal(t, uint64(4), resp2.Index)
	assert.Equal(t, 1, len(resp2.Tokens))
}

func TestACLEndpoint_DeleteTokens(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{p1})

	// Lookup the tokens
	req := &structs.ACLTokenDeleteRequest{
		AccessorIDs: []string{p1.AccessorID},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.DeleteTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
}

func TestACLEndpoint_DeleteTokens_WithNonexistentToken(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	nonexistentToken := mock.ACLToken()

	// Lookup the policies
	req := &structs.ACLTokenDeleteRequest{
		AccessorIDs: []string{nonexistentToken.AccessorID},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ACL.DeleteTokens", req, &resp)

	assert.NotNil(err)
	expectedError := fmt.Sprintf("Cannot delete nonexistent tokens: %s", nonexistentToken.AccessorID)
	assert.Contains(err.Error(), expectedError)
}

func TestACLEndpoint_Bootstrap(t *testing.T) {
	ci.Parallel(t)
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Lookup the tokens
	req := &structs.ACLTokenBootstrapRequest{
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.ACLTokenUpsertResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Bootstrap", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
	assert.NotNil(t, resp.Tokens[0])

	// Get the token out from the response
	created := resp.Tokens[0]
	assert.NotEqual(t, "", created.AccessorID)
	assert.NotEqual(t, "", created.SecretID)
	assert.NotEqual(t, time.Time{}, created.CreateTime)
	assert.Equal(t, structs.ACLManagementToken, created.Type)
	assert.Equal(t, "Bootstrap Token", created.Name)
	assert.Equal(t, true, created.Global)

	// Check we created the token
	out, err := s1.fsm.State().ACLTokenByAccessorID(nil, created.AccessorID)
	assert.Nil(t, err)
	assert.Equal(t, created, out)
}

func TestACLEndpoint_BootstrapOperator(t *testing.T) {
	ci.Parallel(t)
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Lookup the tokens
	req := &structs.ACLTokenBootstrapRequest{
		WriteRequest:    structs.WriteRequest{Region: "global"},
		BootstrapSecret: "2b778dd9-f5f1-6f29-b4b4-9a5fa948757a",
	}
	var resp structs.ACLTokenUpsertResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Bootstrap", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
	assert.NotNil(t, resp.Tokens[0])

	// Get the token out from the response
	created := resp.Tokens[0]
	assert.NotEqual(t, "", created.AccessorID)
	assert.NotEqual(t, "", created.SecretID)
	assert.NotEqual(t, time.Time{}, created.CreateTime)
	assert.Equal(t, structs.ACLManagementToken, created.Type)
	assert.Equal(t, "Bootstrap Token", created.Name)
	assert.Equal(t, true, created.Global)

	// Check we created the token
	out, err := s1.fsm.State().ACLTokenByAccessorID(nil, created.AccessorID)
	assert.Nil(t, err)
	assert.Equal(t, created, out)
	// Check we have the correct operator token
	tokenout, err := s1.fsm.State().ACLTokenBySecretID(nil, created.SecretID)
	assert.Nil(t, err)
	assert.Equal(t, created, tokenout)
}

func TestACLEndpoint_Bootstrap_Reset(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
		c.DataDir = dir
		c.DevMode = false
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Lookup the tokens
	req := &structs.ACLTokenBootstrapRequest{
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.ACLTokenUpsertResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Bootstrap", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
	assert.NotNil(t, resp.Tokens[0])
	resetIdx := resp.Tokens[0].CreateIndex

	// Try again, should fail
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Bootstrap", req, &resp); err == nil {
		t.Fatalf("expected err")
	}

	// Create the reset file
	output := []byte(fmt.Sprintf("%d", resetIdx))
	path := filepath.Join(dir, aclBootstrapReset)
	assert.Nil(t, os.WriteFile(path, output, 0755))

	// Try again, should work with reset
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Bootstrap", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
	assert.NotNil(t, resp.Tokens[0])

	// Get the token out from the response
	created := resp.Tokens[0]
	assert.NotEqual(t, "", created.AccessorID)
	assert.NotEqual(t, "", created.SecretID)
	assert.NotEqual(t, time.Time{}, created.CreateTime)
	assert.Equal(t, structs.ACLManagementToken, created.Type)
	assert.Equal(t, "Bootstrap Token", created.Name)
	assert.Equal(t, true, created.Global)

	// Check we created the token
	out, err := s1.fsm.State().ACLTokenByAccessorID(nil, created.AccessorID)
	assert.Nil(t, err)
	assert.Equal(t, created, out)

	// Try again, should fail
	if err := msgpackrpc.CallWithCodec(codec, "ACL.Bootstrap", req, &resp); err == nil {
		t.Fatalf("expected err")
	}
}

func TestACLEndpoint_UpsertTokens(t *testing.T) {
	ci.Parallel(t)

	// Each sub-test uses the same server to avoid creating a new one for each
	// test. This means some care has to be taken with resource naming.
	testServer, rootACLToken, testServerCleanup := TestACLServer(t, nil)
	defer testServerCleanup()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	testCases := []struct {
		name   string
		testFn func(testServer *Server, aclToken *structs.ACLToken)
	}{
		{
			name: "valid client token",
			testFn: func(testServer *Server, aclToken *structs.ACLToken) {

				// Create the register request with a mocked token. We must set
				// an empty accessorID, otherwise Nomad treats this as an
				// update request.
				p1 := mock.ACLToken()
				p1.AccessorID = ""

				req := &structs.ACLTokenUpsertRequest{
					Tokens: []*structs.ACLToken{p1},
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						AuthToken: aclToken.SecretID,
					},
				}
				var resp structs.ACLTokenUpsertResponse
				require.NoError(t, msgpackrpc.CallWithCodec(codec, structs.ACLUpsertTokensRPCMethod, req, &resp))
				must.Positive(t, resp.Index)

				// Get the token out from the response.
				created := resp.Tokens[0]
				require.NotEqual(t, "", created.AccessorID)
				require.NotEqual(t, "", created.SecretID)
				require.NotEqual(t, time.Time{}, created.CreateTime)
				require.Equal(t, p1.Type, created.Type)
				require.Equal(t, p1.Policies, created.Policies)
				require.Equal(t, p1.Name, created.Name)

				// Check we created the token.
				out, err := testServer.fsm.State().ACLTokenByAccessorID(nil, created.AccessorID)
				require.Nil(t, err)
				require.Equal(t, created, out)

				// Update the token type and policy list so we can try updating
				// it.
				req.Tokens[0] = created
				created.Type = "management"
				created.Policies = nil

				// Track the first upsert index, so we can test the next
				// response against this and perform the update.
				originalIndex := resp.Index

				require.NoError(t, msgpackrpc.CallWithCodec(codec, structs.ACLUpsertTokensRPCMethod, req, &resp))
				require.Greater(t, resp.Index, originalIndex)

				// Read the token from state and perform an equality check to
				// ensure everything matches as we expect.
				out, err = testServer.fsm.State().ACLTokenByAccessorID(nil, created.AccessorID)
				require.Nil(t, err)
				require.Equal(t, created, out)
			},
		},
		{
			name: "valid management token with expiration",
			testFn: func(testServer *Server, aclToken *structs.ACLToken) {

				// Create our RPC request object which includes a management
				// token with a TTL.
				req := &structs.ACLTokenUpsertRequest{
					Tokens: []*structs.ACLToken{
						{
							Name:          "my-management-token-" + uuid.Generate(),
							Type:          structs.ACLManagementToken,
							ExpirationTTL: 10 * time.Minute,
						},
					},
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						AuthToken: aclToken.SecretID,
					},
				}

				// Send the RPC request and ensure the expiration time is as
				// expected.
				var resp structs.ACLTokenUpsertResponse
				require.NoError(t, msgpackrpc.CallWithCodec(codec, structs.ACLUpsertTokensRPCMethod, req, &resp))
				require.Equal(t, 10*time.Minute, resp.Tokens[0].ExpirationTime.Sub(resp.Tokens[0].CreateTime))
			},
		},
		{
			name: "valid client token with expiration",
			testFn: func(testServer *Server, aclToken *structs.ACLToken) {

				// Create an ACL policy so this can be associated to our client
				// token.
				policyReq := &structs.ACLPolicyUpsertRequest{
					Policies: []*structs.ACLPolicy{mock.ACLPolicy()},
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						AuthToken: aclToken.SecretID,
					},
				}

				var policyResp structs.GenericResponse
				require.NoError(t, msgpackrpc.CallWithCodec(codec, structs.ACLUpsertPoliciesRPCMethod, policyReq, &policyResp))

				// Create our RPC request object which includes a client token
				// with a TTL that is associated to policies above.
				tokenReq := &structs.ACLTokenUpsertRequest{
					Tokens: []*structs.ACLToken{
						{
							Name:          "my-client-token-" + uuid.Generate(),
							Type:          structs.ACLClientToken,
							Policies:      []string{policyReq.Policies[0].Name},
							ExpirationTTL: 10 * time.Minute,
						},
					},
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						AuthToken: aclToken.SecretID,
					},
				}

				// Send the RPC request and ensure the expiration time is as
				// expected.
				var tokenResp structs.ACLTokenUpsertResponse
				require.NoError(t, msgpackrpc.CallWithCodec(codec, structs.ACLUpsertTokensRPCMethod, tokenReq, &tokenResp))
				require.Equal(t, 10*time.Minute, tokenResp.Tokens[0].ExpirationTime.Sub(tokenResp.Tokens[0].CreateTime))
			},
		},
		{
			name: "invalid token type",
			testFn: func(testServer *Server, aclToken *structs.ACLToken) {

				// Create our RPC request object which includes a token with an
				// unknown type. This allows us to ensure the RPC handler calls
				// the validation func.
				tokenReq := &structs.ACLTokenUpsertRequest{
					Tokens: []*structs.ACLToken{
						{
							Name: "my-blah-token-" + uuid.Generate(),
							Type: "blah",
						},
					},
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						AuthToken: aclToken.SecretID,
					},
				}

				// Send the RPC request and ensure the expiration time is as
				// expected.
				var tokenResp structs.ACLTokenUpsertResponse
				err := msgpackrpc.CallWithCodec(codec, structs.ACLUpsertTokensRPCMethod, tokenReq, &tokenResp)
				require.ErrorContains(t, err, "token type must be client or management")
				require.Empty(t, tokenResp.Tokens)
			},
		},
		{
			name: "token with role links",
			testFn: func(testServer *Server, aclToken *structs.ACLToken) {

				// Attempt to create a token with a link to a role that does
				// not exist in state.
				tokenReq1 := &structs.ACLTokenUpsertRequest{
					Tokens: []*structs.ACLToken{
						{
							Name:  "my-lovely-token-" + uuid.Generate(),
							Type:  structs.ACLClientToken,
							Roles: []*structs.ACLTokenRoleLink{{Name: "cant-find-me"}},
						},
					},
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						AuthToken: aclToken.SecretID,
					},
				}

				// Send the RPC request and ensure the expiration time is as
				// expected.
				var tokenResp1 structs.ACLTokenUpsertResponse
				err := msgpackrpc.CallWithCodec(codec, structs.ACLUpsertTokensRPCMethod, tokenReq1, &tokenResp1)
				require.ErrorContains(t, err, "cannot find role cant-find-me")
				require.Empty(t, tokenResp1.Tokens)

				// Create an ACL policy that will be linked from an ACL role
				// and enter this into state.
				policy1 := mock.ACLPolicy()

				require.NoError(t, testServer.fsm.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1}))

				// Create an ACL role that links to the above policy.
				aclRole1 := mock.ACLRole()
				aclRole1.Policies = []*structs.ACLRolePolicyLink{{Name: policy1.Name}}

				require.NoError(t, testServer.fsm.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 20, []*structs.ACLRole{aclRole1}, false))

				// Create a token which references the created ACL role. This
				// role reference is duplicated to ensure the handler
				// de-duplicates this before putting it into state.
				// not exist in state.
				tokenReq2 := &structs.ACLTokenUpsertRequest{
					Tokens: []*structs.ACLToken{
						{
							Name: "my-lovely-token-" + uuid.Generate(),
							Type: structs.ACLClientToken,
							Roles: []*structs.ACLTokenRoleLink{
								{ID: aclRole1.ID},
								{ID: aclRole1.ID},
								{ID: aclRole1.ID},
							},
						},
					},
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						AuthToken: aclToken.SecretID,
					},
				}

				// Send the RPC request and ensure the returned token is as
				// expected.
				var tokenResp2 structs.ACLTokenUpsertResponse
				err = msgpackrpc.CallWithCodec(codec, structs.ACLUpsertTokensRPCMethod, tokenReq2, &tokenResp2)
				require.NoError(t, err)
				require.Len(t, tokenResp2.Tokens, 1)
				require.Len(t, tokenResp2.Tokens[0].Policies, 0)
				require.Len(t, tokenResp2.Tokens[0].Roles, 1)
				require.Equal(t, []*structs.ACLTokenRoleLink{{
					ID: aclRole1.ID, Name: aclRole1.Name}}, tokenResp2.Tokens[0].Roles)
			},
		},
		{
			name: "token with role and policy links",
			testFn: func(testServer *Server, aclToken *structs.ACLToken) {

				// Create two ACL policies that will be used for ACL role and
				// policy linking.
				policy1 := mock.ACLPolicy()
				policy2 := mock.ACLPolicy()

				require.NoError(t, testServer.fsm.State().UpsertACLPolicies(
					structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

				// Create an ACL role that links to one of the above policies.
				aclRole1 := mock.ACLRole()
				aclRole1.Policies = []*structs.ACLRolePolicyLink{{Name: policy1.Name}}

				require.NoError(t, testServer.fsm.State().UpsertACLRoles(
					structs.MsgTypeTestSetup, 20, []*structs.ACLRole{aclRole1}, false))

				// Create an ACL token with both ACL role and policy links.
				tokenReq1 := &structs.ACLTokenUpsertRequest{
					Tokens: []*structs.ACLToken{
						{
							Name:     "my-lovely-token-" + uuid.Generate(),
							Type:     structs.ACLClientToken,
							Policies: []string{policy2.Name},
							Roles:    []*structs.ACLTokenRoleLink{{ID: aclRole1.ID}},
						},
					},
					WriteRequest: structs.WriteRequest{
						Region:    DefaultRegion,
						AuthToken: aclToken.SecretID,
					},
				}

				// Send the RPC request and ensure the returned token has
				// policy and ACL role links as expected.
				var tokenResp1 structs.ACLTokenUpsertResponse
				err := msgpackrpc.CallWithCodec(codec, structs.ACLUpsertTokensRPCMethod, tokenReq1, &tokenResp1)
				require.NoError(t, err)
				require.Len(t, tokenResp1.Tokens, 1)
				require.Len(t, tokenResp1.Tokens[0].Policies, 1)
				require.Len(t, tokenResp1.Tokens[0].Roles, 1)
				require.Equal(t, policy2.Name, tokenResp1.Tokens[0].Policies[0])
				require.Equal(t, []*structs.ACLTokenRoleLink{{
					ID: aclRole1.ID, Name: aclRole1.Name}}, tokenResp1.Tokens[0].Roles)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn(testServer, rootACLToken)
		})
	}
}

func TestACLEndpoint_ResolveToken(t *testing.T) {
	ci.Parallel(t)
	s1, _, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	token := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{token})

	// Lookup the token
	get := &structs.ResolveACLTokenRequest{
		SecretID:     token.SecretID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var resp structs.ResolveACLTokenResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ResolveToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, token, resp.Token)

	// Lookup non-existing token
	get.SecretID = uuid.Generate()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ResolveToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Nil(t, resp.Token)
}

func TestACLEndpoint_OneTimeToken(t *testing.T) {
	ci.Parallel(t)

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// create an ACL token

	p1 := mock.ACLToken()
	p1.AccessorID = "" // has to be blank to create
	aclReq := &structs.ACLTokenUpsertRequest{
		Tokens: []*structs.ACLToken{p1},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: root.SecretID,
		},
	}
	var aclResp structs.ACLTokenUpsertResponse
	err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertTokens", aclReq, &aclResp)
	require.NoError(t, err)
	aclToken := aclResp.Tokens[0]

	// Generate a one-time token for this ACL token
	upReq := &structs.OneTimeTokenUpsertRequest{
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: aclToken.SecretID,
		}}

	var upResp structs.OneTimeTokenUpsertResponse

	// Call the upsert RPC
	err = msgpackrpc.CallWithCodec(codec, "ACL.UpsertOneTimeToken", upReq, &upResp)
	require.NoError(t, err)
	result := upResp.OneTimeToken
	require.True(t, time.Now().Before(result.ExpiresAt))
	require.Equal(t, aclToken.AccessorID, result.AccessorID)

	// make sure we can get it back out
	ott, err := s1.fsm.State().OneTimeTokenBySecret(nil, result.OneTimeSecretID)
	require.NoError(t, err)
	require.NotNil(t, ott)

	exReq := &structs.OneTimeTokenExchangeRequest{
		OneTimeSecretID: result.OneTimeSecretID,
		WriteRequest: structs.WriteRequest{
			Region: "global", // note: not authenticated!
		}}
	var exResp structs.OneTimeTokenExchangeResponse

	// Call the exchange RPC
	err = msgpackrpc.CallWithCodec(codec, "ACL.ExchangeOneTimeToken", exReq, &exResp)
	require.NoError(t, err)
	token := exResp.Token
	require.Equal(t, aclToken.AccessorID, token.AccessorID)
	require.Equal(t, aclToken.SecretID, token.SecretID)

	// Make sure the one-time token is gone
	ott, err = s1.fsm.State().OneTimeTokenBySecret(nil, result.OneTimeSecretID)
	require.NoError(t, err)
	require.Nil(t, ott)

	// directly write the OTT to the state store so that we can write an
	// expired OTT, and query to ensure it's been written
	index := exResp.Index
	index += 10
	ott = &structs.OneTimeToken{
		OneTimeSecretID: uuid.Generate(),
		AccessorID:      token.AccessorID,
		ExpiresAt:       time.Now().Add(-1 * time.Minute),
	}

	err = s1.fsm.State().UpsertOneTimeToken(structs.MsgTypeTestSetup, index, ott)
	require.NoError(t, err)
	ott, err = s1.fsm.State().OneTimeTokenBySecret(nil, ott.OneTimeSecretID)
	require.NoError(t, err)
	require.NotNil(t, ott)

	// Call the exchange RPC; we should not get an exchange for an expired
	// token
	err = msgpackrpc.CallWithCodec(codec, "ACL.ExchangeOneTimeToken", exReq, &exResp)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	// expired token should be left in place (until GC comes along)
	ott, err = s1.fsm.State().OneTimeTokenBySecret(nil, ott.OneTimeSecretID)
	require.NoError(t, err)
	require.NotNil(t, ott)

	// Call the delete RPC, should fail without proper auth
	expReq := &structs.OneTimeTokenExpireRequest{
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: aclToken.SecretID,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "ACL.ExpireOneTimeTokens",
		expReq, &structs.GenericResponse{})
	require.EqualError(t, err, structs.ErrPermissionDenied.Error(),
		"one-time token garbage collection requires management ACL")

	// should not have caused an expiration either!
	ott, err = s1.fsm.State().OneTimeTokenBySecret(nil, ott.OneTimeSecretID)
	require.NoError(t, err)
	require.NotNil(t, ott)

	// Call with correct permissions
	expReq.WriteRequest.AuthToken = root.SecretID
	err = msgpackrpc.CallWithCodec(codec, "ACL.ExpireOneTimeTokens",
		expReq, &structs.GenericResponse{})
	require.NoError(t, err)

	// Now the expired OTT should be gone
	ott, err = s1.fsm.State().OneTimeTokenBySecret(nil, result.OneTimeSecretID)
	require.NoError(t, err)
	require.Nil(t, ott)
}

func TestACL_UpsertRoles(t *testing.T) {
	ci.Parallel(t)

	testServer, aclRootToken, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	// Create a mock ACL role and remove the ID so this looks like a creation.
	aclRole1 := mock.ACLRole()
	aclRole1.ID = ""

	// Attempt to upsert this role without setting an ACL token. This should
	// fail.
	aclRoleReq1 := &structs.ACLRolesUpsertRequest{
		ACLRoles: []*structs.ACLRole{aclRole1},
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}
	var aclRoleResp1 structs.ACLRolesUpsertResponse
	err := msgpackrpc.CallWithCodec(codec, structs.ACLUpsertRolesRPCMethod, aclRoleReq1, &aclRoleResp1)
	require.ErrorContains(t, err, "Permission denied")

	// Attempt to upsert this role again, this time setting the ACL root token.
	// This should fail because the linked policies do not exist within state.
	aclRoleReq2 := &structs.ACLRolesUpsertRequest{
		ACLRoles: []*structs.ACLRole{aclRole1},
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp2 structs.ACLRolesUpsertResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLUpsertRolesRPCMethod, aclRoleReq2, &aclRoleResp2)
	require.ErrorContains(t, err, "cannot find policy")

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testServer.fsm.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Try the upsert a third time, which should succeed.
	aclRoleReq3 := &structs.ACLRolesUpsertRequest{
		ACLRoles: []*structs.ACLRole{aclRole1},
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp3 structs.ACLRolesUpsertResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLUpsertRolesRPCMethod, aclRoleReq3, &aclRoleResp3)
	require.NoError(t, err)
	require.Len(t, aclRoleResp3.ACLRoles, 1)
	require.True(t, aclRole1.Equals(aclRoleResp3.ACLRoles[0]))

	// Perform an update of the ACL role by removing a policy and changing the
	// name.
	aclRole1Copy := aclRole1.Copy()
	aclRole1Copy.Name = "updated-role-name"
	aclRole1Copy.Policies = append(aclRole1Copy.Policies[:1], aclRole1Copy.Policies[1+1:]...)
	aclRole1Copy.SetHash()

	aclRoleReq4 := &structs.ACLRolesUpsertRequest{
		ACLRoles: []*structs.ACLRole{aclRole1Copy},
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp4 structs.ACLRolesUpsertResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLUpsertRolesRPCMethod, aclRoleReq4, &aclRoleResp4)
	require.NoError(t, err)
	require.Len(t, aclRoleResp4.ACLRoles, 1)
	require.True(t, aclRole1Copy.Equals(aclRoleResp4.ACLRoles[0]))
	require.Greater(t, aclRoleResp4.ACLRoles[0].ModifyIndex, aclRoleResp3.ACLRoles[0].ModifyIndex)

	// Create another ACL role that will fail validation. Attempting to upsert
	// this ensures the handler is triggering the validation function.
	aclRole2 := mock.ACLRole()
	aclRole2.Policies = nil

	aclRoleReq5 := &structs.ACLRolesUpsertRequest{
		ACLRoles: []*structs.ACLRole{aclRole2},
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp5 structs.ACLRolesUpsertResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLUpsertRolesRPCMethod, aclRoleReq5, &aclRoleResp5)
	require.Error(t, err)
	require.NotContains(t, err, "Permission denied")

	// Try and create a role with a name that already exists within state.
	aclRole3 := mock.ACLRole()
	aclRole3.ID = ""
	aclRole3.Name = aclRole1.Name

	aclRoleReq6 := &structs.ACLRolesUpsertRequest{
		ACLRoles: []*structs.ACLRole{aclRole3},
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp6 structs.ACLRolesUpsertResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLUpsertRolesRPCMethod, aclRoleReq6, &aclRoleResp6)
	require.ErrorContains(t, err, fmt.Sprintf("role with name %s already exists", aclRole1.Name))
}

func TestACL_DeleteRolesByID(t *testing.T) {
	ci.Parallel(t)

	testServer, aclRootToken, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testServer.fsm.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Create two ACL roles and put these directly into state.
	aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testServer.State().UpsertACLRoles(structs.MsgTypeTestSetup, 10, aclRoles, false))

	// Attempt to delete an ACL role without setting an auth token. This should
	// fail.
	aclRoleReq1 := &structs.ACLRolesDeleteByIDRequest{
		ACLRoleIDs: []string{aclRoles[0].ID},
		WriteRequest: structs.WriteRequest{
			Region: DefaultRegion,
		},
	}
	var aclRoleResp1 structs.ACLRolesDeleteByIDResponse
	err := msgpackrpc.CallWithCodec(codec, structs.ACLDeleteRolesByIDRPCMethod, aclRoleReq1, &aclRoleResp1)
	require.ErrorContains(t, err, "Permission denied")

	// Attempt to delete an ACL role now using a valid management token which
	// should succeed.
	aclRoleReq2 := &structs.ACLRolesDeleteByIDRequest{
		ACLRoleIDs: []string{aclRoles[0].ID},
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp2 structs.ACLRolesDeleteByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLDeleteRolesByIDRPCMethod, aclRoleReq2, &aclRoleResp2)
	require.NoError(t, err)

	// Ensure the deleted role is not found within state and that the other is.
	ws := memdb.NewWatchSet()
	iter, err := testServer.State().GetACLRoles(ws)
	require.NoError(t, err)

	var aclRolesLookup []*structs.ACLRole
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		aclRolesLookup = append(aclRolesLookup, raw.(*structs.ACLRole))
	}

	require.Len(t, aclRolesLookup, 1)
	require.True(t, aclRolesLookup[0].Equals(aclRoles[1]))

	// Try to delete the previously deleted ACL role, this should fail.
	aclRoleReq3 := &structs.ACLRolesDeleteByIDRequest{
		ACLRoleIDs: []string{aclRoles[0].ID},
		WriteRequest: structs.WriteRequest{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp3 structs.ACLRolesDeleteByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLDeleteRolesByIDRPCMethod, aclRoleReq3, &aclRoleResp3)
	require.ErrorContains(t, err, "ACL role not found")
}

func TestACL_ListRoles(t *testing.T) {
	ci.Parallel(t)

	testServer, aclRootToken, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testServer.fsm.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Create two ACL roles with a known prefix and put these directly into
	// state.
	aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	aclRoles[0].ID = "prefix-" + uuid.Generate()
	aclRoles[1].ID = "prefix-" + uuid.Generate()
	require.NoError(t, testServer.State().UpsertACLRoles(structs.MsgTypeTestSetup, 10, aclRoles, false))

	// Try listing roles without a valid ACL token.
	aclRoleReq1 := &structs.ACLRolesListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: uuid.Generate(),
		},
	}
	var aclRoleResp1 structs.ACLRolesListResponse
	err := msgpackrpc.CallWithCodec(codec, structs.ACLListRolesRPCMethod, aclRoleReq1, &aclRoleResp1)
	require.ErrorContains(t, err, "ACL token not found")

	// Try listing roles with a valid ACL token.
	aclRoleReq2 := &structs.ACLRolesListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp2 structs.ACLRolesListResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLListRolesRPCMethod, aclRoleReq2, &aclRoleResp2)
	require.NoError(t, err)
	require.Len(t, aclRoleResp2.ACLRoles, 2)

	// Try listing roles with a valid ACL token using a prefix that doesn't
	// match anything.
	aclRoleReq3 := &structs.ACLRolesListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
			Prefix:    "please",
		},
	}
	var aclRoleResp3 structs.ACLRolesListResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLListRolesRPCMethod, aclRoleReq3, &aclRoleResp3)
	require.NoError(t, err)
	require.Len(t, aclRoleResp3.ACLRoles, 0)

	// Try listing roles with a valid ACL token using a prefix that matches two
	// entries.
	aclRoleReq4 := &structs.ACLRolesListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
			Prefix:    "prefix-",
		},
	}
	var aclRoleResp4 structs.ACLRolesListResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLListRolesRPCMethod, aclRoleReq4, &aclRoleResp4)
	require.NoError(t, err)
	require.Len(t, aclRoleResp4.ACLRoles, 2)

	// Generate and upsert an ACL Token which links to only one of the two
	// roles within state.
	aclToken := mock.ACLToken()
	aclToken.Policies = nil
	aclToken.Roles = []*structs.ACLTokenRoleLink{{ID: aclRoles[1].ID}}

	err = testServer.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 20, []*structs.ACLToken{aclToken})
	require.NoError(t, err)

	aclRoleReq5 := &structs.ACLRolesListRequest{
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclToken.SecretID,
		},
	}
	var aclRoleResp5 structs.ACLRolesListResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLListRolesRPCMethod, aclRoleReq5, &aclRoleResp5)
	require.NoError(t, err)
	require.Len(t, aclRoleResp5.ACLRoles, 1)
	require.Equal(t, aclRoleResp5.ACLRoles[0].ID, aclRoles[1].ID)
	require.Equal(t, aclRoleResp5.ACLRoles[0].Name, aclRoles[1].Name)

	// Now test a blocking query, where we wait for an update to the list which
	// is triggered by a deletion.
	type res struct {
		err   error
		reply *structs.ACLRolesListResponse
	}
	resultCh := make(chan *res)

	go func(resultCh chan *res) {
		aclRoleReq6 := &structs.ACLRolesListRequest{
			QueryOptions: structs.QueryOptions{
				Region:        DefaultRegion,
				AuthToken:     aclRootToken.SecretID,
				MinQueryIndex: aclRoleResp4.Index,
				MaxQueryTime:  10 * time.Second,
			},
		}
		var aclRoleResp6 structs.ACLRolesListResponse
		err = msgpackrpc.CallWithCodec(codec, structs.ACLListRolesRPCMethod, aclRoleReq6, &aclRoleResp6)
		resultCh <- &res{err: err, reply: &aclRoleResp6}
	}(resultCh)

	// Delete an ACL role from state which should return the blocking query.
	require.NoError(t, testServer.fsm.State().DeleteACLRolesByID(
		structs.MsgTypeTestSetup, aclRoleResp4.Index+10, []string{aclRoles[0].ID}))

	// Wait until the test within the routine is complete.
	result := <-resultCh
	require.NoError(t, result.err)
	require.Len(t, result.reply.ACLRoles, 1)
	require.NotEqual(t, result.reply.ACLRoles[0].ID, aclRoles[0].ID)
}

func TestACL_GetRolesByID(t *testing.T) {
	ci.Parallel(t)

	testServer, aclRootToken, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	// Try reading a role without setting a correct auth token.
	aclRoleReq1 := &structs.ACLRolesByIDRequest{
		ACLRoleIDs: []string{"nope"},
		QueryOptions: structs.QueryOptions{
			Region: DefaultRegion,
		},
	}
	var aclRoleResp1 structs.ACLRolesByIDResponse
	err := msgpackrpc.CallWithCodec(codec, structs.ACLGetRolesByIDRPCMethod, aclRoleReq1, &aclRoleResp1)
	require.ErrorContains(t, err, "Permission denied")
	require.Empty(t, aclRoleResp1.ACLRoles)

	// Try reading a role that doesn't exist.
	aclRoleReq2 := &structs.ACLRolesByIDRequest{
		ACLRoleIDs: []string{"nope"},
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp2 structs.ACLRolesByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRolesByIDRPCMethod, aclRoleReq2, &aclRoleResp2)
	require.NoError(t, err)
	require.Empty(t, aclRoleResp2.ACLRoles)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testServer.fsm.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Create two ACL roles and put these directly into state.
	aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testServer.State().UpsertACLRoles(structs.MsgTypeTestSetup, 20, aclRoles, false))

	// Try reading both roles that are within state.
	aclRoleReq3 := &structs.ACLRolesByIDRequest{
		ACLRoleIDs: []string{aclRoles[0].ID, aclRoles[1].ID},
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp3 structs.ACLRolesByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRolesByIDRPCMethod, aclRoleReq3, &aclRoleResp3)
	require.NoError(t, err)
	require.Len(t, aclRoleResp3.ACLRoles, 2)
	require.Contains(t, aclRoleResp3.ACLRoles, aclRoles[0].ID)
	require.Contains(t, aclRoleResp3.ACLRoles, aclRoles[1].ID)

	// Create a client token which allows us to test client tokens looking up
	// their own role assignments.
	clientToken1 := &structs.ACLToken{
		AccessorID: uuid.Generate(),
		SecretID:   uuid.Generate(),
		Name:       "acl-endpoint-test-role",
		Type:       structs.ACLClientToken,
		Roles:      []*structs.ACLTokenRoleLink{{ID: aclRoles[0].ID}},
	}
	clientToken1.SetHash()

	require.NoError(t, testServer.fsm.State().UpsertACLTokens(
		structs.MsgTypeTestSetup, 10, []*structs.ACLToken{clientToken1}))

	// Use the client token in an attempt to look up an ACL role which is
	// assigned to the token, and therefore should work.
	aclRoleReq4 := &structs.ACLRolesByIDRequest{
		ACLRoleIDs: []string{aclRoles[0].ID},
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: clientToken1.SecretID,
		},
	}
	var aclRoleResp4 structs.ACLRolesByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRolesByIDRPCMethod, aclRoleReq4, &aclRoleResp4)
	require.NoError(t, err)
	require.Len(t, aclRoleResp4.ACLRoles, 1)
	require.Contains(t, aclRoleResp4.ACLRoles, aclRoles[0].ID)

	// Use the client token in an attempt to look up an ACL role which is NOT
	// assigned to the token which should fail.
	aclRoleReq5 := &structs.ACLRolesByIDRequest{
		ACLRoleIDs: []string{aclRoles[1].ID},
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: clientToken1.SecretID,
		},
	}
	var aclRoleResp5 structs.ACLRolesByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRolesByIDRPCMethod, aclRoleReq5, &aclRoleResp5)
	require.ErrorContains(t, err, "Permission denied")

	// Now test a blocking query, where we wait for an update to the set which
	// is triggered by a deletion.
	type res struct {
		err   error
		reply *structs.ACLRolesByIDResponse
	}
	resultCh := make(chan *res)

	go func(resultCh chan *res) {
		aclRoleReq6 := &structs.ACLRolesByIDRequest{
			ACLRoleIDs: []string{aclRoles[0].ID, aclRoles[1].ID},
			QueryOptions: structs.QueryOptions{
				Region:        DefaultRegion,
				AuthToken:     aclRootToken.SecretID,
				MinQueryIndex: aclRoleResp3.Index,
				MaxQueryTime:  10 * time.Second,
			},
		}
		var aclRoleResp6 structs.ACLRolesByIDResponse
		err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRolesByIDRPCMethod, aclRoleReq6, &aclRoleResp6)
		resultCh <- &res{err: err, reply: &aclRoleResp6}
	}(resultCh)

	// Delete an ACL role from state which should return the blocking query.
	require.NoError(t, testServer.fsm.State().DeleteACLRolesByID(
		structs.MsgTypeTestSetup, aclRoleResp3.Index+10, []string{aclRoles[0].ID}))

	// Wait for the result and then test it.
	result := <-resultCh
	require.NoError(t, result.err)
	require.Len(t, result.reply.ACLRoles, 1)
	_, ok := result.reply.ACLRoles[aclRoles[1].ID]
	require.True(t, ok)
}

func TestACL_GetRoleByID(t *testing.T) {
	ci.Parallel(t)

	testServer, aclRootToken, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testServer.fsm.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Create two ACL roles and put these directly into state.
	aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testServer.State().UpsertACLRoles(structs.MsgTypeTestSetup, 10, aclRoles, false))

	// Try reading a role without setting a correct auth token.
	aclRoleReq1 := &structs.ACLRoleByIDRequest{
		QueryOptions: structs.QueryOptions{
			Region: DefaultRegion,
		},
	}
	var aclRoleResp1 structs.ACLRoleByIDResponse
	err := msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByIDRPCMethod, aclRoleReq1, &aclRoleResp1)
	require.ErrorContains(t, err, "Permission denied")

	// Try reading a role that doesn't exist.
	aclRoleReq2 := &structs.ACLRoleByIDRequest{
		RoleID: "nope",
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp2 structs.ACLRoleByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByIDRPCMethod, aclRoleReq2, &aclRoleResp2)
	require.NoError(t, err)
	require.Nil(t, aclRoleResp2.ACLRole)

	// Read both our available ACL roles using a valid auth token.
	aclRoleReq3 := &structs.ACLRoleByIDRequest{
		RoleID: aclRoles[0].ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp3 structs.ACLRoleByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByIDRPCMethod, aclRoleReq3, &aclRoleResp3)
	require.NoError(t, err)
	require.True(t, aclRoleResp3.ACLRole.Equals(aclRoles[0]))

	aclRoleReq4 := &structs.ACLRoleByIDRequest{
		RoleID: aclRoles[1].ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp4 structs.ACLRoleByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByIDRPCMethod, aclRoleReq4, &aclRoleResp4)
	require.NoError(t, err)
	require.True(t, aclRoleResp4.ACLRole.Equals(aclRoles[1]))

	// Generate and upsert an ACL Token which links to only one of the two
	// roles within state.
	aclToken := mock.ACLToken()
	aclToken.Policies = nil
	aclToken.Roles = []*structs.ACLTokenRoleLink{{ID: aclRoles[1].ID}}

	err = testServer.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 20, []*structs.ACLToken{aclToken})
	require.NoError(t, err)

	// Try detailing the role that is tried to our ACL token.
	aclRoleReq5 := &structs.ACLRoleByIDRequest{
		RoleID: aclRoles[1].ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclToken.SecretID,
		},
	}
	var aclRoleResp5 structs.ACLRoleByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByIDRPCMethod, aclRoleReq5, &aclRoleResp5)
	require.NoError(t, err)
	require.NotNil(t, aclRoleResp5.ACLRole)
	require.Equal(t, aclRoleResp5.ACLRole.ID, aclRoles[1].ID)

	// Try detailing the role that is NOT tried to our ACL token.
	aclRoleReq6 := &structs.ACLRoleByIDRequest{
		RoleID: aclRoles[0].ID,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclToken.SecretID,
		},
	}
	var aclRoleResp6 structs.ACLRoleByIDResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByIDRPCMethod, aclRoleReq6, &aclRoleResp6)
	require.ErrorContains(t, err, "Permission denied")
}

func TestACL_GetRoleByName(t *testing.T) {
	ci.Parallel(t)

	testServer, aclRootToken, testServerCleanupFn := TestACLServer(t, nil)
	defer testServerCleanupFn()
	codec := rpcClient(t, testServer)
	testutil.WaitForLeader(t, testServer.RPC)

	// Create the policies our ACL roles wants to link to.
	policy1 := mock.ACLPolicy()
	policy1.Name = "mocked-test-policy-1"
	policy2 := mock.ACLPolicy()
	policy2.Name = "mocked-test-policy-2"

	require.NoError(t, testServer.fsm.State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{policy1, policy2}))

	// Create two ACL roles and put these directly into state.
	aclRoles := []*structs.ACLRole{mock.ACLRole(), mock.ACLRole()}
	require.NoError(t, testServer.State().UpsertACLRoles(structs.MsgTypeTestSetup, 10, aclRoles, false))

	// Try reading a role without setting a correct auth token.
	aclRoleReq1 := &structs.ACLRoleByNameRequest{
		QueryOptions: structs.QueryOptions{
			Region: DefaultRegion,
		},
	}
	var aclRoleResp1 structs.ACLRoleByNameResponse
	err := msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByNameRPCMethod, aclRoleReq1, &aclRoleResp1)
	require.ErrorContains(t, err, "Permission denied")

	// Try reading a role that doesn't exist.
	aclRoleReq2 := &structs.ACLRoleByNameRequest{
		RoleName: "nope",
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp2 structs.ACLRoleByNameResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByNameRPCMethod, aclRoleReq2, &aclRoleResp2)
	require.NoError(t, err)
	require.Nil(t, aclRoleResp2.ACLRole)

	// Read both our available ACL roles using a valid auth token.
	aclRoleReq3 := &structs.ACLRoleByNameRequest{
		RoleName: aclRoles[0].Name,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp3 structs.ACLRoleByNameResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByNameRPCMethod, aclRoleReq3, &aclRoleResp3)
	require.NoError(t, err)
	require.True(t, aclRoleResp3.ACLRole.Equals(aclRoles[0]))

	aclRoleReq4 := &structs.ACLRoleByNameRequest{
		RoleName: aclRoles[1].Name,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclRootToken.SecretID,
		},
	}
	var aclRoleResp4 structs.ACLRoleByNameResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByNameRPCMethod, aclRoleReq4, &aclRoleResp4)
	require.NoError(t, err)
	require.True(t, aclRoleResp4.ACLRole.Equals(aclRoles[1]))

	// Generate and upsert an ACL Token which links to only one of the two
	// roles within state.
	aclToken := mock.ACLToken()
	aclToken.Policies = nil
	aclToken.Roles = []*structs.ACLTokenRoleLink{{ID: aclRoles[1].ID}}

	err = testServer.fsm.State().UpsertACLTokens(structs.MsgTypeTestSetup, 20, []*structs.ACLToken{aclToken})
	require.NoError(t, err)

	// Try detailing the role that is tried to our ACL token.
	aclRoleReq5 := &structs.ACLRoleByNameRequest{
		RoleName: aclRoles[1].Name,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclToken.SecretID,
		},
	}
	var aclRoleResp5 structs.ACLRoleByNameResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByNameRPCMethod, aclRoleReq5, &aclRoleResp5)
	require.NoError(t, err)
	require.NotNil(t, aclRoleResp5.ACLRole)
	require.Equal(t, aclRoleResp5.ACLRole.ID, aclRoles[1].ID)
	require.Equal(t, aclRoleResp5.ACLRole.Name, aclRoles[1].Name)

	// Try detailing the role that is NOT tried to our ACL token.
	aclRoleReq6 := &structs.ACLRoleByNameRequest{
		RoleName: aclRoles[0].Name,
		QueryOptions: structs.QueryOptions{
			Region:    DefaultRegion,
			AuthToken: aclToken.SecretID,
		},
	}
	var aclRoleResp6 structs.ACLRoleByNameResponse
	err = msgpackrpc.CallWithCodec(codec, structs.ACLGetRoleByNameRPCMethod, aclRoleReq6, &aclRoleResp6)
	require.ErrorContains(t, err, "Permission denied")
}
