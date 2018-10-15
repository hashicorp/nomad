package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/stretchr/testify/require"
)

func deviceRequest(name string, count uint64,
	constraints []*structs.Constraint, affinities []*structs.Affinity) *structs.RequestedDevice {
	return &structs.RequestedDevice{
		Name:        name,
		Count:       count,
		Constraints: constraints,
		Affinities:  affinities,
	}
}

func nvidiaAllocatedDevice() *structs.AllocatedDeviceResource {
	return &structs.AllocatedDeviceResource{
		Type:      "gpu",
		Vendor:    "nvidia",
		Name:      "1080ti",
		DeviceIDs: []string{uuid.Generate()},
	}
}

func nvidiaAlloc() *structs.Allocation {
	a := mock.Alloc()
	a.AllocatedResources.Tasks["web"].Devices = []*structs.AllocatedDeviceResource{
		nvidiaAllocatedDevice(),
	}
	return a
}

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

// Make sure that the device allocator works even if the node has no devices
func TestDeviceAllocator_AddAllocs_NoDeviceNode(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := mock.Node()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Create three allocations, one with a device, one without, and one
	// terminal
	a1, a2, a3 := mock.Alloc(), nvidiaAlloc(), mock.Alloc()
	allocs := []*structs.Allocation{a1, a2, a3}
	a3.DesiredStatus = structs.AllocDesiredStatusStop

	require.False(d.AddAllocs(allocs))
	require.Len(d.devices, 0)
}

// Add allocs to a node with a device
func TestDeviceAllocator_AddAllocs(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Create three allocations, one with a device, one without, and one
	// terminal
	a1, a2, a3 := mock.Alloc(), nvidiaAlloc(), mock.Alloc()

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID
	intelDev0ID := n.NodeResources.Devices[1].Instances[0].ID
	a2.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}

	allocs := []*structs.Allocation{a1, a2, a3}
	a3.DesiredStatus = structs.AllocDesiredStatusStop

	require.False(d.AddAllocs(allocs))
	require.Len(d.devices, 2)

	// Check that we have two devices for nvidia and that one of them is used
	nvidiaDevice, ok := d.devices[*n.NodeResources.Devices[0].ID()]
	require.True(ok)
	require.Len(nvidiaDevice.instances, 2)
	require.Contains(nvidiaDevice.instances, nvidiaDev0ID)
	require.Equal(1, nvidiaDevice.instances[nvidiaDev0ID])

	// Check only one instance of the intel device is set up since the other is
	// unhealthy
	intelDevice, ok := d.devices[*n.NodeResources.Devices[1].ID()]
	require.True(ok)
	require.Len(intelDevice.instances, 1)
	require.Equal(0, intelDevice.instances[intelDev0ID])
}

// Add alloc with unknown ID to a node with devices. This tests that we can
// operate on previous allocs even if the device has changed to unhealthy and we
// don't track it
func TestDeviceAllocator_AddAllocs_UnknownID(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Create three allocations, one with a device, one without, and one
	// terminal
	a1, a2, a3 := mock.Alloc(), nvidiaAlloc(), mock.Alloc()

	// a2 will have a random ID since it is generated

	allocs := []*structs.Allocation{a1, a2, a3}
	a3.DesiredStatus = structs.AllocDesiredStatusStop

	require.False(d.AddAllocs(allocs))
	require.Len(d.devices, 2)

	// Check that we have two devices for nvidia and that one of them is used
	nvidiaDevice, ok := d.devices[*n.NodeResources.Devices[0].ID()]
	require.True(ok)
	require.Len(nvidiaDevice.instances, 2)
	for _, v := range nvidiaDevice.instances {
		require.Equal(0, v)
	}
}

// Test that collision detection works
func TestDeviceAllocator_AddAllocs_Collision(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Create two allocations, both with the same device
	a1, a2 := nvidiaAlloc(), nvidiaAlloc()

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID
	a1.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}
	a2.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}

	allocs := []*structs.Allocation{a1, a2}
	require.True(d.AddAllocs(allocs))
}

// Make sure that the device allocator works even if the node has no devices
func TestDeviceAllocator_AddReserved_NoDeviceNode(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := mock.Node()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	require.False(d.AddReserved(nvidiaAllocatedDevice()))
	require.Len(d.devices, 0)
}

// Add reserved to a node with a device
func TestDeviceAllocator_AddReserved(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID
	intelDev0ID := n.NodeResources.Devices[1].Instances[0].ID

	res := nvidiaAllocatedDevice()
	res.DeviceIDs = []string{nvidiaDev0ID}

	require.False(d.AddReserved(res))
	require.Len(d.devices, 2)

	// Check that we have two devices for nvidia and that one of them is used
	nvidiaDevice, ok := d.devices[*n.NodeResources.Devices[0].ID()]
	require.True(ok)
	require.Len(nvidiaDevice.instances, 2)
	require.Contains(nvidiaDevice.instances, nvidiaDev0ID)
	require.Equal(1, nvidiaDevice.instances[nvidiaDev0ID])

	// Check only one instance of the intel device is set up since the other is
	// unhealthy
	intelDevice, ok := d.devices[*n.NodeResources.Devices[1].ID()]
	require.True(ok)
	require.Len(intelDevice.instances, 1)
	require.Equal(0, intelDevice.instances[intelDev0ID])
}

