package nomad

import (
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestPlanEndpoint_Submit(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	evalOut, token, err := s1.evalBroker.Dequeue([]string{eval1.Type}, time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if evalOut != eval1 {
		t.Fatalf("Bad eval")
	}

	// Submit a plan
	plan := mock.Plan()
	plan.EvalID = eval1.ID
	plan.EvalToken = token
	plan.Job = mock.Job()
	req := &structs.PlanRequest{
		Plan:         plan,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.PlanResponse
	if err := msgpackrpc.CallWithCodec(codec, "Plan.Submit", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Result == nil {
		t.Fatalf("missing result")
	}
}

// TestPlanEndpoint_Submit_Bad asserts that the Plan.Submit endpoint rejects
// bad data with an error instead of panicking.
func TestPlanEndpoint_Submit_Bad(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Mock a valid eval being dequeued by a worker
	eval := mock.Eval()
	s1.evalBroker.Enqueue(eval)

	evalOut, _, err := s1.evalBroker.Dequeue([]string{eval.Type}, time.Second)
	require.NoError(t, err)
	require.Equal(t, eval, evalOut)

	cases := []struct {
		Name string
		Plan *structs.Plan
		Err  string
	}{
		{
			Name: "Nil",
			Plan: nil,
			Err:  "cannot submit nil plan",
		},
		{
			Name: "Empty",
			Plan: &structs.Plan{},
			Err:  "evaluation is not outstanding",
		},
		{
			Name: "BadEvalID",
			Plan: &structs.Plan{
				EvalID: "1234", // does not exist
			},
			Err: "evaluation is not outstanding",
		},
		{
			Name: "MissingToken",
			Plan: &structs.Plan{
				EvalID: eval.ID,
			},
			Err: "evaluation token does not match",
		},
		{
			Name: "InvalidToken",
			Plan: &structs.Plan{
				EvalID:    eval.ID,
				EvalToken: "1234", // invalid
			},
			Err: "evaluation token does not match",
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			req := &structs.PlanRequest{
				Plan:         tc.Plan,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			var resp structs.PlanResponse
			err := msgpackrpc.CallWithCodec(codec, "Plan.Submit", req, &resp)
			require.EqualError(t, err, tc.Err)
			require.Nil(t, resp.Result)
		})
	}

	// Ensure no plans were enqueued
	require.Zero(t, s1.planner.planQueue.Stats().Depth)
}
