package structs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func MockJob() *Job {
	job := &Job{
		Region:      "global",
		ID:          GenerateUUID(),
		Name:        "my-job",
		Type:        JobTypeService,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*Constraint{
			&Constraint{
				LTarget: "${attr.kernel.name}",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*TaskGroup{
			&TaskGroup{
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
				Tasks: []*Task{
					&Task{
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
								&NetworkResource{
									MBits:        50,
									DynamicPorts: []Port{{Label: "http"}, {Label: "admin"}},
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

func TestJob_SentinelObject(t *testing.T) {
	j := MockJob()
	jSent := j.SentinelObject()
	expected := map[string]interface{}{
		"id":          j.ID,
		"parent_id":   "",
		"region":      "global",
		"name":        "my-job",
		"type":        "service",
		"priority":    50,
		"all_at_once": false,
		"datacenters": []string{"dc1"},
		"constraints": []map[string]interface{}{
			map[string]interface{}{
				"operand":      "=",
				"left_target":  "${attr.kernel.name}",
				"right_target": "linux",
				"string":       "${attr.kernel.name} = linux",
			},
		},
		"task_groups": []map[string]interface{}{
			map[string]interface{}{
				"name":        "web",
				"count":       10,
				"update":      map[string]interface{}(nil),
				"constraints": nil,
				"restart_policy": map[string]interface{}{
					"attempts": 3,
					"interval": 10 * time.Minute,
					"delay":    1 * time.Minute,
					"mode":     RestartPolicyModeDelay,
				},
				"ephemeral_disk": map[string]interface{}{
					"size_mb": 150,
					"migrate": false,
					"sticky":  false,
				},
				"meta": map[string]string{
					"elb_check_type":     "http",
					"elb_check_interval": "30s",
					"elb_check_min":      "3",
				},
				"tasks": []map[string]interface{}{
					map[string]interface{}{
						"name":   "web",
						"driver": "exec",
						"config": map[string]interface{}{
							"command": "/bin/date",
						},
						"env": map[string]string{
							"FOO": "bar",
						},
						"log_config": map[string]interface{}{
							"max_files":       10,
							"max_filesize_mb": 10,
						},
						"meta": map[string]string{
							"foo": "bar",
						},
						"leader":           false,
						"kill_timeout":     5 * time.Second,
						"constraints":      nil,
						"dispatch_payload": map[string]interface{}(nil),
						"artifacts":        nil,
						"templates":        nil,
						"vault":            map[string]interface{}(nil),
						"user":             "",
						"resources": map[string]interface{}{
							"cpu":       500,
							"memory_mb": 256,
							"disk_mb":   0,
							"iops":      0,
							"networks": []map[string]interface{}{
								map[string]interface{}{
									"device":         "",
									"cidr":           "",
									"ip":             "",
									"mbits":          50,
									"reserved_ports": nil,
									"dynamic_ports": []map[string]interface{}{
										map[string]interface{}{
											"label": "http",
											"value": 0,
										},
										map[string]interface{}{
											"label": "admin",
											"value": 0,
										},
									},
								},
							},
						},
						"services": []map[string]interface{}{
							map[string]interface{}{
								"name":         "web-frontend",
								"port_label":   "http",
								"address_mode": "",
								"tags": []string{
									"pci:${meta.pci-dss}",
									"datacenter:${node.datacenter}",
								},
								"checks": []map[string]interface{}{
									map[string]interface{}{
										"name":    "check-table",
										"type":    ServiceCheckScript,
										"command": "/usr/local/check-table-${meta.database}",
										"args": []string{
											"${meta.version}",
										},
										"interval":        30 * time.Second,
										"timeout":         5 * time.Second,
										"initial_status":  "",
										"path":            "",
										"port_label":      "",
										"protocol":        "",
										"tls_skip_verify": false,
									},
								},
							},
							map[string]interface{}{
								"name":         "web-admin",
								"port_label":   "admin",
								"address_mode": "",
								"tags":         []string(nil),
								"checks":       nil,
							},
						},
					},
				},
			},
		},
		"periodic":      map[string]interface{}(nil),
		"parameterized": map[string]interface{}(nil),
		"payload":       []byte(nil),
		"meta": map[string]string{
			"owner": "armon",
		},
	}
	assert.Equal(t, expected, jSent)
}
