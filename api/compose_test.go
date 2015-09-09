package api

import (
	"reflect"
	"testing"
)

func TestCompose(t *testing.T) {
	// Compose a task
	task := NewTask("mytask", "docker").
		SetConfig("foo", "bar").
		SetConfig("baz", "zip")

	// Require some amount of resources
	task.Require(&Resources{
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
	grp := NewTaskGroup("mygroup", 2).
		Constrain(HardConstraint("kernel.name", "=", "linux")).
		Constrain(SoftConstraint("memory.totalbytes", ">=", "128000000", 1)).
		SetMeta("foo", "bar").
		SetMeta("baz", "zip").
		AddTask(task)

	// Check that the composed result looks correct
	expect := &TaskGroup{
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
			&Constraint{
				Hard:    false,
				LTarget: "memory.totalbytes",
				RTarget: "128000000",
				Operand: ">=",
				Weight:  1,
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
	}
	if !reflect.DeepEqual(grp, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, grp)
	}
}
