// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

// Before a slot has a target, there is no selected task group's update policy.
// Revert on a selection deadline only when every eligible alternative requests
// it. Once selected, the target's ordinary deployment state controls rollback.
func (w *deploymentWatcher) unplacedSelectionAutoRevert(name string) bool {
	selection := w.j.LookupGroupSelection(name)
	if selection == nil {
		return false
	}
	eligible := false
	for _, name := range selection.Groups {
		group := w.j.LookupTaskGroup(name)
		if group == nil || group.Count == 0 {
			continue
		}
		eligible = true
		if group.Update == nil || !group.Update.AutoRevert {
			return false
		}
	}
	return eligible
}
