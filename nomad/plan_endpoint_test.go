// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"sync"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestPlanEndpoint_Submit(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForKeyring(t, s1.RPC, s1.Region())

	// Create the register request
	eval1 := mock.Eval()
	s1.evalBroker.Enqueue(eval1)

	evalOut, token, err := s1.evalBroker.Dequeue([]string{eval1.Type}, time.Second)
	must.NoError(t, err)
	must.Eq(t, eval1, evalOut)

	// Submit a plan
	plan := mock.Plan()
	plan.EvalID = eval1.ID
	plan.EvalToken = token
	job := mock.Job()
	plan.JobInfo = &structs.PlanJobTuple{
		Namespace: job.Namespace,
		ID:        job.ID,
	}
	req := &structs.PlanRequest{
		Plan:         plan,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.PlanResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Plan.Submit", req, &resp))
	must.NotNil(t, resp.Result)
}

// TestPlanEndpoint_Submit_Bad asserts that the Plan.Submit endpoint rejects
// bad data with an error instead of panicking.
func TestPlanEndpoint_Submit_Bad(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForKeyring(t, s1.RPC, s1.Region())

	// Mock a valid eval being dequeued by a worker
	eval := mock.Eval()
	s1.evalBroker.Enqueue(eval)

	evalOut, _, err := s1.evalBroker.Dequeue([]string{eval.Type}, time.Second)
	must.NoError(t, err)
	must.Eq(t, eval, evalOut)

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
			must.EqError(t, err, tc.Err)
			must.Nil(t, resp.Result)
		})
	}

	// Ensure no plans were enqueued
	must.Zero(t, s1.planner.planQueue.Stats().Depth)
}

func TestPlanEndpoint_ApplyConcurrent(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanupS1()
	testutil.WaitForKeyring(t, s1.RPC, s1.Region())

	plans := []*structs.Plan{}

	for range 5 {
		// Create a node to place on
		node := mock.Node()
		store := s1.fsm.State()
		must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 100, node))

		// Create the eval
		eval1 := mock.Eval()
		s1.evalBroker.Enqueue(eval1)
		must.NoError(t, store.UpsertEvals(
			structs.MsgTypeTestSetup, 150, []*structs.Evaluation{eval1}))

		evalOut, token, err := s1.evalBroker.Dequeue([]string{eval1.Type}, time.Second)
		must.NoError(t, err)
		must.Eq(t, eval1, evalOut)

		// Submit a plan
		plan := mock.Plan()
		plan.EvalID = eval1.ID
		plan.EvalToken = token
		job := mock.Job()
		must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
		plan.JobInfo = &structs.PlanJobTuple{
			Namespace: job.Namespace,
			ID:        job.ID,
		}

		alloc := mock.Alloc()
		alloc.JobID = job.ID
		alloc.Job = job

		plan.NodeAllocation = map[string][]*structs.Allocation{node.ID: {alloc}}

		plans = append(plans, plan)
	}

	var wg sync.WaitGroup
	for _, plan := range plans {
		wg.Go(func() {
			req := &structs.PlanRequest{
				Plan:         plan,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			var resp structs.PlanResponse
			err := s1.RPC("Plan.Submit", req, &resp)
			must.NoError(t, err)
			must.NotNil(t, resp.Result, must.Sprint("missing result"))
		})
	}

	wg.Wait()
}

func TestPlanEndpoint_Submit_FullJobAndJobInfo(t *testing.T) {
	ci.Parallel(t)

	s1, cleanup := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	defer cleanup()
	codec := rpcClient(t, s1)
	testutil.WaitForKeyring(t, s1.RPC, s1.Region())

	store := s1.fsm.State()

	cases := []struct {
		Name        string
		ProvideFull bool
	}{
		{"FullJob", true},
		{"JobInfo", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			eval := mock.Eval()
			s1.evalBroker.Enqueue(eval)
			must.NoError(t, store.UpsertEvals(structs.MsgTypeTestSetup, 100, []*structs.Evaluation{eval}))

			evalOut, token, err := s1.evalBroker.Dequeue([]string{eval.Type}, time.Second)
			must.NoError(t, err)
			must.Eq(t, eval, evalOut)

			// Ensure a job and node exist in state for the planner to use.
			job := mock.Job()
			must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
			node := mock.Node()
			must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 100, node))

			plan := mock.Plan()
			plan.EvalID = eval.ID
			plan.EvalToken = token

			if tc.ProvideFull {
				plan.Job = job
				alloc := mock.Alloc()
				alloc.JobID = job.ID
				alloc.Job = job
				plan.NodeAllocation = map[string][]*structs.Allocation{node.ID: {alloc}}
			} else {
				plan.Job = nil
				plan.JobInfo = &structs.PlanJobTuple{
					Namespace: job.Namespace,
					ID:        job.ID,
				}
				alloc := mock.Alloc()
				alloc.JobID = job.ID
				plan.NodeAllocation = map[string][]*structs.Allocation{node.ID: {alloc}}
			}

			req := &structs.PlanRequest{
				Plan:         plan,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			var resp structs.PlanResponse
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "Plan.Submit", req, &resp))
			must.NotNil(t, resp.Result)
		})
	}
}
