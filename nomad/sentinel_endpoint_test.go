package nomad

import (
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestSentinelEndpoint_GetPolicy(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.SentinelPolicy()
	s1.fsm.State().UpsertSentinelPolicies(1000, []*structs.SentinelPolicy{policy})

	// Lookup the policy
	get := &structs.SentinelPolicySpecificRequest{
		Name: policy.Name,
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.SingleSentinelPolicyResponse
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.GetPolicy", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, policy, resp.Policy)

	// Lookup non-existing policy
	get.Name = structs.GenerateUUID()
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.GetPolicy", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Nil(t, resp.Policy)
}

func TestSentinelEndpoint_GetPolicy_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policies
	p1 := mock.SentinelPolicy()
	p2 := mock.SentinelPolicy()

	// First create an unrelated policy
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertSentinelPolicies(100, []*structs.SentinelPolicy{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the policy we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertSentinelPolicies(200, []*structs.SentinelPolicy{p2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the policy
	req := &structs.SentinelPolicySpecificRequest{
		Name: p2.Name,
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
			SecretID:      root.SecretID,
		},
	}
	var resp structs.SingleSentinelPolicyResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.GetPolicy", req, &resp); err != nil {
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
		err := state.DeleteSentinelPolicies(300, []string{p2.Name})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SingleSentinelPolicyResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.GetPolicy", req, &resp2); err != nil {
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

func TestSentinelEndpoint_GetPolicies(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	policy := mock.SentinelPolicy()
	policy2 := mock.SentinelPolicy()
	s1.fsm.State().UpsertSentinelPolicies(1000, []*structs.SentinelPolicy{policy, policy2})

	// Lookup the policy
	get := &structs.SentinelPolicySetRequest{
		Names: []string{policy.Name, policy2.Name},
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.SentinelPolicySetResponse
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.GetPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 2, len(resp.Policies))
	assert.Equal(t, policy, resp.Policies[policy.Name])
	assert.Equal(t, policy2, resp.Policies[policy2.Name])

	// Lookup non-existing policy
	get.Names = []string{structs.GenerateUUID()}
	resp = structs.SentinelPolicySetResponse{}
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.GetPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 0, len(resp.Policies))
}

func TestSentinelEndpoint_GetPolicies_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policies
	p1 := mock.SentinelPolicy()
	p2 := mock.SentinelPolicy()

	// First create an unrelated policy
	time.AfterFunc(100*time.Millisecond, func() {
		err := state.UpsertSentinelPolicies(100, []*structs.SentinelPolicy{p1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Upsert the policy we are watching later
	time.AfterFunc(200*time.Millisecond, func() {
		err := state.UpsertSentinelPolicies(200, []*structs.SentinelPolicy{p2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Lookup the policy
	req := &structs.SentinelPolicySetRequest{
		Names: []string{p2.Name},
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 150,
			SecretID:      root.SecretID,
		},
	}
	var resp structs.SentinelPolicySetResponse
	start := time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.GetPolicies", req, &resp); err != nil {
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
		err := state.DeleteSentinelPolicies(300, []string{p2.Name})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.QueryOptions.MinQueryIndex = 250
	var resp2 structs.SentinelPolicySetResponse
	start = time.Now()
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.GetPolicies", req, &resp2); err != nil {
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

func TestSentinelEndpoint_ListPolicies(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.SentinelPolicy()
	p2 := mock.SentinelPolicy()

	p1.Name = "aaaaaaaa-3350-4b4b-d185-0e1992ed43e9"
	p2.Name = "aaaabbbb-3350-4b4b-d185-0e1992ed43e9"
	s1.fsm.State().UpsertSentinelPolicies(1000, []*structs.SentinelPolicy{p1, p2})

	// Lookup the policies
	get := &structs.SentinelPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.SentinelPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.ListPolicies", get, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp.Index)
	assert.Equal(t, 2, len(resp.Policies))

	// Lookup the policies by prefix
	get = &structs.SentinelPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:   "global",
			Prefix:   "aaaabb",
			SecretID: root.SecretID,
		},
	}
	var resp2 structs.SentinelPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.ListPolicies", get, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, uint64(1000), resp2.Index)
	assert.Equal(t, 1, len(resp2.Policies))
}

func TestSentinelEndpoint_ListPolicies_Blocking(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	state := s1.fsm.State()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the policy
	policy := mock.SentinelPolicy()

	// Upsert eval triggers watches
	time.AfterFunc(100*time.Millisecond, func() {
		if err := state.UpsertSentinelPolicies(2, []*structs.SentinelPolicy{policy}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req := &structs.SentinelPolicyListRequest{
		QueryOptions: structs.QueryOptions{
			Region:        "global",
			MinQueryIndex: 1,
			SecretID:      root.SecretID,
		},
	}
	start := time.Now()
	var resp structs.SentinelPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.ListPolicies", req, &resp); err != nil {
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
		if err := state.DeleteSentinelPolicies(3, []string{policy.Name}); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	req.MinQueryIndex = 2
	start = time.Now()
	var resp2 structs.SentinelPolicyListResponse
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.ListPolicies", req, &resp2); err != nil {
		t.Fatalf("err: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Fatalf("should block (returned in %s) %#v", elapsed, resp2)
	}
	assert.Equal(t, uint64(3), resp2.Index)
	assert.Equal(t, 0, len(resp2.Policies))
}

func TestSentinelEndpoint_DeletePolicies(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.SentinelPolicy()
	s1.fsm.State().UpsertSentinelPolicies(1000, []*structs.SentinelPolicy{p1})

	// Lookup the policies
	req := &structs.SentinelPolicyDeleteRequest{
		Names: []string{p1.Name},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.DeletePolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)
}

func TestSentinelEndpoint_UpsertPolicies(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.SentinelPolicy()

	// Lookup the policies
	req := &structs.SentinelPolicyUpsertRequest{
		Policies: []*structs.SentinelPolicy{p1},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	if err := msgpackrpc.CallWithCodec(codec, "Sentinel.UpsertPolicies", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.NotEqual(t, uint64(0), resp.Index)

	// Check we created the policy
	out, err := s1.fsm.State().SentinelPolicyByName(nil, p1.Name)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestSentinelEndpoint_UpsertPolicies_Invalid(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	p1 := mock.SentinelPolicy()
	p1.EnforcementLevel = "foobar"

	// Lookup the policies
	req := &structs.SentinelPolicyUpsertRequest{
		Policies: []*structs.SentinelPolicy{p1},
		WriteRequest: structs.WriteRequest{
			Region:   "global",
			SecretID: root.SecretID,
		},
	}
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "Sentinel.UpsertPolicies", req, &resp)
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "invalid enforcement level") {
		t.Fatalf("bad: %s", err)
	}
}
