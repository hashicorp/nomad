package api

import (
	"reflect"
	"testing"
)

func TestCompose(t *testing.T) {
	// Compose a task
	task := NewTask("task1", "exec").
		SetConfig("foo", "bar").
		SetMeta("foo", "bar").
		Constrain(HardConstraint("kernel.name", "=", "linux")).
		Require(&Resources{
		CPU:      1.25,
		MemoryMB: 1024,
		DiskMB:   2048,
		IOPS:     1024,
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:          "0.0.0.0/0",
				MBits:         100,
				ReservedPorts: []int{80, 443},
			},
		},
	})

	// Compose a task group
	grp := NewTaskGroup("grp1", 2).
		Constrain(HardConstraint("kernel.name", "=", "linux")).
		SetMeta("foo", "bar").
		AddTask(task)

	// Compose a job
	job := NewServiceJob("job1", "myjob", "region1", 2).
		SetMeta("foo", "bar").
		AddDatacenter("dc1").
		Constrain(HardConstraint("kernel.name", "=", "linux")).
		AddTaskGroup(grp)

	// Check that the composed result looks correct
	expect := &Job{
		Region:   "region1",
		ID:       "job1",
		Name:     "myjob",
		Type:     JobTypeService,
		Priority: 2,
		Datacenters: []string{
			"dc1",
		},
		Meta: map[string]string{
			"foo": "bar",
		},
		Constraints: []*Constraint{
			&Constraint{
				Hard:    true,
				LTarget: "kernel.name",
				RTarget: "linux",
				Operand: "=",
				Weight:  0,
			},
		},
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name:  "grp1",
				Count: 2,
				Constraints: []*Constraint{
					&Constraint{
						Hard:    true,
						LTarget: "kernel.name",
						RTarget: "linux",
						Operand: "=",
						Weight:  0,
					},
				},
				Tasks: []*Task{
					&Task{
						Name:   "task1",
						Driver: "exec",
						Resources: &Resources{
							CPU:      1.25,
							MemoryMB: 1024,
							DiskMB:   2048,
							IOPS:     1024,
							Networks: []*NetworkResource{
								&NetworkResource{
									CIDR:  "0.0.0.0/0",
									MBits: 100,
									ReservedPorts: []int{
										80,
										443,
									},
								},
							},
						},
						Constraints: []*Constraint{
							&Constraint{
								Hard:    true,
								LTarget: "kernel.name",
								RTarget: "linux",
								Operand: "=",
								Weight:  0,
							},
						},
						Config: map[string]string{
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
