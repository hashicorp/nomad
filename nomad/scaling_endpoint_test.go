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
