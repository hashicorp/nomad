// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestFormatDeploymentGroupSelections(t *testing.T) {
	ci.Parallel(t)
	out := formatDeploymentGroupSelections(map[string]*api.DeploymentGroupSelection{
		"runtime": {Count: 2, Slots: map[int]*api.DeploymentGroupSelectionSlot{
			0: {TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"},
			2: {PreviousTaskGroup: "encoder", PreviousCohort: "retiring"},
		}},
		"other": {Count: 1},
	})
	lines := strings.Split(out, "\n")
	must.Eq(t, []string{"Selection", "Desired", "Selected", "Unplaced"}, strings.Fields(lines[0]))
	must.Eq(t, []string{"other", "1", "(none)", "1"}, strings.Fields(lines[1]))
	must.Eq(t, []string{"runtime", "2", "thor", "1"}, strings.Fields(lines[2]))
}
