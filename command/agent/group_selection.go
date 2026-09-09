// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"slices"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ApiGroupSelectionsToStructs copies group selections into their internal form.
// Nil entries are retained so that validation can reject them.
func ApiGroupSelectionsToStructs(selections []*api.TaskGroupSelection) []*structs.TaskGroupSelection {
	if selections == nil {
		return nil
	}
	result := make([]*structs.TaskGroupSelection, len(selections))
	for i, selection := range selections {
		if selection == nil {
			continue
		}
		count := 1
		if selection.Count != nil {
			count = *selection.Count
		}
		result[i] = &structs.TaskGroupSelection{
			Name:   selection.Name,
			Count:  count,
			Groups: slices.Clone(selection.Groups),
		}
	}
	return result
}
