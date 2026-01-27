// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import "fmt"

// Plan is used to submit a commit plan for task allocations. These
// are submitted to the leader which verifies that resources have
// not been overcommitted before admitting the plan.
type Plan struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	// EvalID is the evaluation ID this plan is associated with
	EvalID string

	// EvalToken is used to prevent a split-brain processing of
	// an evaluation. There should only be a single scheduler running
	// an Eval at a time, but this could be violated after a leadership
	// transition. This unique token is used to reject plans that are
	// being submitted from a different leader.
	EvalToken string

	// Priority is the priority of the upstream job
	Priority int

	// AllAtOnce is used to control if incremental scheduling of task groups
	// is allowed or if we must do a gang scheduling of the entire job.
	// If this is false, a plan may be partially applied. Otherwise, the
	// entire plan must be able to make progress.
	AllAtOnce bool

	// JobTuple contains namespace, job ID and version of all the allocations
	// in the Plan. This is so that we don't serialize the whole Job object in the
	// Plan.Submit RPC.
	JobTuple *PlanJobTuple

	// NodeUpdate contains all the allocations to be stopped or evicted for
	// each node.
	NodeUpdate map[string][]*Allocation

	// NodeAllocation contains all the allocations for each node.
	// The evicts must be considered prior to the allocations.
	NodeAllocation map[string][]*Allocation

	// Annotations contains annotations by the scheduler to be used by operators
	// to understand the decisions made by the scheduler.
	Annotations *PlanAnnotations

	// Deployment is the deployment created or updated by the scheduler that
	// should be applied by the planner.
	Deployment *Deployment

	// DeploymentUpdates is a set of status updates to apply to the given
	// deployments. This allows the scheduler to cancel any unneeded deployment
	// because the job is stopped or the update block is removed.
	DeploymentUpdates []*DeploymentStatusUpdate

	// NodePreemptions is a map from node id to a set of allocations from other
	// lower priority jobs that are preempted. Preempted allocations are marked
	// as evicted.
	NodePreemptions map[string][]*Allocation

	// SnapshotIndex is the Raft index of the snapshot used to create the
	// Plan. The leader will wait to evaluate the plan until its StateStore
	// has reached at least this index.
	SnapshotIndex uint64
}

// PlanJobTuple contains namespace, job ID and version of all the allocations
// in the Plan. This is so that we don't serialize the whole Job object in the
// Plan.Submit RPC.
type PlanJobTuple struct {
	Namespace string
	ID        string
	Version   uint64
}

func (p *Plan) GoString() string {
	out := fmt.Sprintf("(eval %s", p.EvalID[:8])
	if p.JobTuple != nil {
		out += fmt.Sprintf(", job %s", p.JobTuple.ID)
	}
	if p.Deployment != nil {
		out += fmt.Sprintf(", deploy %s", p.Deployment.ID[:8])
	}
	if len(p.NodeUpdate) > 0 {
		out += ", NodeUpdates: "
		for node, allocs := range p.NodeUpdate {
			out += fmt.Sprintf("(node[%s]", node[:8])
			for _, alloc := range allocs {
				out += fmt.Sprintf(" (%s stop/evict)", alloc.ID[:8])
			}
			out += ")"
		}
	}
	if len(p.NodeAllocation) > 0 {
		out += ", NodeAllocations: "
		for node, allocs := range p.NodeAllocation {
			out += fmt.Sprintf("(node[%s]", node[:8])
			for _, alloc := range allocs {
				out += fmt.Sprintf(" (%s %s %s)",
					alloc.ID[:8], alloc.Name, alloc.DesiredStatus,
				)
			}
			out += ")"
		}
	}
	if len(p.NodePreemptions) > 0 {
		out += ", NodePreemptions: "
		for node, allocs := range p.NodePreemptions {
			out += fmt.Sprintf("(node[%s]", node[:8])
			for _, alloc := range allocs {
				out += fmt.Sprintf(" (%s %s %s)",
					alloc.ID[:8], alloc.Name, alloc.DesiredStatus,
				)
			}
			out += ")"
		}
	}
	if len(p.DeploymentUpdates) > 0 {
		out += ", DeploymentUpdates: "
		for _, dupdate := range p.DeploymentUpdates {
			out += fmt.Sprintf("(%s %s)",
				dupdate.DeploymentID[:8], dupdate.Status)
		}
	}
	if p.Annotations != nil {
		out += ", Annotations: "
		for tg, updates := range p.Annotations.DesiredTGUpdates {
			out += fmt.Sprintf("(update[%s] %v)", tg, updates)
		}
		for _, preempted := range p.Annotations.PreemptedAllocs {
			out += fmt.Sprintf("(preempt %s)", preempted.ID[:8])
		}
	}

	out += ")"
	return out
}

