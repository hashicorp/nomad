// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
)

// TaskGroupSelection selects Count distinct groups from Groups. Each selected
// group retains its own replica count and scheduling requirements.
type TaskGroupSelection struct {
	Name   string
	Count  int
	Groups []string
}

func (s *TaskGroupSelection) Copy() *TaskGroupSelection {
	if s == nil {
		return nil
	}
	c := *s
	c.Groups = slices.Clone(s.Groups)
	return &c
}

func (s *TaskGroupSelection) Equal(other *TaskGroupSelection) bool {
	if s == nil || other == nil {
		return s == other
	}
	return s.Name == other.Name && s.Count == other.Count && slices.Equal(s.Groups, other.Groups)
}

func (s *TaskGroupSelection) Validate() error {
	if s == nil {
		return fmt.Errorf("group selection must not be null")
	}
	var errs multierror.Error
	if s.Name == "" || strings.ContainsRune(s.Name, '\x00') {
		errs.Errors = append(errs.Errors, fmt.Errorf("group selection name must be non-empty and must not contain a null character"))
	}
	if s.Count < 0 {
		errs.Errors = append(errs.Errors, fmt.Errorf("group selection count must be non-negative"))
	}
	if len(s.Groups) == 0 {
		errs.Errors = append(errs.Errors, fmt.Errorf("group selection must name at least one task group"))
	}
	seen := make(map[string]struct{}, len(s.Groups))
	for _, name := range s.Groups {
		if name == "" {
			errs.Errors = append(errs.Errors, fmt.Errorf("group selection task group name must not be empty"))
		}
		if _, exists := seen[name]; exists {
			errs.Errors = append(errs.Errors, fmt.Errorf("task group %q is repeated in group selection", name))
		}
		seen[name] = struct{}{}
	}
	return errs.ErrorOrNil()
}

// LookupGroupSelection finds a named selection in the job.
func (j *Job) LookupGroupSelection(name string) *TaskGroupSelection {
	if j == nil {
		return nil
	}
	for _, selection := range j.GroupSelections {
		if selection != nil && selection.Name == name {
			return selection
		}
	}
	return nil
}

// TaskGroupSelection returns the selection containing the task group, or nil
// for an ordinary required group.
func (j *Job) TaskGroupSelection(group string) *TaskGroupSelection {
	if j == nil {
		return nil
	}
	for _, selection := range j.GroupSelections {
		if selection != nil && slices.Contains(selection.Groups, group) {
			return selection
		}
	}
	return nil
}

func (j *Job) validateGroupSelections() error {
	if len(j.GroupSelections) == 0 {
		return nil
	}
	var errs multierror.Error
	if j.Type != JobTypeService {
		errs.Errors = append(errs.Errors, fmt.Errorf("group selections are only supported for service jobs"))
	}
	names := make(map[string]struct{}, len(j.GroupSelections))
	owners := make(map[string]string)
	for _, selection := range j.GroupSelections {
		if err := selection.Validate(); err != nil {
			errs.Errors = append(errs.Errors, err)
		}
		if selection == nil {
			continue
		}
		if _, exists := names[selection.Name]; exists {
			errs.Errors = append(errs.Errors, fmt.Errorf("group selection %q is defined more than once", selection.Name))
		}
		names[selection.Name] = struct{}{}
		eligible := 0
		for _, name := range selection.Groups {
			if owner, exists := owners[name]; exists {
				if owner != selection.Name {
					errs.Errors = append(errs.Errors, fmt.Errorf("task group %q belongs to both group selections %q and %q", name, owner, selection.Name))
				}
				continue
			}
			owners[name] = selection.Name
			group := j.LookupTaskGroup(name)
			if group == nil {
				errs.Errors = append(errs.Errors, fmt.Errorf("group selection %q references unknown task group %q", selection.Name, name))
				continue
			}
			if group.Count > 0 {
				eligible++
			}
		}
		if selection.Count > eligible {
			errs.Errors = append(errs.Errors, fmt.Errorf("group selection %q count %d exceeds its %d task groups with a positive count", selection.Name, selection.Count, eligible))
		}
	}
	return errs.ErrorOrNil()
}

// AllocationGroupSelection identifies the selection slot and rollout cohort
// owning an allocation. Replicas of a selected group share this identity.
type AllocationGroupSelection struct {
	Name   string
	Slot   int
	Cohort string
}

func (s *AllocationGroupSelection) Copy() *AllocationGroupSelection {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

func (s *AllocationGroupSelection) Equal(other *AllocationGroupSelection) bool {
	if s == nil || other == nil {
		return s == other
	}
	return *s == *other
}

// DeploymentGroupSelection tracks the accepted ownership of a selection's slots.
// Promotion remains authoritative in Deployment.TaskGroups.
type DeploymentGroupSelection struct {
	Count             int
	Slots             map[int]*DeploymentGroupSelectionSlot
	RequireProgressBy time.Time
}

func (s *DeploymentGroupSelection) Copy() *DeploymentGroupSelection {
	if s == nil {
		return nil
	}
	c := *s
	if s.Slots != nil {
		c.Slots = make(map[int]*DeploymentGroupSelectionSlot, len(s.Slots))
		for slot, state := range s.Slots {
			c.Slots[slot] = state.Copy()
		}
	}
	return &c
}

func (s *DeploymentGroupSelection) Equal(other *DeploymentGroupSelection) bool {
	if s == nil || other == nil {
		return s == other
	}
	if s.Count != other.Count || !s.RequireProgressBy.Equal(other.RequireProgressBy) || len(s.Slots) != len(other.Slots) {
		return false
	}
	for slot, state := range s.Slots {
		next, ok := other.Slots[slot]
		if !ok || !state.Equal(next) {
			return false
		}
	}
	return true
}

// DeploymentGroupSelectionSlot records the target and previous ownership during
// a rollout, including changes between different task groups.
type DeploymentGroupSelectionSlot struct {
	TaskGroup         string
	Cohort            string
	PreviousTaskGroup string
	PreviousCohort    string
}

func (s *DeploymentGroupSelectionSlot) Copy() *DeploymentGroupSelectionSlot {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

func (s *DeploymentGroupSelectionSlot) Equal(other *DeploymentGroupSelectionSlot) bool {
	if s == nil || other == nil {
		return s == other
	}
	return *s == *other
}

// groupSelectionProgressDeadline bounds how long an unfilled selection may
// wait. Every eligible candidate receives its configured progress window. A
// candidate without a deadline makes this initial selection window unbounded.
// Once all slots are bound, the selected groups' native deadlines take over.
func (j *Job) groupSelectionProgressDeadline(selection *TaskGroupSelection, now int64) time.Time {
	if selection == nil || selection.Count == 0 {
		return time.Time{}
	}
	var longest time.Duration
	for _, name := range selection.Groups {
		group := j.LookupTaskGroup(name)
		if group == nil || group.Count == 0 {
			continue
		}
		if group.Update == nil || group.Update.ProgressDeadline == 0 {
			return time.Time{}
		}
		longest = max(longest, group.Update.ProgressDeadline)
	}
	if longest == 0 {
		return time.Time{}
	}
	return time.Unix(0, now).Add(longest)
}
