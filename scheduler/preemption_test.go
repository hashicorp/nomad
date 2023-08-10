// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/stretchr/testify/require"
)

func TestResourceDistance(t *testing.T) {
	ci.Parallel(t)

	resourceAsk := &structs.ComparableResources{
		Flattened: structs.AllocatedTaskResources{
			Cpu: structs.AllocatedCpuResources{
				CpuShares: 2048,
			},
			Memory: structs.AllocatedMemoryResources{
				MemoryMB: 512,
			},
			Networks: []*structs.NetworkResource{
				{
					Device: "eth0",
					MBits:  1024,
				},
			},
		},
		Shared: structs.AllocatedSharedResources{
			DiskMB: 4096,
		},
	}

	type testCase struct {
		allocResource    *structs.ComparableResources
		expectedDistance string
	}

	testCases := []*testCase{
		{
			&structs.ComparableResources{
				Flattened: structs.AllocatedTaskResources{
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 512,
					},
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							MBits:  1024,
						},
					},
				},
				Shared: structs.AllocatedSharedResources{
					DiskMB: 4096,
				},
			},
			"0.000",
		},
		{
			&structs.ComparableResources{
				Flattened: structs.AllocatedTaskResources{
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 400,
					},
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							MBits:  1024,
						},
					},
				},
				Shared: structs.AllocatedSharedResources{
					DiskMB: 1024,
				},
			},
			"0.928",
		},
		{
			&structs.ComparableResources{
				Flattened: structs.AllocatedTaskResources{
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 8192,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 200,
					},
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							MBits:  512,
						},
					},
				},
				Shared: structs.AllocatedSharedResources{
					DiskMB: 1024,
				},
			},
			"3.152",
		},
		{
			&structs.ComparableResources{
				Flattened: structs.AllocatedTaskResources{
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 500,
					},
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							MBits:  1024,
						},
					},
				},
				Shared: structs.AllocatedSharedResources{
					DiskMB: 4096,
				},
			},
			"0.023",
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			require := require.New(t)
			actualDistance := fmt.Sprintf("%3.3f", basicResourceDistance(resourceAsk, tc.allocResource))
			require.Equal(tc.expectedDistance, actualDistance)
		})

	}

}

