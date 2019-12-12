package agent

import (
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
)

func MockJob() *api.Job {
	job := &api.Job{
		Region:      helper.StringToPtr("global"),
		ID:          helper.StringToPtr(uuid.Generate()),
		Name:        helper.StringToPtr("my-job"),
		Type:        helper.StringToPtr("service"),
		Priority:    helper.IntToPtr(50),
		AllAtOnce:   helper.BoolToPtr(false),
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
				Name:  helper.StringToPtr("web"),
				Count: helper.IntToPtr(10),
				EphemeralDisk: &api.EphemeralDisk{
					SizeMB: helper.IntToPtr(150),
				},
				RestartPolicy: &api.RestartPolicy{
					Attempts: helper.IntToPtr(3),
					Interval: helper.TimeToPtr(10 * time.Minute),
					Delay:    helper.TimeToPtr(1 * time.Minute),
					Mode:     helper.StringToPtr("delay"),
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
							CPU:      helper.IntToPtr(500),
							MemoryMB: helper.IntToPtr(256),
							Networks: []*api.NetworkResource{
								{
									MBits:        helper.IntToPtr(50),
									DynamicPorts: []api.Port{{Label: "http"}, {Label: "admin"}},
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
	}
	job.Canonicalize()
	return job
}

func MockRegionalJob() *api.Job {
	j := MockJob()
	j.Region = helper.StringToPtr("north-america")
	return j
}
