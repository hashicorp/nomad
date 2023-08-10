// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

func Node() *structs.Node {
	node := &structs.Node{
		ID:         uuid.Generate(),
		SecretID:   uuid.Generate(),
		Datacenter: "dc1",
		Name:       "foobar",
		Drivers: map[string]*structs.DriverInfo{
			"exec": {
				Detected: true,
				Healthy:  true,
			},
			"mock_driver": {
				Detected: true,
				Healthy:  true,
			},
		},
		Attributes: map[string]string{
			"kernel.name":        "linux",
			"arch":               "x86",
			"nomad.version":      "0.5.0",
			"driver.exec":        "1",
			"driver.mock_driver": "1",
			"consul.version":     "1.11.4",
		},

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      4000,
			MemoryMB: 8192,
			DiskMB:   100 * 1024,
		},
		Reserved: &structs.Resources{
			CPU:      100,
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

		NodeResources: &structs.NodeResources{
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
					Mode:   "host",
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
			NodeNetworks: []*structs.NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Speed:  1000,
					Addresses: []structs.NodeNetworkAddress{
						{
							Alias:   "default",
							Address: "192.168.0.100",
							Family:  structs.NodeNetworkAF_IPv4,
						},
					},
				},
			},
		},
		ReservedResources: &structs.NodeReservedResources{
			Cpu: structs.NodeReservedCpuResources{
				CpuShares: 100,
			},
			Memory: structs.NodeReservedMemoryResources{
				MemoryMB: 256,
			},
			Disk: structs.NodeReservedDiskResources{
				DiskMB: 4 * 1024,
			},
			Networks: structs.NodeReservedNetworkResources{
				ReservedHostPorts: "22",
			},
		},
		Links: map[string]string{
			"consul": "foobar.dc1",
		},
		Meta: map[string]string{
			"pci-dss":  "true",
			"database": "mysql",
			"version":  "5.6",
		},
		NodeClass:             "linux-medium-pci",
		NodePool:              structs.NodePoolDefault,
		Status:                structs.NodeStatusReady,
		SchedulingEligibility: structs.NodeSchedulingEligible,
	}
	_ = node.ComputeClass()
	return node
}

func DrainNode() *structs.Node {
	node := Node()
	node.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{},
	}
	node.Canonicalize()
	return node
}

// NvidiaNode returns a node with two instances of an Nvidia GPU
func NvidiaNode() *structs.Node {
	n := Node()
	n.NodeResources.Devices = []*structs.NodeDeviceResource{
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
					ID:      uuid.Generate(),
					Healthy: true,
				},
				{
					ID:      uuid.Generate(),
					Healthy: true,
				},
			},
		},
	}
	_ = n.ComputeClass()
	return n
}
