package api

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/assert"
)

func TestTaskGroup_NewTaskGroup(t *testing.T) {
	t.Parallel()
	grp := NewTaskGroup("grp1", 2)
	expect := &TaskGroup{
		Name:  helper.StringToPtr("grp1"),
		Count: helper.IntToPtr(2),
	}
	if !reflect.DeepEqual(grp, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, grp)
	}
}

func TestTaskGroup_Constrain(t *testing.T) {
	t.Parallel()
	grp := NewTaskGroup("grp1", 1)

	// Add a constraint to the group
	out := grp.Constrain(NewConstraint("kernel.name", "=", "darwin"))
	if n := len(grp.Constraints); n != 1 {
		t.Fatalf("expected 1 constraint, got: %d", n)
	}

	// Check that the group was returned
	if out != grp {
		t.Fatalf("expected: %#v, got: %#v", grp, out)
	}

	// Add a second constraint
	grp.Constrain(NewConstraint("memory.totalbytes", ">=", "128000000"))
	expect := []*Constraint{
		&Constraint{
			LTarget: "kernel.name",
			RTarget: "darwin",
			Operand: "=",
		},
		&Constraint{
			LTarget: "memory.totalbytes",
			RTarget: "128000000",
			Operand: ">=",
		},
	}
	if !reflect.DeepEqual(grp.Constraints, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, grp.Constraints)
	}
}

func TestTaskGroup_SetMeta(t *testing.T) {
	t.Parallel()
	grp := NewTaskGroup("grp1", 1)

	// Initializes an empty map
	out := grp.SetMeta("foo", "bar")
	if grp.Meta == nil {
		t.Fatalf("should be initialized")
	}

	// Check that we returned the group
	if out != grp {
		t.Fatalf("expect: %#v, got: %#v", grp, out)
	}

	// Add a second meta k/v
	grp.SetMeta("baz", "zip")
	expect := map[string]string{"foo": "bar", "baz": "zip"}
	if !reflect.DeepEqual(grp.Meta, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, grp.Meta)
	}
}

func TestTaskGroup_AddTask(t *testing.T) {
	t.Parallel()
	grp := NewTaskGroup("grp1", 1)

	// Add the task to the task group
	out := grp.AddTask(NewTask("task1", "java"))
	if n := len(grp.Tasks); n != 1 {
		t.Fatalf("expected 1 task, got: %d", n)
	}

	// Check that we returned the group
	if out != grp {
		t.Fatalf("expect: %#v, got: %#v", grp, out)
	}

	// Add a second task
	grp.AddTask(NewTask("task2", "exec"))
	expect := []*Task{
		&Task{
			Name:   "task1",
			Driver: "java",
		},
		&Task{
			Name:   "task2",
			Driver: "exec",
		},
	}
	if !reflect.DeepEqual(grp.Tasks, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, grp.Tasks)
	}
}

func TestTask_NewTask(t *testing.T) {
	t.Parallel()
	task := NewTask("task1", "exec")
	expect := &Task{
		Name:   "task1",
		Driver: "exec",
	}
	if !reflect.DeepEqual(task, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, task)
	}
}

func TestTask_SetConfig(t *testing.T) {
	t.Parallel()
	task := NewTask("task1", "exec")

	// Initializes an empty map
	out := task.SetConfig("foo", "bar")
	if task.Config == nil {
		t.Fatalf("should be initialized")
	}

	// Check that we returned the task
	if out != task {
		t.Fatalf("expect: %#v, got: %#v", task, out)
	}

	// Set another config value
	task.SetConfig("baz", "zip")
	expect := map[string]interface{}{"foo": "bar", "baz": "zip"}
	if !reflect.DeepEqual(task.Config, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, task.Config)
	}
}

func TestTask_SetMeta(t *testing.T) {
	t.Parallel()
	task := NewTask("task1", "exec")

	// Initializes an empty map
	out := task.SetMeta("foo", "bar")
	if task.Meta == nil {
		t.Fatalf("should be initialized")
	}

	// Check that we returned the task
	if out != task {
		t.Fatalf("expect: %#v, got: %#v", task, out)
	}

	// Set another meta k/v
	task.SetMeta("baz", "zip")
	expect := map[string]string{"foo": "bar", "baz": "zip"}
	if !reflect.DeepEqual(task.Meta, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, task.Meta)
	}
}

func TestTask_Require(t *testing.T) {
	t.Parallel()
	task := NewTask("task1", "exec")

	// Create some require resources
	resources := &Resources{
		CPU:      helper.IntToPtr(1250),
		MemoryMB: helper.IntToPtr(128),
		DiskMB:   helper.IntToPtr(2048),
		IOPS:     helper.IntToPtr(500),
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:          "0.0.0.0/0",
				MBits:         helper.IntToPtr(100),
				ReservedPorts: []Port{{"", 80}, {"", 443}},
			},
		},
	}
	out := task.Require(resources)
	if !reflect.DeepEqual(task.Resources, resources) {
		t.Fatalf("expect: %#v, got: %#v", resources, task.Resources)
	}

	// Check that we returned the task
	if out != task {
		t.Fatalf("expect: %#v, got: %#v", task, out)
	}
}

func TestTask_Constrain(t *testing.T) {
	t.Parallel()
	task := NewTask("task1", "exec")

	// Add a constraint to the task
	out := task.Constrain(NewConstraint("kernel.name", "=", "darwin"))
	if n := len(task.Constraints); n != 1 {
		t.Fatalf("expected 1 constraint, got: %d", n)
	}

	// Check that the task was returned
	if out != task {
		t.Fatalf("expected: %#v, got: %#v", task, out)
	}

	// Add a second constraint
	task.Constrain(NewConstraint("memory.totalbytes", ">=", "128000000"))
	expect := []*Constraint{
		&Constraint{
			LTarget: "kernel.name",
			RTarget: "darwin",
			Operand: "=",
		},
		&Constraint{
			LTarget: "memory.totalbytes",
			RTarget: "128000000",
			Operand: ">=",
		},
	}
	if !reflect.DeepEqual(task.Constraints, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, task.Constraints)
	}
}

func TestTask_Artifact(t *testing.T) {
	t.Parallel()
	a := TaskArtifact{
		GetterSource: helper.StringToPtr("http://localhost/foo.txt"),
		GetterMode:   helper.StringToPtr("file"),
	}
	a.Canonicalize()
	if *a.GetterMode != "file" {
		t.Errorf("expected file but found %q", *a.GetterMode)
	}
	if *a.RelativeDest != "local/foo.txt" {
		t.Errorf("expected local/foo.txt but found %q", *a.RelativeDest)
	}
}

// Ensures no regression on https://github.com/hashicorp/nomad/issues/3132
func TestTaskGroup_Canonicalize_Update(t *testing.T) {
	job := &Job{
		ID: helper.StringToPtr("test"),
		Update: &UpdateStrategy{
			AutoRevert:      helper.BoolToPtr(false),
			Canary:          helper.IntToPtr(0),
			HealthCheck:     helper.StringToPtr(""),
			HealthyDeadline: helper.TimeToPtr(0),
			MaxParallel:     helper.IntToPtr(0),
			MinHealthyTime:  helper.TimeToPtr(0),
			Stagger:         helper.TimeToPtr(0),
		},
	}
	job.Canonicalize()
	tg := &TaskGroup{
		Name: helper.StringToPtr("foo"),
	}
	tg.Canonicalize(job)
	assert.Nil(t, tg.Update)
}
