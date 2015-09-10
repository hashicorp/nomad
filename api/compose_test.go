package api

import (
	"reflect"
	"testing"
)

func TestCompose(t *testing.T) {
	// Compose a task
	task := NewTask("mytask", "docker").
		SetConfig("foo", "bar").
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
	grp := NewTaskGroup("mygroup").
		SetCount(2).
		Constrain(HardConstraint("kernel.name", "=", "linux")).
		SetMeta("foo", "bar").
		AddTask(task)

	// Compose a job
	job := NewServiceJob("job1", "myjob", 2).
		SetMeta("foo", "bar").
		AddDatacenter("dc1").
		Constrain(HardConstraint("kernel.name", "=", "linux")).
		AddTaskGroup(grp)

	// Check that the composed result looks correct
	expect := &Job{
		ID:          "job1",
		Name:        "myjob",
		Type:        JobTypeService,
		Priority:    2,
		Datacenters: []string{"dc1"},
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
				Name:  "mygroup",
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
						Name:   "mytask",
						Driver: "docker",
						Resources: &Resources{
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
							"baz": "zip",
						},
					},
				},
				Meta: map[string]string{
					"foo": "bar",
					"baz": "zip",
				},
			},
		},
	}
	if !reflect.DeepEqual(job, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job)
	}
}
