package api

import (
	"reflect"
	"testing"
)

func TestCompose(t *testing.T) {
	t.Parallel()
	// Compose a task
	task := NewTask("task1", "exec").
		SetConfig("foo", "bar").
		SetMeta("foo", "bar").
		Constrain(NewConstraint("kernel.name", "=", "linux")).
		Require(&Resources{
			CPU:      intToPtr(1250),
			MemoryMB: intToPtr(1024),
			DiskMB:   intToPtr(2048),
			Networks: []*NetworkResource{
				{
					CIDR:          "0.0.0.0/0",
					MBits:         intToPtr(100),
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
		Region:   stringToPtr("global"),
		ID:       stringToPtr("job1"),
		Name:     stringToPtr("myjob"),
		Type:     stringToPtr(JobTypeService),
		Priority: intToPtr(2),
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
				Name:  stringToPtr("grp1"),
				Count: intToPtr(2),
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
						Weight:  int8ToPtr(50),
					},
				},
				Spreads: []*Spread{
					{
						Attribute: "${node.datacenter}",
						Weight:    int8ToPtr(30),
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
							CPU:      intToPtr(1250),
							MemoryMB: intToPtr(1024),
							DiskMB:   intToPtr(2048),
							Networks: []*NetworkResource{
								{
									CIDR:  "0.0.0.0/0",
									MBits: intToPtr(100),
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
