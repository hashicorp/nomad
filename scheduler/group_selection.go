// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"maps"
	"slices"
	"sort"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/feasible"
	"github.com/hashicorp/nomad/scheduler/reconciler"
)

// prepareGroupSelections resolves physical reconciliation targets while keeping
// placement ownership separate from the immutable job specification. Preview
// offers reserve resources against later previews, but never reach the leader.
func (s *GenericScheduler) prepareGroupSelections(allocs []*structs.Allocation, tainted map[string]*structs.Node) ([]*structs.Allocation, error) {
	s.groupSelectionTargets = nil
	if len(s.job.GroupSelections) == 0 || s.job.Stopped() {
		return allocs, nil
	}
	s.groupSelectionTargets = make(map[string]*reconciler.GroupSelectionTarget)
	allocs = slices.Clone(allocs)
	sort.SliceStable(allocs, func(i, j int) bool {
		if allocs[i].Job.Version != allocs[j].Job.Version {
			return allocs[i].Job.Version > allocs[j].Job.Version
		}
		return allocs[i].CreateIndex < allocs[j].CreateIndex
	})

	// Adding a selection to a running job adopts existing selected groups.
	// The original Job remains attached to the metadata update so adoption is
	// not itself an application update.
	for _, selection := range s.job.GroupSelections {
		adopted := make(map[string]*structs.AllocationGroupSelection)
		taken := make(map[int]bool)
		for _, alloc := range allocs {
			if alloc.GroupSelection != nil && alloc.GroupSelection.Name == selection.Name && !alloc.TerminalStatus() {
				adopted[alloc.TaskGroup] = alloc.GroupSelection
				taken[alloc.GroupSelection.Slot] = true
			}
		}
		for index, alloc := range allocs {
			if alloc.GroupSelection != nil || alloc.TerminalStatus() || !slices.Contains(selection.Groups, alloc.TaskGroup) {
				continue
			}
			identity := adopted[alloc.TaskGroup]
			if identity == nil {
				slot := 0
				for taken[slot] {
					slot++
				}
				if slot >= selection.Count {
					continue
				}
				identity = &structs.AllocationGroupSelection{Name: selection.Name, Slot: slot, Cohort: uuid.Generate()}
				adopted[alloc.TaskGroup] = identity
				taken[slot] = true
			}
			copy := alloc.Copy()
			copy.GroupSelection = identity.Copy()
			allocs[index] = copy
			s.plan.AppendAlloc(copy.Copy(), alloc.Job)
		}
	}

	original := s.plan.NodeAllocation
	s.plan.NodeAllocation = maps.Clone(original)
	for node, values := range s.plan.NodeAllocation {
		s.plan.NodeAllocation[node] = slices.Clone(values)
	}
	originalPreemptions := s.plan.NodePreemptions
	s.plan.NodePreemptions = maps.Clone(originalPreemptions)
	if s.plan.NodePreemptions == nil {
		s.plan.NodePreemptions = make(map[string][]*structs.Allocation)
	}
	for node, values := range s.plan.NodePreemptions {
		s.plan.NodePreemptions[node] = slices.Clone(values)
	}
	defer func() { s.plan.NodeAllocation = original; s.plan.NodePreemptions = originalPreemptions }()
	_, _, err := s.setNodes(s.job)
	if err != nil {
		return nil, err
	}
	usedGroups := make(map[string]bool)
	// A free earlier slot must not steal an implementation still serving a
	// later slot. Reserve all viable incumbent claims before choosing offers.
	reservedGroups := make(map[string]*structs.AllocationGroupSelection)
	for _, alloc := range allocs {
		identity := alloc.GroupSelection
		if identity == nil || alloc.ServerTerminalStatus() || !s.selectionAllocationAvailable(alloc, tainted) {
			continue
		}
		selection := s.job.LookupGroupSelection(identity.Name)
		if selection != nil && identity.Slot < selection.Count && slices.Contains(selection.Groups, alloc.TaskGroup) {
			reservedGroups[alloc.TaskGroup] = identity
		}
	}
	for _, selection := range s.job.GroupSelections {
		for slot := 0; slot < selection.Count; slot++ {
			var incumbent *structs.Allocation
			var current *structs.DeploymentGroupSelectionSlot
			if s.deployment != nil && s.deployment.JobVersion == s.job.Version && s.deployment.Active() {
				if state := s.deployment.GroupSelections[selection.Name]; state != nil {
					current = state.Slots[slot]
				}
			}
			var slotAllocs []*structs.Allocation
			for _, alloc := range allocs {
				identity := alloc.GroupSelection
				if identity == nil || identity.Name != selection.Name || identity.Slot != slot || alloc.Job.CreateIndex != s.job.CreateIndex {
					continue
				}
				slotAllocs = append(slotAllocs, alloc)
				if alloc.ServerTerminalStatus() || alloc.DeploymentStatus.IsCanary() {
					continue
				}
				if incumbent == nil || (!s.selectionAllocationAvailable(incumbent, tainted) && s.selectionAllocationAvailable(alloc, tainted)) || (incumbent.ClientTerminalStatus() && !alloc.ClientTerminalStatus()) {
					incumbent = alloc
				}
			}
			reconnecting := false
			if preferred := s.reconnectingSelectionCohort(selection, slotAllocs, tainted); preferred != nil {
				incumbent, reconnecting = preferred, true
			}
			// An accepted target remains fixed for the rollout. This is distinct
			// from a preview-only mapping which had no allocation accepted.
			if current != nil && current.TaskGroup != "" {
				accepted := false
				for _, alloc := range slotAllocs {
					if alloc.GroupSelection.Cohort == current.Cohort && !alloc.ServerTerminalStatus() && s.selectionAllocationAvailable(alloc, tainted) {
						accepted = true
						break
					}
				}
				if accepted && !reconnecting && !usedGroups[current.TaskGroup] && s.job.LookupTaskGroup(current.TaskGroup) != nil {
					s.groupSelectionTargets[current.TaskGroup] = &reconciler.GroupSelectionTarget{
						Selection:         &structs.AllocationGroupSelection{Name: selection.Name, Slot: slot, Cohort: current.Cohort},
						PreviousTaskGroup: current.PreviousTaskGroup, PreviousCohort: current.PreviousCohort,
					}
					usedGroups[current.TaskGroup] = true
					continue
				}
			}

			keep := incumbent != nil && !incumbent.ClientTerminalStatus() && s.selectionAllocationAvailable(incumbent, tainted)
			if incumbent != nil {
				tg := s.job.LookupTaskGroup(incumbent.TaskGroup)
				if tg == nil || tg.Count == 0 || !slices.Contains(selection.Groups, incumbent.TaskGroup) {
					keep = false
				} else if incumbent.Job.JobModifyIndex != s.job.JobModifyIndex && incumbent.Job.LookupTaskGroup(tg.Name) != nil {
					if tasksUpdated(s.job, incumbent.Job, tg.Name).modified {
						keep = false
					} else if keep {
						// In-place changes can invalidate the incumbent's node (for
						// example a constraint change). Reuse the native check before
						// fixing the profile, including the control-only fast path.
						_, destructive, _ := s.groupSelectionAllocUpdateFn()(incumbent, s.job, tg)
						keep = !destructive
						if _, _, err := s.setNodes(s.job); err != nil {
							return nil, err
						}
					}
				} else if node := tainted[incumbent.NodeID]; node != nil && (node.Status == structs.NodeStatusDown || node.DrainStrategy != nil) {
					keep = false
				}
				if incumbent.ClientStatus == structs.AllocClientStatusFailed {
					when, eligible := incumbent.NextRescheduleTime()
					if !eligible || (when.After(time.Now().Add(time.Second)) && incumbent.FollowupEvalID != s.eval.ID) {
						keep = true
					}
				}
			}
			if keep && !usedGroups[incumbent.TaskGroup] {
				s.groupSelectionTargets[incumbent.TaskGroup] = &reconciler.GroupSelectionTarget{Selection: incumbent.GroupSelection.Copy(), Reconnecting: reconnecting}
				usedGroups[incumbent.TaskGroup] = true
				continue
			}

			candidates := slices.Clone(selection.Groups)
			if incumbent != nil && slices.Contains(candidates, incumbent.TaskGroup) {
				candidates = append([]string{incumbent.TaskGroup}, slices.DeleteFunc(candidates, func(name string) bool { return name == incumbent.TaskGroup })...)
			}
			var chosen *structs.TaskGroup
			var offer *feasible.RankedNode
			// Exhaust non-preempting choices before considering preemption.
			for _, preempt := range []bool{false, true} {
				if preempt {
					_, config, _ := s.ctx.State().SchedulerConfig()
					if config != nil && !config.PreemptionConfig.ServiceSchedulerEnabled {
						break
					}
				}
				for _, name := range candidates {
					tg := s.job.LookupTaskGroup(name)
					if owner := reservedGroups[name]; owner != nil && (owner.Name != selection.Name || owner.Slot != slot) {
						continue
					}
					if tg == nil || tg.Count == 0 || usedGroups[name] {
						continue
					}
					options := getSelectOptions(incumbent, nil)
					options.Preempt = preempt
					options.AllocName = structs.AllocName(s.job.ID, tg.Name, 0)
					if possible := s.stack.Select(tg, options); possible != nil {
						chosen, offer = tg, possible
						break
					}
				}
				if chosen != nil {
					break
				}
			}
			if chosen == nil {
				// Retain a concrete pending placement so that ordinary blocked
				// evaluations retry this selection when capacity changes.
				for _, name := range candidates {
					tg := s.job.LookupTaskGroup(name)
					if owner := reservedGroups[name]; owner != nil && (owner.Name != selection.Name || owner.Slot != slot) {
						continue
					}
					if tg != nil && tg.Count > 0 && !usedGroups[name] {
						chosen = tg
						break
					}
				}
			}
			if chosen == nil {
				continue
			}
			target := &reconciler.GroupSelectionTarget{Selection: &structs.AllocationGroupSelection{Name: selection.Name, Slot: slot, Cohort: uuid.Generate()}}
			if incumbent != nil {
				target.PreviousTaskGroup = incumbent.TaskGroup
				target.PreviousCohort = incumbent.GroupSelection.Cohort
			}
			if offer != nil {
				target.PreferredNodeID = offer.Node.ID
				s.appendGroupSelectionPreview(chosen, target, offer, 0)
				previewCount := chosen.Count
				if incumbent != nil && !incumbent.ClientTerminalStatus() && s.selectionAllocationAvailable(incumbent, tainted) && !chosen.Update.IsEmpty() {
					previewCount = chosen.Update.MaxParallel
					if chosen.Update.Canary > 0 {
						previewCount = chosen.Update.Canary
					}
				}
				// Reserve every currently placeable replica in the preview so a
				// later slot sees the same resources as ordinary group placement.
				// A partial offer still selects the group: this is not a gang.
				for replica := 1; replica < previewCount; replica++ {
					options := getSelectOptions(incumbent, nil)
					options.AllocName = structs.AllocName(s.job.ID, chosen.Name, uint(replica))
					next := s.selectNextOption(chosen, options)
					if next == nil {
						break
					}
					s.appendGroupSelectionPreview(chosen, target, next, uint(replica))
				}
			}

			s.groupSelectionTargets[chosen.Name] = target
			usedGroups[chosen.Name] = true
		}
	}
	return allocs, nil
}

