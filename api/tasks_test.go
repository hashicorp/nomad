package api

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskGroup_NewTaskGroup(t *testing.T) {
	t.Parallel()
	grp := NewTaskGroup("grp1", 2)
	expect := &TaskGroup{
		Name:  stringToPtr("grp1"),
		Count: intToPtr(2),
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

func TestTaskGroup_AddAffinity(t *testing.T) {
	t.Parallel()
	grp := NewTaskGroup("grp1", 1)

	// Add an affinity to the group
	out := grp.AddAffinity(NewAffinity("kernel.version", "=", "4.6", 100))
	if n := len(grp.Affinities); n != 1 {
		t.Fatalf("expected 1 affinity, got: %d", n)
	}

	// Check that the group was returned
	if out != grp {
		t.Fatalf("expected: %#v, got: %#v", grp, out)
	}

	// Add a second affinity
	grp.AddAffinity(NewAffinity("${node.affinity}", "=", "dc2", 50))
	expect := []*Affinity{
		{
			LTarget: "kernel.version",
			RTarget: "4.6",
			Operand: "=",
			Weight:  int8ToPtr(100),
		},
		{
			LTarget: "${node.affinity}",
			RTarget: "dc2",
			Operand: "=",
			Weight:  int8ToPtr(50),
		},
	}
	if !reflect.DeepEqual(grp.Affinities, expect) {
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

func TestTaskGroup_AddSpread(t *testing.T) {
	t.Parallel()
	grp := NewTaskGroup("grp1", 1)

	// Create and add spread
	spreadTarget := NewSpreadTarget("r1", 50)
	spread := NewSpread("${meta.rack}", 100, []*SpreadTarget{spreadTarget})

	out := grp.AddSpread(spread)
	if n := len(grp.Spreads); n != 1 {
		t.Fatalf("expected 1 spread, got: %d", n)
	}

	// Check that the group was returned
	if out != grp {
		t.Fatalf("expected: %#v, got: %#v", grp, out)
	}

	// Add a second spread
	spreadTarget2 := NewSpreadTarget("dc1", 100)
	spread2 := NewSpread("${node.datacenter}", 100, []*SpreadTarget{spreadTarget2})

	grp.AddSpread(spread2)

	expect := []*Spread{
		{
			Attribute: "${meta.rack}",
			Weight:    int8ToPtr(100),
			SpreadTarget: []*SpreadTarget{
				{
					Value:   "r1",
					Percent: 50,
				},
			},
		},
		{
			Attribute: "${node.datacenter}",
			Weight:    int8ToPtr(100),
			SpreadTarget: []*SpreadTarget{
				{
					Value:   "dc1",
					Percent: 100,
				},
			},
		},
	}
	if !reflect.DeepEqual(grp.Spreads, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, grp.Spreads)
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
		CPU:      intToPtr(1250),
		MemoryMB: intToPtr(128),
		DiskMB:   intToPtr(2048),
		Networks: []*NetworkResource{
			{
				CIDR:          "0.0.0.0/0",
				MBits:         intToPtr(100),
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

func TestTask_AddAffinity(t *testing.T) {
	t.Parallel()
	task := NewTask("task1", "exec")

	// Add an affinity to the task
	out := task.AddAffinity(NewAffinity("kernel.version", "=", "4.6", 100))
	require := require.New(t)
	require.Len(out.Affinities, 1)

	// Check that the task was returned
	if out != task {
		t.Fatalf("expected: %#v, got: %#v", task, out)
	}

	// Add a second affinity
	task.AddAffinity(NewAffinity("${node.datacenter}", "=", "dc2", 50))
	expect := []*Affinity{
		{
			LTarget: "kernel.version",
			RTarget: "4.6",
			Operand: "=",
			Weight:  int8ToPtr(100),
		},
		{
			LTarget: "${node.datacenter}",
			RTarget: "dc2",
			Operand: "=",
			Weight:  int8ToPtr(50),
		},
	}
	if !reflect.DeepEqual(task.Affinities, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, task.Affinities)
	}
}

func TestTask_Artifact(t *testing.T) {
	t.Parallel()
	a := TaskArtifact{
		GetterSource: stringToPtr("http://localhost/foo.txt"),
		GetterMode:   stringToPtr("file"),
	}
	a.Canonicalize()
	if *a.GetterMode != "file" {
		t.Errorf("expected file but found %q", *a.GetterMode)
	}
	if filepath.ToSlash(*a.RelativeDest) != "local/foo.txt" {
		t.Errorf("expected local/foo.txt but found %q", *a.RelativeDest)
	}
}

// Ensures no regression on https://github.com/hashicorp/nomad/issues/3132
func TestTaskGroup_Canonicalize_Update(t *testing.T) {
	job := &Job{
		ID: stringToPtr("test"),
		Update: &UpdateStrategy{
			AutoRevert:       boolToPtr(false),
			Canary:           intToPtr(0),
			HealthCheck:      stringToPtr(""),
			HealthyDeadline:  timeToPtr(0),
			ProgressDeadline: timeToPtr(0),
			MaxParallel:      intToPtr(0),
			MinHealthyTime:   timeToPtr(0),
			Stagger:          timeToPtr(0),
		},
	}
	job.Canonicalize()
	tg := &TaskGroup{
		Name: stringToPtr("foo"),
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
				Attempts:      intToPtr(structs.DefaultBatchJobReschedulePolicy.Attempts),
				Interval:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         timeToPtr(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: stringToPtr(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     boolToPtr(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
		{
			desc: "Empty job reschedule policy",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts:      intToPtr(0),
				Interval:      timeToPtr(0),
				Delay:         timeToPtr(0),
				MaxDelay:      timeToPtr(0),
				DelayFunction: stringToPtr(""),
				Unlimited:     boolToPtr(false),
			},
			taskReschedulePolicy: nil,
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(0),
				Interval:      timeToPtr(0),
				Delay:         timeToPtr(0),
				MaxDelay:      timeToPtr(0),
				DelayFunction: stringToPtr(""),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc: "Inherit from job",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(20 * time.Second),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
			taskReschedulePolicy: nil,
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(20 * time.Second),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc:                "Set in task",
			jobReschedulePolicy: nil,
			taskReschedulePolicy: &ReschedulePolicy{
				Attempts:      intToPtr(5),
				Interval:      timeToPtr(2 * time.Minute),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(5),
				Interval:      timeToPtr(2 * time.Minute),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc: "Merge from job",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts: intToPtr(1),
				Delay:    timeToPtr(20 * time.Second),
				MaxDelay: timeToPtr(10 * time.Minute),
			},
			taskReschedulePolicy: &ReschedulePolicy{
				Interval:      timeToPtr(5 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(5 * time.Minute),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(10 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc: "Override from group",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts: intToPtr(1),
				MaxDelay: timeToPtr(10 * time.Second),
			},
			taskReschedulePolicy: &ReschedulePolicy{
				Attempts:      intToPtr(5),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(20 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(5),
				Interval:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         timeToPtr(20 * time.Second),
				MaxDelay:      timeToPtr(20 * time.Minute),
				DelayFunction: stringToPtr("constant"),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc: "Attempts from job, default interval",
			jobReschedulePolicy: &ReschedulePolicy{
				Attempts: intToPtr(1),
			},
			taskReschedulePolicy: nil,
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.Interval),
				Delay:         timeToPtr(structs.DefaultBatchJobReschedulePolicy.Delay),
				DelayFunction: stringToPtr(structs.DefaultBatchJobReschedulePolicy.DelayFunction),
				MaxDelay:      timeToPtr(structs.DefaultBatchJobReschedulePolicy.MaxDelay),
				Unlimited:     boolToPtr(structs.DefaultBatchJobReschedulePolicy.Unlimited),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			job := &Job{
				ID:         stringToPtr("test"),
				Reschedule: tc.jobReschedulePolicy,
				Type:       stringToPtr(JobTypeBatch),
			}
			job.Canonicalize()
			tg := &TaskGroup{
				Name:             stringToPtr("foo"),
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
				MaxParallel:     intToPtr(1),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(10 * time.Second),
				HealthyDeadline: timeToPtr(5 * time.Minute),
			},
		},
		{
			desc:    "Empty job migrate strategy",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel:     intToPtr(0),
				HealthCheck:     stringToPtr(""),
				MinHealthyTime:  timeToPtr(0),
				HealthyDeadline: timeToPtr(0),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     intToPtr(0),
				HealthCheck:     stringToPtr(""),
				MinHealthyTime:  timeToPtr(0),
				HealthyDeadline: timeToPtr(0),
			},
		},
		{
			desc:    "Inherit from job",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel:     intToPtr(3),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(2),
				HealthyDeadline: timeToPtr(2),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     intToPtr(3),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(2),
				HealthyDeadline: timeToPtr(2),
			},
		},
		{
			desc:       "Set in task",
			jobType:    "service",
			jobMigrate: nil,
			taskMigrate: &MigrateStrategy{
				MaxParallel:     intToPtr(3),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(2),
				HealthyDeadline: timeToPtr(2),
			},
			expected: &MigrateStrategy{
				MaxParallel:     intToPtr(3),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(2),
				HealthyDeadline: timeToPtr(2),
			},
		},
		{
			desc:    "Merge from job",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: intToPtr(11),
			},
			taskMigrate: &MigrateStrategy{
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(2),
				HealthyDeadline: timeToPtr(2),
			},
			expected: &MigrateStrategy{
				MaxParallel:     intToPtr(11),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(2),
				HealthyDeadline: timeToPtr(2),
			},
		},
		{
			desc:    "Override from group",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: intToPtr(11),
			},
			taskMigrate: &MigrateStrategy{
				MaxParallel:     intToPtr(5),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(2),
				HealthyDeadline: timeToPtr(2),
			},
			expected: &MigrateStrategy{
				MaxParallel:     intToPtr(5),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(2),
				HealthyDeadline: timeToPtr(2),
			},
		},
		{
			desc:    "Parallel from job, defaulting",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: intToPtr(5),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     intToPtr(5),
				HealthCheck:     stringToPtr("checks"),
				MinHealthyTime:  timeToPtr(10 * time.Second),
				HealthyDeadline: timeToPtr(5 * time.Minute),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			job := &Job{
				ID:      stringToPtr("test"),
				Migrate: tc.jobMigrate,
				Type:    stringToPtr(tc.jobType),
			}
			job.Canonicalize()
			tg := &TaskGroup{
				Name:    stringToPtr("foo"),
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
	job := &Job{Name: stringToPtr("job")}
	tg := &TaskGroup{Name: stringToPtr("group")}
	task := &Task{Name: "task"}
	service := &Service{
		CheckRestart: &CheckRestart{
			Limit:          11,
			Grace:          timeToPtr(11 * time.Second),
			IgnoreWarnings: true,
		},
		Checks: []ServiceCheck{
			{
				Name: "all-set",
				CheckRestart: &CheckRestart{
					Limit:          22,
					Grace:          timeToPtr(22 * time.Second),
					IgnoreWarnings: true,
				},
			},
			{
				Name: "some-set",
				CheckRestart: &CheckRestart{
					Limit: 33,
					Grace: timeToPtr(33 * time.Second),
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

// TestSpread_Canonicalize asserts that the spread stanza is canonicalized correctly
func TestSpread_Canonicalize(t *testing.T) {
	job := &Job{
		ID:   stringToPtr("test"),
		Type: stringToPtr("batch"),
	}
	job.Canonicalize()
	tg := &TaskGroup{
		Name: stringToPtr("foo"),
	}
	type testCase struct {
		desc           string
		spread         *Spread
		expectedWeight int8
	}
	cases := []testCase{
		{
			"Nil spread",
			&Spread{
				Attribute: "test",
				Weight:    nil,
			},
			50,
		},
		{
			"Zero spread",
			&Spread{
				Attribute: "test",
				Weight:    int8ToPtr(0),
			},
			0,
		},
		{
			"Non Zero spread",
			&Spread{
				Attribute: "test",
				Weight:    int8ToPtr(100),
			},
			100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			require := require.New(t)
			tg.Spreads = []*Spread{tc.spread}
			tg.Canonicalize(job)
			for _, spr := range tg.Spreads {
				require.Equal(tc.expectedWeight, *spr.Weight)
			}
		})
	}
}
