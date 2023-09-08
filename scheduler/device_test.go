// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

// deviceRequest takes the name, count and potential constraints and affinities
// and returns a device request.
func deviceRequest(name string, count uint64,
	constraints []*structs.Constraint, affinities []*structs.Affinity) *structs.RequestedDevice {
	return &structs.RequestedDevice{
		Name:        name,
		Count:       count,
		Constraints: constraints,
		Affinities:  affinities,
	}
}

// devNode returns a node containing two devices, an nvidia gpu and an intel
// FPGA.
func devNode() *structs.Node {
	n := mock.NvidiaNode()
	n.NodeResources.Devices = append(n.NodeResources.Devices, &structs.NodeDeviceResource{
		Type:   "fpga",
		Vendor: "intel",
		Name:   "F100",
		Attributes: map[string]*psstructs.Attribute{
			"memory": psstructs.NewIntAttribute(4, psstructs.UnitGiB),
		},
		Instances: []*structs.NodeDevice{
			{
				ID:      uuid.Generate(),
				Healthy: true,
			},
			{
				ID:      uuid.Generate(),
				Healthy: false,
			},
		},
	})
	return n
}

// multipleNvidiaNode returns a node containing multiple nvidia device types.
func multipleNvidiaNode() *structs.Node {
	n := mock.NvidiaNode()
	n.NodeResources.Devices = append(n.NodeResources.Devices, &structs.NodeDeviceResource{
		Type:   "gpu",
		Vendor: "nvidia",
		Name:   "2080ti",
		Attributes: map[string]*psstructs.Attribute{
			"memory":           psstructs.NewIntAttribute(11, psstructs.UnitGiB),
			"cuda_cores":       psstructs.NewIntAttribute(4352, ""),
			"graphics_clock":   psstructs.NewIntAttribute(1350, psstructs.UnitMHz),
			"memory_bandwidth": psstructs.NewIntAttribute(14, psstructs.UnitGBPerS),
		},
		Instances: []*structs.NodeDevice{
			{
				ID:      uuid.Generate(),
				Healthy: true,
			},
			{
				ID:      uuid.Generate(),
				Healthy: true,
			},
		},
	})
	return n

}

// collectInstanceIDs returns the IDs of the device instances
func collectInstanceIDs(devices ...*structs.NodeDeviceResource) []string {
	var out []string
	for _, d := range devices {
		for _, i := range d.Instances {
			out = append(out, i.ID)
		}
	}
	return out
}

// Test that asking for a device that isn't fully specified works.
func TestDeviceAllocator_Allocate_GenericRequest(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Build the request
	ask := deviceRequest("gpu", 1, nil, nil)

	out, score, err := d.AssignDevice(ask)
	require.NotNil(out)
	require.Zero(score)
	require.NoError(err)

	// Check that we got the nvidia device
	require.Len(out.DeviceIDs, 1)
	require.Contains(collectInstanceIDs(n.NodeResources.Devices[0]), out.DeviceIDs[0])
}

// Test that asking for a device that is fully specified works.
func TestDeviceAllocator_Allocate_FullyQualifiedRequest(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Build the request
	ask := deviceRequest("intel/fpga/F100", 1, nil, nil)

	out, score, err := d.AssignDevice(ask)
	require.NotNil(out)
	require.Zero(score)
	require.NoError(err)

	// Check that we got the nvidia device
	require.Len(out.DeviceIDs, 1)
	require.Contains(collectInstanceIDs(n.NodeResources.Devices[1]), out.DeviceIDs[0])
}

// Test that asking for a device with too much count doesn't place
func TestDeviceAllocator_Allocate_NotEnoughInstances(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Build the request
	ask := deviceRequest("gpu", 4, nil, nil)

	out, _, err := d.AssignDevice(ask)
	require.Nil(out)
	require.Error(err)
	require.Contains(err.Error(), "no devices match request")
}