// groupSelectionAllocUpdateFn keeps selection-control changes from rechecking
// placement constraints on an otherwise unchanged running allocation. Dynamic
// availability metadata can correctly become false after the allocation starts.
func (s *GenericScheduler) groupSelectionAllocUpdateFn() reconciler.AllocUpdateType {
	ordinary := genericAllocUpdateFn(s.ctx, s.stack, s.eval.ID)
	return func(existing *structs.Allocation, job *structs.Job, group *structs.TaskGroup) (bool, bool, *structs.Allocation) {
		if existing.GroupSelection != nil && existing.TaskGroup == group.Name && existing.Job.JobModifyIndex != job.JobModifyIndex {
			projected := job.Copy()
			projected.GroupSelections = existing.Job.GroupSelections
			for _, tg := range projected.TaskGroups {
				if previous := existing.Job.LookupTaskGroup(tg.Name); previous != nil {
					tg.Count = previous.Count
				}
			}
			if !existing.Job.SpecChanged(projected) {
				update := existing.Copy()
				update.Job = nil
				update.EvalID = s.eval.ID
				if job.TaskGroupSelection(group.Name) == nil {
					update.GroupSelection = nil
				}
				return false, false, update
			}
		}
		ignore, destructive, update := ordinary(existing, job, group)
		if existing.GroupSelection != nil && job.TaskGroupSelection(group.Name) == nil && !destructive {
			if update == nil {
				update = existing.Copy()
				update.Job = nil
				update.EvalID = s.eval.ID
			}
			update.GroupSelection = nil
			ignore = false
		}
		return ignore, destructive, update
	}
}

