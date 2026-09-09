// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// evaluatePlanGroupSelections checks the job-wide result after node admission.
// Checking nodes independently cannot detect competing claims for a selection
// slot, and a rejected stop may leave a previous claim in place.
func evaluatePlanGroupSelections(snap *state.StateSnapshot, plan *structs.Plan, result *structs.PlanResult) (bool, string, error) {
	if plan.Job == nil || len(plan.Job.GroupSelections) == 0 || result.IsNoOp() {
		return true, "", nil
	}

	existing, err := snap.AllocsByJob(nil, plan.Job.Namespace, plan.Job.ID, false)
	if err != nil {
		return false, "", err
	}
	projected := projectGroupSelectionAllocs(existing, result, plan.Job)
	existingIDs := make(map[string]struct{}, len(existing))
	committedAllocs := make(map[string]*structs.Allocation, len(existing))
	for _, alloc := range existing {
		existingIDs[alloc.ID] = struct{}{}
		committedAllocs[alloc.ID] = alloc
	}

	var committed *structs.Deployment
	if result.Deployment != nil {
		committed, err = snap.DeploymentByID(nil, result.Deployment.ID)
		if err != nil {
			return false, "", err
		}
		correctDeploymentGroupSelections(result.Deployment, committed, projected)
	}

	// Allocations created before a selection was added to the job may remain
	// during a rolling update. New member allocations must carry a claim.
	for _, allocs := range result.NodeAllocation {
		for _, allocation := range allocs {
			alloc := projected[allocation.ID]
			if alloc == nil {
				continue
			}
			if alloc.TerminalStatus() {
				continue
			}
			selection := plan.Job.TaskGroupSelection(alloc.TaskGroup)
			if selection == nil {
				if alloc.GroupSelection != nil {
					return false, fmt.Sprintf("allocation %q claims a selection for non-member group %q", alloc.ID, alloc.TaskGroup), nil
				}
				continue
			}
			claim := alloc.GroupSelection
			if claim == nil {
				if _, ok := existingIDs[alloc.ID]; ok {
					continue
				}
				return false, fmt.Sprintf("allocation %q is missing its group selection claim", alloc.ID), nil
			}
			_, alreadyPlaced := existingIDs[alloc.ID]
			if claim.Name != selection.Name || claim.Slot < 0 || claim.Cohort == "" ||
				(claim.Slot >= selection.Count && !alreadyPlaced && !restoresPreviousSelection(alloc, committedAllocs, result.Deployment)) {
				return false, fmt.Sprintf("allocation %q has an invalid group selection claim", alloc.ID), nil
			}
		}
	}

	deployment := result.Deployment
	if deployment == nil {
		deployment, err = snap.LatestDeploymentByJobID(nil, plan.Job.Namespace, plan.Job.ID)
		if err != nil {
			return false, "", err
		}
		if deployment != nil && !deployment.Active() {
			deployment = nil
		}
		for _, update := range result.DeploymentUpdates {
			if deployment != nil && update.DeploymentID == deployment.ID &&
				update.Status == structs.DeploymentStatusCancelled {
				deployment = nil
			}
		}
	}

	claims, reason, err := activeGroupSelectionClaims(snap, committedAllocs, projected)
	if err != nil || reason != "" {
		return false, reason, err
	}
	for _, selection := range plan.Job.GroupSelections {
		var selected *structs.DeploymentGroupSelection
		if deployment != nil {
			selected = deployment.GroupSelections[selection.Name]
		}
		if selected == nil && result.Deployment != nil {
			return false, fmt.Sprintf("deployment is missing group selection %q", selection.Name), nil
		}
		if selected == nil {
			// Jobs without deployment tracking still need cross-node exclusion.
			selected = &structs.DeploymentGroupSelection{
				Count: selection.Count,
				Slots: make(map[int]*structs.DeploymentGroupSelectionSlot),
			}
			for _, alloc := range claims {
				claim := alloc.GroupSelection
				if claim == nil || claim.Name != selection.Name {
					continue
				}
				slot := selected.Slots[claim.Slot]
				if slot == nil {
					selected.Slots[claim.Slot] = &structs.DeploymentGroupSelectionSlot{
						TaskGroup: alloc.TaskGroup,
						Cohort:    claim.Cohort,
					}
				} else if slot.TaskGroup != alloc.TaskGroup || slot.Cohort != claim.Cohort {
					return false, fmt.Sprintf("group selection %q slot %d has competing claims", selection.Name, claim.Slot), nil
				}
			}
		}
		if reason := validateGroupSelectionClaims(selection, selected, claims); reason != "" {
			return false, reason, nil
		}
	}
	return true, "", nil
}

