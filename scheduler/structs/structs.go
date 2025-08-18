// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// PortCollisionEvent is an event that can happen during scheduling when
// an unexpected port collision is detected.
type PortCollisionEvent struct {
	Reason      string
	Node        *structs.Node
	Allocations []*structs.Allocation

	// TODO: this is a large struct, but may be required to debug unexpected
	// port collisions. Re-evaluate its need in the future if the bug is fixed
	// or not caused by this field.
	NetIndex *structs.NetworkIndex
}

func (ev *PortCollisionEvent) Copy() *PortCollisionEvent {
	if ev == nil {
		return nil
	}
	c := new(PortCollisionEvent)
	*c = *ev
	c.Node = ev.Node.Copy()
	if len(ev.Allocations) > 0 {
		for i, a := range ev.Allocations {
			c.Allocations[i] = a.Copy()
		}

	}
	c.NetIndex = ev.NetIndex.Copy()
	return c
}

func (ev *PortCollisionEvent) Sanitize() *PortCollisionEvent {
	if ev == nil {
		return nil
	}
	clean := ev.Copy()

	clean.Node = ev.Node.Sanitize()
	clean.Node.Meta = make(map[string]string)

	for i, alloc := range ev.Allocations {
		clean.Allocations[i] = alloc.CopySkipJob()
		clean.Allocations[i].Job = nil
	}

	return clean
}

// NewPlanWithSateAndIndex is used in the testing harness
func NewPlanWithSateAndIndex(state *state.StateStore, nextIndex uint64, serversMeetMinimumVersion bool) *Plan {
	return &Plan{State: state, nextIndex: nextIndex, serversMeetMinimumVersion: serversMeetMinimumVersion}
}

// Plan is used to submit plans.
type Plan struct {
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

	// don't actually write plans back to state
	noSubmit bool
}

// SubmitPlan is used to handle plan submission
func (p *Plan) SubmitPlan(plan *structs.Plan) (*structs.PlanResult, State, error) {
	// Ensure sequential plan application
	p.planLock.Lock()
	defer p.planLock.Unlock()

	// Store the plan
	p.Plans = append(p.Plans, plan)

	// Check for custom planner
	if p.Planner != nil {
		return p.Planner.SubmitPlan(plan)
	}

	// Get the index
	index := p.NextIndex()

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

	if p.optimizePlan {
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

	if p.noSubmit {
		return result, nil, nil
	}

	// Apply the full plan
	err := p.State.UpsertPlanResults(structs.MsgTypeTestSetup, index, &req)
	return result, nil, err
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

func (p *Plan) SetNoSubmit() {
	p.noSubmit = true
}

func (p *Plan) UpdateEval(eval *structs.Evaluation) error {
	// Ensure sequential plan application
	p.planLock.Lock()
	defer p.planLock.Unlock()

	// Store the eval
	p.Evals = append(p.Evals, eval)

	return nil
}

func (p *Plan) CreateEval(eval *structs.Evaluation) error {
	// Ensure sequential plan application
	p.planLock.Lock()
	defer p.planLock.Unlock()

	// Store the eval
	p.CreateEvals = append(p.CreateEvals, eval)

	return nil
}

func (p *Plan) ReblockEval(eval *structs.Evaluation) error {
	// Ensure sequential plan application
	p.planLock.Lock()
	defer p.planLock.Unlock()

	// Check that the evaluation was already blocked.
	ws := memdb.NewWatchSet()
	old, err := p.State.EvalByID(ws, eval.ID)
	if err != nil {
		return err
	}

	if old == nil {
		return fmt.Errorf("evaluation does not exist to be reblocked")
	}
	if old.Status != structs.EvalStatusBlocked {
		return fmt.Errorf("evaluation %q is not already in a blocked state", old.ID)
	}

	p.ReblockEvals = append(p.ReblockEvals, eval)
	return nil
}

func (p *Plan) ServersMeetMinimumVersion(_ *version.Version, _ bool) bool {
	return p.serversMeetMinimumVersion
}

// NextIndex returns the next index
func (p *Plan) NextIndex() uint64 {
	p.nextIndexLock.Lock()
	defer p.nextIndexLock.Unlock()
	idx := p.nextIndex
	p.nextIndex += 1
	return idx
}
