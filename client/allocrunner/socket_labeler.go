// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/clientsockets"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// taskDriverCapabilities looks up the driver capabilities of an alloc's tasks.
// It is implemented by allocRunner and faked in tests.
type taskDriverCapabilities interface {
	GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error)
}

var _ taskDriverCapabilities = (*allocRunner)(nil)

// taskSocketLabeler reports whether an alloc has a task that runs as a
// confined container, for the alloc-level sockets its tasks share. A nil
// labeler reports none.
type taskSocketLabeler struct {
	logger  hclog.Logger
	drivers taskDriverCapabilities
}

func newTaskSocketLabeler(logger hclog.Logger, taskDrivers taskDriverCapabilities) *taskSocketLabeler {
	return &taskSocketLabeler{logger: logger, drivers: taskDrivers}
}

// labelForAlloc reports whether any task in the alloc's group runs as a
// confined container. The sockets are shared by the whole group, so one such
// task is enough. Call it only once the task runners exist, as from a Prerun
// hook: the hooks are built before the task runners, and until then every task
// is "not found".
func (l *taskSocketLabeler) labelForAlloc(alloc *structs.Allocation) bool {
	if l == nil {
		return false
	}
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return false
	}
	for _, task := range tg.Tasks {
		caps, err := l.drivers.GetTaskDriverCapabilities(task.Name)
		if err != nil {
			l.logger.Warn("unable to get task driver capabilities", "task", task.Name, "error", err)
			continue
		}
		if caps != nil && clientsockets.Confined(caps.FSIsolation) {
			return true
		}
	}
	return false
}
