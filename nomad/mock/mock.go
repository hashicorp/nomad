// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

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

// HCLVar returns a the HCL of job that requires a HCL variables S, N,
// and B to be set. Also returns the content of a vars-file to satisfy
// those variables.
func HCLVar() (string, string) {
	return `
variable "S" {
  type = string
}

variable "N" {
  type = number
}

variable "B" {
  type = bool
}

job "var-job" {
	type = "batch"
	constraint {
		attribute = "${attr.kernel.name}"
		value = "linux"
	}
	group "group" {
		task "task" {
			driver = "raw_exec"
			config {
				command = "echo"
                args = ["S is ${var.S}, N is ${var.N}, B is ${var.B}"]
			}
			resources {
				cpu = 10
				memory = 32
			}
		}
	}
}
`, `
S = "stringy"
N = 42
B = true
`
}

func Eval() *structs.Evaluation {
	now := time.Now().UTC().UnixNano()
	eval := &structs.Evaluation{
		ID:         uuid.Generate(),
		Namespace:  structs.DefaultNamespace,
		Priority:   50,
		Type:       structs.JobTypeService,
		JobID:      uuid.Generate(),
		Status:     structs.EvalStatusPending,
		CreateTime: now,
		ModifyTime: now,
	}
	return eval
}

func BlockedEval() *structs.Evaluation {
	e := Eval()
	e.Status = structs.EvalStatusBlocked
	e.FailedTGAllocs = map[string]*structs.AllocMetric{
		"cache": {
			DimensionExhausted: map[string]int{
				"memory": 1,
			},
			ResourcesExhausted: map[string]*structs.Resources{
				"redis": {
					CPU:      100,
					MemoryMB: 1024,
				},
			},
		},
	}

	return e
}

func JobSummary(jobID string) *structs.JobSummary {
	return &structs.JobSummary{
		JobID:     jobID,
		Namespace: structs.DefaultNamespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Queued:   0,
				Starting: 0,
			},
		},
	}
}

func JobSysBatchSummary(jobID string) *structs.JobSummary {
	return &structs.JobSummary{
		JobID:     jobID,
		Namespace: structs.DefaultNamespace,
		Summary: map[string]structs.TaskGroupSummary{
			"pinger": {
				Queued:   0,
				Starting: 0,
			},
		},
	}
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

func SITokenAccessor() *structs.SITokenAccessor {
	return &structs.SITokenAccessor{
		NodeID:     uuid.Generate(),
		AllocID:    uuid.Generate(),
		AccessorID: uuid.Generate(),
		TaskName:   "foo",
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

func ScalingPolicy() *structs.ScalingPolicy {
	return &structs.ScalingPolicy{
		ID:   uuid.Generate(),
		Min:  1,
		Max:  100,
		Type: structs.ScalingPolicyTypeHorizontal,
		Target: map[string]string{
			structs.ScalingTargetNamespace: structs.DefaultNamespace,
			structs.ScalingTargetJob:       uuid.Generate(),
			structs.ScalingTargetGroup:     uuid.Generate(),
			structs.ScalingTargetTask:      uuid.Generate(),
		},
		Policy: map[string]interface{}{
			"a": "b",
		},
		Enabled: true,
	}
}

func Events(index uint64) *structs.Events {
	return &structs.Events{
		Index: index,
		Events: []structs.Event{
			{
				Index:   index,
				Topic:   "Node",
				Type:    "update",
				Key:     uuid.Generate(),
				Payload: Node(),
			},
			{
				Index:   index,
				Topic:   "Eval",
				Type:    "update",
				Key:     uuid.Generate(),
				Payload: Eval(),
			},
		},
	}
}

func Namespace() *structs.Namespace {
	id := uuid.Generate()
	ns := &structs.Namespace{
		Name:        fmt.Sprintf("team-%s", id),
		Meta:        map[string]string{"team": id},
		Description: "test namespace",
		CreateIndex: 100,
		ModifyIndex: 200,
	}
	ns.Canonicalize()
	ns.SetHash()
	return ns
}

func NodePool() *structs.NodePool {
	pool := &structs.NodePool{
		Name:        fmt.Sprintf("pool-%s", uuid.Short()),
		Description: "test node pool",
		Meta:        map[string]string{"team": "test"},
	}
	pool.SetHash()
	return pool
}

// ServiceRegistrations generates an array containing two unique service
// registrations.
func ServiceRegistrations() []*structs.ServiceRegistration {
	return []*structs.ServiceRegistration{
		{
			ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
			ServiceName: "example-cache",
			Namespace:   "default",
			NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
			Datacenter:  "dc1",
			JobID:       "example",
			AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
			Tags:        []string{"foo"},
			Address:     "192.168.10.1",
			Port:        23000,
		},
		{
			ID:          "_nomad-task-ca60e901-675a-0ab2-2e57-2f3b05fdc540-group-api-countdash-api-http",
			ServiceName: "countdash-api",
			Namespace:   "platform",
			NodeID:      "ba991c17-7ce5-9c20-78b7-311e63578583",
			Datacenter:  "dc2",
			JobID:       "countdash-api",
			AllocID:     "ca60e901-675a-0ab2-2e57-2f3b05fdc540",
			Tags:        []string{"bar"},
			Address:     "192.168.200.200",
			Port:        29000,
		},
	}
}
