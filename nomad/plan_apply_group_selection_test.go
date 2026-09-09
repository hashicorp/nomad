// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func selectionPlanJob(count int) *structs.Job {
	job := mock.Job()
	base := job.TaskGroups[0]
	job.TaskGroups = nil
	for _, name := range []string{"encoder", "orin", "thor"} {
		group := base.Copy()
		group.Name = name
		group.Count = 1
		job.TaskGroups = append(job.TaskGroups, group)
	}
	job.GroupSelections = []*structs.TaskGroupSelection{{
		Name: "recognition", Count: count, Groups: []string{"encoder", "orin", "thor"},
	}}
	return job
}

func selectionPlanAlloc(job *structs.Job, node, group, cohort string, slot int) *structs.Allocation {
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.Namespace = job.Namespace
	alloc.TaskGroup = group
	alloc.NodeID = node
	alloc.GroupSelection = &structs.AllocationGroupSelection{Name: "recognition", Slot: slot, Cohort: cohort}
	return alloc
}

func TestPlanApply_GroupSelection_CompetingClaims(t *testing.T) {
	ci.Parallel(t)
	for _, samePlan := range []bool{false, true} {
		name := "committed"
		if samePlan {
			name = "same plan"
		}
		t.Run(name, func(t *testing.T) {
			state := testStateStore(t)
			job := selectionPlanJob(1)
			require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
			nodeA, nodeB := mock.Node(), mock.Node()
			require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1001, nodeA))
			require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1002, nodeB))
			winner := selectionPlanAlloc(job, nodeA.ID, "encoder", "incumbent", 0)
			challenger := selectionPlanAlloc(job, nodeB.ID, "thor", "challenger", 0)
			plan := &structs.Plan{
				Job: job, JobInfo: &structs.PlanJobTuple{Namespace: job.Namespace, ID: job.ID},
				NodeAllocation: map[string][]*structs.Allocation{nodeB.ID: {challenger}},
			}
			if samePlan {
				plan.NodeAllocation[nodeA.ID] = []*structs.Allocation{winner}
			} else {
				require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{winner}))
			}
			snap, err := state.Snapshot()
			require.NoError(t, err)
			pool := NewEvaluatePool(workerPoolSize, workerPoolBufferSize)
			defer pool.Shutdown()
			result, err := evaluatePlan(pool, snap, plan, testlog.HCLogger(t))
			require.NoError(t, err)
			require.True(t, result.IsNoOp())
			require.GreaterOrEqual(t, result.RefreshIndex, uint64(1002))
			require.Empty(t, result.RejectedNodes)
		})
	}
}

func TestPlanApply_GroupSelection_CanaryCohorts(t *testing.T) {
	ci.Parallel(t)
	for _, promoted := range []bool{false, true} {
		name := "canary"
		if promoted {
			name = "promoted"
		}
		t.Run(name, func(t *testing.T) {
			state := testStateStore(t)
			job := selectionPlanJob(1)
			require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
			nodeA, nodeB := mock.Node(), mock.Node()
			require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1001, nodeA))
			require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1002, nodeB))
			old := selectionPlanAlloc(job, nodeA.ID, "encoder", "old", 0)
			require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{old}))
			canary := selectionPlanAlloc(job, nodeB.ID, "thor", "new", 0)
			canary.DeploymentStatus = &structs.AllocDeploymentStatus{Canary: !promoted}
			deployment := structs.NewDeployment(job, 50, time.Now().UnixNano())
			deployment.GroupSelections["recognition"].Slots[0] = &structs.DeploymentGroupSelectionSlot{
				TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "encoder", PreviousCohort: "old",
			}
			plan := &structs.Plan{
				Job: job, JobInfo: &structs.PlanJobTuple{Namespace: job.Namespace, ID: job.ID},
				NodeAllocation: map[string][]*structs.Allocation{nodeB.ID: {canary}}, Deployment: deployment,
			}
			snap, err := state.Snapshot()
			require.NoError(t, err)
			pool := NewEvaluatePool(workerPoolSize, workerPoolBufferSize)
			defer pool.Shutdown()
			result, err := evaluatePlan(pool, snap, plan, testlog.HCLogger(t))
			require.NoError(t, err)
			require.Zero(t, result.RefreshIndex)
			require.Equal(t, plan.NodeAllocation, result.NodeAllocation)
			require.Equal(t, deployment.GroupSelections, result.Deployment.GroupSelections)
		})
	}
}

