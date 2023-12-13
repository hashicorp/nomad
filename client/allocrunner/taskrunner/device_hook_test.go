// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

func TestDeviceHook_CorrectDevice(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	dm := devicemanager.NoopMockManager()
	l := testlog.HCLogger(t)
	h := newDeviceHook(dm, l)

	reqDev := &structs.AllocatedDeviceResource{
		Vendor:    "foo",
		Type:      "bar",
		Name:      "baz",
		DeviceIDs: []string{"123"},
	}

	// Build the hook request
	req := &interfaces.TaskPrestartRequest{
		TaskResources: &structs.AllocatedTaskResources{
			Devices: []*structs.AllocatedDeviceResource{
				reqDev,
			},
		},
	}

	// Setup the device manager to return a response
	dm.ReserveF = func(d *structs.AllocatedDeviceResource) (*device.ContainerReservation, error) {
		if d.Vendor != reqDev.Vendor || d.Type != reqDev.Type ||
			d.Name != reqDev.Name || len(d.DeviceIDs) != 1 || d.DeviceIDs[0] != reqDev.DeviceIDs[0] {
			return nil, fmt.Errorf("unexpected request: %+v", d)
		}

		res := &device.ContainerReservation{
			Envs: map[string]string{
				"123": "456",
			},
			Mounts: []*device.Mount{
				{
					ReadOnly: true,
					TaskPath: "foo",
					HostPath: "bar",
				},
			},
			Devices: []*device.DeviceSpec{
				{
					TaskPath:    "foo",
					HostPath:    "bar",
					CgroupPerms: "123",
				},
			},
		}
		return res, nil
	}

	var resp interfaces.TaskPrestartResponse
	err := h.Prestart(context.Background(), req, &resp)
	require.NoError(err)
	require.NotNil(resp)

	expEnv := map[string]string{
		"123": "456",
	}
	require.EqualValues(expEnv, resp.Env)

	expMounts := []*drivers.MountConfig{
		{
			Readonly: true,
			TaskPath: "foo",
			HostPath: "bar",
		},
	}
	require.EqualValues(expMounts, resp.Mounts)

	expDevices := []*drivers.DeviceConfig{
		{
			TaskPath:    "foo",
			HostPath:    "bar",
			Permissions: "123",
		},
	}
	require.EqualValues(expDevices, resp.Devices)
}

func TestDeviceHook_IncorrectDevice(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	dm := devicemanager.NoopMockManager()
	l := testlog.HCLogger(t)
	h := newDeviceHook(dm, l)

	reqDev := &structs.AllocatedDeviceResource{
		Vendor:    "foo",
		Type:      "bar",
		Name:      "baz",
		DeviceIDs: []string{"123"},
	}

	// Build the hook request
	req := &interfaces.TaskPrestartRequest{
		TaskResources: &structs.AllocatedTaskResources{
			Devices: []*structs.AllocatedDeviceResource{
				reqDev,
			},
		},
	}

	// Setup the device manager to return a response
	dm.ReserveF = func(d *structs.AllocatedDeviceResource) (*device.ContainerReservation, error) {
		return nil, fmt.Errorf("bad request")
	}

	var resp interfaces.TaskPrestartResponse
	err := h.Prestart(context.Background(), req, &resp)
	require.Error(err)
}
