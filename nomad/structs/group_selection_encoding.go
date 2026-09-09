// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"strconv"
	"time"
)

// deploymentGroupSelectionExt preserves integer slot indexes in Raft while
// emitting string object keys in HTTP JSON. go-msgpack's default JSON encoder
// writes integer map keys as unquoted numbers, which is not valid JSON.
func deploymentGroupSelectionExt(value any) any {
	var selection *DeploymentGroupSelection
	switch value := value.(type) {
	case *DeploymentGroupSelection:
		selection = value
	case DeploymentGroupSelection:
		selection = &value
	}
	if selection == nil {
		return nil
	}
	var slots map[string]*DeploymentGroupSelectionSlot
	if selection.Slots != nil {
		slots = make(map[string]*DeploymentGroupSelectionSlot, len(selection.Slots))
		for index, slot := range selection.Slots {
			slots[strconv.Itoa(index)] = slot
		}
	}
	return &struct {
		Count             int
		Slots             map[string]*DeploymentGroupSelectionSlot
		RequireProgressBy time.Time
	}{
		Count:             selection.Count,
		Slots:             slots,
		RequireProgressBy: selection.RequireProgressBy,
	}
}
