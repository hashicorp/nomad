package mock

import (
	"fmt"
	"time"

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
		Attributes: map[string]string{
			"kernel.name":        "linux",
			"arch":               "x86",
			"nomad.version":      "0.5.0",
			"driver.exec":        "1",
			"driver.mock_driver": "1",
		},

		// TODO Remove once clientv2 gets merged
		Resources: &structs.Resources{
			CPU:      4000,
			MemoryMB: 8192,
			DiskMB:   100 * 1024,
			IOPS:     150,
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
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
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
		Status:                structs.NodeStatusReady,
		SchedulingEligibility: structs.NodeSchedulingEligible,
	}
	node.ComputeClass()
	return node
}

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
	n.ComputeClass()
	return n
}

func HCL() string {
	return `job "my-job" {
	datacenters = ["dc1"]
	type = "service"
	constraint {
		attribute = "${attr.kernel.name}"
		value = "linux"
	}

	group "web" {
		count = 10
		restart {
			attempts = 3
			interval = "10m"
			delay = "1m"
			mode = "delay"
		}
		task "web" {
			driver = "exec"
			config {
				command = "/bin/date"
			}
			resources {
				cpu = 500
				memory = 256
			}
		}
	}
}
`
}

func Job() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		ID:          fmt.Sprintf("mock-service-%s", uuid.Generate()),
		Name:        "my-job",
		Namespace:   structs.DefaultNamespace,
		Type:        structs.JobTypeService,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*structs.Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:  "web",
				Count: 10,
				EphemeralDisk: &structs.EphemeralDisk{
					SizeMB: 150,
				},
				RestartPolicy: &structs.RestartPolicy{
					Attempts: 3,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
					Mode:     structs.RestartPolicyModeDelay,
				},
				ReschedulePolicy: &structs.ReschedulePolicy{
					Attempts:      2,
					Interval:      10 * time.Minute,
					Delay:         5 * time.Second,
					DelayFunction: "constant",
				},
				Migrate: structs.DefaultMigrateStrategy(),
				Tasks: []*structs.Task{
					{
						Name:   "web",
						Driver: "exec",
						Config: map[string]interface{}{
							"command": "/bin/date",
						},
						Env: map[string]string{
							"FOO": "bar",
						},
						Services: []*structs.Service{
							{
								Name:      "${TASK}-frontend",
								PortLabel: "http",
								Tags:      []string{"pci:${meta.pci-dss}", "datacenter:${node.datacenter}"},
								Checks: []*structs.ServiceCheck{
									{
										Name:     "check-table",
										Type:     structs.ServiceCheckScript,
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
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      500,
							MemoryMB: 256,
							Networks: []*structs.NetworkResource{
								{
									MBits: 50,
									DynamicPorts: []structs.Port{
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
		Status:         structs.JobStatusPending,
		Version:        0,
		CreateIndex:    42,
		ModifyIndex:    99,
		JobModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

func BatchJob() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		ID:          fmt.Sprintf("mock-batch-%s", uuid.Generate()),
		Name:        "batch-job",
		Namespace:   structs.DefaultNamespace,
		Type:        structs.JobTypeBatch,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:  "worker",
				Count: 10,
				EphemeralDisk: &structs.EphemeralDisk{
					SizeMB: 150,
				},
				RestartPolicy: &structs.RestartPolicy{
					Attempts: 3,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
					Mode:     structs.RestartPolicyModeDelay,
				},
				ReschedulePolicy: &structs.ReschedulePolicy{
					Attempts:      2,
					Interval:      10 * time.Minute,
					Delay:         5 * time.Second,
					DelayFunction: "constant",
				},
				Tasks: []*structs.Task{
					{
						Name:   "worker",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"run_for": 500 * time.Millisecond,
						},
						Env: map[string]string{
							"FOO": "bar",
						},
						LogConfig: structs.DefaultLogConfig(),
						Resources: &structs.Resources{
							CPU:      100,
							MemoryMB: 100,
							Networks: []*structs.NetworkResource{
								{
									MBits: 50,
								},
							},
						},
						Meta: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
		},
		Status:         structs.JobStatusPending,
		Version:        0,
		CreateIndex:    43,
		ModifyIndex:    99,
		JobModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

func SystemJob() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		Namespace:   structs.DefaultNamespace,
		ID:          fmt.Sprintf("mock-system-%s", uuid.Generate()),
		Name:        "my-job",
		Type:        structs.JobTypeSystem,
		Priority:    100,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*structs.Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			{
				Name:  "web",
				Count: 1,
				RestartPolicy: &structs.RestartPolicy{
					Attempts: 3,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
					Mode:     structs.RestartPolicyModeDelay,
				},
				EphemeralDisk: structs.DefaultEphemeralDisk(),
				Tasks: []*structs.Task{
					{
						Name:   "web",
						Driver: "exec",
						Config: map[string]interface{}{
							"command": "/bin/date",
						},
						Env: map[string]string{},
						Resources: &structs.Resources{
							CPU:      500,
							MemoryMB: 256,
							Networks: []*structs.NetworkResource{
								{
									MBits:        50,
									DynamicPorts: []structs.Port{{Label: "http"}},
								},
							},
						},
						LogConfig: structs.DefaultLogConfig(),
					},
				},
			},
		},
		Meta: map[string]string{
			"owner": "armon",
		},
		Status:      structs.JobStatusPending,
		CreateIndex: 42,
		ModifyIndex: 99,
	}
	job.Canonicalize()
	return job
}

func PeriodicJob() *structs.Job {
	job := Job()
	job.Type = structs.JobTypeBatch
	job.Periodic = &structs.PeriodicConfig{
		Enabled:  true,
		SpecType: structs.PeriodicSpecCron,
		Spec:     "*/30 * * * *",
	}
	job.Status = structs.JobStatusRunning
	job.TaskGroups[0].Migrate = nil
	return job
}

func Eval() *structs.Evaluation {
	eval := &structs.Evaluation{
		ID:        uuid.Generate(),
		Namespace: structs.DefaultNamespace,
		Priority:  50,
		Type:      structs.JobTypeService,
		JobID:     uuid.Generate(),
		Status:    structs.EvalStatusPending,
	}
	return eval
}

func JobSummary(jobID string) *structs.JobSummary {
	js := &structs.JobSummary{
		JobID:     jobID,
		Namespace: structs.DefaultNamespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Queued:   0,
				Starting: 0,
			},
		},
	}
	return js
}

func Alloc() *structs.Allocation {
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
		Job:           Job(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}

func BatchAlloc() *structs.Allocation {
	alloc := &structs.Allocation{
		ID:        uuid.Generate(),
		EvalID:    uuid.Generate(),
		NodeID:    "12345678-abcd-efab-cdef-123456789abc",
		Namespace: structs.DefaultNamespace,
		TaskGroup: "worker",

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
		Job:           BatchJob(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
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

func VaultAccessor() *structs.VaultAccessor {
	return &structs.VaultAccessor{
		Accessor:    uuid.Generate(),
		NodeID:      uuid.Generate(),
		AllocID:     uuid.Generate(),
		CreationTTL: 86400,
		Task:        "foo",
	}
}

func Deployment() *structs.Deployment {
	return &structs.Deployment{
		ID:             uuid.Generate(),
		JobID:          uuid.Generate(),
		Namespace:      structs.DefaultNamespace,
		JobVersion:     2,
		JobModifyIndex: 20,
		JobCreateIndex: 18,
		TaskGroups: map[string]*structs.DeploymentState{
			"web": {
				DesiredTotal: 10,
			},
		},
		Status:            structs.DeploymentStatusRunning,
		StatusDescription: structs.DeploymentStatusDescriptionRunning,
		ModifyIndex:       23,
		CreateIndex:       21,
	}
}

func Plan() *structs.Plan {
	return &structs.Plan{
		Priority: 50,
	}
}

func PlanResult() *structs.PlanResult {
	return &structs.PlanResult{}
}

func ACLPolicy() *structs.ACLPolicy {
	ap := &structs.ACLPolicy{
		Name:        fmt.Sprintf("policy-%s", uuid.Generate()),
		Description: "Super cool policy!",
		Rules: `
		namespace "default" {
			policy = "write"
		}
		node {
			policy = "read"
		}
		agent {
			policy = "read"
		}
		`,
		CreateIndex: 10,
		ModifyIndex: 20,
	}
	ap.SetHash()
	return ap
}

func ACLToken() *structs.ACLToken {
	tk := &structs.ACLToken{
		AccessorID:  uuid.Generate(),
		SecretID:    uuid.Generate(),
		Name:        "my cool token " + uuid.Generate(),
		Type:        "client",
		Policies:    []string{"foo", "bar"},
		Global:      false,
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 20,
	}
	tk.SetHash()
	return tk
}

func ACLManagementToken() *structs.ACLToken {
	return &structs.ACLToken{
		AccessorID:  uuid.Generate(),
		SecretID:    uuid.Generate(),
		Name:        "management " + uuid.Generate(),
		Type:        "management",
		Global:      true,
		CreateTime:  time.Now().UTC(),
		CreateIndex: 10,
		ModifyIndex: 20,
	}
}
