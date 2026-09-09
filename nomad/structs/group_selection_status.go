// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"slices"
	"strconv"
)

// JobGroupSelectionStatus reports accepted group choices separately from the
// allocations required by those groups. An unresolved choice has no concrete
// allocation count until a group has been selected.
type JobGroupSelectionStatus struct {
	Count    int
	Groups   []string
	Slots    map[string]*JobGroupSelectionSlotStatus
	Unplaced int
}

type JobGroupSelectionSlotStatus struct {
	TaskGroup string
	Cohort    string
}

// GroupSelectionStatuses derives current choices from accepted deployment
// slots or allocation claims. Historical cohorts do not add to desired demand.
func (j *Job) GroupSelectionStatuses(allocs []*Allocation, deployment *Deployment) map[string]*JobGroupSelectionStatus {
	if len(j.GroupSelections) == 0 {
		return nil
	}
	statuses := make(map[string]*JobGroupSelectionStatus, len(j.GroupSelections))
	tracking := deployment != nil && deployment.Active() &&
		deployment.JobCreateIndex == j.CreateIndex && deployment.JobVersion == j.Version
	for _, selection := range j.GroupSelections {
		status := &JobGroupSelectionStatus{
			Count: selection.Count, Groups: slices.Clone(selection.Groups),
			Slots: make(map[string]*JobGroupSelectionSlotStatus), Unplaced: selection.Count,
		}
		statuses[selection.Name] = status
		if tracking {
			if current := deployment.GroupSelections[selection.Name]; current != nil {
				for index, slot := range current.Slots {
					if slot == nil || index < 0 || index >= selection.Count || slot.Cohort == "" ||
						!slices.Contains(selection.Groups, slot.TaskGroup) {
						continue
					}
					if group := j.LookupTaskGroup(slot.TaskGroup); group != nil && group.Count > 0 {
						status.Slots[strconv.Itoa(index)] = &JobGroupSelectionSlotStatus{TaskGroup: slot.TaskGroup, Cohort: slot.Cohort}
					}
				}
			}
		} else {
			incumbents := make(map[int]*Allocation)
			for _, alloc := range allocs {
				claim := alloc.GroupSelection
				if claim == nil || claim.Name != selection.Name || claim.Slot < 0 || claim.Slot >= selection.Count ||
					claim.Cohort == "" || alloc.Job == nil || alloc.Job.CreateIndex != j.CreateIndex ||
					alloc.ServerTerminalStatus() || alloc.DeploymentStatus.IsCanary() ||
					!slices.Contains(selection.Groups, alloc.TaskGroup) {
					continue
				}
				group := j.LookupTaskGroup(alloc.TaskGroup)
				if group == nil || group.Count == 0 {
					continue
				}
				previous := incumbents[claim.Slot]
				if previous == nil || newerGroupSelectionClaim(alloc, previous) {
					incumbents[claim.Slot] = alloc
				}
			}
			for index, alloc := range incumbents {
				status.Slots[strconv.Itoa(index)] = &JobGroupSelectionSlotStatus{TaskGroup: alloc.TaskGroup, Cohort: alloc.GroupSelection.Cohort}
			}
		}
		status.Unplaced -= len(status.Slots)
	}
	return statuses
}

func newerGroupSelectionClaim(candidate, previous *Allocation) bool {
	if candidate.ClientTerminalStatus() != previous.ClientTerminalStatus() {
		return !candidate.ClientTerminalStatus()
	}
	if candidate.Job.Version != previous.Job.Version {
		return candidate.Job.Version > previous.Job.Version
	}
	return candidate.CreateIndex > previous.CreateIndex
}
