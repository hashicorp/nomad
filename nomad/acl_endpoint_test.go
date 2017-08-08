package nomad

import (
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.ACLPolicy()
	s1.fsm.State().UpsertACLPolicies(1000, []*structs.ACLPolicy{policy})

	// Lookup the policy
	get := &structs.ACLPolicySpecificRequest{
		Name:         policy.Name,
		QueryOptions: structs.QueryOptions{Region: "global"},
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
	s1 := testServer(t, nil)
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

func TestACLEndpoint_List(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
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
		QueryOptions: structs.QueryOptions{Region: "global"},
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
			Region: "global",
			Prefix: "aaaabb",
		},
	}
	var resp2 structs.ACLPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "ACL.ListPolicies", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp2.Index)
	assert.Equal(t, 1, len(resp2.Policies))
}

func TestACLEndpoint_List_Blocking(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
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
