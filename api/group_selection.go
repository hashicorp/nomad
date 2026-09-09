// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"slices"
	"time"
)

// TaskGroupSelection selects Count distinct task groups. The selected groups
// retain their own replica counts, constraints, and resource requirements.
type TaskGroupSelection struct {
	Name   string   `hcl:"name,label"`
	Count  *int     `hcl:"count,optional"`
	Groups []string `hcl:"groups"`
}

func (s *TaskGroupSelection) Canonicalize() {
	if s != nil && s.Count == nil {
		s.Count = new(1)
	}
}

func (s *TaskGroupSelection) Copy() *TaskGroupSelection {
	if s == nil {
		return nil
	}
	c := *s
	if s.Count != nil {
		c.Count = new(*s.Count)
	}
	c.Groups = slices.Clone(s.Groups)
	return &c
}

// AllocationGroupSelection identifies the slot and rollout cohort owning an
// allocation. All replicas of a selected task group share this identity.
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

// DeploymentGroupSelection records a selection's desired size and accepted slots.
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

// DeploymentGroupSelectionSlot tracks a selected task group through a rollout.
// Promotion is reported by the target group's DeploymentState.Promoted field.
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
