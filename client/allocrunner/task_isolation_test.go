// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
)

// testTaskDrivers maps task names to the filesystem isolation of their driver.
// Looking up a task that is not in the map fails.
type testTaskDrivers map[string]fsisolation.Mode

func (d testTaskDrivers) GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error) {
	mode, ok := d[taskName]
	if !ok {
		return nil, fmt.Errorf("task %q not found", taskName)
	}
	return &drivers.Capabilities{FSIsolation: mode}, nil
}

// setConnectTasks replaces the tasks of the alloc's group with one task of
// each connect kind, and returns the drivers of those tasks. Each kind also
// has a task whose driver lookup fails.
func setConnectTasks(alloc *structs.Allocation) testTaskDrivers {
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	tg.Tasks = []*structs.Task{
		{Name: "main"},
		{Name: "proxy", Kind: structs.NewTaskKind(structs.ConnectProxyPrefix, "api")},
		{Name: "gateway", Kind: structs.NewTaskKind(structs.ConnectIngressPrefix, "ingress")},
		{Name: "native", Kind: structs.NewTaskKind(structs.ConnectNativePrefix, "native")},
		{Name: "missing-proxy", Kind: structs.NewTaskKind(structs.ConnectProxyPrefix, "other")},
		{Name: "missing-native", Kind: structs.NewTaskKind(structs.ConnectNativePrefix, "other")},
	}
	return testTaskDrivers{
		"main":    fsisolation.None,
		"proxy":   fsisolation.Image,
		"gateway": fsisolation.Image,
		"native":  fsisolation.Chroot,
	}
}