// selectionAllocationAvailable distinguishes a committed claim from a cohort
// whose nodes can no longer run it. Unknown/disconnected allocations remain
// claims until the native disconnect reconciliation permits replacement.
func (s *GenericScheduler) selectionAllocationAvailable(alloc *structs.Allocation, tainted map[string]*structs.Node) bool {
	now := time.Now()
	if alloc.ClientStatus == structs.AllocClientStatusLost || alloc.Expired(now) {
		return false
	}
	if node := tainted[alloc.NodeID]; node != nil {
		if node.Status == structs.NodeStatusDown || node.DrainStrategy != nil {
			return false
		}
		if node.Status == structs.NodeStatusDisconnected {
			if alloc.ClientStatus == structs.AllocClientStatusPending || alloc.DisconnectTimeout(now).Equal(now) {
				return false
			}
			if alloc.ReplaceOnDisconnect() {
				from := alloc.LastUnknown()
				if from.IsZero() {
					from = now
				}
				when, eligible := alloc.NextRescheduleTimeByTime(from)
				if eligible && (alloc.FollowupEvalID == s.eval.ID || !when.After(now.Add(time.Second))) {
					return false
				}
			}
		}
	}
	return true
}

// reconnectingSelectionCohort lifts the native per-replica reconnect choice to
// a single implementation for the slot. Retired or superseded allocations
// cannot reclaim a slot from the current release.
func (s *GenericScheduler) reconnectingSelectionCohort(selection *structs.TaskGroupSelection, allocations []*structs.Allocation, tainted map[string]*structs.Node) *structs.Allocation {
	byID := make(map[string]*structs.Allocation, len(allocations))
	for _, alloc := range allocations {
		byID[alloc.ID] = alloc
	}
	for _, original := range allocations {
		if !original.NeedsToReconnect() || original.ClientStatus != structs.AllocClientStatusRunning || original.ServerTerminalStatus() || original.Job.Version != s.job.Version || original.Job.CreateIndex != s.job.CreateIndex || original.DesiredTransition.ShouldMigrate() || original.DesiredTransition.ShouldReschedule() || original.DesiredTransition.ShouldForceReschedule() {
			continue
		}
		if node := tainted[original.NodeID]; node != nil && (node.Status == structs.NodeStatusDisconnected || node.Status == structs.NodeStatusDown || node.DrainStrategy != nil) {
			continue
		}
		if !slices.Contains(selection.Groups, original.TaskGroup) || s.job.LookupTaskGroup(original.TaskGroup).Count == 0 {
			continue
		}
		replacement := byID[original.NextAllocation]
		seen := make(map[string]bool)
		for replacement != nil && replacement.NextAllocation != "" && !seen[replacement.ID] {
			seen[replacement.ID] = true
			next := byID[replacement.NextAllocation]
			if next == nil {
				break
			}
			replacement = next
		}
		if replacement == nil || replacement.ServerTerminalStatus() || replacement.GroupSelection.Cohort == original.GroupSelection.Cohort {
			continue
		}
		var oldCohort, newCohort []*structs.Allocation
		for _, alloc := range allocations {
			if alloc.ServerTerminalStatus() {
				continue
			}
			switch alloc.GroupSelection.Cohort {
			case original.GroupSelection.Cohort:
				oldCohort = append(oldCohort, alloc)
			case replacement.GroupSelection.Cohort:
				newCohort = append(newCohort, alloc)
			}
		}
		if reconciler.PickGroupSelectionCohort(original.Job.LookupTaskGroup(original.TaskGroup).Disconnect, oldCohort, newCohort) {
			return original
		}
		return replacement
	}
	return nil
}

