package api

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

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
				ReservedPorts: []Port{{"", 80, 0, ""}, {"", 443, 0, ""}},
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
		GetterSource:  stringToPtr("http://localhost/foo.txt"),
		GetterMode:    stringToPtr("file"),
		GetterHeaders: make(map[string]string),
		GetterOptions: make(map[string]string),
	}
	a.Canonicalize()
	require.Equal(t, "file", *a.GetterMode)
	require.Equal(t, "local/foo.txt", filepath.ToSlash(*a.RelativeDest))
	require.Nil(t, a.GetterOptions)
	require.Nil(t, a.GetterHeaders)
}

func TestTask_VolumeMount(t *testing.T) {
	t.Parallel()
	vm := &VolumeMount{}
	vm.Canonicalize()
	require.NotNil(t, vm.PropagationMode)
	require.Equal(t, *vm.PropagationMode, "private")
}

func TestTask_Canonicalize_TaskLifecycle(t *testing.T) {
	testCases := []struct {
		name     string
		expected *TaskLifecycle
		task     *Task
	}{
		{
			name: "empty",
			task: &Task{
				Lifecycle: &TaskLifecycle{},
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tg := &TaskGroup{
				Name: stringToPtr("foo"),
			}
			j := &Job{
				ID: stringToPtr("test"),
			}
			tc.task.Canonicalize(tg, j)
			require.Equal(t, tc.expected, tc.task.Lifecycle)

		})
	}
}

// Ensures no regression on https://github.com/hashicorp/nomad/issues/3132
func TestTaskGroup_Canonicalize_Update(t *testing.T) {
	// Job with an Empty() Update
	job := &Job{
		ID: stringToPtr("test"),
		Update: &UpdateStrategy{
			AutoRevert:       boolToPtr(false),
			AutoPromote:      boolToPtr(false),
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
	assert.NotNil(t, job.Update)
	assert.Nil(t, tg.Update)
}

func TestTaskGroup_Canonicalize_Scaling(t *testing.T) {
	require := require.New(t)

	job := &Job{
		ID: stringToPtr("test"),
	}
	job.Canonicalize()
	tg := &TaskGroup{
		Name:  stringToPtr("foo"),
		Count: nil,
		Scaling: &ScalingPolicy{
			Min:         nil,
			Max:         int64ToPtr(10),
			Policy:      nil,
			Enabled:     nil,
			CreateIndex: 0,
			ModifyIndex: 0,
		},
	}
	job.TaskGroups = []*TaskGroup{tg}

	// both nil => both == 1
	tg.Canonicalize(job)
	require.NotNil(tg.Count)
	require.NotNil(tg.Scaling.Min)
	require.EqualValues(1, *tg.Count)
	require.EqualValues(*tg.Count, *tg.Scaling.Min)

	// count == nil => count = Scaling.Min
	tg.Count = nil
	tg.Scaling.Min = int64ToPtr(5)
	tg.Canonicalize(job)
	require.NotNil(tg.Count)
	require.NotNil(tg.Scaling.Min)
	require.EqualValues(5, *tg.Count)
	require.EqualValues(*tg.Count, *tg.Scaling.Min)

	// Scaling.Min == nil => Scaling.Min == count
	tg.Count = intToPtr(5)
	tg.Scaling.Min = nil
	tg.Canonicalize(job)
	require.NotNil(tg.Count)
	require.NotNil(tg.Scaling.Min)
	require.EqualValues(5, *tg.Scaling.Min)
	require.EqualValues(*tg.Scaling.Min, *tg.Count)

	// both present, both persisted
	tg.Count = intToPtr(5)
	tg.Scaling.Min = int64ToPtr(1)
	tg.Canonicalize(job)
	require.NotNil(tg.Count)
	require.NotNil(tg.Scaling.Min)
	require.EqualValues(1, *tg.Scaling.Min)
	require.EqualValues(5, *tg.Count)
}

func TestTaskGroup_Merge_Update(t *testing.T) {
	job := &Job{
		ID:     stringToPtr("test"),
		Update: &UpdateStrategy{},
	}
	job.Canonicalize()

	// Merge and canonicalize part of an update stanza
	tg := &TaskGroup{
		Name: stringToPtr("foo"),
		Update: &UpdateStrategy{
			AutoRevert:  boolToPtr(true),
			Canary:      intToPtr(5),
			HealthCheck: stringToPtr("foo"),
		},
	}

	tg.Canonicalize(job)
	require.Equal(t, &UpdateStrategy{
		AutoRevert:       boolToPtr(true),
		AutoPromote:      boolToPtr(false),
		Canary:           intToPtr(5),
		HealthCheck:      stringToPtr("foo"),
		HealthyDeadline:  timeToPtr(5 * time.Minute),
		ProgressDeadline: timeToPtr(10 * time.Minute),
		MaxParallel:      intToPtr(1),
		MinHealthyTime:   timeToPtr(10 * time.Second),
		Stagger:          timeToPtr(30 * time.Second),
	}, tg.Update)
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

func Test_NewDefaultReschedulePolicy(t *testing.T) {
	testCases := []struct {
		desc         string
		inputJobType string
		expected     *ReschedulePolicy
	}{
		{
			desc:         "service job type",
			inputJobType: "service",
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(0),
				Interval:      timeToPtr(0),
				Delay:         timeToPtr(30 * time.Second),
				DelayFunction: stringToPtr("exponential"),
				MaxDelay:      timeToPtr(1 * time.Hour),
				Unlimited:     boolToPtr(true),
			},
		},
		{
			desc:         "batch job type",
			inputJobType: "batch",
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(1),
				Interval:      timeToPtr(24 * time.Hour),
				Delay:         timeToPtr(5 * time.Second),
				DelayFunction: stringToPtr("constant"),
				MaxDelay:      timeToPtr(0),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc:         "system job type",
			inputJobType: "system",
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(0),
				Interval:      timeToPtr(0),
				Delay:         timeToPtr(0),
				DelayFunction: stringToPtr(""),
				MaxDelay:      timeToPtr(0),
				Unlimited:     boolToPtr(false),
			},
		},
		{
			desc:         "unrecognised job type",
			inputJobType: "unrecognised",
			expected: &ReschedulePolicy{
				Attempts:      intToPtr(0),
				Interval:      timeToPtr(0),
				Delay:         timeToPtr(0),
				DelayFunction: stringToPtr(""),
				MaxDelay:      timeToPtr(0),
				Unlimited:     boolToPtr(false),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := NewDefaultReschedulePolicy(tc.inputJobType)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
