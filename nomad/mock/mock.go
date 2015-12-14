package mock

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

func Node() *structs.Node {
	node := &structs.Node{
		ID:         structs.GenerateUUID(),
		Datacenter: "dc1",
		Name:       "foobar",
		Attributes: map[string]string{
			"kernel.name": "linux",
			"arch":        "x86",
			"version":     "0.1.0",
			"driver.exec": "1",
		},
		Resources: &structs.Resources{
			CPU:      4000,
			MemoryMB: 8192,
			DiskMB:   100 * 1024,
			IOPS:     150,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		},
		Reserved: &structs.Resources{
			CPU:      100,
			MemoryMB: 256,
			DiskMB:   4 * 1024,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []structs.Port{{Label: "main", Value: 22}},
					MBits:         1,
				},
			},
		},
		Links: map[string]string{
			"consul": "foobar.dc1",
		},
		Meta: map[string]string{
			"pci-dss": "true",
		},
		NodeClass: "linux-medium-pci",
		Status:    structs.NodeStatusReady,
	}
	return node
}

func Job() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		ID:          structs.GenerateUUID(),
		Name:        "my-job",
		Type:        structs.JobTypeService,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*structs.Constraint{
			&structs.Constraint{
				LTarget: "$attr.kernel.name",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			&structs.TaskGroup{
				Name:  "web",
				Count: 10,
				RestartPolicy: &structs.RestartPolicy{
					Attempts: 3,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
				},
				Tasks: []*structs.Task{
					&structs.Task{
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
							},
						},
						Resources: &structs.Resources{
							CPU:      500,
							MemoryMB: 256,
							Networks: []*structs.NetworkResource{
								&structs.NetworkResource{
									MBits:        50,
									DynamicPorts: []structs.Port{{Label: "http"}},
								},
							},
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
		Status:      structs.JobStatusPending,
		CreateIndex: 42,
		ModifyIndex: 99,
	}
	return job
}

func SystemJob() *structs.Job {
	job := &structs.Job{
		Region:      "global",
		ID:          structs.GenerateUUID(),
		Name:        "my-job",
		Type:        structs.JobTypeSystem,
		Priority:    100,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*structs.Constraint{
			&structs.Constraint{
				LTarget: "$attr.kernel.name",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*structs.TaskGroup{
			&structs.TaskGroup{
				Name:  "web",
				Count: 1,
				RestartPolicy: &structs.RestartPolicy{
					Attempts: 3,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
				},
				Tasks: []*structs.Task{
					&structs.Task{
						Name:   "web",
						Driver: "exec",
						Config: map[string]interface{}{
							"command": "/bin/date",
						},
						Resources: &structs.Resources{
							CPU:      500,
							MemoryMB: 256,
							Networks: []*structs.NetworkResource{
								&structs.NetworkResource{
									MBits:        50,
									DynamicPorts: []structs.Port{{Label: "http"}},
								},
							},
						},
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
	return job
}

func Eval() *structs.Evaluation {
	eval := &structs.Evaluation{
		ID:       structs.GenerateUUID(),
		Priority: 50,
		Type:     structs.JobTypeService,
		JobID:    structs.GenerateUUID(),
		Status:   structs.EvalStatusPending,
	}
	return eval
}

func Alloc() *structs.Allocation {
	alloc := &structs.Allocation{
		ID:        structs.GenerateUUID(),
		EvalID:    structs.GenerateUUID(),
		NodeID:    "foo",
		TaskGroup: "web",
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []structs.Port{{Label: "main", Value: 12345}},
					MBits:         100,
					DynamicPorts:  []structs.Port{{Label: "http"}},
				},
			},
		},
		TaskResources: map[string]*structs.Resources{
			"web": &structs.Resources{
				CPU:      500,
				MemoryMB: 256,
				Networks: []*structs.NetworkResource{
					&structs.NetworkResource{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "main", Value: 5000}},
						MBits:         50,
						DynamicPorts:  []structs.Port{{Label: "http"}},
					},
				},
			},
		},
		TaskStates: map[string]*structs.TaskState{
			"web": &structs.TaskState{
				State: structs.TaskStatePending,
			},
		},
		Job:           Job(),
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
	}
	alloc.JobID = alloc.Job.ID
	return alloc
}

func Plan() *structs.Plan {
	return &structs.Plan{
		Priority: 50,
	}
}

func PlanResult() *structs.PlanResult {
	return &structs.PlanResult{}
}