// Test that collision detection works
func TestDeviceAllocator_AddReserved_Collision(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	nvidiaDev0ID := n.NodeResources.Devices[0].Instances[0].ID

	// Create an alloc with nvidia
	a1 := nvidiaAlloc()
	a1.AllocatedResources.Tasks["web"].Devices[0].DeviceIDs = []string{nvidiaDev0ID}
	require.False(d.AddAllocs([]*structs.Allocation{a1}))

	// Reserve the same device
	res := nvidiaAllocatedDevice()
	res.DeviceIDs = []string{nvidiaDev0ID}
	require.True(d.AddReserved(res))
}

// Test that asking for a device on a node with no devices doesn't work
func TestDeviceAllocator_Allocate_NoDeviceNode(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := mock.Node()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Build the request
	ask := deviceRequest("nvidia/gpu", 1, nil, nil)

	out, err := d.AssignDevice(ask)
	require.Nil(out)
	require.Error(err)
	require.Contains(err.Error(), "no devices available")
}

// Test that asking for a device that isn't fully specified works.
func TestDeviceAllocator_Allocate_GenericRequest(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Build the request
	ask := deviceRequest("gpu", 1, nil, nil)

	out, err := d.AssignDevice(ask)
	require.NotNil(out)
	require.NoError(err)

	// Check that we got the nvidia device
	require.Len(out.DeviceIDs, 1)
	require.Contains(collectInstanceIDs(n.NodeResources.Devices[0]), out.DeviceIDs[0])
}

// Test that asking for a device that is fully specified works.
func TestDeviceAllocator_Allocate_FullyQualifiedRequest(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Build the request
	ask := deviceRequest("intel/fpga/F100", 1, nil, nil)

	out, err := d.AssignDevice(ask)
	require.NotNil(out)
	require.NoError(err)

	// Check that we got the nvidia device
	require.Len(out.DeviceIDs, 1)
	require.Contains(collectInstanceIDs(n.NodeResources.Devices[1]), out.DeviceIDs[0])
}

// Test that asking for a device with too much count doesn't place
func TestDeviceAllocator_Allocate_NotEnoughInstances(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)
	n := devNode()
	d := newDeviceAllocator(ctx, n)
	require.NotNil(d)

	// Build the request
	ask := deviceRequest("gpu", 4, nil, nil)

	out, err := d.AssignDevice(ask)
	require.Nil(out)
	require.Error(err)
	require.Contains(err.Error(), "no devices match request")
}

// Test that asking for a device with constraints works
func TestDeviceAllocator_Allocate_Constraints(t *testing.T) {
	n := multipleNvidiaNode()
	nvidia0 := n.NodeResources.Devices[0]
	nvidia1 := n.NodeResources.Devices[1]

	cases := []struct {
		Name           string
		Constraints    []*structs.Constraint
		ExpectedDevice *structs.NodeDeviceResource
		NoPlacement    bool
	}{
		{
			Name: "gpu",
			Constraints: []*structs.Constraint{
				{
					LTarget: "${driver.attr.cuda_cores}",
					Operand: ">",
					RTarget: "4000",
				},
			},
			ExpectedDevice: nvidia1,
		},
		{
			Name: "gpu",
			Constraints: []*structs.Constraint{
				{
					LTarget: "${driver.attr.cuda_cores}",
					Operand: "<",
					RTarget: "4000",
				},
			},
			ExpectedDevice: nvidia0,
		},
		{
			Name: "nvidia/gpu",
			Constraints: []*structs.Constraint{
				// First two are shared across both devices
				{
					LTarget: "${driver.attr.memory_bandwidth}",
					Operand: ">",
					RTarget: "10 GB/s",
				},
				{
					LTarget: "${driver.attr.memory}",
					Operand: "is",
					RTarget: "11264 MiB",
				},
				{
					LTarget: "${driver.attr.graphics_clock}",
					Operand: ">",
					RTarget: "1.4 GHz",
				},
			},
			ExpectedDevice: nvidia0,
		},
		{
			Name:        "intel/gpu",
			NoPlacement: true,
		},
		{
			Name: "nvidia/gpu",
			Constraints: []*structs.Constraint{
				{
					LTarget: "${driver.attr.memory_bandwidth}",
					Operand: ">",
					RTarget: "10 GB/s",
				},
				{
					LTarget: "${driver.attr.memory}",
					Operand: "is",
					RTarget: "11264 MiB",
				},
				// Rules both out
				{
					LTarget: "${driver.attr.graphics_clock}",
					Operand: ">",
					RTarget: "2.4 GHz",
				},
			},
			NoPlacement: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			require := require.New(t)
			_, ctx := testContext(t)
			d := newDeviceAllocator(ctx, n)
			require.NotNil(d)

			// Build the request
			ask := deviceRequest(c.Name, 1, c.Constraints, nil)

			out, err := d.AssignDevice(ask)
			if c.NoPlacement {
				require.Nil(out)
			} else {
				require.NotNil(out)
				require.NoError(err)

				// Check that we got the nvidia device
				require.Len(out.DeviceIDs, 1)
				require.Contains(collectInstanceIDs(c.ExpectedDevice), out.DeviceIDs[0])
			}
		})
	}
}

// TODO
// Assign with priorities to pick the best one
