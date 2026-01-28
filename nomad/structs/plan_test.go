// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func TestPlan_NormalizeAllocations(t *testing.T) {
	ci.Parallel(t)
	plan := &Plan{
		NodeUpdate:      make(map[string][]*Allocation),
		NodePreemptions: make(map[string][]*Allocation),
	}
	stoppedAlloc := MockAlloc()
	desiredDesc := "Desired desc"
	plan.AppendStoppedAlloc(stoppedAlloc, desiredDesc, AllocClientStatusLost, "followup-eval-id")
	preemptedAlloc := MockAlloc()
	preemptingAllocID := uuid.Generate()
	plan.AppendPreemptedAlloc(preemptedAlloc, preemptingAllocID)

	plan.NormalizeAllocations()

	actualStoppedAlloc := plan.NodeUpdate[stoppedAlloc.NodeID][0]
	expectedStoppedAlloc := &Allocation{
		ID:                 stoppedAlloc.ID,
		DesiredDescription: desiredDesc,
		ClientStatus:       AllocClientStatusLost,
		FollowupEvalID:     "followup-eval-id",
	}
	must.Eq(t, expectedStoppedAlloc, actualStoppedAlloc)
	actualPreemptedAlloc := plan.NodePreemptions[preemptedAlloc.NodeID][0]
	expectedPreemptedAlloc := &Allocation{
		ID:                    preemptedAlloc.ID,
		PreemptedByAllocation: preemptingAllocID,
	}
	must.Eq(t, expectedPreemptedAlloc, actualPreemptedAlloc)
}

func TestPlan_AppendStoppedAllocAppendsAllocWithUpdatedAttrs(t *testing.T) {
	ci.Parallel(t)
	plan := &Plan{
		NodeUpdate: make(map[string][]*Allocation),
	}
	alloc := MockAlloc()
	desiredDesc := "Desired desc"

	plan.AppendStoppedAlloc(alloc, desiredDesc, AllocClientStatusLost, "")

	expectedAlloc := new(Allocation)
	*expectedAlloc = *alloc
	expectedAlloc.DesiredDescription = desiredDesc
	expectedAlloc.DesiredStatus = AllocDesiredStatusStop
	expectedAlloc.ClientStatus = AllocClientStatusLost
	expectedAlloc.Job = nil
	expectedAlloc.AllocStates = []*AllocState{{
		Field: AllocStateFieldClientStatus,
		Value: "lost",
	}}

	// This value is set to time.Now() in AppendStoppedAlloc, so clear it
	appendedAlloc := plan.NodeUpdate[alloc.NodeID][0]
	appendedAlloc.AllocStates[0].Time = time.Time{}

	must.Eq(t, expectedAlloc, appendedAlloc)
	must.Eq(t, alloc.Job.ID, plan.JobInfo.ID)
	must.Eq(t, alloc.Job.Namespace, plan.JobInfo.Namespace)
	must.Eq(t, alloc.Job.Version, plan.JobInfo.Version)
}

func TestPlan_AppendPreemptedAllocAppendsAllocWithUpdatedAttrs(t *testing.T) {
	ci.Parallel(t)
	plan := &Plan{
		NodePreemptions: make(map[string][]*Allocation),
	}
	alloc := MockAlloc()
	preemptingAllocID := uuid.Generate()

	plan.AppendPreemptedAlloc(alloc, preemptingAllocID)

	appendedAlloc := plan.NodePreemptions[alloc.NodeID][0]
	expectedAlloc := &Allocation{
		ID:                    alloc.ID,
		PreemptedByAllocation: preemptingAllocID,
		JobID:                 alloc.JobID,
		Namespace:             alloc.Namespace,
		DesiredStatus:         AllocDesiredStatusEvict,
		DesiredDescription:    fmt.Sprintf("Preempted by alloc ID %v", preemptingAllocID),
		AllocatedResources:    alloc.AllocatedResources,
		TaskResources:         alloc.TaskResources,
		SharedResources:       alloc.SharedResources,
	}
	must.Eq(t, expectedAlloc, appendedAlloc)
}
