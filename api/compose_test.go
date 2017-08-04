package api

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/helper"
)

func TestCompose(t *testing.T) {
	t.Parallel()
	// Compose a task
	task := NewTask("task1", "exec").
		SetConfig("foo", "bar").
		SetMeta("foo", "bar").
		Constrain(NewConstraint("kernel.name", "=", "linux")).
		Require(&Resources{
			CPU:      helper.IntToPtr(1250),
			MemoryMB: helper.IntToPtr(1024),
			DiskMB:   helper.IntToPtr(2048),
			IOPS:     helper.IntToPtr(500),
			Networks: []*NetworkResource{
				&NetworkResource{
					CIDR:          "0.0.0.0/0",
					MBits:         helper.IntToPtr(100),
					ReservedPorts: []Port{{"", 80}, {"", 443}},
				},
			},
		})

	// Compose a task group
	grp := NewTaskGroup("grp1", 2).
		Constrain(NewConstraint("kernel.name", "=", "linux")).
		SetMeta("foo", "bar").
		AddTask(task)

	// Compose a job
	job := NewServiceJob("job1", "myjob", "region1", 2).
		SetMeta("foo", "bar").
		AddDatacenter("dc1").
		Constrain(NewConstraint("kernel.name", "=", "linux")).
		AddTaskGroup(grp)

	// Check that the composed result looks correct
	expect := &Job{
		Region:   helper.StringToPtr("region1"),
		ID:       helper.StringToPtr("job1"),
		Name:     helper.StringToPtr("myjob"),
		Type:     helper.StringToPtr(JobTypeService),
		Priority: helper.IntToPtr(2),
		Datacenters: []string{
			"dc1",
		},
		Meta: map[string]string{
			"foo": "bar",
		},
		Constraints: []*Constraint{
			&Constraint{
				LTarget: "kernel.name",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name:  helper.StringToPtr("grp1"),
				Count: helper.IntToPtr(2),
				Constraints: []*Constraint{
					&Constraint{
						LTarget: "kernel.name",
						RTarget: "linux",
						Operand: "=",
					},
				},
				Tasks: []*Task{
					&Task{
						Name:   "task1",
						Driver: "exec",
						Resources: &Resources{
							CPU:      helper.IntToPtr(1250),
							MemoryMB: helper.IntToPtr(1024),
							DiskMB:   helper.IntToPtr(2048),
							IOPS:     helper.IntToPtr(500),
							Networks: []*NetworkResource{
								&NetworkResource{
									CIDR:  "0.0.0.0/0",
									MBits: helper.IntToPtr(100),
									ReservedPorts: []Port{
										{"", 80},
										{"", 443},
									},
								},
							},
						},
						Constraints: []*Constraint{
							&Constraint{
								LTarget: "kernel.name",
								RTarget: "linux",
								Operand: "=",
							},
						},
						Config: map[string]interface{}{
							"foo": "bar",
						},
						Meta: map[string]string{
							"foo": "bar",
						},
					},
				},
				Meta: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
	if !reflect.DeepEqual(job, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job)
	}
}
