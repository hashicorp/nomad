//+build linux,lxc

package lxc

import (
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

func TestLXCDriver_Mounts(t *testing.T) {
	t.Parallel()

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 2,
				},
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 1024,
				},
			},
			LinuxResources: &drivers.LinuxResources{
				CPUShares:        1024,
				MemoryLimitBytes: 2 * 1024,
			},
		},
		Mounts: []*drivers.MountConfig{
			{HostPath: "/dev", TaskPath: "/task-mounts/dev-path"},
			{HostPath: "/bin/sh", TaskPath: "/task-mounts/task-path-ro", Readonly: true},
		},
		Devices: []*drivers.DeviceConfig{
			{HostPath: "/dev", TaskPath: "/task-devices/dev-path", Permissions: "rw"},
			{HostPath: "/bin/sh", TaskPath: "/task-devices/task-path-ro", Permissions: "ro"},
		},
	}
	taskConfig := TaskConfig{
		Template: "busybox",
		Volumes: []string{
			"relative/path:/usr-config/container/path",
			"relative/path2:usr-config/container/relative",
		},
	}

	d := NewLXCDriver(testlog.HCLogger(t)).(*Driver)
	d.config.Enabled = true

	entries, err := d.mountEntries(task, taskConfig)
	require.NoError(t, err)

	expectedEntries := []string{
		"test/relative/path usr-config/container/path none rw,bind,create=dir",
		"test/relative/path2 usr-config/container/relative none rw,bind,create=dir",
		"/dev task-mounts/dev-path none rw,bind,create=dir",
		"/bin/sh task-mounts/task-path-ro none ro,bind,create=file",
		"/dev task-devices/dev-path none rw,bind,create=dir",
		"/bin/sh task-devices/task-path-ro none ro,bind,create=file",
	}

	for _, e := range expectedEntries {
		require.Contains(t, entries, e)
	}
}

func TestLXCDriver_DevicesCgroup(t *testing.T) {
	t.Parallel()

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
		Devices: []*drivers.DeviceConfig{
			{HostPath: "/dev/random", TaskPath: "/task-devices/devrandom", Permissions: "rw"},
			{HostPath: "/dev/null", TaskPath: "/task-devices/devnull", Permissions: "rwm"},
		},
	}

	d := NewLXCDriver(testlog.HCLogger(t)).(*Driver)
	d.config.Enabled = true

	cgroupEntries, err := d.devicesCgroupEntries(task)
	require.NoError(t, err)

	expected := []string{
		"c 1:8 rw",
		"c 1:3 rwm",
	}
	require.EqualValues(t, expected, cgroupEntries)
}
