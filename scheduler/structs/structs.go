// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper"
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

// NewPlanWithStateAndIndex is used in the testing harness
func NewPlanWithStateAndIndex(state *state.StateStore, nextIndex uint64, serversMeetMinimumVersion bool) *PlanBuilder {
	return &PlanBuilder{State: state, nextIndex: nextIndex, serversMeetMinimumVersion: serversMeetMinimumVersion}
}

// PlanBuilder is used to create plans outside the usual scheduler worker flow,
// such as testing, recalculating queued allocs during snapshot restore in the
// FSM, or the online plans created in the Job.Plan RPC
type PlanBuilder struct {
	State *state.StateStore

	Planner  Planner
	planLock sync.Mutex

	Plans        []*structs.Plan
	Evals        []*structs.Evaluation
	CreateEvals  []*structs.Evaluation
	ReblockEvals []*structs.Evaluation

	nextIndex     uint64
	nextIndexLock sync.Mutex

	serversMeetMinimumVersion bool

	// don't actually write plans back to state
	noSubmit bool
}

// SubmitPlan is used to handle plan submission
func (p *PlanBuilder) SubmitPlan(plan *structs.Plan) (*structs.PlanResult, State, error) {
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

	now := time.Now().UTC().UnixNano()

	allocsUpdated := make([]*structs.Allocation, 0, len(result.NodeAllocation))
	for _, allocList := range plan.NodeAllocation {
		allocsUpdated = append(allocsUpdated, allocList...)
	}
	updateCreateTimestamp(allocsUpdated, now)

	snap, _ := p.State.Snapshot()

	// pull the job from the state
	job, err := snap.JobByID(nil, plan.JobTuple.Namespace, plan.JobTuple.JobID)
	if err != nil {
		return result, nil, err
	}

	if job == nil {
		return result, nil, fmt.Errorf("unable to find job ID %s in the state", plan.JobTuple.JobID)
	}

	// make sure these are denormalized the same way they would be in the real
	// plan applier
	allocsStopped := make([]*structs.AllocationDiff, 0, len(result.NodeUpdate))
	for _, updateList := range plan.NodeUpdate {
		stopped, _ := snap.DenormalizeAllocationSlice(updateList)
		allocsStopped = append(allocsStopped, helper.ConvertSlice(stopped,
			func(a *structs.Allocation) *structs.AllocationDiff { return a.AllocationDiff() })...)

	}

	// make sure these are denormalized the same way they would be in the real
	// plan applier
	allocsPreempted := make([]*structs.AllocationDiff, 0, len(result.NodePreemptions))
	for _, preemptionList := range result.NodePreemptions {
		preemptions, _ := snap.DenormalizeAllocationSlice(preemptionList)
		allocsPreempted = append(allocsPreempted, helper.ConvertSlice(preemptions,
			func(a *structs.Allocation) *structs.AllocationDiff { return a.AllocationDiff() })...)
	}

	// Setup the update request
	req := structs.ApplyPlanResultsRequest{
		AllocsStopped:     allocsStopped,
		AllocsUpdated:     allocsUpdated,
		AllocsPreempted:   allocsPreempted,
		Job:               job,
		Deployment:        plan.Deployment,
		DeploymentUpdates: plan.DeploymentUpdates,
		EvalID:            plan.EvalID,
	}

	if p.noSubmit {
		return result, nil, nil
	}

	// Apply the full plan
	err = p.State.UpsertPlanResults(structs.MsgTypeTestSetup, index, &req)
	return result, nil, err
}

func updateCreateTimestamp(allocations []*structs.Allocation, now int64) {
	// Set the time the alloc was applied for the first time. This can be used
	// to approximate the scheduling time.
	for _, alloc := range allocations {
		if alloc.CreateTime == 0 {
			alloc.CreateTime = now
		}
		alloc.ModifyTime = now
	}
}

func (p *PlanBuilder) SetNoSubmit() {
	p.noSubmit = true
}

func (p *PlanBuilder) UpdateEval(eval *structs.Evaluation) error {
	// Ensure sequential plan application
	p.planLock.Lock()
	defer p.planLock.Unlock()

	// Store the eval
	p.Evals = append(p.Evals, eval)

	return nil
}

func (p *PlanBuilder) CreateEval(eval *structs.Evaluation) error {
	// Ensure sequential plan application
	p.planLock.Lock()
	defer p.planLock.Unlock()

	// Store the eval
	p.CreateEvals = append(p.CreateEvals, eval)

	return nil
}

func (p *PlanBuilder) ReblockEval(eval *structs.Evaluation) error {
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

func (p *PlanBuilder) ServersMeetMinimumVersion(_ *version.Version, _ bool) bool {
	return p.serversMeetMinimumVersion
}

// NextIndex returns the next index
func (p *PlanBuilder) NextIndex() uint64 {
	p.nextIndexLock.Lock()
	defer p.nextIndexLock.Unlock()
	idx := p.nextIndex
	p.nextIndex += 1
	return idx
}
