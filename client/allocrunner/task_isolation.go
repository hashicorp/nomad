// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
)

// taskDriverCapabilities looks up the driver capabilities of an alloc's tasks.
// It is implemented by allocRunner and faked in tests.
type taskDriverCapabilities interface {
	GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error)
}

var _ taskDriverCapabilities = (*allocRunner)(nil)

// taskIsolationModes returns the filesystem isolation mode of every task in
// the alloc's group that matches kind, for the sockets those tasks share. A
// task whose driver cannot be looked up is skipped with a warning. Call it only
// once the task runners exist, as from Prerun: the runner hooks are built
// before the task runners, and until then every task is "not found".
func taskIsolationModes(
	logger hclog.Logger,
	caps taskDriverCapabilities,
	alloc *structs.Allocation,
	kind func(structs.TaskKind) bool,
) []fsisolation.Mode {
	var modes []fsisolation.Mode
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, task := range tg.Tasks {
		if !kind(task.Kind) {
			continue
		}
		taskCaps, err := caps.GetTaskDriverCapabilities(task.Name)
		if err != nil {
			logger.Warn("unable to get task driver capabilities", "task", task.Name, "error", err)
			continue
		}
		if taskCaps != nil {
			modes = append(modes, taskCaps.FSIsolation)
		}
	}
	return modes
}