// restoresPreviousSelection admits recovery of a serving cohort retained by a
// scale-down canary. It must replace a recorded allocation using that cohort's
// original job, rather than create new demand outside the current slot count.
func restoresPreviousSelection(alloc *structs.Allocation, existing map[string]*structs.Allocation, deployment *structs.Deployment) bool {
	previous := existing[alloc.PreviousAllocation]
	if deployment == nil || !sameSelectionSlot(alloc, previous) || alloc.TaskGroup != previous.TaskGroup ||
		alloc.GroupSelection.Cohort != previous.GroupSelection.Cohort ||
		alloc.Job.Version != previous.Job.Version || alloc.Job.JobModifyIndex != previous.Job.JobModifyIndex ||
		alloc.Job.SpecChanged(previous.Job) ||
		alloc.DeploymentStatus.IsCanary() {
		return false
	}
	selection := deployment.GroupSelections[alloc.GroupSelection.Name]
	if selection == nil {
		return false
	}
	slot := selection.Slots[alloc.GroupSelection.Slot]
	return slot != nil && slot.PreviousTaskGroup == alloc.TaskGroup && slot.PreviousCohort == alloc.GroupSelection.Cohort
}

func sameSelectionSlot(alloc, previous *structs.Allocation) bool {
	return alloc != nil && previous != nil && alloc.Job != nil && previous.Job != nil &&
		alloc.GroupSelection != nil && previous.GroupSelection != nil &&
		alloc.JobID == previous.JobID && alloc.Namespace == previous.Namespace &&
		alloc.Job.CreateIndex == previous.Job.CreateIndex &&
		alloc.GroupSelection.Name == previous.GroupSelection.Name && alloc.GroupSelection.Slot == previous.GroupSelection.Slot
}

// activeGroupSelectionClaims excludes only ancestors of native disconnected
// replacements. A chain can retain several unknown cohorts, but unrelated or
// branching replacement claims still compete for the same logical slot.
func activeGroupSelectionClaims(snap *state.StateSnapshot, existing, projected map[string]*structs.Allocation) (map[string]*structs.Allocation, string, error) {
	type cohort struct {
		name, group, id string
		slot            int
	}
	key := func(alloc *structs.Allocation) cohort {
		return cohort{alloc.GroupSelection.Name, alloc.TaskGroup, alloc.GroupSelection.Cohort, alloc.GroupSelection.Slot}
	}
	successors := make(map[cohort]cohort)
	queue := make([]*structs.Allocation, 0, len(projected))
	for _, alloc := range projected {
		queue = append(queue, alloc)
	}
	visited := make(map[string]bool)
	for len(queue) > 0 {
		alloc := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if visited[alloc.ID] {
			continue
		}
		visited[alloc.ID] = true
		previous := existing[alloc.PreviousAllocation]
		if !sameSelectionSlot(alloc, previous) ||
			alloc.DeploymentStatus.IsCanary() || previous.DeploymentStatus.IsCanary() {
			continue
		}
		// Once committed, the native predecessor link survives reconnects.
		// An in-place update cannot manufacture a historical authorization.
		saved := existing[alloc.ID]
		historical := saved != nil && saved.PreviousAllocation == previous.ID &&
			saved.TaskGroup == alloc.TaskGroup && saved.GroupSelection.Equal(alloc.GroupSelection)
		if historical {
			queue = append(queue, previous)
		}
		if alloc.GroupSelection.Cohort == previous.GroupSelection.Cohort {
			continue
		}
		if !historical {
			allowed, err := disconnectedSelectionReplacement(snap, previous)
			if err != nil {
				return nil, "", err
			}
			if !allowed && previous.ClientTerminalStatus() && sameSelectionSlot(previous, existing[previous.PreviousAllocation]) {
				// A failed replacement may itself be rescheduled while an older
				// disconnected ancestor remains. Its native failure policy must
				// allow that retry, and no live replica may still own its cohort.
				when, eligible := previous.NextRescheduleTime()
				allowed = eligible && !when.After(time.Now().Add(time.Second))
				for _, current := range projected {
					if sameSelectionSlot(current, previous) && key(current) == key(previous) {
						allowed = false
						break
					}
				}
			}
			if !allowed {
				continue
			}
			if next := projected[previous.NextAllocation]; next != nil && next.ID != alloc.ID {
				return nil, fmt.Sprintf("allocation %q already has a live disconnected replacement", previous.ID), nil
			}
		}
		queue = append(queue, previous)
		from, to := key(previous), key(alloc)
		if next, ok := successors[from]; ok && next != to {
			return nil, fmt.Sprintf("group selection %q slot %d has competing disconnected replacements", from.name, from.slot), nil
		}
		successors[from] = to
	}
	for start := range successors {
		seen := make(map[cohort]bool)
		for current := start; ; {
			if seen[current] {
				return nil, fmt.Sprintf("group selection %q slot %d has a cyclic replacement chain", start.name, start.slot), nil
			}
			seen[current] = true
			next, ok := successors[current]
			if !ok {
				break
			}
			current = next
		}
	}
	active := make(map[string]*structs.Allocation, len(projected))
	for id, alloc := range projected {
		if alloc.GroupSelection != nil {
			if _, replaced := successors[key(alloc)]; replaced {
				continue
			}
		}
		active[id] = alloc
	}
	return active, "", nil
}