func TestPreemption(t *testing.T) {
	ci.Parallel(t)

	type testCase struct {
		desc                 string
		currentAllocations   []*structs.Allocation
		nodeReservedCapacity *structs.NodeReservedResources
		nodeCapacity         *structs.NodeResources
		resourceAsk          *structs.Resources
		jobPriority          int
		currentPreemptions   []*structs.Allocation
		preemptedAllocIDs    map[string]struct{}
	}

	highPrioJob := mock.Job()
	highPrioJob.Priority = 100

	lowPrioJob := mock.Job()
	lowPrioJob.Priority = 30

	lowPrioJob2 := mock.Job()
	lowPrioJob2.Priority = 40

	// Create some persistent alloc ids to use in test cases
	allocIDs := []string{uuid.Generate(), uuid.Generate(), uuid.Generate(), uuid.Generate(), uuid.Generate(), uuid.Generate()}

	var deviceIDs []string
	for i := 0; i < 10; i++ {
		deviceIDs = append(deviceIDs, "dev"+strconv.Itoa(i))
	}

	defaultNodeResources := &structs.NodeResources{
		Cpu: structs.NodeCpuResources{
			CpuShares: 4000,
		},
		Memory: structs.NodeMemoryResources{
			MemoryMB: 8192,
		},
		Disk: structs.NodeDiskResources{
			DiskMB: 100 * 1024,
		},
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				CIDR:   "192.168.0.100/32",
				MBits:  1000,
			},
		},
		Devices: []*structs.NodeDeviceResource{
			{
				Type:   "gpu",
				Vendor: "nvidia",
				Name:   "1080ti",
				Attributes: map[string]*psstructs.Attribute{
					"memory":           psstructs.NewIntAttribute(11, psstructs.UnitGiB),
					"cuda_cores":       psstructs.NewIntAttribute(3584, ""),
					"graphics_clock":   psstructs.NewIntAttribute(1480, psstructs.UnitMHz),
					"memory_bandwidth": psstructs.NewIntAttribute(11, psstructs.UnitGBPerS),
				},
				Instances: []*structs.NodeDevice{
					{
						ID:      deviceIDs[0],
						Healthy: true,
					},
					{
						ID:      deviceIDs[1],
						Healthy: true,
					},
					{
						ID:      deviceIDs[2],
						Healthy: true,
					},
					{
						ID:      deviceIDs[3],
						Healthy: true,
					},
				},
			},
			{
				Type:   "gpu",
				Vendor: "nvidia",
				Name:   "2080ti",
				Attributes: map[string]*psstructs.Attribute{
					"memory":           psstructs.NewIntAttribute(11, psstructs.UnitGiB),
					"cuda_cores":       psstructs.NewIntAttribute(3584, ""),
					"graphics_clock":   psstructs.NewIntAttribute(1480, psstructs.UnitMHz),
					"memory_bandwidth": psstructs.NewIntAttribute(11, psstructs.UnitGBPerS),
				},
				Instances: []*structs.NodeDevice{
					{
						ID:      deviceIDs[4],
						Healthy: true,
					},
					{
						ID:      deviceIDs[5],
						Healthy: true,
					},
					{
						ID:      deviceIDs[6],
						Healthy: true,
					},
					{
						ID:      deviceIDs[7],
						Healthy: true,
					},
					{
						ID:      deviceIDs[8],
						Healthy: true,
					},
				},
			},
			{
				Type:   "fpga",
				Vendor: "intel",
				Name:   "F100",
				Attributes: map[string]*psstructs.Attribute{
					"memory": psstructs.NewIntAttribute(4, psstructs.UnitGiB),
				},
				Instances: []*structs.NodeDevice{
					{
						ID:      "fpga1",
						Healthy: true,
					},
					{
						ID:      "fpga2",
						Healthy: false,
					},
				},
			},
		},
	}

	reservedNodeResources := &structs.NodeReservedResources{
		Cpu: structs.NodeReservedCpuResources{
			CpuShares: 100,
		},
		Memory: structs.NodeReservedMemoryResources{
			MemoryMB: 256,
		},
		Disk: structs.NodeReservedDiskResources{
			DiskMB: 4 * 1024,
		},
	}

	testCases := []testCase{
		{
			desc: "No preemption because existing allocs are not low priority",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      3200,
					MemoryMB: 7256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  50,
						},
					},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      2000,
				MemoryMB: 256,
				DiskMB:   4 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "ssh", Value: 22}},
						MBits:         1,
					},
				},
			},
		},
		{
			desc: "Preempting low priority allocs not enough to meet resource ask",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      3200,
					MemoryMB: 7256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  50,
						},
					},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      4000,
				MemoryMB: 8192,
				DiskMB:   4 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "ssh", Value: 22}},
						MBits:         1,
					},
				},
			},
		},
		{
			desc: "preemption impossible - static port needed is used by higher priority alloc",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], highPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  600,
							ReservedPorts: []structs.Port{
								{
									Label: "db",
									Value: 88,
								},
							},
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      600,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  700,
						ReservedPorts: []structs.Port{
							{
								Label: "db",
								Value: 88,
							},
						},
					},
				},
			},
		},
		{
			desc: "preempt only from device that has allocation with unused reserved port",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], highPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth1",
							IP:     "192.168.0.200",
							MBits:  600,
							ReservedPorts: []structs.Port{
								{
									Label: "db",
									Value: 88,
								},
							},
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  600,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			// This test sets up a node with two NICs
			nodeCapacity: &structs.NodeResources{
				Cpu: structs.NodeCpuResources{
					CpuShares: 4000,
				},
				Memory: structs.NodeMemoryResources{
					MemoryMB: 8192,
				},
				Disk: structs.NodeDiskResources{
					DiskMB: 100 * 1024,
				},
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						CIDR:   "192.168.0.100/32",
						MBits:  1000,
					},
					{
						Device: "eth1",
						CIDR:   "192.168.1.100/32",
						MBits:  1000,
					},
				},
			},
			jobPriority: 100,
			resourceAsk: &structs.Resources{
				CPU:      600,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						IP:    "192.168.0.100",
						MBits: 700,
						ReservedPorts: []structs.Port{
							{
								Label: "db",
								Value: 88,
							},
						},
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[2]: {},
			},
		},
		{
			desc: "Combination of high/low priority allocs, without static ports",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      2800,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAllocWithTaskgroupNetwork(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  200,
						},
					},
				}, &structs.NetworkResource{
					Device: "eth0",
					IP:     "192.168.0.201",
					MBits:  300,
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  300,
						},
					},
				}),
				createAlloc(allocIDs[3], lowPrioJob, &structs.Resources{
					CPU:      700,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1100,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  840,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
				allocIDs[2]: {},
				allocIDs[3]: {},
			},
		},
		{
			desc: "preempt allocs with network devices",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      2800,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  800,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1100,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  840,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
			},
		},
		{
			desc: "ignore allocs with close enough priority for network devices",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      2800,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  800,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          lowPrioJob.Priority + 5,
			resourceAsk: &structs.Resources{
				CPU:      1100,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  840,
					},
				},
			},
			preemptedAllocIDs: nil,
		},
		{
			desc: "Preemption needed for all resources except network",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      2800,
					MemoryMB: 2256,
					DiskMB:   40 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  50,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 512,
					DiskMB:   25 * 1024,
				}),
				createAlloc(allocIDs[3], lowPrioJob, &structs.Resources{
					CPU:      700,
					MemoryMB: 276,
					DiskMB:   20 * 1024,
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 3000,
				DiskMB:   50 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  50,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
				allocIDs[2]: {},
				allocIDs[3]: {},
			},
		},
		{
			desc: "Only one low priority alloc needs to be preempted",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  500,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  320,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      300,
				MemoryMB: 500,
				DiskMB:   5 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  320,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[2]: {},
			},
		},
		{
			desc: "one alloc meets static port need, another meets remaining mbits needed",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  500,
							ReservedPorts: []structs.Port{
								{
									Label: "db",
									Value: 88,
								},
							},
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  200,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      2700,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  800,
						ReservedPorts: []structs.Port{
							{
								Label: "db",
								Value: 88,
							},
						},
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
				allocIDs[2]: {},
			},
		},
		{
			desc: "alloc that meets static port need also meets other needs",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  600,
							ReservedPorts: []structs.Port{
								{
									Label: "db",
									Value: 88,
								},
							},
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  100,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      600,
				MemoryMB: 1000,
				DiskMB:   25 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  700,
						ReservedPorts: []structs.Port{
							{
								Label: "db",
								Value: 88,
							},
						},
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
			},
		},
		{
			desc: "alloc from job that has existing evictions not chosen for preemption",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1200,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  500,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob2, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  300,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      300,
				MemoryMB: 500,
				DiskMB:   5 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  320,
					},
				},
			},
			currentPreemptions: []*structs.Allocation{
				createAlloc(allocIDs[4], lowPrioJob2, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  300,
						},
					},
				}),
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
			},
		},
		{
			desc: "Preemption with one device instance per alloc",
			// Add allocations that use two device instances
			currentAllocations: []*structs.Allocation{
				createAllocWithDevice(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      500,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "1080ti",
					DeviceIDs: []string{deviceIDs[0]},
				}),
				createAllocWithDevice(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "1080ti",
					DeviceIDs: []string{deviceIDs[1]},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 512,
				DiskMB:   4 * 1024,
				Devices: []*structs.RequestedDevice{
					{
						Name:  "nvidia/gpu/1080ti",
						Count: 4,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[0]: {},
				allocIDs[1]: {},
			},
		},
		{
			desc: "Preemption multiple devices used",
			currentAllocations: []*structs.Allocation{
				createAllocWithDevice(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      500,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "1080ti",
					DeviceIDs: []string{deviceIDs[0], deviceIDs[1], deviceIDs[2], deviceIDs[3]},
				}),
				createAllocWithDevice(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "fpga",
					Vendor:    "intel",
					Name:      "F100",
					DeviceIDs: []string{"fpga1"},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 512,
				DiskMB:   4 * 1024,
				Devices: []*structs.RequestedDevice{
					{
						Name:  "nvidia/gpu/1080ti",
						Count: 4,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[0]: {},
			},
		},
		{
			// This test cases creates allocations across two GPUs
			// Both GPUs are eligible for the task, but only allocs sharing the
			// same device should be chosen for preemption
			desc: "Preemption with allocs across multiple devices that match",
			currentAllocations: []*structs.Allocation{
				createAllocWithDevice(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      500,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "1080ti",
					DeviceIDs: []string{deviceIDs[0], deviceIDs[1]},
				}),
				createAllocWithDevice(allocIDs[1], highPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 100,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "1080ti",
					DeviceIDs: []string{deviceIDs[2]},
				}),
				createAllocWithDevice(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "2080ti",
					DeviceIDs: []string{deviceIDs[4], deviceIDs[5]},
				}),
				createAllocWithDevice(allocIDs[3], lowPrioJob, &structs.Resources{
					CPU:      100,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "2080ti",
					DeviceIDs: []string{deviceIDs[6], deviceIDs[7]},
				}),
				createAllocWithDevice(allocIDs[4], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "fpga",
					Vendor:    "intel",
					Name:      "F100",
					DeviceIDs: []string{"fpga1"},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 512,
				DiskMB:   4 * 1024,
				Devices: []*structs.RequestedDevice{
					{
						Name:  "gpu",
						Count: 4,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[2]: {},
				allocIDs[3]: {},
			},
		},
		{
			// This test cases creates allocations across two GPUs
			// Both GPUs are eligible for the task, but only allocs with the lower
			// priority are chosen
			desc: "Preemption with lower/higher priority combinations",
			currentAllocations: []*structs.Allocation{
				createAllocWithDevice(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      500,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "1080ti",
					DeviceIDs: []string{deviceIDs[0], deviceIDs[1]},
				}),
				createAllocWithDevice(allocIDs[1], lowPrioJob2, &structs.Resources{
					CPU:      200,
					MemoryMB: 100,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "1080ti",
					DeviceIDs: []string{deviceIDs[2], deviceIDs[3]},
				}),
				createAllocWithDevice(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "2080ti",
					DeviceIDs: []string{deviceIDs[4], deviceIDs[5]},
				}),
				createAllocWithDevice(allocIDs[3], lowPrioJob, &structs.Resources{
					CPU:      100,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "2080ti",
					DeviceIDs: []string{deviceIDs[6], deviceIDs[7]},
				}),
				createAllocWithDevice(allocIDs[4], lowPrioJob, &structs.Resources{
					CPU:      100,
					MemoryMB: 256,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "2080ti",
					DeviceIDs: []string{deviceIDs[8]},
				}),
				createAllocWithDevice(allocIDs[5], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "fpga",
					Vendor:    "intel",
					Name:      "F100",
					DeviceIDs: []string{"fpga1"},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 512,
				DiskMB:   4 * 1024,
				Devices: []*structs.RequestedDevice{
					{
						Name:  "gpu",
						Count: 4,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[2]: {},
				allocIDs[3]: {},
			},
		},
		{
			desc: "Device preemption not possible due to more instances needed than available",
			currentAllocations: []*structs.Allocation{
				createAllocWithDevice(allocIDs[0], lowPrioJob, &structs.Resources{
					CPU:      500,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "gpu",
					Vendor:    "nvidia",
					Name:      "1080ti",
					DeviceIDs: []string{deviceIDs[0], deviceIDs[1], deviceIDs[2], deviceIDs[3]},
				}),
				createAllocWithDevice(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      200,
					MemoryMB: 512,
					DiskMB:   4 * 1024,
				}, &structs.AllocatedDeviceResource{
					Type:      "fpga",
					Vendor:    "intel",
					Name:      "F100",
					DeviceIDs: []string{"fpga1"},
				})},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 512,
				DiskMB:   4 * 1024,
				Devices: []*structs.RequestedDevice{
					{
						Name:  "gpu",
						Count: 6,
					},
				},
			},
		},
		// This test case exercises the code path for a final filtering step that tries to
		// minimize the number of preemptible allocations
		{
			desc: "Filter out allocs whose resource usage superset is also in the preemption list",
			currentAllocations: []*structs.Allocation{
				createAlloc(allocIDs[0], highPrioJob, &structs.Resources{
					CPU:      1800,
					MemoryMB: 2256,
					DiskMB:   4 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  150,
						},
					},
				}),
				createAlloc(allocIDs[1], lowPrioJob, &structs.Resources{
					CPU:      1500,
					MemoryMB: 256,
					DiskMB:   5 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.100",
							MBits:  100,
						},
					},
				}),
				createAlloc(allocIDs[2], lowPrioJob, &structs.Resources{
					CPU:      600,
					MemoryMB: 256,
					DiskMB:   5 * 1024,
					Networks: []*structs.NetworkResource{
						{
							Device: "eth0",
							IP:     "192.168.0.200",
							MBits:  300,
						},
					},
				}),
			},
			nodeReservedCapacity: reservedNodeResources,
			nodeCapacity:         defaultNodeResources,
			jobPriority:          100,
			resourceAsk: &structs.Resources{
				CPU:      1000,
				MemoryMB: 256,
				DiskMB:   5 * 1024,
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  50,
					},
				},
			},
			preemptedAllocIDs: map[string]struct{}{
				allocIDs[1]: {},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			node := mock.Node()
			node.NodeResources = tc.nodeCapacity
			node.ReservedResources = tc.nodeReservedCapacity

			state, ctx := testContext(t)

			nodes := []*RankedNode{
				{
					Node: node,
				},
			}
			state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
			for _, alloc := range tc.currentAllocations {
				alloc.NodeID = node.ID
			}
			require := require.New(t)
			err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, tc.currentAllocations)

			require.Nil(err)
			if tc.currentPreemptions != nil {
				ctx.plan.NodePreemptions[node.ID] = tc.currentPreemptions
			}
			static := NewStaticRankIterator(ctx, nodes)
			binPackIter := NewBinPackIterator(ctx, static, true, tc.jobPriority)
			job := mock.Job()
			job.Priority = tc.jobPriority
			binPackIter.SetJob(job)
			binPackIter.SetSchedulerConfiguration(testSchedulerConfig)

			taskGroup := &structs.TaskGroup{
				EphemeralDisk: &structs.EphemeralDisk{},
				Tasks: []*structs.Task{
					{
						Name:      "web",
						Resources: tc.resourceAsk,
					},
				},
			}

			binPackIter.SetTaskGroup(taskGroup)
			option := binPackIter.Next()
			if tc.preemptedAllocIDs == nil {
				require.Nil(option)
			} else {
				require.NotNil(option)
				preemptedAllocs := option.PreemptedAllocs
				require.Equal(len(tc.preemptedAllocIDs), len(preemptedAllocs))
				for _, alloc := range preemptedAllocs {
					_, ok := tc.preemptedAllocIDs[alloc.ID]
					require.Truef(ok, "alloc %s was preempted unexpectedly", alloc.ID)
				}
			}
		})
	}
}

