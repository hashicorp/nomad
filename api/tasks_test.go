// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestTaskGroup_NewTaskGroup(t *testing.T) {
	testutil.Parallel(t)

	grp := NewTaskGroup("grp1", 2)
	expect := &TaskGroup{
		Name:  pointerOf("grp1"),
		Count: pointerOf(2),
	}
	must.Eq(t, expect, grp)
}

func TestTaskGroup_Constrain(t *testing.T) {
	testutil.Parallel(t)

	grp := NewTaskGroup("grp1", 1)

	// Add a constraint to the group
	out := grp.Constrain(NewConstraint("kernel.name", "=", "darwin"))
	must.Len(t, 1, grp.Constraints)

	// Check that the group was returned
	must.Eq(t, grp, out)

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
	must.Eq(t, expect, grp.Constraints)
}

func TestTaskGroup_AddAffinity(t *testing.T) {
	testutil.Parallel(t)

	grp := NewTaskGroup("grp1", 1)

	// Add an affinity to the group
	out := grp.AddAffinity(NewAffinity("kernel.version", "=", "4.6", 100))
	must.Len(t, 1, grp.Affinities)

	// Check that the group was returned
	must.Eq(t, grp, out)

	// Add a second affinity
	grp.AddAffinity(NewAffinity("${node.affinity}", "=", "dc2", 50))
	expect := []*Affinity{
		{
			LTarget: "kernel.version",
			RTarget: "4.6",
			Operand: "=",
			Weight:  pointerOf(int8(100)),
		},
		{
			LTarget: "${node.affinity}",
			RTarget: "dc2",
			Operand: "=",
			Weight:  pointerOf(int8(50)),
		},
	}
	must.Eq(t, expect, grp.Affinities)
}

func TestTaskGroup_SetMeta(t *testing.T) {
	testutil.Parallel(t)

	grp := NewTaskGroup("grp1", 1)

	// Initializes an empty map
	out := grp.SetMeta("foo", "bar")
	must.NotNil(t, grp.Meta)

	// Check that we returned the group
	must.Eq(t, grp, out)

	// Add a second meta k/v
	grp.SetMeta("baz", "zip")
	expect := map[string]string{"foo": "bar", "baz": "zip"}
	must.Eq(t, expect, grp.Meta)
}

func TestTaskGroup_AddSpread(t *testing.T) {
	testutil.Parallel(t)

	grp := NewTaskGroup("grp1", 1)

	// Create and add spread
	spreadTarget := NewSpreadTarget("r1", 50)
	spread := NewSpread("${meta.rack}", 100, []*SpreadTarget{spreadTarget})

	out := grp.AddSpread(spread)
	must.Len(t, 1, grp.Spreads)

	// Check that the group was returned
	must.Eq(t, grp, out)

	// Add a second spread
	spreadTarget2 := NewSpreadTarget("dc1", 100)
	spread2 := NewSpread("${node.datacenter}", 100, []*SpreadTarget{spreadTarget2})

	grp.AddSpread(spread2)

	expect := []*Spread{
		{
			Attribute: "${meta.rack}",
			Weight:    pointerOf(int8(100)),
			SpreadTarget: []*SpreadTarget{
				{
					Value:   "r1",
					Percent: 50,
				},
			},
		},
		{
			Attribute: "${node.datacenter}",
			Weight:    pointerOf(int8(100)),
			SpreadTarget: []*SpreadTarget{
				{
					Value:   "dc1",
					Percent: 100,
				},
			},
		},
	}
	must.Eq(t, expect, grp.Spreads)
}

func TestTaskGroup_AddTask(t *testing.T) {
	testutil.Parallel(t)

	grp := NewTaskGroup("grp1", 1)

	// Add the task to the task group
	out := grp.AddTask(NewTask("task1", "java"))
	must.Len(t, 1, out.Tasks)

	// Check that we returned the group
	must.Eq(t, grp, out)

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
	must.Eq(t, expect, grp.Tasks)
}