func disconnectedSelectionReplacement(snap *state.StateSnapshot, previous *structs.Allocation) (bool, error) {
	if !previous.ReplaceOnDisconnect() || previous.TerminalStatus() {
		return false, nil
	}
	node, err := snap.NodeByID(nil, previous.NodeID)
	if err != nil || node == nil || node.Status != structs.NodeStatusDisconnected {
		return false, err
	}
	from := previous.LastUnknown()
	if from.IsZero() {
		from = time.Now()
	}
	when, eligible := previous.NextRescheduleTimeByTime(from)
	return eligible && !when.After(time.Now().Add(time.Second)), nil
}

// projectGroupSelectionAllocs includes only changes accepted by node admission.
// Allocation IDs also cover in-place updates and disconnected-client updates.
func projectGroupSelectionAllocs(existing []*structs.Allocation, result *structs.PlanResult, job *structs.Job) map[string]*structs.Allocation {
	projected := make(map[string]*structs.Allocation, len(existing))
	for _, alloc := range existing {
		if !alloc.TerminalStatus() {
			projected[alloc.ID] = alloc
		}
	}
	for _, allocs := range result.NodeUpdate {
		for _, alloc := range allocs {
			delete(projected, alloc.ID)
		}
	}
	for _, allocs := range result.NodePreemptions {
		for _, alloc := range allocs {
			delete(projected, alloc.ID)
		}
	}
	for _, allocs := range result.NodeAllocation {
		for _, alloc := range allocs {
			delete(projected, alloc.ID)
			if !alloc.TerminalStatus() {
				// Normal placements inherit Plan.Job; only downgraded placements
				// carry an allocation-level job on the wire. Do not mutate the plan.
				if alloc.Job == nil {
					copy := *alloc
					copy.Job = job
					alloc = &copy
				}
				projected[alloc.ID] = alloc
			}
		}
	}
	return projected
}

