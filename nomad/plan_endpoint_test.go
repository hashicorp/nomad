package nomad

import (
	"testing"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestPlanEndpoint_Submit(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Submit a plan
	plan := mockPlan()
	req := &structs.PlanRequest{
		Plan:         plan,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}
	var resp structs.PlanResponse
	if err := msgpackrpc.CallWithCodec(codec, "Plan.Submit", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Result == nil {
		t.Fatalf("missing result")
	}
}
