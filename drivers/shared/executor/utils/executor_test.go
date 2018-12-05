package utils

import (
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestToExecDevices(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix required for test")
	}

	input := []*drivers.DeviceConfig{
		{
			HostPath:    "/dev/null",
			TaskPath:    "/task/dev/null",
			Permissions: "rwm",
		},
	}

	expected := &executor.Device{
		Path:        "/task/dev/null",
		Type:        99,
		Major:       1,
		Minor:       3,
		Permissions: "rwm",
	}

	found, err := ToExecDevices(input)
	require.NoError(t, err)
	require.Len(t, found, 1)

	// ignore permission and ownership
	d := found[0]
	d.FileMode = 0
	d.Uid = 0
	d.Gid = 0

	require.EqualValues(t, expected, d)
}

func TestToExecMounts(t *testing.T) {
	input := []*drivers.MountConfig{
		{
			HostPath: "/host/path-ro",
			TaskPath: "/task/path-ro",
			Readonly: true,
		},
		{
			HostPath: "/host/path-rw",
			TaskPath: "/task/path-rw",
			Readonly: false,
		},
	}

	expected := []*executor.Mount{
		{
			Source:      "/host/path-ro",
			Destination: "/task/path-ro",
			Flags:       unix.MS_BIND | unix.MS_RDONLY,
			Device:      "bind",
		},
		{
			Source:      "/host/path-rw",
			Destination: "/task/path-rw",
			Flags:       unix.MS_BIND,
			Device:      "bind",
		},
	}

	require.EqualValues(t, expected, ToExecMounts(input))
}
