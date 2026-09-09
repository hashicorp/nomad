// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import "github.com/hashicorp/nomad/nomad/structs"

// GroupSelectionTarget binds a logical selection slot to its physical task
// group. Previous and target allocations retain their original task groups;
// only the reconciliation target is shared during a cross-group rollout.
type GroupSelectionTarget struct {
	Selection         *structs.AllocationGroupSelection
	PreviousTaskGroup string
	PreviousCohort    string
	PreferredNodeID   string
	Reconnecting      bool
}

// selectionMatrix projects allocation cohorts onto reconciliation targets. It
// does not modify the registered job, allocation names, or task group counts.
func (a *AllocReconciler) selectionMatrix(result *ReconcileResults) allocMatrix {
	m := make(allocMatrix)
	for _, tg := range a.jobState.Job.TaskGroups {
		if a.jobState.Job.TaskGroupSelection(tg.Name) == nil {
			m[tg.Name] = make(allocSet)
		}
	}
	for group := range a.jobState.GroupSelectionTargets {
		m[group] = make(allocSet)
	}
	retired := make(allocSet)
	for _, alloc := range a.jobState.ExistingAllocs {
		// Removing a group from a selection makes it an ordinary required
		// group again. Its allocation can be retained and untagged in place.
		if a.jobState.Job.LookupTaskGroup(alloc.TaskGroup) != nil && a.jobState.Job.TaskGroupSelection(alloc.TaskGroup) == nil {
			m[alloc.TaskGroup][alloc.ID] = alloc
			continue
		}
		if alloc.GroupSelection == nil {
			if a.jobState.Job.TaskGroupSelection(alloc.TaskGroup) == nil {
				if m[alloc.TaskGroup] == nil {
					m[alloc.TaskGroup] = make(allocSet)
				}
				m[alloc.TaskGroup][alloc.ID] = alloc
			} else {
				retired[alloc.ID] = alloc
			}
			continue
		}
		found := false
		for group, target := range a.jobState.GroupSelectionTargets {
			if target.Selection.Name == alloc.GroupSelection.Name && target.Selection.Slot == alloc.GroupSelection.Slot {
				if target.Reconnecting && target.Selection.Cohort != alloc.GroupSelection.Cohort {
					retired[alloc.ID] = alloc
				} else {
					m[group][alloc.ID] = alloc
				}
				found = true
				break
			}
		}
		if !found && !a.preserveScaledDownSelectionCohort(alloc) {
			retired[alloc.ID] = alloc
		}
	}
	if len(retired) != 0 {
		_, stops := retired.filterAndStopAll(a.clusterState)
		result.Stop = append(result.Stop, stops...)
		for _, stop := range stops {
			if result.DesiredTGUpdates == nil {
				result.DesiredTGUpdates = make(map[string]*structs.DesiredUpdates)
			}
			if result.DesiredTGUpdates[stop.Alloc.TaskGroup] == nil {
				result.DesiredTGUpdates[stop.Alloc.TaskGroup] = new(structs.DesiredUpdates)
			}
			result.DesiredTGUpdates[stop.Alloc.TaskGroup].Stop++
		}
	}
	return m
}

func (a *AllocReconciler) recordGroupSelections(result *ReconcileResults) {
	d := a.jobState.DeploymentCurrent
	if d == nil || !d.Active() || len(a.jobState.Job.GroupSelections) == 0 {
		return
	}
	if d.GroupSelections == nil {
		d.GroupSelections = make(map[string]*structs.DeploymentGroupSelection)
	}
	for _, selection := range a.jobState.Job.GroupSelections {
		state := d.GroupSelections[selection.Name]
		if state == nil {
			state = &structs.DeploymentGroupSelection{Slots: make(map[int]*structs.DeploymentGroupSelectionSlot)}
			d.GroupSelections[selection.Name] = state
		}
		state.Count = selection.Count
		if state.Slots == nil {
			state.Slots = make(map[int]*structs.DeploymentGroupSelectionSlot)
		}
	}
	// Only targets contribute task-group deployment demand. An initial
	// placement can lose its entire cohort before it becomes healthy and
	// choose another candidate without leaving an impossible old demand.
	for group := range d.TaskGroups {
		if a.jobState.Job.TaskGroupSelection(group) != nil && a.jobState.GroupSelectionTargets[group] == nil {
			delete(d.TaskGroups, group)
		}
	}
	for group, target := range a.jobState.GroupSelectionTargets {
		state := d.GroupSelections[target.Selection.Name]
		if state == nil {
			continue
		}
		state.Slots[target.Selection.Slot] = &structs.DeploymentGroupSelectionSlot{
			TaskGroup: group, Cohort: target.Selection.Cohort,
			PreviousTaskGroup: target.PreviousTaskGroup, PreviousCohort: target.PreviousCohort,
		}
	}
	for _, state := range d.GroupSelections {
		for slot := range state.Slots {
			if slot >= state.Count {
				delete(state.Slots, slot)
			}
		}
	}
	for _, alloc := range a.jobState.ExistingAllocs {
		if !a.preserveScaledDownSelectionCohort(alloc) {
			continue
		}
		identity := alloc.GroupSelection
		d.GroupSelections[identity.Name].Slots[identity.Slot] = &structs.DeploymentGroupSelectionSlot{
			PreviousTaskGroup: alloc.TaskGroup, PreviousCohort: identity.Cohort,
		}
	}
	result.Deployment = d
}

