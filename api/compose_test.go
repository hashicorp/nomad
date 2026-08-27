// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestCompose(t *testing.T) {
	testutil.Parallel(t)
	// Compose a task
	task := NewTask("task1", "exec").
		SetConfig("foo", "bar").
		SetMeta("foo", "bar").
		Constrain(NewConstraint("kernel.name", "=", "linux")).
		Require(&Resources{
			CPU:      new(1250),
			MemoryMB: new(1024),
			DiskMB:   new(2048),
			Networks: []*NetworkResource{
				{
					CIDR:          "0.0.0.0/0",
					MBits:         new(100),
					ReservedPorts: []Port{{Label: "", Value: 80}, {Label: "", Value: 443}},
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
		Region:   new("global"),
		ID:       new("job1"),
		Name:     new("myjob"),
		Type:     new(JobTypeService),
		Priority: new(2),
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
				Name:  new("grp1"),
				Count: new(2),
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
						Weight:  new(int8(50)),
					},
				},
				Spreads: []*Spread{
					{
						Attribute: "${node.datacenter}",
						Weight:    new(int8(30)),
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
							CPU:      new(1250),
							MemoryMB: new(1024),
							DiskMB:   new(2048),
							Networks: []*NetworkResource{
								{
									CIDR:  "0.0.0.0/0",
									MBits: new(100),
									ReservedPorts: []Port{
										{Label: "", Value: 80},
										{Label: "", Value: 443},
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
						Config: map[string]any{
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
	must.Eq(t, expect, job)
}