func TestTask_NewTask(t *testing.T) {
	testutil.Parallel(t)

	task := NewTask("task1", "exec")
	expect := &Task{
		Name:   "task1",
		Driver: "exec",
	}
	must.Eq(t, expect, task)
}

func TestTask_SetConfig(t *testing.T) {
	testutil.Parallel(t)

	task := NewTask("task1", "exec")

	// Initializes an empty map
	out := task.SetConfig("foo", "bar")
	must.NotNil(t, task.Config)

	// Check that we returned the task
	must.Eq(t, task, out)

	// Set another config value
	task.SetConfig("baz", "zip")
	expect := map[string]interface{}{"foo": "bar", "baz": "zip"}
	must.Eq(t, expect, task.Config)
}

func TestTask_SetMeta(t *testing.T) {
	testutil.Parallel(t)

	task := NewTask("task1", "exec")

	// Initializes an empty map
	out := task.SetMeta("foo", "bar")
	must.NotNil(t, out)

	// Check that we returned the task
	must.Eq(t, task, out)

	// Set another meta k/v
	task.SetMeta("baz", "zip")
	expect := map[string]string{"foo": "bar", "baz": "zip"}
	must.Eq(t, expect, task.Meta)
}

func TestTask_Require(t *testing.T) {
	testutil.Parallel(t)

	task := NewTask("task1", "exec")

	// Create some require resources
	resources := &Resources{
		CPU:      pointerOf(1250),
		MemoryMB: pointerOf(128),
		DiskMB:   pointerOf(2048),
		Networks: []*NetworkResource{
			{
				CIDR:          "0.0.0.0/0",
				MBits:         pointerOf(100),
				ReservedPorts: []Port{{Label: "", Value: 80}, {Label: "", Value: 443}},
			},
		},
	}
	out := task.Require(resources)
	must.Eq(t, resources, task.Resources)

	// Check that we returned the task
	must.Eq(t, task, out)
}

func TestTask_Constrain(t *testing.T) {
	testutil.Parallel(t)

	task := NewTask("task1", "exec")

	// Add a constraint to the task
	out := task.Constrain(NewConstraint("kernel.name", "=", "darwin"))
	must.Len(t, 1, task.Constraints)

	// Check that the task was returned
	must.Eq(t, task, out)

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
	must.Eq(t, expect, task.Constraints)
}

func TestTask_AddAffinity(t *testing.T) {
	testutil.Parallel(t)

	task := NewTask("task1", "exec")

	// Add an affinity to the task
	out := task.AddAffinity(NewAffinity("kernel.version", "=", "4.6", 100))
	must.Len(t, 1, out.Affinities)

	// Check that the task was returned
	must.Eq(t, task, out)

	// Add a second affinity
	task.AddAffinity(NewAffinity("${node.datacenter}", "=", "dc2", 50))
	expect := []*Affinity{
		{
			LTarget: "kernel.version",
			RTarget: "4.6",
			Operand: "=",
			Weight:  pointerOf(int8(100)),
		},
		{
			LTarget: "${node.datacenter}",
			RTarget: "dc2",
			Operand: "=",
			Weight:  pointerOf(int8(50)),
		},
	}
	must.Eq(t, expect, task.Affinities)
}

func TestTask_Artifact(t *testing.T) {
	testutil.Parallel(t)

	a := TaskArtifact{
		GetterSource:  pointerOf("http://localhost/foo.txt"),
		GetterMode:    pointerOf("file"),
		GetterHeaders: make(map[string]string),
		GetterOptions: make(map[string]string),
	}
	a.Canonicalize()
	must.Eq(t, "file", *a.GetterMode)
	must.Eq(t, false, *a.GetterInsecure)
	must.Eq(t, "local/foo.txt", filepath.ToSlash(*a.RelativeDest))
	must.Nil(t, a.GetterOptions)
	must.Nil(t, a.GetterHeaders)
	must.Eq(t, false, a.Chown)
}

