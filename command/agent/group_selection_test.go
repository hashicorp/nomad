// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/jobspec2"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestApiGroupSelectionsToStructs(t *testing.T) {
	ci.Parallel(t)
	selection := &api.TaskGroupSelection{Name: "runtime", Groups: []string{"orin", "thor"}}
	job := ApiJobToStructJob(&api.Job{ID: new("example"), GroupSelections: []*api.TaskGroupSelection{selection}})
	must.Eq(t, 1, job.GroupSelections[0].Count)
	job.GroupSelections[0].Groups[0] = "changed"
	must.Eq(t, "orin", selection.Groups[0])
	selection.Count = new(0)
	must.Zero(t, ApiGroupSelectionsToStructs([]*api.TaskGroupSelection{selection})[0].Count)
	must.Nil(t, ApiGroupSelectionsToStructs(nil))
	must.Nil(t, ApiGroupSelectionsToStructs([]*api.TaskGroupSelection{nil})[0])
}

func TestGroupSelection_HCLInternalAPIRoundTrip(t *testing.T) {
	ci.Parallel(t)
	path := "../../jobspec2/test-fixtures/group-selection.hcl"
	source, err := os.ReadFile(path)
	must.NoError(t, err)
	parsed, err := jobspec2.ParseWithConfig(&jobspec2.ParseConfig{Path: path, Body: source, Strict: true})
	must.NoError(t, err)
	job := ApiJobToStructJob(parsed)
	job.Canonicalize()
	must.NoError(t, job.Validate())
	must.Eq(t, 2, job.GroupSelections[0].Count)
	encoded, err := json.Marshal(job)
	must.NoError(t, err)
	var response api.Job
	must.NoError(t, json.Unmarshal(encoded, &response))
	must.Eq(t, parsed.GroupSelections, response.GroupSelections)
	for i, want := range []int{2, 3, 5} {
		must.Eq(t, want, job.TaskGroups[i].Count)
	}
	for i, want := range []int{1000, 800, 500} {
		must.Eq(t, want, job.TaskGroups[i].Tasks[0].Resources.CPU)
		must.Eq(t, want, job.TaskGroups[i].Tasks[0].Resources.MemoryMB)
	}
}

func TestGroupSelection_RuntimeAPIResponse(t *testing.T) {
	ci.Parallel(t)
	allocation := &structs.Allocation{Job: &structs.Job{Type: structs.JobTypeService}, GroupSelection: &structs.AllocationGroupSelection{Name: "runtime", Slot: 0, Cohort: "new"}}
	data, err := json.Marshal(allocation)
	must.NoError(t, err)
	var response api.Allocation
	must.NoError(t, json.Unmarshal(data, &response))
	must.Eq(t, &api.AllocationGroupSelection{Name: "runtime", Slot: 0, Cohort: "new"}, response.GroupSelection)
	data, err = json.Marshal(allocation.Stub(nil))
	must.NoError(t, err)
	var stub api.AllocationListStub
	must.NoError(t, json.Unmarshal(data, &stub))
	must.Eq(t, response.GroupSelection, stub.GroupSelection)
	deadline := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	deployment := &structs.Deployment{GroupSelections: map[string]*structs.DeploymentGroupSelection{"runtime": {Count: 1, RequireProgressBy: deadline, Slots: map[int]*structs.DeploymentGroupSelectionSlot{0: {TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"}}}}}
	data, err = json.Marshal(deployment)
	must.NoError(t, err)
	var restored api.Deployment
	must.NoError(t, json.Unmarshal(data, &restored))
	must.Eq(t, &api.DeploymentGroupSelection{Count: 1, RequireProgressBy: deadline, Slots: map[int]*api.DeploymentGroupSelectionSlot{0: {TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"}}}, restored.GroupSelections["runtime"])
}
