package utils

import (
	"fmt"

	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/plugins/drivers"
	ldevices "github.com/opencontainers/runc/libcontainer/devices"
	"golang.org/x/sys/unix"
)

// ToExecResources converts driver.Resources into excutor.Resources.
func ToExecResources(resources *drivers.Resources) *executor.Resources {
	if resources == nil || resources.NomadResources == nil {
		return nil
	}

	return &executor.Resources{
		CPU:      resources.NomadResources.CPU,
		MemoryMB: resources.NomadResources.MemoryMB,
		DiskMB:   resources.NomadResources.DiskMB,
	}
}

// ToExecDevices converts a list of driver.DeviceConfigs into excutor.Devices.
func ToExecDevices(devices []*drivers.DeviceConfig) ([]*executor.Device, error) {
	if len(devices) == 0 {
		return nil, nil
	}

	r := make([]*executor.Device, len(devices))

	for i, d := range devices {
		ed, err := ldevices.DeviceFromPath(d.HostPath, d.Permissions)
		if err != nil {
			return nil, fmt.Errorf("failed to make device out for %s: %v", d.HostPath, err)
		}
		ed.Path = d.TaskPath
		r[i] = ed
	}

	return r, nil
}

// ToExecMounts converts a list of driver.MountConfigs into excutor.Mounts.
func ToExecMounts(mounts []*drivers.MountConfig) []*executor.Mount {
	if len(mounts) == 0 {
		return nil
	}

	r := make([]*executor.Mount, len(mounts))

	for i, m := range mounts {
		flags := unix.MS_BIND
		if m.Readonly {
			flags |= unix.MS_RDONLY
		}
		r[i] = &executor.Mount{
			Source:      m.HostPath,
			Destination: m.TaskPath,
			Device:      "bind",
			Flags:       flags,
		}
	}

	return r
}