// AppendStoppedAlloc marks an allocation to be stopped. The clientStatus of the
// allocation may be optionally set by passing in a non-empty value.
func (p *Plan) AppendStoppedAlloc(alloc *Allocation, desiredDesc, clientStatus, followupEvalID string) {
	newAlloc := new(Allocation)
	*newAlloc = *alloc

	// If the job tuple is not set in the plan we are deregistering a job so we
	// extract the job information from the allocation.
	if p.JobTuple == nil && newAlloc.Job != nil {
		p.JobTuple = &PlanJobTuple{
			Namespace: newAlloc.Job.Namespace,
			ID:        newAlloc.Job.ID,
			Version:   newAlloc.Job.Version,
		}
	}

	// Normalize the job
	newAlloc.Job = nil

	// Strip the resources as it can be rebuilt.
	newAlloc.Resources = nil

	newAlloc.DesiredStatus = AllocDesiredStatusStop
	newAlloc.DesiredDescription = desiredDesc

	if clientStatus != "" {
		newAlloc.ClientStatus = clientStatus
	}

	newAlloc.AppendState(AllocStateFieldClientStatus, clientStatus)

	if followupEvalID != "" {
		newAlloc.FollowupEvalID = followupEvalID
	}

	node := alloc.NodeID
	existing := p.NodeUpdate[node]
	p.NodeUpdate[node] = append(existing, newAlloc)
}

// AppendPreemptedAlloc is used to append an allocation that's being preempted to the plan.
// To minimize the size of the plan, this only sets a minimal set of fields in the allocation
func (p *Plan) AppendPreemptedAlloc(alloc *Allocation, preemptingAllocID string) {
	newAlloc := &Allocation{}
	newAlloc.ID = alloc.ID
	newAlloc.JobID = alloc.JobID
	newAlloc.Namespace = alloc.Namespace
	newAlloc.DesiredStatus = AllocDesiredStatusEvict
	newAlloc.PreemptedByAllocation = preemptingAllocID

	desiredDesc := fmt.Sprintf("Preempted by alloc ID %v", preemptingAllocID)
	newAlloc.DesiredDescription = desiredDesc

	// TaskResources are needed by the plan applier to check if allocations fit
	// after removing preempted allocations
	if alloc.AllocatedResources != nil {
		newAlloc.AllocatedResources = alloc.AllocatedResources
	} else {
		// COMPAT Remove in version 0.11
		newAlloc.TaskResources = alloc.TaskResources
		newAlloc.SharedResources = alloc.SharedResources
	}

	// Append this alloc to slice for this node
	node := alloc.NodeID
	existing := p.NodePreemptions[node]
	p.NodePreemptions[node] = append(existing, newAlloc)
}

// AppendUnknownAlloc marks an allocation as unknown.
func (p *Plan) AppendUnknownAlloc(alloc *Allocation) {
	// Strip the resources as they can be rebuilt.
	alloc.Resources = nil

	existing := p.NodeAllocation[alloc.NodeID]
	p.NodeAllocation[alloc.NodeID] = append(existing, alloc)
}

func (p *Plan) PopUpdate(alloc *Allocation) {
	existing := p.NodeUpdate[alloc.NodeID]
	n := len(existing)
	if n > 0 && existing[n-1].ID == alloc.ID {
		existing = existing[:n-1]
		if len(existing) > 0 {
			p.NodeUpdate[alloc.NodeID] = existing
		} else {
			delete(p.NodeUpdate, alloc.NodeID)
		}
	}
}