func TestPlanApply_GroupSelection_PartialAssignment(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	job := selectionPlanJob(2)
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
	nodeA, nodeB := mock.Node(), mock.Node()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1001, nodeA))
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1002, nodeB))
	accepted := selectionPlanAlloc(job, nodeA.ID, "encoder", "accepted", 0)
	rejected := selectionPlanAlloc(job, nodeB.ID, "thor", "rejected", 1)
	rejected.AllocatedResources = structs.NodeResourcesToAllocatedResources(nodeB.NodeResources)
	deployment := structs.NewDeployment(job, 50, time.Now().UnixNano())
	deployment.GroupSelections["recognition"].Slots[0] = &structs.DeploymentGroupSelectionSlot{TaskGroup: "encoder", Cohort: "accepted"}
	deployment.GroupSelections["recognition"].Slots[1] = &structs.DeploymentGroupSelectionSlot{TaskGroup: "thor", Cohort: "rejected"}
	plan := &structs.Plan{
		Job: job, JobInfo: &structs.PlanJobTuple{Namespace: job.Namespace, ID: job.ID},
		NodeAllocation: map[string][]*structs.Allocation{nodeA.ID: {accepted}, nodeB.ID: {rejected}}, Deployment: deployment,
	}
	snap, err := state.Snapshot()
	require.NoError(t, err)
	pool := NewEvaluatePool(workerPoolSize, workerPoolBufferSize)
	defer pool.Shutdown()
	result, err := evaluatePlan(pool, snap, plan, testlog.HCLogger(t))
	require.NoError(t, err)
	require.Equal(t, uint64(1002), result.RefreshIndex)
	require.Len(t, result.NodeAllocation, 1)
	require.Len(t, result.Deployment.GroupSelections["recognition"].Slots, 1)
	require.Equal(t, 2, result.Deployment.GroupSelections["recognition"].Count)
	require.Contains(t, result.Deployment.GroupSelections["recognition"].Slots, 0)
	// The submitted plan remains intact for the scheduler's FullCommit check.
	require.Len(t, plan.Deployment.GroupSelections["recognition"].Slots, 2)
}

func TestPlanApply_GroupSelection_PartialPreservesCommittedAssignment(t *testing.T) {
	ci.Parallel(t)
	job := selectionPlanJob(1)
	committed := structs.NewDeployment(job, 50, time.Now().UnixNano())
	committed.GroupSelections["recognition"].Slots[0] = &structs.DeploymentGroupSelectionSlot{TaskGroup: "encoder", Cohort: "committed"}
	committed.TaskGroups["encoder"] = &structs.DeploymentState{DesiredTotal: 1, HealthyAllocs: 1}
	tentative := committed.Copy()
	delete(tentative.TaskGroups, "encoder")
	tentative.TaskGroups["thor"] = &structs.DeploymentState{DesiredTotal: 1, DesiredCanaries: 1}
	tentative.GroupSelections["recognition"].Slots[0] = &structs.DeploymentGroupSelectionSlot{
		TaskGroup: "thor", Cohort: "rejected", PreviousTaskGroup: "orin", PreviousCohort: "stale",
	}
	incumbent := selectionPlanAlloc(job, "node", "encoder", "committed", 0)
	correctDeploymentGroupSelections(tentative, committed, map[string]*structs.Allocation{incumbent.ID: incumbent})
	require.Equal(t, committed.GroupSelections, tentative.GroupSelections)
	require.Equal(t, committed.TaskGroups, tentative.TaskGroups)
	tentative.GroupSelections["recognition"].Slots[0].Cohort = "changed"
	require.Equal(t, "committed", committed.GroupSelections["recognition"].Slots[0].Cohort)
}

func TestPlanApply_GroupSelection_UnplacedCanaryPreservesPrevious(t *testing.T) {
	ci.Parallel(t)
	job := selectionPlanJob(1)
	deployment := structs.NewDeployment(job, 50, time.Now().UnixNano())
	deployment.GroupSelections["recognition"].Slots[0] = &structs.DeploymentGroupSelectionSlot{
		TaskGroup: "thor", Cohort: "rejected", PreviousTaskGroup: "encoder", PreviousCohort: "old",
	}
	old := selectionPlanAlloc(job, "node", "encoder", "old", 0)
	projected := map[string]*structs.Allocation{old.ID: old}
	correctDeploymentGroupSelections(deployment, nil, projected)
	require.Equal(t, &structs.DeploymentGroupSelectionSlot{
		PreviousTaskGroup: "encoder", PreviousCohort: "old",
	}, deployment.GroupSelections["recognition"].Slots[0])
	require.Empty(t, validateGroupSelectionClaims(job.GroupSelections[0], deployment.GroupSelections["recognition"], projected))
}

