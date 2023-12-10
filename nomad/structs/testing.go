// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/helper/uuid"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

// NodeResourcesToAllocatedResources converts a node resources to an allocated
// resources. The task name used is "web" and network is omitted. This is
// useful when trying to make an allocation fill an entire node.
func NodeResourcesToAllocatedResources(n *NodeResources) *AllocatedResources {
	if n == nil {
		return nil
	}

	return &AllocatedResources{
		Tasks: map[string]*AllocatedTaskResources{
			"web": {
				Cpu: AllocatedCpuResources{
					CpuShares: int64(n.Processors.Topology.UsableCompute()),
				},
				Memory: AllocatedMemoryResources{
					MemoryMB: n.Memory.MemoryMB,
				},
			},
		},
		Shared: AllocatedSharedResources{
			DiskMB: n.Disk.DiskMB,
		},
	}
}

// MockBasicTopology returns a numalib.Topology that looks likes a simple VM;
// - 1 socket, 1 NUMA node
// - 4 cores @ 3500 MHz (14,000 MHz total)
// - no client config overrides
func MockBasicTopology() *numalib.Topology {
	cores := make([]numalib.Core, 4)
	for i := 0; i < 4; i++ {
		cores[i] = numalib.Core{
			SocketID:  0,
			NodeID:    0,
			ID:        hw.CoreID(i),
			Grade:     numalib.Performance,
			Disable:   false,
			BaseSpeed: 3500,
		}
	}
	return &numalib.Topology{
		NodeIDs:                idset.From[hw.NodeID]([]hw.NodeID{0}),
		Distances:              numalib.SLIT{[]numalib.Cost{10}},
		Cores:                  cores,
		OverrideTotalCompute:   0,
		OverrideWitholdCompute: 0,
	}
}

// MockWorkstationTopology returns a numalib.Topology that looks like a typical
// workstation;
// - 2 socket, 2 NUMA node (200% penalty)
// - 16 cores / 32 threads @ 3000 MHz (96,000 MHz total)
// - node0: odd cores, node1: even cores
// - no client config overrides
func MockWorkstationTopology() *numalib.Topology {
	cores := make([]numalib.Core, 32)
	for i := 0; i < 32; i++ {
		cores[i] = numalib.Core{
			SocketID:  hw.SocketID(i % 2),
			NodeID:    hw.NodeID(i % 2),
			ID:        hw.CoreID(i),
			Grade:     numalib.Performance,
			Disable:   false,
			BaseSpeed: 3_000,
		}
	}
	return &numalib.Topology{
		NodeIDs:   idset.From[hw.NodeID]([]hw.NodeID{0, 1}),
		Distances: numalib.SLIT{[]numalib.Cost{10, 20}, {20, 10}},
		Cores:     cores,
	}
}

// MockR6aTopology returns a numalib.Topology that looks like an EC2 r6a.metal
// instance type:
// - 2 socket, 4 NUMA node
// - 120% penalty for intra socket, 320% penalty for cross socket
// - 192 logical cores @ 3731 MHz (716362)
// - node0: 0-23, 96-119   (socket 0)
// - node1: 24-47, 120-143 (socket 0)
// - node2: 48-71, 144-167 (socket 1)
// - node3: 72-95, 168-191 (socket 1)
func MockR6aTopology() *numalib.Topology {
	cores := make([]numalib.Core, 192)
	makeCore := func(socketID hw.SocketID, nodeID hw.NodeID, id int) numalib.Core {
		return numalib.Core{
			SocketID:  socketID,
			NodeID:    nodeID,
			ID:        hw.CoreID(id),
			Grade:     numalib.Performance,
			BaseSpeed: 3731,
		}
	}
	for i := 0; i <= 23; i++ {
		cores[i] = makeCore(0, 0, i)
		cores[i+96] = makeCore(0, 0, i+96)
	}
	for i := 24; i <= 47; i++ {
		cores[i] = makeCore(0, 1, i)
		cores[i+96] = makeCore(0, 1, i+96)
	}
	for i := 48; i <= 71; i++ {
		cores[i] = makeCore(1, 2, i)
		cores[i+96] = makeCore(1, 2, i+96)
	}
	for i := 72; i <= 95; i++ {
		cores[i] = makeCore(1, 3, i)
		cores[i+96] = makeCore(1, 3, i+96)
	}

	distances := numalib.SLIT{
		[]numalib.Cost{10, 12, 32, 32},
		[]numalib.Cost{12, 10, 32, 32},
		[]numalib.Cost{32, 32, 10, 12},
		[]numalib.Cost{32, 32, 12, 10},
	}

	return &numalib.Topology{
		NodeIDs:   idset.From[hw.NodeID]([]hw.NodeID{0, 1, 2, 3}),
		Distances: distances,
		Cores:     cores,
	}
}

