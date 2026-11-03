// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
	"github.com/shoenig/test/must"
)

// testTaskDrivers maps task names to the filesystem isolation of their driver.
// Tasks not in the map have no driver.
type testTaskDrivers map[string]fsisolation.Mode

func (d testTaskDrivers) GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error) {
	mode, ok := d[taskName]
	if !ok {
		return nil, fmt.Errorf("task %q not found", taskName)
	}
	return &drivers.Capabilities{FSIsolation: mode}, nil
}

func TestTaskSocketLabeler_LabelForAlloc(t *testing.T) {
	ci.Parallel(t)

	// An alloc with two tasks in its group.
	alloc := mock.Alloc()
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	sidecar := tg.Tasks[0].Copy()
	sidecar.Name = "sidecar"
	tg.Tasks = append(tg.Tasks, sidecar)
	mainTask := tg.Tasks[0].Name

	cases := []struct {
		name    string
		enabled bool
		drivers testTaskDrivers
		exp     bool
	}{
		{
			name:    "option off",
			enabled: false,
			drivers: testTaskDrivers{mainTask: fsisolation.Image, "sidecar": fsisolation.Image},
			exp:     false,
		},
		{
			name:    "all image drivers",
			enabled: true,
			drivers: testTaskDrivers{mainTask: fsisolation.Image, "sidecar": fsisolation.Image},
			exp:     true,
		},
		{
			name:    "one image driver",
			enabled: true,
			drivers: testTaskDrivers{mainTask: fsisolation.None, "sidecar": fsisolation.Image},
			exp:     true,
		},
		{
			name:    "no image drivers",
			enabled: true,
			drivers: testTaskDrivers{mainTask: fsisolation.None, "sidecar": fsisolation.Chroot},
			exp:     false,
		},
		{
			name:    "missing driver is skipped",
			enabled: true,
			drivers: testTaskDrivers{"sidecar": fsisolation.Image},
			exp:     true,
		},
		{
			name:    "no drivers",
			enabled: true,
			drivers: testTaskDrivers{},
			exp:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newTaskSocketLabeler(testlog.HCLogger(t), tc.enabled, tc.drivers)
			must.Eq(t, tc.exp, l.labelForAlloc(alloc))
		})
	}
}

func TestTaskSocketLabeler_Nil(t *testing.T) {
	ci.Parallel(t)

	var l *taskSocketLabeler
	must.False(t, l.labelForAlloc(mock.Alloc()))

	// An alloc whose task group is missing is never labeled.
	alloc := mock.Alloc()
	alloc.TaskGroup = "missing"
	l = newTaskSocketLabeler(testlog.HCLogger(t), true, testTaskDrivers{})
	must.False(t, l.labelForAlloc(alloc))
}