func (s *GenericScheduler) appendGroupSelectionPreview(group *structs.TaskGroup, target *reconciler.GroupSelectionTarget, offer *feasible.RankedNode, index uint) {
	resources := &structs.AllocatedResources{Tasks: offer.TaskResources, TaskLifecycles: offer.TaskLifecycles,
		Shared: structs.AllocatedSharedResources{DiskMB: int64(group.EphemeralDisk.SizeMB)}}
	if offer.AllocResources != nil {
		resources.Shared.Networks = offer.AllocResources.Networks
		resources.Shared.Ports = offer.AllocResources.Ports
	}
	preview := &structs.Allocation{ID: uuid.Generate(), JobID: s.job.ID, Namespace: s.job.Namespace, TaskGroup: group.Name,
		Name: structs.AllocName(s.job.ID, group.Name, index), NodeID: offer.Node.ID,
		DesiredStatus: structs.AllocDesiredStatusRun, ClientStatus: structs.AllocClientStatusPending,
		AllocatedResources: resources, TaskResources: resources.OldTaskResources(), GroupSelection: target.Selection.Copy()}
	s.plan.AppendAlloc(preview, s.job)
	for _, victim := range offer.PreemptedAllocs {
		s.plan.AppendPreemptedAlloc(victim, preview.ID)
	}
}
