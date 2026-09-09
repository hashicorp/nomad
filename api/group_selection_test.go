// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestGroupSelection_CanonicalizeCopy(t *testing.T) {
	testutil.Parallel(t)
	for _, count := range []*int{nil, new(0), new(2)} {
		selection := &TaskGroupSelection{Name: "runtime", Count: count, Groups: []string{"encoder", "orin", "thor"}}
		want := 1
		if count != nil {
			want = *count
		}
		job := &Job{GroupSelections: []*TaskGroupSelection{selection}}
		job.Canonicalize()
		must.Eq(t, want, *selection.Count)
		clone := selection.Copy()
		*clone.Count = 5
		clone.Groups[0] = "changed"
		must.Eq(t, want, *selection.Count)
		must.Eq(t, "encoder", selection.Groups[0])
	}
	must.Nil(t, (*TaskGroupSelection)(nil).Copy())
	selection := (&TaskGroupSelection{}).Copy()
	must.Nil(t, selection.Count)
	must.Nil(t, selection.Groups)
	// API nulls must reach server validation instead of panicking in canonicalization.
	(&Job{GroupSelections: []*TaskGroupSelection{nil}}).Canonicalize()
}

func TestGroupSelection_RuntimeCopy(t *testing.T) {
	testutil.Parallel(t)
	allocation := &Allocation{GroupSelection: &AllocationGroupSelection{Name: "runtime", Slot: 0, Cohort: "new"}}
	stub := allocation.Stub()
	stub.GroupSelection.Cohort = "changed"
	must.Eq(t, "new", allocation.GroupSelection.Cohort)
	selection := &DeploymentGroupSelection{Count: 1, RequireProgressBy: time.Now(), Slots: map[int]*DeploymentGroupSelectionSlot{0: {TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"}}}
	clone := selection.Copy()
	clone.Slots[0].PreviousTaskGroup = "changed"
	clone.Slots[1] = nil
	must.Eq(t, "orin", selection.Slots[0].PreviousTaskGroup)
	must.MapLen(t, 1, selection.Slots)
	must.Nil(t, (*AllocationGroupSelection)(nil).Copy())
	must.Nil(t, (*DeploymentGroupSelection)(nil).Copy())
	must.Nil(t, (*DeploymentGroupSelectionSlot)(nil).Copy())
}

func TestGroupSelection_JSON(t *testing.T) {
	testutil.Parallel(t)
	for _, value := range []any{&Job{}, &Allocation{}, &AllocationListStub{}, &Deployment{}} {
		data, err := json.Marshal(value)
		must.NoError(t, err)
		must.False(t, strings.Contains(string(data), "GroupSelection"))
	}
	job := &Job{GroupSelections: []*TaskGroupSelection{{Name: "runtime", Count: new(0), Groups: []string{"orin", "thor"}}}}
	data, err := json.Marshal(job)
	must.NoError(t, err)
	var restored Job
	must.NoError(t, json.Unmarshal(data, &restored))
	must.Eq(t, job.GroupSelections, restored.GroupSelections)
	allocation := &Allocation{GroupSelection: &AllocationGroupSelection{Name: "runtime", Slot: 0, Cohort: "new"}}
	data, err = json.Marshal(allocation)
	must.NoError(t, err)
	must.StrContains(t, string(data), `"Slot":0`)
	var restoredAllocation Allocation
	must.NoError(t, json.Unmarshal(data, &restoredAllocation))
	must.Eq(t, allocation.GroupSelection, restoredAllocation.GroupSelection)
}