// Test that asking for a device with constraints works
func TestDeviceAllocator_Allocate_Constraints(t *testing.T) {
	ci.Parallel(t)

	n := multipleNvidiaNode()
	nvidia0 := n.NodeResources.Devices[0]
	nvidia1 := n.NodeResources.Devices[1]

	cases := []struct {
		Name              string
		Note              string
		Constraints       []*structs.Constraint
		ExpectedDevice    *structs.NodeDeviceResource
		ExpectedDeviceIDs []string
		NoPlacement       bool
	}{
		{
			Name: "gpu",
			Note: "-gt",
			Constraints: []*structs.Constraint{
				{
					LTarget: "${device.attr.cuda_cores}",
					Operand: ">",
					RTarget: "4000",
				},
			},
			ExpectedDevice:    nvidia1,
			ExpectedDeviceIDs: collectInstanceIDs(nvidia1),
		},
		{
			Name: "gpu",
			Note: "-lt",
			Constraints: []*structs.Constraint{
				{
					LTarget: "${device.attr.cuda_cores}",
					Operand: "<",
					RTarget: "4000",
				},
			},
			ExpectedDevice:    nvidia0,
			ExpectedDeviceIDs: collectInstanceIDs(nvidia0),
		},
		{
			Name: "nvidia/gpu",
			Constraints: []*structs.Constraint{
				// First two are shared across both devices
				{
					LTarget: "${device.attr.memory_bandwidth}",
					Operand: ">",
					RTarget: "10 GB/s",
				},
				{
					LTarget: "${device.attr.memory}",
					Operand: "is",
					RTarget: "11264 MiB",
				},
				{
					LTarget: "${device.attr.graphics_clock}",
					Operand: ">",
					RTarget: "1.4 GHz",
				},
			},
			ExpectedDevice:    nvidia0,
			ExpectedDeviceIDs: collectInstanceIDs(nvidia0),
		},
		{
			Name:        "intel/gpu",
			NoPlacement: true,
		},
		{
			Name: "nvidia/gpu",
			Note: "-no-placement",
			Constraints: []*structs.Constraint{
				{
					LTarget: "${device.attr.memory_bandwidth}",
					Operand: ">",
					RTarget: "10 GB/s",
				},
				{
					LTarget: "${device.attr.memory}",
					Operand: "is",
					RTarget: "11264 MiB",
				},
				// Rules both out
				{
					LTarget: "${device.attr.graphics_clock}",
					Operand: ">",
					RTarget: "2.4 GHz",
				},
			},
			NoPlacement: true,
		},
		{
			Name: "nvidia/gpu",
			Note: "-contains-id",
			Constraints: []*structs.Constraint{
				{
					LTarget: "${device.ids}",
					Operand: "set_contains",
					RTarget: nvidia0.Instances[1].ID,
				},
			},
			ExpectedDevice:    nvidia0,
			ExpectedDeviceIDs: []string{nvidia0.Instances[1].ID},
		},
	}

	for _, c := range cases {
		t.Run(c.Name+c.Note, func(t *testing.T) {
			_, ctx := testContext(t)
			d := newDeviceAllocator(ctx, n)
			must.NotNil(t, d)

			// Build the request
			ask := deviceRequest(c.Name, 1, c.Constraints, nil)

			out, score, err := d.AssignDevice(ask)
			if c.NoPlacement {
				require.Nil(t, out)
			} else {
				must.NotNil(t, out)
				must.Zero(t, score)
				must.NoError(t, err)

				// Check that we got the right nvidia device instance, and
				// specific device instance IDs if required
				must.Len(t, 1, out.DeviceIDs)
				must.SliceContains(t, collectInstanceIDs(c.ExpectedDevice), out.DeviceIDs[0])
				must.SliceContainsSubset(t, c.ExpectedDeviceIDs, out.DeviceIDs)
			}
		})
	}
}

// Test that asking for a device with affinities works
func TestDeviceAllocator_Allocate_Affinities(t *testing.T) {
	ci.Parallel(t)

	n := multipleNvidiaNode()
	nvidia0 := n.NodeResources.Devices[0]
	nvidia1 := n.NodeResources.Devices[1]

	cases := []struct {
		Name           string
		Affinities     []*structs.Affinity
		ExpectedDevice *structs.NodeDeviceResource
		ZeroScore      bool
	}{
		{
			Name: "gpu",
			Affinities: []*structs.Affinity{
				{
					LTarget: "${device.attr.cuda_cores}",
					Operand: ">",
					RTarget: "4000",
					Weight:  60,
				},
			},
			ExpectedDevice: nvidia1,
		},
		{
			Name: "gpu",
			Affinities: []*structs.Affinity{
				{
					LTarget: "${device.attr.cuda_cores}",
					Operand: "<",
					RTarget: "4000",
					Weight:  10,
				},
			},
			ExpectedDevice: nvidia0,
		},
		{
			Name: "gpu",
			Affinities: []*structs.Affinity{
				{
					LTarget: "${device.attr.cuda_cores}",
					Operand: ">",
					RTarget: "4000",
					Weight:  -20,
				},
			},
			ZeroScore:      true,
			ExpectedDevice: nvidia0,
		},
		{
			Name: "nvidia/gpu",
			Affinities: []*structs.Affinity{
				// First two are shared across both devices
				{
					LTarget: "${device.attr.memory_bandwidth}",
					Operand: ">",
					RTarget: "10 GB/s",
					Weight:  20,
				},
				{
					LTarget: "${device.attr.memory}",
					Operand: "is",
					RTarget: "11264 MiB",
					Weight:  20,
				},
				{
					LTarget: "${device.attr.graphics_clock}",
					Operand: ">",
					RTarget: "1.4 GHz",
					Weight:  90,
				},
			},
			ExpectedDevice: nvidia0,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			require := require.New(t)
			_, ctx := testContext(t)
			d := newDeviceAllocator(ctx, n)
			require.NotNil(d)

			// Build the request
			ask := deviceRequest(c.Name, 1, nil, c.Affinities)

			out, score, err := d.AssignDevice(ask)
			require.NotNil(out)
			require.NoError(err)
			if c.ZeroScore {
				require.Zero(score)
			} else {
				require.NotZero(score)
			}

			// Check that we got the nvidia device
			require.Len(out.DeviceIDs, 1)
			require.Contains(collectInstanceIDs(c.ExpectedDevice), out.DeviceIDs[0])
		})
	}
}
