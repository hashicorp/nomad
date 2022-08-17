package api

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/hashicorp/nomad/helper/pointer"
)

func TestCompose(t *testing.T) {
	testutil.Parallel(t)
	// Compose a task
	task := NewTask("task1", "exec").
		SetConfig("foo", "bar").
		SetMeta("foo", "bar").
		Constrain(NewConstraint("kernel.name", "=", "linux")).
		Require(&Resources{
			CPU:      pointer.Of(1250),
			MemoryMB: pointer.Of(1024),
			DiskMB:   pointer.Of(2048),
			Networks: []*NetworkResource{
				{
					CIDR:          "0.0.0.0/0",
					MBits:         pointer.Of(100),
					ReservedPorts: []Port{{"", 80, 0, ""}, {"", 443, 0, ""}},
				},
			},
		})

	// Compose a task group

	st1 := NewSpreadTarget("dc1", 80)
	st2 := NewSpreadTarget("dc2", 20)
	grp := NewTaskGroup("grp1", 2).
		Constrain(NewConstraint("kernel.name", "=", "linux")).
		AddAffinity(NewAffinity("${node.class}", "=", "large", 50)).
		AddSpread(NewSpread("${node.datacenter}", 30, []*SpreadTarget{st1, st2})).
		SetMeta("foo", "bar").
		AddTask(task)

	// Compose a job
	job := NewServiceJob("job1", "myjob", "global", 2).
		SetMeta("foo", "bar").
		AddDatacenter("dc1").
		Constrain(NewConstraint("kernel.name", "=", "linux")).
		AddTaskGroup(grp)

	// Check that the composed result looks correct
	expect := &Job{
		Region:   pointer.Of("global"),
		ID:       pointer.Of("job1"),
		Name:     pointer.Of("myjob"),
		Type:     pointer.Of(JobTypeService),
		Priority: pointer.Of(2),
		Datacenters: []string{
			"dc1",
		},
		Meta: map[string]string{
			"foo": "bar",
		},
		Constraints: []*Constraint{
			{
				LTarget: "kernel.name",
				RTarget: "linux",
				Operand: "=",
			},
		},
		TaskGroups: []*TaskGroup{
			{
				Name:  pointer.Of("grp1"),
				Count: pointer.Of(2),
				Constraints: []*Constraint{
					{
						LTarget: "kernel.name",
						RTarget: "linux",
						Operand: "=",
					},
				},
				Affinities: []*Affinity{
					{
						LTarget: "${node.class}",
						RTarget: "large",
						Operand: "=",
						Weight:  pointer.Of(int8(50)),
					},
				},
				Spreads: []*Spread{
					{
						Attribute: "${node.datacenter}",
						Weight:    pointer.Of(int8(30)),
						SpreadTarget: []*SpreadTarget{
							{
								Value:   "dc1",
								Percent: 80,
							},
							{
								Value:   "dc2",
								Percent: 20,
							},
						},
					},
				},
				Tasks: []*Task{
					{
						Name:   "task1",
						Driver: "exec",
						Resources: &Resources{
							CPU:      pointer.Of(1250),
							MemoryMB: pointer.Of(1024),
							DiskMB:   pointer.Of(2048),
							Networks: []*NetworkResource{
								{
									CIDR:  "0.0.0.0/0",
									MBits: pointer.Of(100),
									ReservedPorts: []Port{
										{"", 80, 0, ""},
										{"", 443, 0, ""},
									},
								},
							},
						},
						Constraints: []*Constraint{
							{
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
