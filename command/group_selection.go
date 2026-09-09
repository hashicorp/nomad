// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/nomad/api"
)

func formatDeploymentGroupSelections(selections map[string]*api.DeploymentGroupSelection) string {
	names := make([]string, 0, len(selections))
	for name := range selections {
		names = append(names, name)
	}
	slices.Sort(names)
	rows := []string{"Selection|Desired|Selected|Unplaced"}
	for _, name := range names {
		selection := selections[name]
		if selection == nil {
			continue
		}
		groups := make([]string, 0, len(selection.Slots))
		for index := 0; index < selection.Count; index++ {
			slot := selection.Slots[index]
			if slot != nil && slot.TaskGroup != "" && slot.Cohort != "" && !slices.Contains(groups, slot.TaskGroup) {
				groups = append(groups, slot.TaskGroup)
			}
		}
		selected := strings.Join(groups, ", ")
		if selected == "" {
			selected = "(none)"
		}
		rows = append(rows, fmt.Sprintf("%s|%d|%s|%d", name, selection.Count, selected, max(0, selection.Count-len(groups))))
	}
	return formatList(rows)
}