// TestPreemptionMultiple tests evicting multiple allocations in the same time
func TestPreemptionMultiple(t *testing.T) {
	ci.Parallel(t)

	// The test setup:
	//  * a node with 4 GPUs
	//  * a low priority job with 4 allocs, each is using 1 GPU
	//
	// Then schedule a high priority job needing 2 allocs, using 2 GPUs each.
	// Expectation:
	// All low priority allocs should preempted to accomodate the high priority job
	h := NewHarness(t)

	// node with 4 GPUs
	node := mock.Node()
	node.NodeResources = &structs.NodeResources{
		Cpu: structs.NodeCpuResources{
			CpuShares: 4000,
		},
		Memory: structs.NodeMemoryResources{
			MemoryMB: 8192,
		},
		Disk: structs.NodeDiskResources{
			DiskMB: 100 * 1024,
		},
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				CIDR:   "192.168.0.100/32",
				MBits:  1000,
			},
		},
		Devices: []*structs.NodeDeviceResource{
			{
				Type:   "gpu",
				Vendor: "nvidia",
				Name:   "1080ti",
				Attributes: map[string]*psstructs.Attribute{
					"memory":           psstructs.NewIntAttribute(11, psstructs.UnitGiB),
					"cuda_cores":       psstructs.NewIntAttribute(3584, ""),
					"graphics_clock":   psstructs.NewIntAttribute(1480, psstructs.UnitMHz),
					"memory_bandwidth": psstructs.NewIntAttribute(11, psstructs.UnitGBPerS),
				},
				Instances: []*structs.NodeDevice{
					{
						ID:      "dev0",
						Healthy: true,
					},
					{
						ID:      "dev1",
						Healthy: true,
					},
					{
						ID:      "dev2",
						Healthy: true,
					},
					{
						ID:      "dev3",
						Healthy: true,
					},
				},
			},
		},
	}

	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// low priority job with 4 allocs using all 4 GPUs
	lowPrioJob := mock.Job()
	lowPrioJob.Priority = 5
	lowPrioJob.TaskGroups[0].Count = 4
	lowPrioJob.TaskGroups[0].Networks = nil
	lowPrioJob.TaskGroups[0].Tasks[0].Services = nil
	lowPrioJob.TaskGroups[0].Tasks[0].Resources.Networks = nil
	lowPrioJob.TaskGroups[0].Tasks[0].Resources.Devices = structs.ResourceDevices{{
		Name:  "gpu",
		Count: 1,
	}}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, lowPrioJob))

	allocs := []*structs.Allocation{}
	allocIDs := map[string]struct{}{}
	for i := 0; i < 4; i++ {
		alloc := createAllocWithDevice(uuid.Generate(), lowPrioJob, lowPrioJob.TaskGroups[0].Tasks[0].Resources, &structs.AllocatedDeviceResource{
			Type:      "gpu",
			Vendor:    "nvidia",
			Name:      "1080ti",
			DeviceIDs: []string{fmt.Sprintf("dev%d", i)},
		})
		alloc.NodeID = node.ID

		allocs = append(allocs, alloc)
		allocIDs[alloc.ID] = struct{}{}
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// new high priority job with 2 allocs, each using 2 GPUs
	highPrioJob := mock.Job()
	highPrioJob.Priority = 100
	highPrioJob.TaskGroups[0].Count = 2
	highPrioJob.TaskGroups[0].Networks = nil
	highPrioJob.TaskGroups[0].Tasks[0].Services = nil
	highPrioJob.TaskGroups[0].Tasks[0].Resources.Networks = nil
	highPrioJob.TaskGroups[0].Tasks[0].Resources.Devices = structs.ResourceDevices{{
		Name:  "gpu",
		Count: 2,
	}}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, highPrioJob))

	// schedule
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    highPrioJob.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       highPrioJob.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	require.NoError(t, h.Process(NewServiceScheduler, eval))
	require.Len(t, h.Plans, 1)
	require.Contains(t, h.Plans[0].NodePreemptions, node.ID)

	preempted := map[string]struct{}{}
	for _, alloc := range h.Plans[0].NodePreemptions[node.ID] {
		preempted[alloc.ID] = struct{}{}
	}
	require.Equal(t, allocIDs, preempted)
}

