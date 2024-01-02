// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

// nvidiaAllocatedDevice returns an allocated nvidia device
func nvidiaAllocatedDevice() *AllocatedDeviceResource {
	return &AllocatedDeviceResource{
		Type:      "gpu",
		Vendor:    "nvidia",
		Name:      "1080ti",
		DeviceIDs: []string{uuid.Generate()},
	}
}

// nvidiaAlloc returns an allocation that has been assigned an nvidia device.
func nvidiaAlloc() *Allocation {
	a := MockAlloc()
	a.AllocatedResources.Tasks["web"].Devices = []*AllocatedDeviceResource{
		nvidiaAllocatedDevice(),
	}
	return a
}

// devNode returns a node containing two devices, an nvidia gpu and an intel
// FPGA.
func devNode() *Node {
	n := MockNvidiaNode()
	n.NodeResources.Devices = append(n.NodeResources.Devices, &NodeDeviceResource{
		Type:   "fpga",
		Vendor: "intel",
		Name:   "F100",
		Attributes: map[string]*psstructs.Attribute{
			"memory": psstructs.NewIntAttribute(4, psstructs.UnitGiB),
		},
		Instances: []*NodeDevice{
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

// Make sure that the device accounter works even if the node has no devices
func TestDeviceAccounter_AddAllocs_NoDeviceNode(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	n := MockNode()
	d := NewDeviceAccounter(n)
	require.NotNil(d)

	// Create three allocations, one with a device, one without, and one
	// terminal
	a1, a2, a3 := MockAlloc(), nvidiaAlloc(), MockAlloc()
	allocs := []*Allocation{a1, a2, a3}
	a3.DesiredStatus = AllocDesiredStatusStop

	require.False(d.AddAllocs(allocs))
	require.Len(d.Devices, 0)
}

// Add allocs to a node with a device
func TestDeviceAccounter_AddAllocs(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	n := devNode()
	d := NewDeviceAccounter(n)
	require.NotNil(d)

	// Create three allocations, one with a device, one without, and one
	// terminal
	a1, a2, a3 := MockAlloc(), nvidiaAlloc(), MockAlloc()

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID
	intelDev0ID := n.NodeResources.Devices[1].Instances[0].ID
	a2.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}

	allocs := []*Allocation{a1, a2, a3}
	a3.DesiredStatus = AllocDesiredStatusStop

	require.False(d.AddAllocs(allocs))
	require.Len(d.Devices, 2)

	// Check that we have two devices for nvidia and that one of them is used
	nvidiaDevice, ok := d.Devices[*n.NodeResources.Devices[0].ID()]
	require.True(ok)
	require.Len(nvidiaDevice.Instances, 2)
	require.Contains(nvidiaDevice.Instances, nvidiaDev0ID)
	require.Equal(1, nvidiaDevice.Instances[nvidiaDev0ID])

	// Check only one instance of the intel device is set up since the other is
	// unhealthy
	intelDevice, ok := d.Devices[*n.NodeResources.Devices[1].ID()]
	require.True(ok)
	require.Len(intelDevice.Instances, 1)
	require.Equal(0, intelDevice.Instances[intelDev0ID])
}

// Add alloc with unknown ID to a node with devices. This tests that we can
// operate on previous allocs even if the device has changed to unhealthy and we
// don't track it
func TestDeviceAccounter_AddAllocs_UnknownID(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	n := devNode()
	d := NewDeviceAccounter(n)
	require.NotNil(d)

	// Create three allocations, one with a device, one without, and one
	// terminal
	a1, a2, a3 := MockAlloc(), nvidiaAlloc(), MockAlloc()

	// a2 will have a random ID since it is generated

	allocs := []*Allocation{a1, a2, a3}
	a3.DesiredStatus = AllocDesiredStatusStop

	require.False(d.AddAllocs(allocs))
	require.Len(d.Devices, 2)

	// Check that we have two devices for nvidia and that one of them is used
	nvidiaDevice, ok := d.Devices[*n.NodeResources.Devices[0].ID()]
	require.True(ok)
	require.Len(nvidiaDevice.Instances, 2)
	for _, v := range nvidiaDevice.Instances {
		require.Equal(0, v)
	}
}

// Test that collision detection works
func TestDeviceAccounter_AddAllocs_Collision(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	n := devNode()
	d := NewDeviceAccounter(n)
	require.NotNil(d)

	// Create two allocations, both with the same device
	a1, a2 := nvidiaAlloc(), nvidiaAlloc()

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID
	a1.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}
	a2.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}

	allocs := []*Allocation{a1, a2}
	require.True(d.AddAllocs(allocs))
}

// Assert that devices are not freed when an alloc's ServerTerminalStatus is
// true, but only when ClientTerminalStatus is true.
func TestDeviceAccounter_AddAllocs_TerminalStatus(t *testing.T) {
	ci.Parallel(t)

	n := devNode()
	d := NewDeviceAccounter(n)

	// Create two allocations, both with the same device. First is being told to
	// stop but has not stopped yet.
	a1, a2 := nvidiaAlloc(), nvidiaAlloc()
	a1.DesiredStatus = AllocDesiredStatusStop
	a1.ClientStatus = AllocClientStatusRunning

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID
	a1.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}
	a2.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}

	allocs := []*Allocation{a1, a2}

	// Since a1 has not stopped on the client, its device is still in use
	must.True(t, d.AddAllocs(allocs))

	// Assert that stop a1 on the client frees the device for use by a2
	a1.ClientStatus = AllocClientStatusComplete
	d = NewDeviceAccounter(n)
	must.False(t, d.AddAllocs(allocs))
}

// Make sure that the device allocator works even if the node has no devices
func TestDeviceAccounter_AddReserved_NoDeviceNode(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	n := MockNode()
	d := NewDeviceAccounter(n)
	require.NotNil(d)

	require.False(d.AddReserved(nvidiaAllocatedDevice()))
	require.Len(d.Devices, 0)
}

// Add reserved to a node with a device
func TestDeviceAccounter_AddReserved(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	n := devNode()
	d := NewDeviceAccounter(n)
	require.NotNil(d)

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID
	intelDev0ID := n.NodeResources.Devices[1].Instances[0].ID

	res := nvidiaAllocatedDevice()
	res.DeviceIDs = []string{nvidiaDev0ID}

	require.False(d.AddReserved(res))
	require.Len(d.Devices, 2)

	// Check that we have two devices for nvidia and that one of them is used
	nvidiaDevice, ok := d.Devices[*n.NodeResources.Devices[0].ID()]
	require.True(ok)
	require.Len(nvidiaDevice.Instances, 2)
	require.Contains(nvidiaDevice.Instances, nvidiaDev0ID)
	require.Equal(1, nvidiaDevice.Instances[nvidiaDev0ID])

	// Check only one instance of the intel device is set up since the other is
	// unhealthy
	intelDevice, ok := d.Devices[*n.NodeResources.Devices[1].ID()]
	require.True(ok)
	require.Len(intelDevice.Instances, 1)
	require.Equal(0, intelDevice.Instances[intelDev0ID])
}

// Test that collision detection works
func TestDeviceAccounter_AddReserved_Collision(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	n := devNode()
	d := NewDeviceAccounter(n)
	require.NotNil(d)

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID

	// Create an alloc with nvidia
	a1 := nvidiaAlloc()
	a1.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}
	require.False(d.AddAllocs([]*Allocation{a1}))

	// Reserve the same device
	res := nvidiaAllocatedDevice()
	res.DeviceIDs = []string{nvidiaDev0ID}
	require.True(d.AddReserved(res))
}