func TestTask_VolumeMount(t *testing.T) {
	testutil.Parallel(t)

	vm := new(VolumeMount)
	vm.Canonicalize()
	must.NotNil(t, vm.PropagationMode)
	must.Eq(t, "private", *vm.PropagationMode)
}

func TestTask_Canonicalize_TaskLifecycle(t *testing.T) {
	testutil.Parallel(t)

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
				Name: pointerOf("foo"),
			}
			j := &Job{
				ID: pointerOf("test"),
			}
			tc.task.Canonicalize(tg, j)
			must.Eq(t, tc.expected, tc.task.Lifecycle)
		})
	}
}

func TestTask_Template_WaitConfig_Canonicalize_and_Copy(t *testing.T) {
	testutil.Parallel(t)

	taskWithWait := func(wc *WaitConfig) *Task {
		return &Task{
			Templates: []*Template{
				{
					Wait: wc,
				},
			},
		}
	}

	testCases := []struct {
		name          string
		canonicalized *WaitConfig
		copied        *WaitConfig
		task          *Task
	}{
		{
			name: "all-fields",
			task: taskWithWait(&WaitConfig{
				Min: pointerOf(time.Duration(5)),
				Max: pointerOf(time.Duration(10)),
			}),
			canonicalized: &WaitConfig{
				Min: pointerOf(time.Duration(5)),
				Max: pointerOf(time.Duration(10)),
			},
			copied: &WaitConfig{
				Min: pointerOf(time.Duration(5)),
				Max: pointerOf(time.Duration(10)),
			},
		},
		{
			name: "no-fields",
			task: taskWithWait(&WaitConfig{}),
			canonicalized: &WaitConfig{
				Min: nil,
				Max: nil,
			},
			copied: &WaitConfig{
				Min: nil,
				Max: nil,
			},
		},
		{
			name: "min-only",
			task: taskWithWait(&WaitConfig{
				Min: pointerOf(time.Duration(5)),
			}),
			canonicalized: &WaitConfig{
				Min: pointerOf(time.Duration(5)),
			},
			copied: &WaitConfig{
				Min: pointerOf(time.Duration(5)),
			},
		},
		{
			name: "max-only",
			task: taskWithWait(&WaitConfig{
				Max: pointerOf(time.Duration(10)),
			}),
			canonicalized: &WaitConfig{
				Max: pointerOf(time.Duration(10)),
			},
			copied: &WaitConfig{
				Max: pointerOf(time.Duration(10)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tg := &TaskGroup{
				Name: pointerOf("foo"),
			}
			j := &Job{
				ID: pointerOf("test"),
			}
			must.Eq(t, tc.copied, tc.task.Templates[0].Wait.Copy())
			tc.task.Canonicalize(tg, j)
			must.Eq(t, tc.canonicalized, tc.task.Templates[0].Wait)
		})
	}
}

func TestTask_Canonicalize_Vault(t *testing.T) {
	testCases := []struct {
		name     string
		input    *Vault
		expected *Vault
	}{
		{
			name:  "empty",
			input: &Vault{},
			expected: &Vault{
				Env:                  pointerOf(true),
				DisableFile:          pointerOf(false),
				Namespace:            pointerOf(""),
				Cluster:              "default",
				ChangeMode:           pointerOf("restart"),
				ChangeSignal:         pointerOf("SIGHUP"),
				AllowTokenExpiration: pointerOf(false),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.Canonicalize()
			must.Eq(t, tc.expected, tc.input)
		})
	}
}

// Ensures no regression on https://github.com/hashicorp/nomad/issues/3132
func TestTaskGroup_Canonicalize_Update(t *testing.T) {
	testutil.Parallel(t)

	// Job with an Empty() Update
	job := &Job{
		ID: pointerOf("test"),
		Update: &UpdateStrategy{
			AutoRevert:       pointerOf(false),
			AutoPromote:      pointerOf(false),
			Canary:           pointerOf(0),
			HealthCheck:      pointerOf(""),
			HealthyDeadline:  pointerOf(time.Duration(0)),
			ProgressDeadline: pointerOf(time.Duration(0)),
			MaxParallel:      pointerOf(0),
			MinHealthyTime:   pointerOf(time.Duration(0)),
			Stagger:          pointerOf(time.Duration(0)),
		},
	}
	job.Canonicalize()
	tg := &TaskGroup{
		Name: pointerOf("foo"),
	}
	tg.Canonicalize(job)
	must.NotNil(t, job.Update)
	must.Nil(t, tg.Update)
}

func TestTaskGroup_Canonicalize_Scaling(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{
		ID: pointerOf("test"),
	}
	job.Canonicalize()
	tg := &TaskGroup{
		Name:  pointerOf("foo"),
		Count: nil,
		Scaling: &ScalingPolicy{
			Min:         nil,
			Max:         pointerOf(int64(10)),
			Policy:      nil,
			Enabled:     nil,
			CreateIndex: 0,
			ModifyIndex: 0,
		},
	}
	job.TaskGroups = []*TaskGroup{tg}

	// both nil => both == 1
	tg.Canonicalize(job)
	must.Positive(t, *tg.Count)
	must.NotNil(t, tg.Scaling.Min)
	must.Eq(t, 1, *tg.Count)
	must.Eq(t, int64(*tg.Count), *tg.Scaling.Min)

	// count == nil => count = Scaling.Min
	tg.Count = nil
	tg.Scaling.Min = pointerOf(int64(5))
	tg.Canonicalize(job)
	must.Positive(t, *tg.Count)
	must.NotNil(t, tg.Scaling.Min)
	must.Eq(t, 5, *tg.Count)
	must.Eq(t, int64(*tg.Count), *tg.Scaling.Min)

	// Scaling.Min == nil => Scaling.Min == count
	tg.Count = pointerOf(5)
	tg.Scaling.Min = nil
	tg.Canonicalize(job)
	must.Positive(t, *tg.Count)
	must.NotNil(t, tg.Scaling.Min)
	must.Eq(t, 5, *tg.Scaling.Min)
	must.Eq(t, int64(*tg.Count), *tg.Scaling.Min)

	// both present, both persisted
	tg.Count = pointerOf(5)
	tg.Scaling.Min = pointerOf(int64(1))
	tg.Canonicalize(job)
	must.Positive(t, *tg.Count)
	must.NotNil(t, tg.Scaling.Min)
	must.Eq(t, 1, *tg.Scaling.Min)
	must.Eq(t, 5, *tg.Count)
}

func TestTaskGroup_Merge_Update(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{
		ID:     pointerOf("test"),
		Update: &UpdateStrategy{},
	}
	job.Canonicalize()

	// Merge and canonicalize part of an update block
	tg := &TaskGroup{
		Name: pointerOf("foo"),
		Update: &UpdateStrategy{
			AutoRevert:  pointerOf(true),
			Canary:      pointerOf(5),
			HealthCheck: pointerOf("foo"),
		},
	}

	tg.Canonicalize(job)
	must.Eq(t, &UpdateStrategy{
		AutoRevert:       pointerOf(true),
		AutoPromote:      pointerOf(false),
		Canary:           pointerOf(5),
		HealthCheck:      pointerOf("foo"),
		HealthyDeadline:  pointerOf(5 * time.Minute),
		ProgressDeadline: pointerOf(10 * time.Minute),
		MaxParallel:      pointerOf(1),
		MinHealthyTime:   pointerOf(10 * time.Second),
		Stagger:          pointerOf(30 * time.Second),
	}, tg.Update)
}

// Verifies that migrate strategy is merged correctly
func TestTaskGroup_Canonicalize_MigrateStrategy(t *testing.T) {
	testutil.Parallel(t)

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
				MaxParallel:     pointerOf(1),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(10 * time.Second),
				HealthyDeadline: pointerOf(5 * time.Minute),
			},
		},
		{
			desc:    "Empty job migrate strategy",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel:     pointerOf(0),
				HealthCheck:     pointerOf(""),
				MinHealthyTime:  pointerOf(time.Duration(0)),
				HealthyDeadline: pointerOf(time.Duration(0)),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     pointerOf(0),
				HealthCheck:     pointerOf(""),
				MinHealthyTime:  pointerOf(time.Duration(0)),
				HealthyDeadline: pointerOf(time.Duration(0)),
			},
		},
		{
			desc:    "Inherit from job",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel:     pointerOf(3),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(time.Duration(2)),
				HealthyDeadline: pointerOf(time.Duration(2)),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     pointerOf(3),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(time.Duration(2)),
				HealthyDeadline: pointerOf(time.Duration(2)),
			},
		},
		{
			desc:       "Set in task",
			jobType:    "service",
			jobMigrate: nil,
			taskMigrate: &MigrateStrategy{
				MaxParallel:     pointerOf(3),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(time.Duration(2)),
				HealthyDeadline: pointerOf(time.Duration(2)),
			},
			expected: &MigrateStrategy{
				MaxParallel:     pointerOf(3),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(time.Duration(2)),
				HealthyDeadline: pointerOf(time.Duration(2)),
			},
		},
		{
			desc:    "Merge from job",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: pointerOf(11),
			},
			taskMigrate: &MigrateStrategy{
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(time.Duration(2)),
				HealthyDeadline: pointerOf(time.Duration(2)),
			},
			expected: &MigrateStrategy{
				MaxParallel:     pointerOf(11),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(time.Duration(2)),
				HealthyDeadline: pointerOf(time.Duration(2)),
			},
		},
		{
			desc:    "Override from group",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: pointerOf(11),
			},
			taskMigrate: &MigrateStrategy{
				MaxParallel:     pointerOf(5),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(time.Duration(2)),
				HealthyDeadline: pointerOf(time.Duration(2)),
			},
			expected: &MigrateStrategy{
				MaxParallel:     pointerOf(5),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(time.Duration(2)),
				HealthyDeadline: pointerOf(time.Duration(2)),
			},
		},
		{
			desc:    "Parallel from job, defaulting",
			jobType: "service",
			jobMigrate: &MigrateStrategy{
				MaxParallel: pointerOf(5),
			},
			taskMigrate: nil,
			expected: &MigrateStrategy{
				MaxParallel:     pointerOf(5),
				HealthCheck:     pointerOf("checks"),
				MinHealthyTime:  pointerOf(10 * time.Second),
				HealthyDeadline: pointerOf(5 * time.Minute),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			job := &Job{
				ID:      pointerOf("test"),
				Migrate: tc.jobMigrate,
				Type:    pointerOf(tc.jobType),
			}
			job.Canonicalize()
			tg := &TaskGroup{
				Name:    pointerOf("foo"),
				Migrate: tc.taskMigrate,
			}
			tg.Canonicalize(job)
			must.Eq(t, tc.expected, tg.Migrate)
		})
	}
}

