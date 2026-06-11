// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/v2/helper/uuid"
)

func MockJob() *api.Job {
	job := &api.Job{
		Region:      new("global"),
		ID:          new(uuid.Generate()),
		Name:        new("my-job"),
		Type:        new("service"),
		Priority:    new(50),
		AllAtOnce:   new(false),
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
				Name:  new("web"),
				Count: new(10),
				EphemeralDisk: &api.EphemeralDisk{
					SizeMB: new(150),
				},
				RestartPolicy: &api.RestartPolicy{
					Attempts:        new(3),
					Interval:        new(10 * time.Minute),
					Delay:           new(1 * time.Minute),
					Mode:            new("delay"),
					RenderTemplates: new(false),
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
						// actions
						Actions: []*api.Action{
							{
								Name:    "date-test",
								Command: "/bin/date",
								Args:    []string{"-u"},
							},
							{
								Name:    "echo-test",
								Command: "/bin/echo",
								Args:    []string{"hello world"},
							},
						},
						LogConfig: api.DefaultLogConfig(),
						Resources: &api.Resources{
							CPU:      new(500),
							MemoryMB: new(256),
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
	j.Region = new("north-america")
	return j
}

// MockRunnableJob returns a mock job that has a configuration that allows it to be
// placed on a TestAgent.
func MockRunnableJob() *api.Job {
	job := MockJob()

	// Configure job so it can be run on a TestAgent
	job.Constraints = nil
	job.TaskGroups[0].Constraints = nil
	job.TaskGroups[0].Count = new(1)
	job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	job.TaskGroups[0].Tasks[0].Services = nil
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
	}

	return job
}