// A scale-down can reduce the number of target slots while a new release is
// canaried. The removed serving cohorts remain until the surviving candidates
// are promoted; previous-only deployment entries authorize that overlap.
func (a *AllocReconciler) preserveScaledDownSelectionCohort(alloc *structs.Allocation) bool {
	identity := alloc.GroupSelection
	if identity == nil || alloc.Job.CreateIndex != a.jobState.Job.CreateIndex || alloc.ServerTerminalStatus() || alloc.DeploymentStatus.IsCanary() {
		return false
	}
	selection := a.jobState.Job.LookupGroupSelection(identity.Name)
	if selection == nil || selection.Count == 0 || identity.Slot < selection.Count {
		return false
	}
	// When a deployment records the serving cohort, ignore unrelated
	// retained history (including older disconnected cohorts in that slot).
	for _, deployment := range []*structs.Deployment{a.jobState.DeploymentCurrent, a.jobState.DeploymentOld} {
		if deployment == nil {
			continue
		}
		if state := deployment.GroupSelections[identity.Name]; state != nil {
			if slot := state.Slots[identity.Slot]; slot != nil {
				cohort := slot.Cohort
				if slot.TaskGroup == "" {
					cohort = slot.PreviousCohort
				}
				if cohort != "" && cohort != identity.Cohort {
					return false
				}
				break
			}
		}
	}
	for group, target := range a.jobState.GroupSelectionTargets {
		if target.Selection.Name != identity.Name || target.PreviousCohort == "" {
			continue
		}
		tg := a.jobState.Job.LookupTaskGroup(group)
		if tg.Update == nil || tg.Update.IsEmpty() || tg.Update.Canary == 0 {
			continue
		}
		if d := a.jobState.DeploymentCurrent; d != nil {
			if state := d.TaskGroups[group]; state != nil && state.Promoted {
				continue
			}
		}
		return true
	}
	return false
}

// reconciliationName gives an existing replica its name in the target group.
// Physical allocation names remain unchanged, including on stop operations.
func reconciliationName(alloc *structs.Allocation, target string) string {
	if alloc.TaskGroup == target {
		return alloc.Name
	}
	return structs.AllocName(alloc.JobID, target, alloc.Index())
}

func reconciliationNames(allocs allocSet, target string) map[string]struct{} {
	names := make(map[string]struct{}, len(allocs))
	for _, alloc := range allocs {
		names[reconciliationName(alloc, target)] = struct{}{}
	}
	return names
}

// reconcilePreviousOnlySelections reuses ordinary group reconciliation against
// the saved serving version for slots removed by an unpromoted canary update.
// These repairs do not become target deployment demand or application updates.
func (a *AllocReconciler) reconcilePreviousOnlySelections(result *ReconcileResults) {
	if len(a.jobState.Job.GroupSelections) == 0 {
		return
	}
	cohorts := make(map[string]allocSet)
	for _, alloc := range a.jobState.ExistingAllocs {
		if !a.preserveScaledDownSelectionCohort(alloc) {
			continue
		}
		key := alloc.GroupSelection.Cohort
		if cohorts[key] == nil {
			cohorts[key] = make(allocSet)
		}
		cohorts[key][alloc.ID] = alloc
	}
	for _, all := range cohorts {
		var representative *structs.Allocation
		for _, alloc := range all {
			representative = alloc
			break
		}
		savedJob := representative.Job
		group := representative.TaskGroup
		projection := savedJob.Copy()
		projection.GroupSelections = nil
		projection.LookupTaskGroup(group).Update = &structs.UpdateStrategy{}
		child := NewAllocReconciler(a.logger, func(*structs.Allocation, *structs.Job, *structs.TaskGroup) (bool, bool, *structs.Allocation) {
			return true, false, nil
		},
			ReconcilerState{Job: projection, JobID: a.jobState.JobID, EvalID: a.jobState.EvalID, EvalPriority: a.jobState.EvalPriority}, a.clusterState)
		repairs, _ := child.computeGroup(group, all)
		source := &GroupSelectionSource{Job: savedJob, Selection: representative.GroupSelection.Copy(), NameIndex: repairs.TaskGroupAllocNameIndexes[group]}
		placements := repairs.Place[:0]
		for _, placement := range repairs.Place {
			if placement.previousAlloc == nil {
				index := structs.AllocIndexFromName(placement.name, savedJob.ID, group)
				for _, alloc := range all {
					if alloc.Index() == index {
						placement.previousAlloc = alloc
						break
					}
				}
			}
			// Recovery must have a recorded source allocation; it cannot create an
			// unrelated new claim outside the target selection count.
			if placement.previousAlloc == nil {
				continue
			}
			placement.source = source
			placement.taskGroup = savedJob.LookupTaskGroup(group)
			placements = append(placements, placement)
		}
		repairs.Place = placements
		for name, updates := range repairs.DesiredTGUpdates {
			if previous := result.DesiredTGUpdates[name]; previous != nil {
				updates.Ignore += previous.Ignore
				updates.Place += previous.Place
				updates.Migrate += previous.Migrate
				updates.Stop += previous.Stop
				updates.InPlaceUpdate += previous.InPlaceUpdate
				updates.DestructiveUpdate += previous.DestructiveUpdate
				updates.Canary += previous.Canary
				updates.Preemptions += previous.Preemptions
				updates.Disconnect += previous.Disconnect
				updates.Reconnect += previous.Reconnect
				updates.RescheduleNow += previous.RescheduleNow
				updates.RescheduleLater += previous.RescheduleLater
			}
		}
		for name, evals := range result.DesiredFollowupEvals {
			repairs.DesiredFollowupEvals[name] = append(repairs.DesiredFollowupEvals[name], evals...)
		}
		repairs.TaskGroupAllocNameIndexes = nil // placements carry the source index
		result.Merge(repairs)
	}
}
