// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// RejectPlan is used to always reject the entire plan and force a state refresh
type RejectPlan struct {
	Harness *Harness
}

func (r *RejectPlan) ServersMeetMinimumVersion(minVersion *version.Version, checkFailedServers bool) bool {
	return r.Harness.serversMeetMinimumVersion
}

func (r *RejectPlan) SubmitPlan(*structs.Plan) (*structs.PlanResult, State, error) {
	result := new(structs.PlanResult)
	result.RefreshIndex = r.Harness.NextIndex()
	return result, r.Harness.State, nil
}

func (r *RejectPlan) UpdateEval(eval *structs.Evaluation) error {
	return nil
}

func (r *RejectPlan) CreateEval(*structs.Evaluation) error {
	return nil
}

func (r *RejectPlan) ReblockEval(*structs.Evaluation) error {
	return nil
}

// Harness is a lightweight testing harness for schedulers. It manages a state
// store copy and provides the planner interface. It can be extended for various
// testing uses or for invoking the scheduler without side effects.
type Harness struct {
	t     testing.TB
	State *state.StateStore

	Planner  Planner
	planLock sync.Mutex

	Plans        []*structs.Plan
	Evals        []*structs.Evaluation
	CreateEvals  []*structs.Evaluation
	ReblockEvals []*structs.Evaluation

	nextIndex     uint64
	nextIndexLock sync.Mutex

	optimizePlan              bool
	serversMeetMinimumVersion bool
}

// NewHarness is used to make a new testing harness
func NewHarness(t testing.TB) *Harness {
	state := state.TestStateStore(t)
	h := &Harness{
		t:                         t,
		State:                     state,
		nextIndex:                 1,
		serversMeetMinimumVersion: true,
	}
	return h
}

// NewHarnessWithState creates a new harness with the given state for testing
// purposes.
func NewHarnessWithState(t testing.TB, state *state.StateStore) *Harness {
	return &Harness{
		t:         t,
		State:     state,
		nextIndex: 1,
	}
}

// SubmitPlan is used to handle plan submission
func (h *Harness) SubmitPlan(plan *structs.Plan) (*structs.PlanResult, State, error) {
	// Ensure sequential plan application
	h.planLock.Lock()
	defer h.planLock.Unlock()

	// Store the plan
	h.Plans = append(h.Plans, plan)

	// Check for custom planner
	if h.Planner != nil {
		return h.Planner.SubmitPlan(plan)
	}

	// Get the index
	index := h.NextIndex()

	// Prepare the result
	result := new(structs.PlanResult)
	result.NodeUpdate = plan.NodeUpdate
	result.NodeAllocation = plan.NodeAllocation
	result.NodePreemptions = plan.NodePreemptions
	result.AllocIndex = index

	// Flatten evicts and allocs
	now := time.Now().UTC().UnixNano()

	allocsUpdated := make([]*structs.Allocation, 0, len(result.NodeAllocation))
	for _, allocList := range plan.NodeAllocation {
		allocsUpdated = append(allocsUpdated, allocList...)
	}
	updateCreateTimestamp(allocsUpdated, now)

	// Setup the update request
	req := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Job: plan.Job,
		},
		Deployment:        plan.Deployment,
		DeploymentUpdates: plan.DeploymentUpdates,
		EvalID:            plan.EvalID,
	}

	if h.optimizePlan {
		stoppedAllocDiffs := make([]*structs.AllocationDiff, 0, len(result.NodeUpdate))
		for _, updateList := range plan.NodeUpdate {
			for _, stoppedAlloc := range updateList {
				stoppedAllocDiffs = append(stoppedAllocDiffs, stoppedAlloc.AllocationDiff())
			}
		}
		req.AllocsStopped = stoppedAllocDiffs

		req.AllocsUpdated = allocsUpdated

		preemptedAllocDiffs := make([]*structs.AllocationDiff, 0, len(result.NodePreemptions))
		for _, preemptions := range plan.NodePreemptions {
			for _, preemptedAlloc := range preemptions {
				allocDiff := preemptedAlloc.AllocationDiff()
				allocDiff.ModifyTime = now
				preemptedAllocDiffs = append(preemptedAllocDiffs, allocDiff)
			}
		}
		req.AllocsPreempted = preemptedAllocDiffs
	} else {
		// COMPAT 0.11: Handles unoptimized log format
		var allocs []*structs.Allocation

		allocsStopped := make([]*structs.Allocation, 0, len(result.NodeUpdate))
		for _, updateList := range plan.NodeUpdate {
			allocsStopped = append(allocsStopped, updateList...)
		}
		allocs = append(allocs, allocsStopped...)

		allocs = append(allocs, allocsUpdated...)
		updateCreateTimestamp(allocs, now)

		req.Alloc = allocs

		// Set modify time for preempted allocs and flatten them
		var preemptedAllocs []*structs.Allocation
		for _, preemptions := range result.NodePreemptions {
			for _, alloc := range preemptions {
				alloc.ModifyTime = now
				preemptedAllocs = append(preemptedAllocs, alloc)
			}
		}

		req.NodePreemptions = preemptedAllocs
	}

	// Apply the full plan
	err := h.State.UpsertPlanResults(structs.MsgTypeTestSetup, index, &req)
	return result, nil, err
}