func MockNode() *Node {
	node := &Node{
		ID:         uuid.Generate(),
		SecretID:   uuid.Generate(),
		Datacenter: "dc1",
		Name:       "foobar",
		Attributes: map[string]string{
			"kernel.name":        "linux",
			"arch":               "x86",
			"nomad.version":      "1.0.0",
			"driver.exec":        "1",
			"driver.mock_driver": "1",
		},
		NodeResources: &NodeResources{
			Processors: NodeProcessorResources{
				Topology: MockBasicTopology(),
			},
			Memory: NodeMemoryResources{
				MemoryMB: 8192,
			},
			Disk: NodeDiskResources{
				DiskMB: 100 * 1024,
			},
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Cpu: NodeReservedCpuResources{
				CpuShares: 100,
			},
			Memory: NodeReservedMemoryResources{
				MemoryMB: 256,
			},
			Disk: NodeReservedDiskResources{
				DiskMB: 4 * 1024,
			},
			Networks: NodeReservedNetworkResources{
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
		Status:                NodeStatusReady,
		SchedulingEligibility: NodeSchedulingEligible,
	}
	err := node.ComputeClass()
	if err != nil {
		panic(fmt.Sprintf("failed to compute node class: %v", err))
	}
	return node
}

// MockNvidiaNode returns a node with two instances of an Nvidia GPU
func MockNvidiaNode() *Node {
	n := MockNode()
	n.NodeResources.Devices = []*NodeDeviceResource{
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
			Instances: []*NodeDevice{
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
	err := n.ComputeClass()
	if err != nil {
		panic(fmt.Sprintf("failed to compute node class: %v", err))
	}
	return n
}

func MockJob() *Job {
	job := &Job{
		Region:      "global",
		ID:          fmt.Sprintf("mock-service-%s", uuid.Generate()),
		Name:        "my-job",
		Namespace:   DefaultNamespace,
		Type:        JobTypeService,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*TaskGroup{
			{
				Name:  "web",
				Count: 10,
				EphemeralDisk: &EphemeralDisk{
					SizeMB: 150,
				},
				RestartPolicy: &RestartPolicy{
					Attempts: 3,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
					Mode:     RestartPolicyModeDelay,
				},
				ReschedulePolicy: &ReschedulePolicy{
					Attempts:      2,
					Interval:      10 * time.Minute,
					Delay:         5 * time.Second,
					DelayFunction: "constant",
				},
				Migrate: DefaultMigrateStrategy(),
				Tasks: []*Task{
					{
						Name:   "web",
						Driver: "exec",
						Config: map[string]interface{}{
							"command": "/bin/date",
						},
						Env: map[string]string{
							"FOO": "bar",
						},
						Services: []*Service{
							{
								Name:      "${TASK}-frontend",
								PortLabel: "http",
								Tags:      []string{"pci:${meta.pci-dss}", "datacenter:${node.datacenter}"},
								Checks: []*ServiceCheck{
									{
										Name:     "check-table",
										Type:     ServiceCheckScript,
										Command:  "/usr/local/check-table-${meta.database}",
										Args:     []string{"${meta.version}"},
										Interval: 30 * time.Second,
										Timeout:  5 * time.Second,
									},
								},
							},
							{
								Name:      "${TASK}-admin",
								PortLabel: "admin",
							},
						},
						LogConfig: DefaultLogConfig(),
						Resources: &Resources{
							CPU:      500,
							MemoryMB: 256,
							Networks: []*NetworkResource{
								{
									MBits: 50,
									DynamicPorts: []Port{
										{Label: "http"},
										{Label: "admin"},
									},
								},
							},
						},
						Meta: map[string]string{
							"foo": "bar",
						},
					},
				},
				Meta: map[string]string{
					"elb_check_type":     "http",
					"elb_check_interval": "30s",
					"elb_check_min":      "3",
				},
			},
		},
		Meta: map[string]string{
			"owner": "armon",
		},
		Status:         JobStatusPending,
		Version:        0,
		CreateIndex:    42,
		ModifyIndex:    99,
		JobModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

func MockAlloc() *Allocation {
	alloc := &Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: DefaultNamespace,
		TaskGroup: "web",
		AllocatedResources: &AllocatedResources{
			Tasks: map[string]*AllocatedTaskResources{
				"web": {
					Cpu: AllocatedCpuResources{
						CpuShares: 500,
					},
					Memory: AllocatedMemoryResources{
						MemoryMB: 256,
					},
					Networks: []*NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							ReservedPorts: []Port{{Label: "admin", Value: 5000}},
							MBits:         50,
							DynamicPorts:  []Port{{Label: "http", Value: 9876}},
						},
					},
				},
			},
			Shared: AllocatedSharedResources{
				DiskMB: 150,
			},
		},
		Job:           MockJob(),
		DesiredStatus: AllocDesiredStatusRun,
		ClientStatus:  AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}
