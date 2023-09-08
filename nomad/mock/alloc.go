// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"math/rand"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func Alloc() *structs.Allocation {
	job := Job()
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "web",

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
			DiskMB:   150,
			Networks: []*structs.NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
					MBits:         50,
					DynamicPorts:  []structs.Port{{Label: "http"}},
				},
			},
		},
		TaskResources: map[string]*structs.Resources{
			"web": {
				CPU:      500,
				MemoryMB: 256,
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
						MBits:         50,
						DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
					},
				},
			},
		},
		SharedResources: &structs.Resources{
			DiskMB: 150,
		},

		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 500,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
					Networks: []*structs.NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
							MBits:         50,
							DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
						},
					},
				},
			},
			Shared: structs.AllocatedSharedResources{
				DiskMB: 150,
			},
		},
		Job:           job,
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	alloc.Canonicalize()
	return alloc
}

func MinAlloc() *structs.Allocation {
	job := MinJob()
	group := job.TaskGroups[0]
	task := group.Tasks[0]
	return &structs.Allocation{
		ID:            uuid.Generate(),
		EvalID:        uuid.Generate(),
		NodeID:        uuid.Generate(),
		Job:           job,
		TaskGroup:     group.Name,
		ClientStatus:  structs.AllocClientStatusPending,
		DesiredStatus: structs.AllocDesiredStatusRun,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				task.Name: {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 100,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
			},
			Shared: structs.AllocatedSharedResources{
				DiskMB: 150,
			},
		},
	}
}

func AllocWithoutReservedPort() *structs.Allocation {
	alloc := Alloc()
	alloc.Resources.Networks[0].ReservedPorts = nil
	alloc.TaskResources["web"].Networks[0].ReservedPorts = nil
	alloc.AllocatedResources.Tasks["web"].Networks[0].ReservedPorts = nil

	return alloc
}

func AllocForNode(n *structs.Node) *structs.Allocation {
	nodeIP := n.NodeResources.NodeNetworks[0].Addresses[0].Address

	dynamicPortRange := structs.DefaultMaxDynamicPort - structs.DefaultMinDynamicPort
	randomDynamicPort := rand.Intn(dynamicPortRange) + structs.DefaultMinDynamicPort

	alloc := Alloc()
	alloc.NodeID = n.ID

	// Set node IP address.
	alloc.Resources.Networks[0].IP = nodeIP
	alloc.TaskResources["web"].Networks[0].IP = nodeIP
	alloc.AllocatedResources.Tasks["web"].Networks[0].IP = nodeIP

	// Set dynamic port to a random value.
	alloc.TaskResources["web"].Networks[0].DynamicPorts = []structs.Port{{Label: "http", Value: randomDynamicPort}}
	alloc.AllocatedResources.Tasks["web"].Networks[0].DynamicPorts = []structs.Port{{Label: "http", Value: randomDynamicPort}}

	return alloc

}

func AllocForNodeWithoutReservedPort(n *structs.Node) *structs.Allocation {
	nodeIP := n.NodeResources.NodeNetworks[0].Addresses[0].Address

	dynamicPortRange := structs.DefaultMaxDynamicPort - structs.DefaultMinDynamicPort
	randomDynamicPort := rand.Intn(dynamicPortRange) + structs.DefaultMinDynamicPort

	alloc := AllocWithoutReservedPort()
	alloc.NodeID = n.ID

	// Set node IP address.
	alloc.Resources.Networks[0].IP = nodeIP
	alloc.TaskResources["web"].Networks[0].IP = nodeIP
	alloc.AllocatedResources.Tasks["web"].Networks[0].IP = nodeIP

	// Set dynamic port to a random value.
	alloc.TaskResources["web"].Networks[0].DynamicPorts = []structs.Port{{Label: "http", Value: randomDynamicPort}}
	alloc.AllocatedResources.Tasks["web"].Networks[0].DynamicPorts = []structs.Port{{Label: "http", Value: randomDynamicPort}}

	return alloc
}

func SysBatchAlloc() *structs.Allocation {
	job := SystemBatchJob()
	return &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "pinger",
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"ping-example": {
					Cpu:    structs.AllocatedCpuResources{CpuShares: 500},
					Memory: structs.AllocatedMemoryResources{MemoryMB: 256},
					Networks: []*structs.NetworkResource{{
						Device: "eth0",
						IP:     "192.168.0.100",
					}},
				},
			},
			Shared: structs.AllocatedSharedResources{DiskMB: 150},
		},
		Job:           job,
		JobID:         job.ID,
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
}

func SystemAlloc() *structs.Allocation {
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "web",

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
			DiskMB:   150,
			Networks: []*structs.NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
					MBits:         50,
					DynamicPorts:  []structs.Port{{Label: "http"}},
				},
			},
		},
		TaskResources: map[string]*structs.Resources{
			"web": {
				CPU:      500,
				MemoryMB: 256,
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
						MBits:         50,
						DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
					},
				},
			},
		},
		SharedResources: &structs.Resources{
			DiskMB: 150,
		},

		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 500,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
					Networks: []*structs.NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
							MBits:         50,
							DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
						},
					},
				},
			},
			Shared: structs.AllocatedSharedResources{
				DiskMB: 150,
			},
		},
		Job:           SystemJob(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}