// TestSpread_Canonicalize asserts that the spread block is canonicalized correctly
func TestSpread_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{
		ID:   pointerOf("test"),
		Type: pointerOf("batch"),
	}
	job.Canonicalize()
	tg := &TaskGroup{
		Name: pointerOf("foo"),
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
				Weight:    pointerOf(int8(0)),
			},
			0,
		},
		{
			"Non Zero spread",
			&Spread{
				Attribute: "test",
				Weight:    pointerOf(int8(100)),
			},
			100,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			tg.Spreads = []*Spread{tc.spread}
			tg.Canonicalize(job)
			for _, spr := range tg.Spreads {
				must.Eq(t, tc.expectedWeight, *spr.Weight)
			}
		})
	}
}

func Test_NewDefaultReschedulePolicy(t *testing.T) {
	testutil.Parallel(t)

	testCases := []struct {
		desc         string
		inputJobType string
		expected     *ReschedulePolicy
	}{
		{
			desc:         "service job type",
			inputJobType: "service",
			expected: &ReschedulePolicy{
				Attempts:      pointerOf(0),
				Interval:      pointerOf(time.Duration(0)),
				Delay:         pointerOf(30 * time.Second),
				DelayFunction: pointerOf("exponential"),
				MaxDelay:      pointerOf(1 * time.Hour),
				Unlimited:     pointerOf(true),
			},
		},
		{
			desc:         "batch job type",
			inputJobType: "batch",
			expected: &ReschedulePolicy{
				Attempts:      pointerOf(1),
				Interval:      pointerOf(24 * time.Hour),
				Delay:         pointerOf(5 * time.Second),
				DelayFunction: pointerOf("constant"),
				MaxDelay:      pointerOf(time.Duration(0)),
				Unlimited:     pointerOf(false),
			},
		},
		{
			desc:         "system job type",
			inputJobType: "system",
			expected: &ReschedulePolicy{
				Attempts:      pointerOf(0),
				Interval:      pointerOf(time.Duration(0)),
				Delay:         pointerOf(time.Duration(0)),
				DelayFunction: pointerOf(""),
				MaxDelay:      pointerOf(time.Duration(0)),
				Unlimited:     pointerOf(false),
			},
		},
		{
			desc:         "unrecognised job type",
			inputJobType: "unrecognised",
			expected: &ReschedulePolicy{
				Attempts:      pointerOf(0),
				Interval:      pointerOf(time.Duration(0)),
				Delay:         pointerOf(time.Duration(0)),
				DelayFunction: pointerOf(""),
				MaxDelay:      pointerOf(time.Duration(0)),
				Unlimited:     pointerOf(false),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := NewDefaultReschedulePolicy(tc.inputJobType)
			must.Eq(t, tc.expected, actual)
		})
	}
}