// AppendAlloc appends the alloc to the plan allocations.
// Uses the passed job if explicitly passed, otherwise
// it is assumed the alloc will use the plan Job version.
func (p *Plan) AppendAlloc(alloc *Allocation, job *Job) {
	node := alloc.NodeID
	existing := p.NodeAllocation[node]

	alloc.Job = job

	p.NodeAllocation[node] = append(existing, alloc)
}

// IsNoOp checks if this plan would do nothing
func (p *Plan) IsNoOp() bool {
	return len(p.NodeUpdate) == 0 &&
		len(p.NodeAllocation) == 0 &&
		p.Deployment == nil &&
		len(p.DeploymentUpdates) == 0
}

// NormalizeAllocations normalizes allocations to remove fields that can
// be fetched from the MemDB instead of sending over the wire
func (p *Plan) NormalizeAllocations() {
	for _, allocs := range p.NodeUpdate {
		for i, alloc := range allocs {
			allocs[i] = &Allocation{
				ID:                 alloc.ID,
				DesiredDescription: alloc.DesiredDescription,
				ClientStatus:       alloc.ClientStatus,
				FollowupEvalID:     alloc.FollowupEvalID,
				RescheduleTracker:  alloc.RescheduleTracker,
			}
		}
	}

	for _, allocs := range p.NodePreemptions {
		for i, alloc := range allocs {
			allocs[i] = &Allocation{
				ID:                    alloc.ID,
				PreemptedByAllocation: alloc.PreemptedByAllocation,
			}
		}
	}
}

// PlanResult is the result of a plan submitted to the leader.
type PlanResult struct {
	// NodeUpdate contains all the evictions and stops that were committed.
	NodeUpdate map[string][]*Allocation

	// NodeAllocation contains all the allocations that were committed.
	NodeAllocation map[string][]*Allocation

	// Deployment is the deployment that was committed.
	Deployment *Deployment

	// DeploymentUpdates is the set of deployment updates that were committed.
	DeploymentUpdates []*DeploymentStatusUpdate

	// NodePreemptions is a map from node id to a set of allocations from other
	// lower priority jobs that are preempted. Preempted allocations are marked
	// as stopped.
	NodePreemptions map[string][]*Allocation

	// RejectedNodes are nodes the scheduler worker has rejected placements for
	// and should be considered for ineligibility by the plan applier to avoid
	// retrying them repeatedly.
	RejectedNodes []string

	// IneligibleNodes are nodes the plan applier has repeatedly rejected
	// placements for and should therefore be considered ineligible by workers
	// to avoid retrying them repeatedly.
	IneligibleNodes []string

	// RefreshIndex is the index the worker should refresh state up to.
	// This allows all evictions and allocations to be materialized.
	// If any allocations were rejected due to stale data (node state,
	// over committed) this can be used to force a worker refresh.
	RefreshIndex uint64

	// AllocIndex is the Raft index in which the evictions and
	// allocations took place. This is used for the write index.
	AllocIndex uint64
}

// IsNoOp checks if this plan result would do nothing
func (p *PlanResult) IsNoOp() bool {
	return len(p.IneligibleNodes) == 0 && len(p.NodeUpdate) == 0 &&
		len(p.NodeAllocation) == 0 && len(p.DeploymentUpdates) == 0 &&
		p.Deployment == nil
}

// FullCommit is used to check if all the allocations in a plan
// were committed as part of the result. Returns if there was
// a match, and the number of expected and actual allocations.
func (p *PlanResult) FullCommit(plan *Plan) (bool, int, int) {
	expected := 0
	actual := 0
	for name, allocList := range plan.NodeAllocation {
		didAlloc := p.NodeAllocation[name]
		expected += len(allocList)
		actual += len(didAlloc)
	}
	return actual == expected, expected, actual
}

// PlanAnnotations holds annotations made by the scheduler to give further debug
// information to operators.
type PlanAnnotations struct {
	// DesiredTGUpdates is the set of desired updates per task group.
	DesiredTGUpdates map[string]*DesiredUpdates

	// PreemptedAllocs is the set of allocations to be preempted to make the placement successful.
	PreemptedAllocs []*AllocListStub
}
