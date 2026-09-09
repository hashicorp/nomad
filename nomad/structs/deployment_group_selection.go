// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

// Placed returns the number of distinct task groups assigned to desired slots.
// Previous-only slots may remain during a scale down, but do not satisfy demand.
func (s *DeploymentGroupSelection) Placed() int {
	if s == nil {
		return 0
	}
	groups := make(map[string]struct{}, len(s.Slots))
	for index, slot := range s.Slots {
		if index < 0 || index >= s.Count || slot == nil || slot.TaskGroup == "" || slot.Cohort == "" {
			continue
		}
		groups[slot.TaskGroup] = struct{}{}
	}
	return len(groups)
}

// HasUnplacedGroupSelections reports demand that cannot be represented by a
// DeploymentState yet because a task group has not been selected for each slot.
func (d *Deployment) HasUnplacedGroupSelections() bool {
	if d == nil {
		return false
	}
	for _, selection := range d.GroupSelections {
		if selection != nil && selection.Placed() < selection.Count {
			return true
		}
	}
	return false
}

// IsGroupSelectionTarget excludes previous cohorts from the target group's
// deployment health. Allocations without selection metadata use the ordinary
// task-group deployment rules.
func (d *Deployment) IsGroupSelectionTarget(taskGroup string, allocation *AllocationGroupSelection) bool {
	if allocation == nil {
		return true
	}
	if d == nil {
		return false
	}
	selection := d.GroupSelections[allocation.Name]
	if selection == nil {
		// Removing a selection makes its surviving groups ordinary required
		// groups. Their retained allocations may still describe the old job.
		return true
	}
	if allocation.Slot < 0 || allocation.Slot >= selection.Count {
		return false
	}
	slot := selection.Slots[allocation.Slot]
	return slot != nil && slot.TaskGroup == taskGroup && slot.Cohort != "" && slot.Cohort == allocation.Cohort
}

// GroupSelectionCohort identifies the target cohort for a task group. The empty
// string denotes a group which is not a target in this deployment.
func (d *Deployment) GroupSelectionCohort(taskGroup string) string {
	if d == nil {
		return ""
	}
	for _, selection := range d.GroupSelections {
		if selection == nil {
			continue
		}
		for index, slot := range selection.Slots {
			if index >= 0 && index < selection.Count && slot != nil && slot.TaskGroup == taskGroup {
				return slot.Cohort
			}
		}
	}
	return ""
}