// correctDeploymentGroupSelections discards assignments that lost all their
// placements during node admission. Existing assignments are retained so a
// partial plan cannot erase a claim committed by another scheduler worker.
func correctDeploymentGroupSelections(deployment, committed *structs.Deployment, projected map[string]*structs.Allocation) {
	type placement struct {
		selection string
		slot      int
		group     string
		cohort    string
	}
	placed := make(map[placement]struct{}, len(projected))
	rejectedGroups := make(map[string]struct{})
	for _, alloc := range projected {
		if claim := alloc.GroupSelection; claim != nil {
			placed[placement{claim.Name, claim.Slot, alloc.TaskGroup, claim.Cohort}] = struct{}{}
		}
	}
	for name, selection := range deployment.GroupSelections {
		if selection == nil {
			continue
		}
		for index, slot := range selection.Slots {
			if slot == nil || slot.TaskGroup == "" {
				continue
			}
			var previous *structs.DeploymentGroupSelectionSlot
			if committed != nil && committed.GroupSelections[name] != nil {
				previous = committed.GroupSelections[name].Slots[index]
			}
			if previous != nil && previous.TaskGroup == slot.TaskGroup && previous.Cohort == slot.Cohort {
				continue
			}
			if _, ok := placed[placement{name, index, slot.TaskGroup, slot.Cohort}]; !ok {
				rejectedGroups[slot.TaskGroup] = struct{}{}
				if previous == nil {
					if slot.PreviousTaskGroup == "" {
						delete(selection.Slots, index)
					} else {
						selection.Slots[index] = &structs.DeploymentGroupSelectionSlot{
							PreviousTaskGroup: slot.PreviousTaskGroup,
							PreviousCohort:    slot.PreviousCohort,
						}
					}
				} else {
					selection.Slots[index] = previous.Copy()
					if group := committed.TaskGroups[previous.TaskGroup]; group != nil {
						if deployment.TaskGroups == nil {
							deployment.TaskGroups = make(map[string]*structs.DeploymentState)
						}
						deployment.TaskGroups[previous.TaskGroup] = group.Copy()
					}
				}
			}
		}
	}
	// Deployment health and promotion must follow the corrected choices too.
	// A group retained by another slot still owns its deployment state.
	for _, selection := range deployment.GroupSelections {
		if selection == nil {
			continue
		}
		for _, slot := range selection.Slots {
			if slot != nil {
				delete(rejectedGroups, slot.TaskGroup)
			}
		}
	}
	for group := range rejectedGroups {
		delete(deployment.TaskGroups, group)
	}
}

func validateGroupSelectionClaims(selection *structs.TaskGroupSelection, selected *structs.DeploymentGroupSelection, projected map[string]*structs.Allocation) string {
	if selected.Count != selection.Count {
		return fmt.Sprintf("group selection %q has a stale count", selection.Name)
	}
	members := make(map[string]struct{}, len(selection.Groups))
	for _, group := range selection.Groups {
		members[group] = struct{}{}
	}
	groups := make(map[string]int, len(selected.Slots))
	for index, slot := range selected.Slots {
		if slot == nil {
			continue
		}
		if index < 0 || (slot.TaskGroup == "") != (slot.Cohort == "") ||
			(slot.PreviousTaskGroup == "") != (slot.PreviousCohort == "") {
			return fmt.Sprintf("group selection %q has an invalid slot %d", selection.Name, index)
		}
		if slot.TaskGroup == "" {
			// Previous-only slots may outlive a decrease in the desired count
			// while their allocations are drained after promotion.
			continue
		}
		if index >= selection.Count {
			return fmt.Sprintf("group selection %q slot %d exceeds its count", selection.Name, index)
		}
		if _, ok := members[slot.TaskGroup]; !ok {
			return fmt.Sprintf("group selection %q selects non-member group %q", selection.Name, slot.TaskGroup)
		}
		if other, ok := groups[slot.TaskGroup]; ok {
			return fmt.Sprintf("group selection %q selects group %q in slots %d and %d", selection.Name, slot.TaskGroup, other, index)
		}
		groups[slot.TaskGroup] = index
	}
	for _, alloc := range projected {
		claim := alloc.GroupSelection
		if claim == nil || claim.Name != selection.Name {
			continue
		}
		slot := selected.Slots[claim.Slot]
		if slot != nil && ((alloc.TaskGroup == slot.TaskGroup && claim.Cohort == slot.Cohort) ||
			(alloc.TaskGroup == slot.PreviousTaskGroup && claim.Cohort == slot.PreviousCohort)) {
			continue
		}
		return fmt.Sprintf("allocation %q conflicts with group selection %q slot %d", alloc.ID, selection.Name, claim.Slot)
	}
	return ""
}