// helper method to create allocations with given jobs and resources
func createAlloc(id string, job *structs.Job, resource *structs.Resources) *structs.Allocation {
	return createAllocInner(id, job, resource, nil, nil)
}

// helper method to create allocation with network at the task group level
func createAllocWithTaskgroupNetwork(id string, job *structs.Job, resource *structs.Resources, tgNet *structs.NetworkResource) *structs.Allocation {
	return createAllocInner(id, job, resource, nil, tgNet)
}

func createAllocWithDevice(id string, job *structs.Job, resource *structs.Resources, allocatedDevices *structs.AllocatedDeviceResource) *structs.Allocation {
	return createAllocInner(id, job, resource, allocatedDevices, nil)
}

func createAllocInner(id string, job *structs.Job, resource *structs.Resources, allocatedDevices *structs.AllocatedDeviceResource, tgNetwork *structs.NetworkResource) *structs.Allocation {
	alloc := &structs.Allocation{
		ID:    id,
		Job:   job,
		JobID: job.ID,
		TaskResources: map[string]*structs.Resources{
			"web": resource,
		},
		Namespace:     structs.DefaultNamespace,
		EvalID:        uuid.Generate(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusRunning,
		TaskGroup:     "web",
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: int64(resource.CPU),
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: int64(resource.MemoryMB),
					},
					Networks: resource.Networks,
				},
			},
		},
	}

	if allocatedDevices != nil {
		alloc.AllocatedResources.Tasks["web"].Devices = []*structs.AllocatedDeviceResource{allocatedDevices}
	}

	if tgNetwork != nil {
		alloc.AllocatedResources.Shared = structs.AllocatedSharedResources{
			Networks: []*structs.NetworkResource{tgNetwork},
		}
	}
	return alloc
}
