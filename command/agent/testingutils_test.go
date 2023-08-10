// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
)

func MockJob() *api.Job {
	job := &api.Job{
		Region:      pointer.Of("global"),
		ID:          pointer.Of(uuid.Generate()),
		Name:        pointer.Of("my-job"),
		Type:        pointer.Of("service"),
		Priority:    pointer.Of(50),
		AllAtOnce:   pointer.Of(false),
		Datacenters: []string{"dc1"},
		Constraints: []*api.Constraint{
			{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*api.TaskGroup{
			{
				Name:  pointer.Of("web"),
				Count: pointer.Of(10),
				EphemeralDisk: &api.EphemeralDisk{
					SizeMB: pointer.Of(150),
				},
				RestartPolicy: &api.RestartPolicy{
					Attempts:        pointer.Of(3),
					Interval:        pointer.Of(10 * time.Minute),
					Delay:           pointer.Of(1 * time.Minute),
					Mode:            pointer.Of("delay"),
					RenderTemplates: pointer.Of(false),
				},
				Networks: []*api.NetworkResource{
					{
						Mode:         "host",
						DynamicPorts: []api.Port{{Label: "http"}, {Label: "admin"}},
					},
				},
				Tasks: []*api.Task{
					{
						Name:   "web",
						Driver: "exec",
						Config: map[string]interface{}{
							"command": "/bin/date",
						},
						Env: map[string]string{
							"FOO": "bar",
						},
						Services: []*api.Service{
							{
								Name:      "${TASK}-frontend",
								PortLabel: "http",
								Tags:      []string{"pci:${meta.pci-dss}", "datacenter:${node.datacenter}"},
								Checks: []api.ServiceCheck{
									{
										Name:     "check-table",
										Type:     "script",
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
						LogConfig: api.DefaultLogConfig(),
						Resources: &api.Resources{
							CPU:      pointer.Of(500),
							MemoryMB: pointer.Of(256),
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
	}
	job.Canonicalize()
	return job
}

func MockRegionalJob() *api.Job {
	j := MockJob()
	j.Region = pointer.Of("north-america")
	return j
}

// MockRunnableJob returns a mock job that has a configuration that allows it to be
// placed on a TestAgent.
func MockRunnableJob() *api.Job {
	job := MockJob()

	// Configure job so it can be run on a TestAgent
	job.Constraints = nil
	job.TaskGroups[0].Constraints = nil
	job.TaskGroups[0].Count = pointer.Of(1)
	job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	job.TaskGroups[0].Tasks[0].Services = nil
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}

	return job
}
