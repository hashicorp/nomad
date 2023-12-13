// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"time"

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
					CpuShares: n.Cpu.CpuShares,
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
			Cpu: NodeCpuResources{
				CpuShares: 4000,
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
