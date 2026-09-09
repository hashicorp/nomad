// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestJobStatuses_GroupSelections(t *testing.T) {
	ci.Parallel(t)
	tests := []struct {
		name       string
		count      int
		groups     []string
		expected   int
		unplaced   int
		canary     bool
		failedOnly bool
	}{
		{name: "unresolved", count: 1, expected: 2, unplaced: 1},
		{name: "one selected", count: 1, groups: []string{"encoder"}, expected: 6},
		{name: "partial selection", count: 2, groups: []string{"thor"}, expected: 3, unplaced: 1},
		{name: "all selected", count: 3, groups: []string{"encoder", "orin", "thor"}, expected: 9},
		{name: "canary replaces profile", count: 1, groups: []string{"thor"}, expected: 3, canary: true},
		{name: "failed selected profile", count: 1, groups: []string{"encoder"}, expected: 6, failedOnly: true},
		{name: "selection scaled down", count: 0, expected: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := testStateStore(t)
			job := selectionPlanJob(tc.count)
			job.TaskGroups[0].Count = 4
			job.TaskGroups[1].Count = 2
			job.TaskGroups[2].Count = 1
			ordinary := job.TaskGroups[0].Copy()
			ordinary.Name, ordinary.Count = "control", 2
			job.TaskGroups = append(job.TaskGroups, ordinary)
			require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
			var allocs []*structs.Allocation
			for slot, group := range tc.groups {
				alloc := selectionPlanAlloc(job, "node", group, "current", slot)
				if tc.failedOnly {
					alloc.ClientStatus = structs.AllocClientStatusFailed
				}
				allocs = append(allocs, alloc)
			}
			if tc.canary {
				old := selectionPlanAlloc(job, "node", "encoder", "previous", 0)
				allocs = append(allocs, old)
				deployment := structs.NewDeployment(job, 50, time.Now().UnixNano())
				deployment.GroupSelections["recognition"].Slots[0] = &structs.DeploymentGroupSelectionSlot{
					TaskGroup: "thor", Cohort: "current", PreviousTaskGroup: "encoder", PreviousCohort: "previous",
				}
				require.NoError(t, state.UpsertDeployment(1001, deployment))
				allocs[0].DeploymentStatus = &structs.AllocDeploymentStatus{Canary: true}
			}
			require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1002, allocs))
			status, _, err := jobStatusesJobFromJob(nil, state, job)
			require.NoError(t, err)
			require.Equal(t, tc.expected, status.GroupCountSum)
			selection := status.GroupSelectionStatuses["recognition"]
			require.Equal(t, tc.count, selection.Count)
			require.Equal(t, tc.unplaced, selection.Unplaced)
			require.Len(t, selection.Slots, tc.count-tc.unplaced)
		})
	}
}

func TestJobStatuses_GroupSelectionHistory(t *testing.T) {
	ci.Parallel(t)
	job := selectionPlanJob(1)
	job.CreateIndex = 1000
	old := selectionPlanAlloc(job, "node", "encoder", "old", 0)
	old.CreateIndex = 1001
	old.ClientStatus = structs.AllocClientStatusFailed
	current := selectionPlanAlloc(job, "node", "thor", "current", 0)
	current.CreateIndex = 1002
	current.ClientStatus = structs.AllocClientStatusRunning
	previousJob := selectionPlanAlloc(job.Copy(), "node", "orin", "other-job", 0)
	previousJob.Job.CreateIndex = 500
	previousJob.CreateIndex = 501
	statuses := job.GroupSelectionStatuses([]*structs.Allocation{previousJob, old, current}, nil)
	require.Equal(t, &structs.JobGroupSelectionSlotStatus{TaskGroup: "thor", Cohort: "current"}, statuses["recognition"].Slots["0"])

	current.DesiredStatus = structs.AllocDesiredStatusStop
	old.DesiredStatus = structs.AllocDesiredStatusStop
	statuses = job.GroupSelectionStatuses([]*structs.Allocation{previousJob, old, current}, nil)
	require.Empty(t, statuses["recognition"].Slots)
	require.Equal(t, 1, statuses["recognition"].Unplaced)
}

func TestJobStatuses_GroupSelectionHTTPJSON(t *testing.T) {
	ci.Parallel(t)
	job := selectionPlanJob(1)
	alloc := selectionPlanAlloc(job, "node", "thor", "current", 0)
	statuses := job.GroupSelectionStatuses([]*structs.Allocation{alloc}, nil)
	for _, handle := range []*codec.JsonHandle{structs.JsonHandleWithExtensions, structs.JsonHandlePretty} {
		var encoded []byte
		require.NoError(t, codec.NewEncoderBytes(&encoded, handle).Encode(statuses))
		require.True(t, json.Valid(encoded), string(encoded))
		var decoded map[string]*structs.JobGroupSelectionStatus
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		require.Equal(t, statuses, decoded)
	}
}
