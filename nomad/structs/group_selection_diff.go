// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"sort"

	"github.com/hashicorp/nomad/helper/flatmap"
)

func groupSelectionDiffs(old, next []*TaskGroupSelection, contextual bool) []*ObjectDiff {
	previous := make(map[string]*TaskGroupSelection, len(old))
	current := make(map[string]*TaskGroupSelection, len(next))
	for _, selection := range old {
		if selection != nil {
			previous[selection.Name] = selection
		}
	}
	for _, selection := range next {
		if selection != nil {
			current[selection.Name] = selection
		}
	}
	var result []*ObjectDiff
	for name, before := range previous {
		if diff := groupSelectionDiff(before, current[name], contextual); diff != nil {
			result = append(result, diff)
		}
	}
	for name, after := range current {
		if _, exists := previous[name]; !exists {
			result = append(result, groupSelectionDiff(nil, after, contextual))
		}
	}
	sort.Sort(ObjectDiffs(result))
	return result
}

func groupSelectionDiff(old, next *TaskGroupSelection, contextual bool) *ObjectDiff {
	if old.Equal(next) {
		return nil
	}
	diff := &ObjectDiff{Name: "GroupSelection", Type: DiffTypeEdited}
	var before, after map[string]string
	if old == nil {
		diff.Type = DiffTypeAdded
	} else {
		before = flatmap.Flatten(old, nil, false)
	}
	if next == nil {
		diff.Type = DiffTypeDeleted
	} else {
		after = flatmap.Flatten(next, nil, false)
	}
	// Flatten the ordered candidate list, so a preference change is visible even
	// when the set of candidate groups stays the same.
	diff.Fields = fieldDiffs(before, after, contextual)
	// Keep the selection's name visible when only its count or candidates change.
	if old != nil && next != nil && !contextual {
		diff.Fields = append(diff.Fields, fieldDiff(old.Name, next.Name, "Name", true))
	}
	sort.Sort(FieldDiffs(diff.Fields))
	return diff
}
