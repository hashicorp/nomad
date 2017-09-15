package nomad

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestACLEndpoint_GetPolicy(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{policy})

	// Lookup the policy
	get := &structs.ACLPolicySpecificRequest{
		Name: policy.Name,
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.SingleACLPolicyResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, policy, resp.Policy)

	// Lookup non-existing policy
	get.Name = structs.GenerateUUID()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicy", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Nil(t, resp.Policy)
}

func TestACLEndpoint_GetPolicy_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policies
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()

	// First create an unrelated policy
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLPolicies(100, []*structs.ACLPolicy{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the policy we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLPolicies(200, []*structs.ACLPolicy{p2})
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
			SecretID:      root.SecretID,
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
		err := state.DeleteACLPolicies(300, []string{p2.Name})
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
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{policy, policy2})

	// Lookup the policy
	get := &structs.ACLPolicySetRequest{
		Names: []string{policy.Name, policy2.Name},
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
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
	get.Names = []string{structs.GenerateUUID()}
	resp = structs.ACLPolicySetResponse{}
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 0, len(resp.Policies))
}

func TestACLEndpoint_GetPolicies_TokenSubset(t *testing.T) {
	t.Parallel()
	s1, _ := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{policy, policy2})

	token := mock.ACLToken()
	token.Policies = []string{policy.Name}
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{token})

	// Lookup the policy which is a subset of our tokens
	get := &structs.ACLPolicySetRequest{
		Names: []string{policy.Name},
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: token.SecretID,
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
}

func TestACLEndpoint_GetPolicies_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policies
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()

	// First create an unrelated policy
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLPolicies(100, []*structs.ACLPolicy{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the policy we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLPolicies(200, []*structs.ACLPolicy{p2})
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
			SecretID:      root.SecretID,
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
		err := state.DeleteACLPolicies(300, []string{p2.Name})
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
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	p2 := mock.ACLPolicy()

	p1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	p2.Name = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{p1, p2})

	// Lookup the policies
	get := &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 2, len(resp.Policies))

	// Lookup the policies by prefix
	get = &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			Prefix:   "aaaabb",
			SecretID: root.SecretID,
		},
	}
	var resp2 structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp2.Index)
	assert.Equal(t, 1, len(resp2.Policies))
}

func TestACLEndpoint_ListPolicies_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policy
	policy := mock.ACLPolicy()

	// Upsert eval triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertACLPolicies(2, []*structs.ACLPolicy{policy}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.ACLPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1,
			SecretID:      root.SecretID,
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
		if err := state.DeleteACLPolicies(3, []string{policy.Name}); err != nil {
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
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{p1})

	// Lookup the policies
	req := &structs.ACLPolicyDeleteRequest{
		Names: []string{p1.Name},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.DeletePolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
}

func TestACLEndpoint_UpsertPolicies(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()

	// Lookup the policies
	req := &structs.ACLPolicyUpsertRequest{
		Policies: []*structs.ACLPolicy{p1},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
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
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLPolicy()
	p1.Rules = "blah blah invalid"

	// Lookup the policies
	req := &structs.ACLPolicyUpsertRequest{
		Policies: []*structs.ACLPolicy{p1},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertPolicies", req, &resp)
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("bad: %s", err)
	}
}

func TestACLEndpoint_GetToken(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	token := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{token})

	// Lookup the token
	get := &structs.ACLTokenSpecificRequest{
		AccessorID: token.AccessorID,
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.SingleACLTokenResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, token, resp.Token)

	// Lookup non-existing token
	get.AccessorID = structs.GenerateUUID()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Nil(t, resp.Token)
}