func TestTaskGroup_Canonicalize_Consul(t *testing.T) {
	testutil.Parallel(t)

	t.Run("override job consul in group", func(t *testing.T) {
		job := &Job{
			ID:              pointerOf("job"),
			ConsulNamespace: pointerOf("ns1"),
		}
		job.Canonicalize()

		tg := &TaskGroup{
			Name:   pointerOf("group"),
			Consul: &Consul{Namespace: "ns2"},
		}
		tg.Canonicalize(job)

		must.Eq(t, "ns1", *job.ConsulNamespace)
		must.Eq(t, "ns2", tg.Consul.Namespace)
	})

	t.Run("inherit job consul in group", func(t *testing.T) {
		job := &Job{
			ID:              pointerOf("job"),
			ConsulNamespace: pointerOf("ns1"),
		}
		job.Canonicalize()

		tg := &TaskGroup{
			Name:   pointerOf("group"),
			Consul: nil, // not set, inherit from job
		}
		tg.Canonicalize(job)

		must.Eq(t, "ns1", *job.ConsulNamespace)
		must.Eq(t, "ns1", tg.Consul.Namespace)
	})

	t.Run("set in group only", func(t *testing.T) {
		job := &Job{
			ID:              pointerOf("job"),
			ConsulNamespace: nil,
		}
		job.Canonicalize()

		tg := &TaskGroup{
			Name:   pointerOf("group"),
			Consul: &Consul{Namespace: "ns2"},
		}
		tg.Canonicalize(job)

		must.Eq(t, "", *job.ConsulNamespace)
		must.Eq(t, "ns2", tg.Consul.Namespace)
	})
}