// OptimizePlan is a function used only for Harness to help set the optimzePlan field,
// since Harness doesn't have access to a Server object
func (h *Harness) OptimizePlan(optimize bool) {
	h.optimizePlan = optimize
}

func updateCreateTimestamp(allocations []*structs.Allocation, now int64) {
	// Set the time the alloc was applied for the first time. This can be used
	// to approximate the scheduling time.
	for _, alloc := range allocations {
		if alloc.CreateTime == 0 {
			alloc.CreateTime = now
		}
	}
}

func (h *Harness) UpdateEval(eval *structs.Evaluation) error {
	// Ensure sequential plan application
	h.planLock.Lock()
	defer h.planLock.Unlock()

	// Store the eval
	h.Evals = append(h.Evals, eval)

	// Check for custom planner
	if h.Planner != nil {
		return h.Planner.UpdateEval(eval)
	}
	return nil
}

func (h *Harness) CreateEval(eval *structs.Evaluation) error {
	// Ensure sequential plan application
	h.planLock.Lock()
	defer h.planLock.Unlock()

	// Store the eval
	h.CreateEvals = append(h.CreateEvals, eval)

	// Check for custom planner
	if h.Planner != nil {
		return h.Planner.CreateEval(eval)
	}
	return nil
}

func (h *Harness) ReblockEval(eval *structs.Evaluation) error {
	// Ensure sequential plan application
	h.planLock.Lock()
	defer h.planLock.Unlock()

	// Check that the evaluation was already blocked.
	ws := memdb.NewWatchSet()
	old, err := h.State.EvalByID(ws, eval.ID)
	if err != nil {
		return err
	}

	if old == nil {
		return fmt.Errorf("evaluation does not exist to be reblocked")
	}
	if old.Status != structs.EvalStatusBlocked {
		return fmt.Errorf("evaluation %q is not already in a blocked state", old.ID)
	}

	h.ReblockEvals = append(h.ReblockEvals, eval)
	return nil
}

func (h *Harness) ServersMeetMinimumVersion(_ *version.Version, _ bool) bool {
	return h.serversMeetMinimumVersion
}

// NextIndex returns the next index
func (h *Harness) NextIndex() uint64 {
	h.nextIndexLock.Lock()
	defer h.nextIndexLock.Unlock()
	idx := h.nextIndex
	h.nextIndex += 1
	return idx
}

// Snapshot is used to snapshot the current state
func (h *Harness) Snapshot() State {
	snap, _ := h.State.Snapshot()
	return snap
}

// Scheduler is used to return a new scheduler from
// a snapshot of current state using the harness for planning.
func (h *Harness) Scheduler(factory Factory) Scheduler {
	logger := testlog.HCLogger(h.t)
	eventsCh := make(chan interface{})

	// Listen for and log events from the scheduler.
	go func() {
		for e := range eventsCh {
			switch event := e.(type) {
			case *PortCollisionEvent:
				h.t.Errorf("unexpected worker eval event: %v", event.Reason)
			}
		}
	}()

	return factory(logger, eventsCh, h.Snapshot(), h)
}

// Process is used to process an evaluation given a factory
// function to create the scheduler
func (h *Harness) Process(factory Factory, eval *structs.Evaluation) error {
	sched := h.Scheduler(factory)
	return sched.Process(eval)
}

func (h *Harness) AssertEvalStatus(t testing.TB, state string) {
	require.Len(t, h.Evals, 1)
	update := h.Evals[0]
	require.Equal(t, state, update.Status)
}