func TestPlanApply_GroupSelection_Replacement(t *testing.T) {
	ci.Parallel(t)
	for _, stopAccepted := range []bool{false, true} {
		name := "stop rejected"
		if stopAccepted {
			name = "stop accepted"
		}
		t.Run(name, func(t *testing.T) {
			state := testStateStore(t)
			job := selectionPlanJob(1)
			require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
			old := selectionPlanAlloc(job, "node-a", "encoder", "old", 0)
			require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{old}))
			replacement := selectionPlanAlloc(job, "node-b", "thor", "new", 0)
			result := &structs.PlanResult{NodeAllocation: map[string][]*structs.Allocation{"node-b": {replacement}}}
			if stopAccepted {
				stopped := old.Copy()
				stopped.DesiredStatus = structs.AllocDesiredStatusStop
				result.NodeUpdate = map[string][]*structs.Allocation{"node-a": {stopped}}
			}
			snap, err := state.Snapshot()
			require.NoError(t, err)
			valid, reason, err := evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
			require.NoError(t, err)
			require.Equal(t, stopAccepted, valid, reason)
		})
	}
}

func TestPlanApply_GroupSelection_LegacyAllocation(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	job := selectionPlanJob(1)
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
	legacy := selectionPlanAlloc(job, "node", "encoder", "", 0)
	legacy.GroupSelection = nil
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{legacy}))
	snap, err := state.Snapshot()
	require.NoError(t, err)
	result := &structs.PlanResult{NodeAllocation: map[string][]*structs.Allocation{"node": {legacy.Copy()}}}
	valid, reason, err := evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.True(t, valid, reason)

	newAlloc := selectionPlanAlloc(job, "node", "thor", "", 0)
	newAlloc.GroupSelection = nil
	result.NodeAllocation["node"] = []*structs.Allocation{newAlloc}
	valid, reason, err = evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.False(t, valid)
	require.Contains(t, reason, "missing its group selection claim")
}

func TestPlanApply_GroupSelection_PreviousOnlySlot(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	job := selectionPlanJob(1)
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
	previous := selectionPlanAlloc(job, "node", "orin", "old", 1)
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{previous}))
	deployment := structs.NewDeployment(job, 50, time.Now().UnixNano())
	deployment.GroupSelections["recognition"].Slots[1] = &structs.DeploymentGroupSelectionSlot{
		PreviousTaskGroup: "orin", PreviousCohort: "old",
	}
	snap, err := state.Snapshot()
	require.NoError(t, err)
	result := &structs.PlanResult{
		NodeAllocation: map[string][]*structs.Allocation{"node": {previous.Copy()}}, Deployment: deployment,
	}
	valid, reason, err := evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.True(t, valid, reason)

	newAlloc := selectionPlanAlloc(job, "node", "orin", "old", 1)
	result.NodeAllocation["node"] = []*structs.Allocation{newAlloc}
	valid, reason, err = evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.False(t, valid)
	require.Contains(t, reason, "invalid group selection claim")

	newAlloc.PreviousAllocation = previous.ID
	valid, reason, err = evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.True(t, valid, reason)

	newAlloc.Job = previous.Job.Copy()
	newAlloc.Job.Version++
	valid, reason, err = evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.False(t, valid)
	require.Contains(t, reason, "invalid group selection claim")
}

