package api

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
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
		{
			LTarget: "kernel.name",
			RTarget: "darwin",
			Operand: "=",
		},
		{
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
		{
			Name:   "task1",
			Driver: "java",
		},
		{
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
			{
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
		{
			LTarget: "kernel.name",
			RTarget: "darwin",
			Operand: "=",
		},
		{
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
			AutoRevert:       helper.BoolToPtr(false),
			Canary:           helper.IntToPtr(0),
			HealthCheck:      helper.StringToPtr(""),
			HealthyDeadline:  helper.TimeToPtr(0),
			ProgressDeadline: helper.TimeToPtr(0),
			MaxParallel:      helper.IntToPtr(0),
			MinHealthyTime:   helper.TimeToPtr(0),
			Stagger:          helper.TimeToPtr(0),
		},
	}
	job.Canonicalize()
	tg := &TaskGroup{
		Name: helper.StringToPtr("foo"),
	}
	tg.Canonicalize(job)
	assert.Nil(t, tg.Update)
}

// Verifies that reschedule policy is merged correctly
func TestTaskGroup_Canonicalize_ReschedulePolicy(t *testing.T) {
	type testCase struct {
		desc                 string
		jobReschedulePolicy  *ReschedulePolicy
		taskReschedulePolicy *ReschedulePolicy
		expected             *ReschedulePolicy
	}

	testCases := []testCase{
		{
			desc:                 "Default",
			jobReschedulePolicy:  nil,
			taskReschedulePolicy: nil,
			expected: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(structs.DefaultBatchJobReschedulePolicy.Attempts),
				Interval:      helper.TimeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         helper.TimeToPtr(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: helper.StringToPtr(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      helper.TimeToPtr(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     helper.BoolToPtr(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
		{
			desc: "Empty job reschedule policy",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(0),
				Interval:      helper.TimeToPtr(0),
				Delay:         helper.TimeToPtr(0),
				MaxDelay:      helper.TimeToPtr(0),
				DelayFunction: helper.StringToPtr(""),
				Unlimited:     helper.BoolToPtr(false),
			},
			taskReschedulePolicy: nil,
			expected: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(0),
				Interval:      helper.TimeToPtr(0),
				Delay:         helper.TimeToPtr(0),
				MaxDelay:      helper.TimeToPtr(0),
				DelayFunction: helper.StringToPtr(""),
				Unlimited:     helper.BoolToPtr(false),
			},
		},
		{
			desc: "Inherit from job",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(1),
				Interval:      helper.TimeToPtr(20 * time.Second),
				Delay:         helper.TimeToPtr(20 * time.Second),
				MaxDelay:      helper.TimeToPtr(10 * time.Minute),
				DelayFunction: helper.StringToPtr("constant"),
				Unlimited:     helper.BoolToPtr(false),
			},
			taskReschedulePolicy: nil,
			expected: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(1),
				Interval:      helper.TimeToPtr(20 * time.Second),
				Delay:         helper.TimeToPtr(20 * time.Second),
				MaxDelay:      helper.TimeToPtr(10 * time.Minute),
				DelayFunction: helper.StringToPtr("constant"),
				Unlimited:     helper.BoolToPtr(false),
			},
		},
		{
			desc:                "Set in task",
			jobReschedulePolicy: nil,
			taskReschedulePolicy: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(5),
				Interval:      helper.TimeToPtr(2 * time.Minute),
				Delay:         helper.TimeToPtr(20 * time.Second),
				MaxDelay:      helper.TimeToPtr(10 * time.Minute),
				DelayFunction: helper.StringToPtr("constant"),
				Unlimited:     helper.BoolToPtr(false),
			},
			expected: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(5),
				Interval:      helper.TimeToPtr(2 * time.Minute),
				Delay:         helper.TimeToPtr(20 * time.Second),
				MaxDelay:      helper.TimeToPtr(10 * time.Minute),
				DelayFunction: helper.StringToPtr("constant"),
				Unlimited:     helper.BoolToPtr(false),
			},
		},
		{
			desc: "Merge from job",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts: helper.IntToPtr(1),
				Delay:    helper.TimeToPtr(20 * time.Second),
				MaxDelay: helper.TimeToPtr(10 * time.Minute),
			},
			taskReschedulePolicy: &ReschedulePolicy{
				Interval:      helper.TimeToPtr(5 * time.Minute),
				DelayFunction: helper.StringToPtr("constant"),
				Unlimited:     helper.BoolToPtr(false),
			},
			expected: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(1),
				Interval:      helper.TimeToPtr(5 * time.Minute),
				Delay:         helper.TimeToPtr(20 * time.Second),
				MaxDelay:      helper.TimeToPtr(10 * time.Minute),
				DelayFunction: helper.StringToPtr("constant"),
				Unlimited:     helper.BoolToPtr(false),
			},
		},
		{
			desc: "Override from group",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts: helper.IntToPtr(1),
				MaxDelay: helper.TimeToPtr(10 * time.Second),
			},
			taskReschedulePolicy: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(5),
				Delay:         helper.TimeToPtr(20 * time.Second),
				MaxDelay:      helper.TimeToPtr(20 * time.Minute),
				DelayFunction: helper.StringToPtr("constant"),
				Unlimited:     helper.BoolToPtr(false),
			},
			expected: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(5),
				Interval:      helper.TimeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         helper.TimeToPtr(20 * time.Second),
				MaxDelay:      helper.TimeToPtr(20 * time.Minute),
				DelayFunction: helper.StringToPtr("constant"),
				Unlimited:     helper.BoolToPtr(false),
			},
		},
		{
			desc: "Attempts from job, default interval",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts: helper.IntToPtr(1),
			},
			taskReschedulePolicy: nil,
			expected: &ReschedulePolicy{
				Attempts:      helper.IntToPtr(1),
				Interval:      helper.TimeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         helper.TimeToPtr(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: helper.StringToPtr(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      helper.TimeToPtr(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     helper.BoolToPtr(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			job := &Job{
				ID:         helper.StringToPtr("test"),
				Reschedule: tc.jobReschedulePolicy,
				Type:       helper.StringToPtr(JobTypeBatch),
			}
			job.Canonicalize()
			tg := &TaskGroup{
				Name:             helper.StringToPtr("foo"),
				ReschedulePolicy: tc.taskReschedulePolicy,
			}
			tg.Canonicalize(job)
			assert.Equal(t, tc.expected, tg.ReschedulePolicy)
		})
	}
}

// Verifies that migrate strategy is merged correctly
func TestTaskGroup_Canonicalize_MigrateStrategy(t *testing.T) {
	type testCase struct {
		desc        string
		jobType     string
		jobMigrate  *MigrateStrategy
		taskMigrate *MigrateStrategy
		expected    *MigrateStrategy
	}

	testCases := []testCase{
		{
			desc:        "Default batch",
			jobType:     "batch",
			jobMigrate:  nil,
			taskMigrate: nil,
			expected:    nil,
		},
		{
			desc:        "Default service",
			jobType:     "service",
			jobMigrate:  nil,
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(1),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(10 * time.Second),
				HealthyDeadline: helper.TimeToPtr(5 * time.Minute),
			},
		},
		{
			desc:    "Empty job migrate strategy",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(0),
				HealthCheck:     helper.StringToPtr(""),
				MinHealthyTime:  helper.TimeToPtr(0),
				HealthyDeadline: helper.TimeToPtr(0),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(0),
				HealthCheck:     helper.StringToPtr(""),
				MinHealthyTime:  helper.TimeToPtr(0),
				HealthyDeadline: helper.TimeToPtr(0),
			},
		},
		{
			desc:    "Inherit from job",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(3),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(2),
				HealthyDeadline: helper.TimeToPtr(2),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(3),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(2),
				HealthyDeadline: helper.TimeToPtr(2),
			},
		},
		{
			desc:       "Set in task",
			jobType:    "service",
			jobMigrate: nil,
			taskMigrate: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(3),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(2),
				HealthyDeadline: helper.TimeToPtr(2),
			},
			expected: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(3),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(2),
				HealthyDeadline: helper.TimeToPtr(2),
			},
		},
		{
			desc:    "Merge from job",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: helper.IntToPtr(11),
			},
			taskMigrate: &MigrateStrategy{
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(2),
				HealthyDeadline: helper.TimeToPtr(2),
			},
			expected: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(11),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(2),
				HealthyDeadline: helper.TimeToPtr(2),
			},
		},
		{
			desc:    "Override from group",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: helper.IntToPtr(11),
			},
			taskMigrate: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(5),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(2),
				HealthyDeadline: helper.TimeToPtr(2),
			},
			expected: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(5),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(2),
				HealthyDeadline: helper.TimeToPtr(2),
			},
		},
		{
			desc:    "Parallel from job, defaulting",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: helper.IntToPtr(5),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     helper.IntToPtr(5),
				HealthCheck:     helper.StringToPtr("checks"),
				MinHealthyTime:  helper.TimeToPtr(10 * time.Second),
				HealthyDeadline: helper.TimeToPtr(5 * time.Minute),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			job := &Job{
				ID:      helper.StringToPtr("test"),
				Migrate: tc.jobMigrate,
				Type:    helper.StringToPtr(tc.jobType),
			}
			job.Canonicalize()
			tg := &TaskGroup{
				Name:    helper.StringToPtr("foo"),
				Migrate: tc.taskMigrate,
			}
			tg.Canonicalize(job)
			assert.Equal(t, tc.expected, tg.Migrate)
		})
	}
}