func TestACLEndpoint_GetToken_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the tokens
	p1 := mock.ACLToken()
	p2 := mock.ACLToken()

	// First create an unrelated token
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLTokens(100, []*structs.ACLToken{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the token we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLTokens(200, []*structs.ACLToken{p2})
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
			SecretID:      root.SecretID,
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
		err := state.DeleteACLTokens(300, []string{p2.AccessorID})
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
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	token := mock.ACLToken()
	token2 := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{token, token2})

	// Lookup the token
	get := &structs.ACLTokenSetRequest{
		AccessorIDS: []string{token.AccessorID, token2.AccessorID},
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
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
	get.AccessorIDS = []string{structs.GenerateUUID()}
	resp = structs.ACLTokenSetResponse{}
	if err := msgpackrpc.CallWithCodec(codec, "ACL.GetTokens", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 0, len(resp.Tokens))
}

func TestACLEndpoint_GetTokens_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the tokens
	p1 := mock.ACLToken()
	p2 := mock.ACLToken()

	// First create an unrelated token
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertACLTokens(100, []*structs.ACLToken{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the token we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertACLTokens(200, []*structs.ACLToken{p2})
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
			SecretID:      root.SecretID,
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
		err := state.DeleteACLTokens(300, []string{p2.AccessorID})
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
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	p2 := mock.ACLToken()
	p2.Global = true

	p1.AccessorID = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	p2.AccessorID = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{p1, p2})

	// Lookup the tokens
	get := &structs.ACLTokenListRequest{
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
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
			Region:   "global",
			Prefix:   "aaaabb",
			SecretID: root.SecretID,
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
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp3 structs.ACLTokenListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListTokens", get, &resp3); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp3.Index)
	assert.Equal(t, 2, len(resp3.Tokens))
}

func TestACLEndpoint_ListTokens_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the token
	token := mock.ACLToken()

	// Upsert eval triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertACLTokens(3, []*structs.ACLToken{token}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.ACLTokenListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 2,
			SecretID:      root.SecretID,
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
		if err := state.DeleteACLTokens(4, []string{token.AccessorID}); err != nil {
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
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{p1})

	// Lookup the tokens
	req := &structs.ACLTokenDeleteRequest{
		AccessorIDs: []string{p1.AccessorID},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.DeleteTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
}

func TestACLEndpoint_Bootstrap(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, func(c *Config) {
		c.ACLEnabled = true
	})
	defer s1.Shutdown()
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

func TestACLEndpoint_Bootstrap_Reset(t *testing.T) {
	t.Parallel()
	dir := tmpDir(t)
	defer os.RemoveAll(dir)
	s1 := testServer(t, func(c *Config) {
		c.ACLEnabled = true
		c.DataDir = dir
		c.DevMode = false
		c.Bootstrap = true
		c.DevDisableBootstrap = false
	})
	defer s1.Shutdown()
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
	assert.Nil(t, ioutil.WriteFile(path, output, 0755))

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
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	p1.AccessorID = "" // Blank to create

	// Lookup the tokens
	req := &structs.ACLTokenUpsertRequest{
		Tokens: []*structs.ACLToken{p1},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.ACLTokenUpsertResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)

	// Get the token out from the response
	created := resp.Tokens[0]
	assert.NotEqual(t, "", created.AccessorID)
	assert.NotEqual(t, "", created.SecretID)
	assert.NotEqual(t, time.Time{}, created.CreateTime)
	assert.Equal(t, p1.Type, created.Type)
	assert.Equal(t, p1.Policies, created.Policies)
	assert.Equal(t, p1.Name, created.Name)

	// Check we created the token
	out, err := s1.fsm.State().ACLTokenByAccessorID(nil, created.AccessorID)
	assert.Nil(t, err)
	assert.Equal(t, created, out)

	// Update the token type
	req.Tokens[0] = created
	created.Type = "management"
	created.Policies = nil

	// Upsert again
	if err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertTokens", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)

	// Check we modified the token
	out, err = s1.fsm.State().ACLTokenByAccessorID(nil, created.AccessorID)
	assert.Nil(t, err)
	assert.Equal(t, created, out)
}

func TestACLEndpoint_UpsertTokens_Invalid(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.ACLToken()
	p1.Type = "blah blah"

	// Lookup the tokens
	req := &structs.ACLTokenUpsertRequest{
		Tokens: []*structs.ACLToken{p1},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ACL.UpsertTokens", req, &resp)
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "client or management") {
		t.Fatalf("bad: %s", err)
	}
}

func TestACLEndpoint_ResolveToken(t *testing.T) {
	t.Parallel()
	s1, _ := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	token := mock.ACLToken()
	s1.fsm.State().UpsertACLTokens(1000, []*structs.ACLToken{token})

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
	get.SecretID = structs.GenerateUUID()
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ResolveToken", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Nil(t, resp.Token)
}