func TestPlanApply_GroupSelection_DisconnectedReplacement(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name   string
		mutate func(*structs.Allocation, *structs.Allocation, *structs.Node)
		valid  bool
	}{
		{name: "native replacement", valid: true},
		{name: "normal wire placement inherits plan job", valid: true, mutate: func(old, next *structs.Allocation, node *structs.Node) {
			next.Job = nil
		}},
		{name: "first disconnect with immediate replacement", valid: true, mutate: func(old, next *structs.Allocation, node *structs.Node) {
			old.ClientStatus = structs.AllocClientStatusRunning
			old.AllocStates = nil
			old.Job.LookupTaskGroup(old.TaskGroup).ReschedulePolicy.Delay = time.Second
			next.Job = nil
		}},
		{name: "replacement disabled", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			old.Job.LookupTaskGroup(old.TaskGroup).Disconnect.Replace = new(false)
		}},
		{name: "delay not reached", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			old.AllocStates[0].Time = time.Now()
		}},
		{name: "ready node", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			node.Status = structs.NodeStatusReady
		}},
		{name: "unrelated failed predecessor", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			// The remaining running replica still owns the original cohort.
			old.ClientStatus = structs.AllocClientStatusFailed
		}},
		{name: "wrong predecessor", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			next.PreviousAllocation = "missing"
		}},
		{name: "wrong incarnation", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			next.Job = next.Job.Copy()
			next.Job.CreateIndex++
		}},
		{name: "wrong job", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			next.JobID = "another-job"
		}},
		{name: "wrong slot", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			next.GroupSelection.Slot++
		}},
		{name: "wrong selection", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			next.GroupSelection.Name = "another-selection"
		}},
		{name: "canary is not a disconnected replacement", mutate: func(old, next *structs.Allocation, node *structs.Node) {
			next.DeploymentStatus = &structs.AllocDeploymentStatus{Canary: true}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := testStateStore(t)
			job := selectionPlanJob(1)
			job.TaskGroups[1].Disconnect = &structs.DisconnectStrategy{LostAfter: time.Hour, Replace: new(true)}
			job.TaskGroups[1].ReschedulePolicy = &structs.ReschedulePolicy{Unlimited: true, Delay: time.Minute, DelayFunction: "constant"}
			require.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
			node := mock.Node()
			node.Status = structs.NodeStatusDisconnected
			old := selectionPlanAlloc(job, node.ID, "orin", "old", 0)
			old.ClientStatus = structs.AllocClientStatusUnknown
			old.AllocStates = []*structs.AllocState{{Field: structs.AllocStateFieldClientStatus, Value: structs.AllocClientStatusUnknown, Time: time.Now().Add(-2 * time.Minute)}}
			replica := old.Copy()
			replica.ID = uuid.Generate()
			next := selectionPlanAlloc(job, "other-node", "thor", "new", 0)
			next.PreviousAllocation = old.ID
			if tc.mutate != nil {
				tc.mutate(old, next, node)
			}
			require.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1001, node))
			require.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{old, replica}))
			snap, err := store.Snapshot()
			require.NoError(t, err)
			result := &structs.PlanResult{NodeAllocation: map[string][]*structs.Allocation{next.NodeID: {next}}}
			valid, reason, err := evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
			require.NoError(t, err)
			require.Equal(t, tc.valid, valid, reason)
		})
	}
}