// TestService_CheckRestart asserts Service.CheckRestart settings are properly
// inherited by Checks.
func TestService_CheckRestart(t *testing.T) {
	job := &Job{Name: helper.StringToPtr("job")}
	tg := &TaskGroup{Name: helper.StringToPtr("group")}
	task := &Task{Name: "task"}
	service := &Service{
		CheckRestart: &CheckRestart{
			Limit:          11,
			Grace:          helper.TimeToPtr(11 * time.Second),
			IgnoreWarnings: true,
		},
		Checks: []ServiceCheck{
			{
				Name: "all-set",
				CheckRestart: &CheckRestart{
					Limit:          22,
					Grace:          helper.TimeToPtr(22 * time.Second),
					IgnoreWarnings: true,
				},
			},
			{
				Name: "some-set",
				CheckRestart: &CheckRestart{
					Limit: 33,
					Grace: helper.TimeToPtr(33 * time.Second),
				},
			},
			{
				Name: "unset",
			},
		},
	}

	service.Canonicalize(task, tg, job)
	assert.Equal(t, service.Checks[0].CheckRestart.Limit, 22)
	assert.Equal(t, *service.Checks[0].CheckRestart.Grace, 22*time.Second)
	assert.True(t, service.Checks[0].CheckRestart.IgnoreWarnings)

	assert.Equal(t, service.Checks[1].CheckRestart.Limit, 33)
	assert.Equal(t, *service.Checks[1].CheckRestart.Grace, 33*time.Second)
	assert.True(t, service.Checks[1].CheckRestart.IgnoreWarnings)

	assert.Equal(t, service.Checks[2].CheckRestart.Limit, 11)
	assert.Equal(t, *service.Checks[2].CheckRestart.Grace, 11*time.Second)
	assert.True(t, service.Checks[2].CheckRestart.IgnoreWarnings)
}