func TestPlanApply_GroupSelection_DisconnectedChain(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)
	job := selectionPlanJob(1)
	for _, group := range job.TaskGroups {
		group.Disconnect = &structs.DisconnectStrategy{LostAfter: time.Hour, Replace: new(true)}
		group.ReschedulePolicy = &structs.ReschedulePolicy{Unlimited: true, Delay: time.Minute, DelayFunction: "constant"}
	}
	require.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
	var existing []*structs.Allocation
	for index, name := range []string{"orin", "thor", "encoder"} {
		node := mock.Node()
		node.Status = structs.NodeStatusDisconnected
		require.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, uint64(1001+index), node))
		alloc := selectionPlanAlloc(job, node.ID, name, name, 0)
		alloc.ClientStatus = structs.AllocClientStatusUnknown
		alloc.AllocStates = []*structs.AllocState{{Field: structs.AllocStateFieldClientStatus, Value: structs.AllocClientStatusUnknown, Time: time.Now().Add(-2 * time.Minute)}}
		if index != 0 {
			alloc.PreviousAllocation = existing[index-1].ID
		}
		existing = append(existing, alloc)
	}
	require.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1010, existing[:2]))
	snap, err := store.Snapshot()
	require.NoError(t, err)
	result := &structs.PlanResult{NodeAllocation: map[string][]*structs.Allocation{existing[2].NodeID: {existing[2]}}}
	valid, reason, err := evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.True(t, valid, reason)
	deployment := structs.NewDeployment(job, 50, time.Now().UnixNano())
	deployment.GroupSelections["recognition"].Slots[0] = &structs.DeploymentGroupSelectionSlot{
		TaskGroup: "encoder", Cohort: "encoder", PreviousTaskGroup: "thor", PreviousCohort: "thor",
	}
	result.Deployment = deployment
	valid, reason, err = evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.True(t, valid, reason)
	result.Deployment = nil

	// Replaying the accepted replacement remains valid after its predecessors
	// reconnect or an intermediate allocation becomes terminal.
	require.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1011, existing[2:]))
	for _, alloc := range existing {
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}
	existing[1].DesiredStatus = structs.AllocDesiredStatusStop
	require.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1012, existing))
	snap, err = store.Snapshot()
	require.NoError(t, err)
	valid, reason, err = evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.True(t, valid, reason)

	// A stale worker cannot branch from the original predecessor into another
	// simultaneously serving cohort, even after that predecessor reconnects.
	branch := selectionPlanAlloc(job, "branch-node", "thor", "unrelated", 0)
	branch.PreviousAllocation = existing[0].ID
	result.NodeAllocation[branch.NodeID] = []*structs.Allocation{branch}
	valid, _, err = evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestPlanApply_GroupSelection_FailedDisconnectedReplacement(t *testing.T) {
	ci.Parallel(t)
	for _, eligible := range []bool{false, true} {
		t.Run(fmt.Sprint(eligible), func(t *testing.T) {
			store := testStateStore(t)
			job := selectionPlanJob(1)
			job.TaskGroups[2].ReschedulePolicy = &structs.ReschedulePolicy{Unlimited: eligible, Delay: time.Minute, DelayFunction: "constant"}
			require.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
			old := selectionPlanAlloc(job, "old-node", "orin", "old", 0)
			old.ClientStatus = structs.AllocClientStatusUnknown
			failed := selectionPlanAlloc(job, "failed-node", "thor", "failed", 0)
			failed.PreviousAllocation = old.ID
			failed.ClientStatus = structs.AllocClientStatusFailed
			failed.TaskStates = map[string]*structs.TaskState{"test": {FinishedAt: time.Now().Add(-2 * time.Minute)}}
			require.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{old, failed}))
			next := selectionPlanAlloc(job, "new-node", "encoder", "new", 0)
			next.PreviousAllocation = failed.ID
			next.Job = nil
			snap, err := store.Snapshot()
			require.NoError(t, err)
			result := &structs.PlanResult{NodeAllocation: map[string][]*structs.Allocation{next.NodeID: {next}}}
			valid, reason, err := evaluatePlanGroupSelections(snap, &structs.Plan{Job: job}, result)
			require.NoError(t, err)
			require.Equal(t, eligible, valid, reason)
			require.Nil(t, next.Job, "leader projection must not mutate the submitted allocation")
		})
	}
}

func TestPlanApply_GroupSelection_Claims(t *testing.T) {
	ci.Parallel(t)
	tests := []struct {
		name  string
		count int
		slots map[int]*structs.DeploymentGroupSelectionSlot
		valid bool
	}{
		{
			name: "distinct targets may overlap previous groups", count: 2, valid: true,
			slots: map[int]*structs.DeploymentGroupSelectionSlot{
				0: {TaskGroup: "orin", Cohort: "new-a", PreviousTaskGroup: "encoder", PreviousCohort: "old-a"},
				1: {TaskGroup: "thor", Cohort: "new-b", PreviousTaskGroup: "orin", PreviousCohort: "old-b"},
			},
		},
		{
			name: "same target in two slots", count: 2,
			slots: map[int]*structs.DeploymentGroupSelectionSlot{
				0: {TaskGroup: "orin", Cohort: "a"}, 1: {TaskGroup: "orin", Cohort: "b"},
			},
		},
		{
			name: "scale down retains previous-only slot", count: 1, valid: true,
			slots: map[int]*structs.DeploymentGroupSelectionSlot{
				0: {TaskGroup: "thor", Cohort: "new"}, 1: {PreviousTaskGroup: "orin", PreviousCohort: "old"},
			},
		},
		{
			name: "current target exceeds count", count: 1,
			slots: map[int]*structs.DeploymentGroupSelectionSlot{1: {TaskGroup: "thor", Cohort: "new"}},
		},
		{
			name: "non-member target", count: 1,
			slots: map[int]*structs.DeploymentGroupSelectionSlot{0: {TaskGroup: "database", Cohort: "new"}},
		},
		{
			name: "missing cohort", count: 1,
			slots: map[int]*structs.DeploymentGroupSelectionSlot{0: {TaskGroup: "thor"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			job := selectionPlanJob(tc.count)
			selection := &structs.DeploymentGroupSelection{Count: tc.count, Slots: tc.slots}
			reason := validateGroupSelectionClaims(job.GroupSelections[0], selection, nil)
			if tc.valid {
				require.Empty(t, reason)
			} else {
				require.NotEmpty(t, reason)
			}
		})
	}
}
