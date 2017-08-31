package structs

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/kr/pretty"
)

func TestJob_Validate(t *testing.T) {
	j := &Job{}
	err := j.Validate()
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "job region") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "job ID") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "job name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[3].Error(), "job type") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[4].Error(), "priority") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[5].Error(), "datacenters") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[6].Error(), "task groups") {
		t.Fatalf("err: %s", err)
	}

	j = &Job{
		Type: "invalid-job-type",
	}
	err = j.Validate()
	if expected := `Invalid job type: "invalid-job-type"`; !strings.Contains(err.Error(), expected) {
		t.Errorf("expected %s but found: %v", expected, err)
	}

	j = &Job{
		Type: JobTypeService,
		Periodic: &PeriodicConfig{
			Enabled: true,
		},
	}
	err = j.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Error(), "Periodic") {
		t.Fatalf("err: %s", err)
	}

	j = &Job{
		Region:      "global",
		ID:          GenerateUUID(),
		Name:        "my-job",
		Type:        JobTypeService,
		Priority:    50,
		Datacenters: []string{"dc1"},
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name: "web",
				RestartPolicy: &RestartPolicy{
					Interval: 5 * time.Minute,
					Delay:    10 * time.Second,
					Attempts: 10,
				},
			},
			&TaskGroup{
				Name: "web",
				RestartPolicy: &RestartPolicy{
					Interval: 5 * time.Minute,
					Delay:    10 * time.Second,
					Attempts: 10,
				},
			},
			&TaskGroup{
				RestartPolicy: &RestartPolicy{
					Interval: 5 * time.Minute,
					Delay:    10 * time.Second,
					Attempts: 10,
				},
			},
		},
	}
	err = j.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "2 redefines 'web' from group 1") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "group 3 missing name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "Task group web validation failed") {
		t.Fatalf("err: %s", err)
	}
}

func TestJob_Warnings(t *testing.T) {
	cases := []struct {
		Name     string
		Job      *Job
		Expected []string
	}{
		{
			Name:     "Higher counts for update stanza",
			Expected: []string{"max parallel count is greater"},
			Job: &Job{
				Type: JobTypeService,
				TaskGroups: []*TaskGroup{
					{
						Name:  "foo",
						Count: 2,
						Update: &UpdateStrategy{
							MaxParallel: 10,
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			warnings := c.Job.Warnings()
			if warnings == nil {
				if len(c.Expected) == 0 {
					return
				} else {
					t.Fatal("Got no warnings when they were expected")
				}
			}

			a := warnings.Error()
			for _, e := range c.Expected {
				if !strings.Contains(a, e) {
					t.Fatalf("Got warnings %q; didn't contain %q", a, e)
				}
			}
		})
	}
}

func TestJob_Canonicalize_Update(t *testing.T) {
	cases := []struct {
		Name     string
		Job      *Job
		Expected *Job
		Warnings []string
	}{
		{
			Name:     "One task group",
			Warnings: []string{"conversion to new update stanza"},
			Job: &Job{
				Type: JobTypeService,
				Update: UpdateStrategy{
					MaxParallel: 2,
					Stagger:     10 * time.Second,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  "foo",
						Count: 2,
					},
				},
			},
			Expected: &Job{
				Type: JobTypeService,
				Update: UpdateStrategy{
					MaxParallel: 2,
					Stagger:     10 * time.Second,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:          "foo",
						Count:         2,
						RestartPolicy: NewRestartPolicy(JobTypeService),
						EphemeralDisk: DefaultEphemeralDisk(),
						Update: &UpdateStrategy{
							Stagger:         30 * time.Second,
							MaxParallel:     2,
							HealthCheck:     UpdateStrategyHealthCheck_Checks,
							MinHealthyTime:  10 * time.Second,
							HealthyDeadline: 5 * time.Minute,
							AutoRevert:      false,
							Canary:          0,
						},
					},
				},
			},
		},
		{
			Name:     "One task group batch",
			Warnings: []string{"Update stanza is disallowed for batch jobs"},
			Job: &Job{
				Type: JobTypeBatch,
				Update: UpdateStrategy{
					MaxParallel: 2,
					Stagger:     10 * time.Second,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  "foo",
						Count: 2,
					},
				},
			},
			Expected: &Job{
				Type:   JobTypeBatch,
				Update: UpdateStrategy{},
				TaskGroups: []*TaskGroup{
					{
						Name:          "foo",
						Count:         2,
						RestartPolicy: NewRestartPolicy(JobTypeBatch),
						EphemeralDisk: DefaultEphemeralDisk(),
					},
				},
			},
		},
		{
			Name:     "One task group batch - new spec",
			Warnings: []string{"Update stanza is disallowed for batch jobs"},
			Job: &Job{
				Type: JobTypeBatch,
				Update: UpdateStrategy{
					Stagger:         2 * time.Second,
					MaxParallel:     2,
					Canary:          2,
					MinHealthyTime:  2 * time.Second,
					HealthyDeadline: 10 * time.Second,
					HealthCheck:     UpdateStrategyHealthCheck_Checks,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  "foo",
						Count: 2,
						Update: &UpdateStrategy{
							Stagger:         2 * time.Second,
							MaxParallel:     2,
							Canary:          2,
							MinHealthyTime:  2 * time.Second,
							HealthyDeadline: 10 * time.Second,
							HealthCheck:     UpdateStrategyHealthCheck_Checks,
						},
					},
				},
			},
			Expected: &Job{
				Type:   JobTypeBatch,
				Update: UpdateStrategy{},
				TaskGroups: []*TaskGroup{
					{
						Name:          "foo",
						Count:         2,
						RestartPolicy: NewRestartPolicy(JobTypeBatch),
						EphemeralDisk: DefaultEphemeralDisk(),
					},
				},
			},
		},
		{
			Name: "One task group service - new spec",
			Job: &Job{
				Type: JobTypeService,
				Update: UpdateStrategy{
					Stagger:         2 * time.Second,
					MaxParallel:     2,
					Canary:          2,
					MinHealthyTime:  2 * time.Second,
					HealthyDeadline: 10 * time.Second,
					HealthCheck:     UpdateStrategyHealthCheck_Checks,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  "foo",
						Count: 2,
						Update: &UpdateStrategy{
							Stagger:         2 * time.Second,
							MaxParallel:     2,
							Canary:          2,
							MinHealthyTime:  2 * time.Second,
							HealthyDeadline: 10 * time.Second,
							HealthCheck:     UpdateStrategyHealthCheck_Checks,
						},
					},
				},
			},
			Expected: &Job{
				Type: JobTypeService,
				Update: UpdateStrategy{
					Stagger:         2 * time.Second,
					MaxParallel:     2,
					Canary:          2,
					MinHealthyTime:  2 * time.Second,
					HealthyDeadline: 10 * time.Second,
					HealthCheck:     UpdateStrategyHealthCheck_Checks,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:          "foo",
						Count:         2,
						RestartPolicy: NewRestartPolicy(JobTypeService),
						EphemeralDisk: DefaultEphemeralDisk(),
						Update: &UpdateStrategy{
							Stagger:         2 * time.Second,
							MaxParallel:     2,
							Canary:          2,
							MinHealthyTime:  2 * time.Second,
							HealthyDeadline: 10 * time.Second,
							HealthCheck:     UpdateStrategyHealthCheck_Checks,
						},
					},
				},
			},
		},
		{
			Name:     "One task group; too high of parallelism",
			Warnings: []string{"conversion to new update stanza"},
			Job: &Job{
				Type: JobTypeService,
				Update: UpdateStrategy{
					MaxParallel: 200,
					Stagger:     10 * time.Second,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  "foo",
						Count: 2,
					},
				},
			},
			Expected: &Job{
				Type: JobTypeService,
				Update: UpdateStrategy{
					MaxParallel: 200,
					Stagger:     10 * time.Second,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:          "foo",
						Count:         2,
						RestartPolicy: NewRestartPolicy(JobTypeService),
						EphemeralDisk: DefaultEphemeralDisk(),
						Update: &UpdateStrategy{
							Stagger:         30 * time.Second,
							MaxParallel:     2,
							HealthCheck:     UpdateStrategyHealthCheck_Checks,
							MinHealthyTime:  10 * time.Second,
							HealthyDeadline: 5 * time.Minute,
							AutoRevert:      false,
							Canary:          0,
						},
					},
				},
			},
		},
		{
			Name:     "Multiple task group; rounding",
			Warnings: []string{"conversion to new update stanza"},
			Job: &Job{
				Type: JobTypeService,
				Update: UpdateStrategy{
					MaxParallel: 2,
					Stagger:     10 * time.Second,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  "foo",
						Count: 2,
					},
					{
						Name:  "bar",
						Count: 14,
					},
					{
						Name:  "foo",
						Count: 26,
					},
				},
			},
			Expected: &Job{
				Type: JobTypeService,
				Update: UpdateStrategy{
					MaxParallel: 2,
					Stagger:     10 * time.Second,
				},
				TaskGroups: []*TaskGroup{
					{
						Name:          "foo",
						Count:         2,
						RestartPolicy: NewRestartPolicy(JobTypeService),
						EphemeralDisk: DefaultEphemeralDisk(),
						Update: &UpdateStrategy{
							Stagger:         30 * time.Second,
							MaxParallel:     1,
							HealthCheck:     UpdateStrategyHealthCheck_Checks,
							MinHealthyTime:  10 * time.Second,
							HealthyDeadline: 5 * time.Minute,
							AutoRevert:      false,
							Canary:          0,
						},
					},
					{
						Name:          "bar",
						Count:         14,
						RestartPolicy: NewRestartPolicy(JobTypeService),
						EphemeralDisk: DefaultEphemeralDisk(),
						Update: &UpdateStrategy{
							Stagger:         30 * time.Second,
							MaxParallel:     1,
							HealthCheck:     UpdateStrategyHealthCheck_Checks,
							MinHealthyTime:  10 * time.Second,
							HealthyDeadline: 5 * time.Minute,
							AutoRevert:      false,
							Canary:          0,
						},
					},
					{
						Name:          "foo",
						Count:         26,
						EphemeralDisk: DefaultEphemeralDisk(),
						RestartPolicy: NewRestartPolicy(JobTypeService),
						Update: &UpdateStrategy{
							Stagger:         30 * time.Second,
							MaxParallel:     3,
							HealthCheck:     UpdateStrategyHealthCheck_Checks,
							MinHealthyTime:  10 * time.Second,
							HealthyDeadline: 5 * time.Minute,
							AutoRevert:      false,
							Canary:          0,
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			warnings := c.Job.Canonicalize()
			if !reflect.DeepEqual(c.Job, c.Expected) {
				t.Fatalf("Diff %#v", pretty.Diff(c.Job, c.Expected))
			}

			wErr := ""
			if warnings != nil {
				wErr = warnings.Error()
			}
			for _, w := range c.Warnings {
				if !strings.Contains(wErr, w) {
					t.Fatalf("Wanted warning %q: got %q", w, wErr)
				}
			}

			if len(c.Warnings) == 0 && warnings != nil {
				t.Fatalf("Wanted no warnings: got %q", wErr)
			}
		})
	}
}

func TestJob_SpecChanged(t *testing.T) {
	// Get a base test job
	base := testJob()

	// Only modify the indexes/mutable state of the job
	mutatedBase := base.Copy()
	mutatedBase.Status = "foo"
	mutatedBase.ModifyIndex = base.ModifyIndex + 100

	// changed contains a spec change that should be detected
	change := base.Copy()
	change.Priority = 99

	cases := []struct {
		Name     string
		Original *Job
		New      *Job
		Changed  bool
	}{
		{
			Name:     "Same job except mutable indexes",
			Changed:  false,
			Original: base,
			New:      mutatedBase,
		},
		{
			Name:     "Different",
			Changed:  true,
			Original: base,
			New:      change,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if actual := c.Original.SpecChanged(c.New); actual != c.Changed {
				t.Fatalf("SpecChanged() returned %v; want %v", actual, c.Changed)
			}
		})
	}
}

func testJob() *Job {
	return &Job{
		Region:      "global",
		ID:          GenerateUUID(),
		Name:        "my-job",
		Type:        JobTypeService,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*Constraint{
			&Constraint{
				LTarget: "$attr.kernel.name",
				RTarget: "linux",
				Operand: "=",
			},
		},
		Periodic: &PeriodicConfig{
			Enabled: false,
		},
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name:          "web",
				Count:         10,
				EphemeralDisk: DefaultEphemeralDisk(),
				RestartPolicy: &RestartPolicy{
					Mode:     RestartPolicyModeFail,
					Attempts: 3,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
				},
				Tasks: []*Task{
					&Task{
						Name:   "web",
						Driver: "exec",
						Config: map[string]interface{}{
							"command": "/bin/date",
						},
						Env: map[string]string{
							"FOO": "bar",
						},
						Artifacts: []*TaskArtifact{
							{
								GetterSource: "http://foo.com",
							},
						},
						Services: []*Service{
							{
								Name:      "${TASK}-frontend",
								PortLabel: "http",
							},
						},
						Resources: &Resources{
							CPU:      500,
							MemoryMB: 256,
							Networks: []*NetworkResource{
								&NetworkResource{
									MBits:        50,
									DynamicPorts: []Port{{Label: "http"}},
								},
							},
						},
						LogConfig: &LogConfig{
							MaxFiles:      10,
							MaxFileSizeMB: 1,
						},
					},
				},
				Meta: map[string]string{
					"elb_check_type":     "http",
					"elb_check_interval": "30s",
					"elb_check_min":      "3",
				},
			},
		},
		Meta: map[string]string{
			"owner": "armon",
		},
	}
}

func TestJob_Copy(t *testing.T) {
	j := testJob()
	c := j.Copy()
	if !reflect.DeepEqual(j, c) {
		t.Fatalf("Copy() returned an unequal Job; got %#v; want %#v", c, j)
	}
}

func TestJob_IsPeriodic(t *testing.T) {
	j := &Job{
		Type: JobTypeService,
		Periodic: &PeriodicConfig{
			Enabled: true,
		},
	}
	if !j.IsPeriodic() {
		t.Fatalf("IsPeriodic() returned false on periodic job")
	}

	j = &Job{
		Type: JobTypeService,
	}
	if j.IsPeriodic() {
		t.Fatalf("IsPeriodic() returned true on non-periodic job")
	}
}

func TestJob_SystemJob_Validate(t *testing.T) {
	j := testJob()
	j.Type = JobTypeSystem
	j.Canonicalize()

	err := j.Validate()
	if err == nil || !strings.Contains(err.Error(), "exceed") {
		t.Fatalf("expect error due to count")
	}

	j.TaskGroups[0].Count = 0
	if err := j.Validate(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	j.TaskGroups[0].Count = 1
	if err := j.Validate(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestJob_VaultPolicies(t *testing.T) {
	j0 := &Job{}
	e0 := make(map[string]map[string]*Vault, 0)

	vj1 := &Vault{
		Policies: []string{
			"p1",
			"p2",
		},
	}
	vj2 := &Vault{
		Policies: []string{
			"p3",
			"p4",
		},
	}
	vj3 := &Vault{
		Policies: []string{
			"p5",
		},
	}
	j1 := &Job{
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name: "foo",
				Tasks: []*Task{
					&Task{
						Name: "t1",
					},
					&Task{
						Name:  "t2",
						Vault: vj1,
					},
				},
			},
			&TaskGroup{
				Name: "bar",
				Tasks: []*Task{
					&Task{
						Name:  "t3",
						Vault: vj2,
					},
					&Task{
						Name:  "t4",
						Vault: vj3,
					},
				},
			},
		},
	}

	e1 := map[string]map[string]*Vault{
		"foo": map[string]*Vault{
			"t2": vj1,
		},
		"bar": map[string]*Vault{
			"t3": vj2,
			"t4": vj3,
		},
	}

	cases := []struct {
		Job      *Job
		Expected map[string]map[string]*Vault
	}{
		{
			Job:      j0,
			Expected: e0,
		},
		{
			Job:      j1,
			Expected: e1,
		},
	}

	for i, c := range cases {
		got := c.Job.VaultPolicies()
		if !reflect.DeepEqual(got, c.Expected) {
			t.Fatalf("case %d: got %#v; want %#v", i+1, got, c.Expected)
		}
	}
}

func TestJob_RequiredSignals(t *testing.T) {
	j0 := &Job{}
	e0 := make(map[string]map[string][]string, 0)

	vj1 := &Vault{
		Policies:   []string{"p1"},
		ChangeMode: VaultChangeModeNoop,
	}
	vj2 := &Vault{
		Policies:     []string{"p1"},
		ChangeMode:   VaultChangeModeSignal,
		ChangeSignal: "SIGUSR1",
	}
	tj1 := &Template{
		SourcePath: "foo",
		DestPath:   "bar",
		ChangeMode: TemplateChangeModeNoop,
	}
	tj2 := &Template{
		SourcePath:   "foo",
		DestPath:     "bar",
		ChangeMode:   TemplateChangeModeSignal,
		ChangeSignal: "SIGUSR2",
	}
	j1 := &Job{
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name: "foo",
				Tasks: []*Task{
					&Task{
						Name: "t1",
					},
					&Task{
						Name:      "t2",
						Vault:     vj2,
						Templates: []*Template{tj2},
					},
				},
			},
			&TaskGroup{
				Name: "bar",
				Tasks: []*Task{
					&Task{
						Name:      "t3",
						Vault:     vj1,
						Templates: []*Template{tj1},
					},
					&Task{
						Name:  "t4",
						Vault: vj2,
					},
				},
			},
		},
	}

	e1 := map[string]map[string][]string{
		"foo": map[string][]string{
			"t2": []string{"SIGUSR1", "SIGUSR2"},
		},
		"bar": map[string][]string{
			"t4": []string{"SIGUSR1"},
		},
	}

	cases := []struct {
		Job      *Job
		Expected map[string]map[string][]string
	}{
		{
			Job:      j0,
			Expected: e0,
		},
		{
			Job:      j1,
			Expected: e1,
		},
	}

	for i, c := range cases {
		got := c.Job.RequiredSignals()
		if !reflect.DeepEqual(got, c.Expected) {
			t.Fatalf("case %d: got %#v; want %#v", i+1, got, c.Expected)
		}
	}
}

func TestTaskGroup_Validate(t *testing.T) {
	j := testJob()
	tg := &TaskGroup{
		Count: -1,
		RestartPolicy: &RestartPolicy{
			Interval: 5 * time.Minute,
			Delay:    10 * time.Second,
			Attempts: 10,
			Mode:     RestartPolicyModeDelay,
		},
	}
	err := tg.Validate(j)
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "group name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "count can't be negative") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "Missing tasks") {
		t.Fatalf("err: %s", err)
	}

	tg = &TaskGroup{
		Tasks: []*Task{
			&Task{
				Name: "task-a",
				Resources: &Resources{
					Networks: []*NetworkResource{
						&NetworkResource{
							ReservedPorts: []Port{{Label: "foo", Value: 123}},
						},
					},
				},
			},
			&Task{
				Name: "task-b",
				Resources: &Resources{
					Networks: []*NetworkResource{
						&NetworkResource{
							ReservedPorts: []Port{{Label: "foo", Value: 123}},
						},
					},
				},
			},
		},
	}
	err = tg.Validate(&Job{})
	expected := `Static port 123 already reserved by task-a:foo`
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected %s but found: %v", expected, err)
	}

	tg = &TaskGroup{
		Tasks: []*Task{
			&Task{
				Name: "task-a",
				Resources: &Resources{
					Networks: []*NetworkResource{
						&NetworkResource{
							ReservedPorts: []Port{
								{Label: "foo", Value: 123},
								{Label: "bar", Value: 123},
							},
						},
					},
				},
			},
		},
	}
	err = tg.Validate(&Job{})
	expected = `Static port 123 already reserved by task-a:foo`
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected %s but found: %v", expected, err)
	}

	tg = &TaskGroup{
		Name:  "web",
		Count: 1,
		Tasks: []*Task{
			&Task{Name: "web", Leader: true},
			&Task{Name: "web", Leader: true},
			&Task{},
		},
		RestartPolicy: &RestartPolicy{
			Interval: 5 * time.Minute,
			Delay:    10 * time.Second,
			Attempts: 10,
			Mode:     RestartPolicyModeDelay,
		},
	}

	err = tg.Validate(j)
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "should have an ephemeral disk object") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "2 redefines 'web' from task 1") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "Task 3 missing name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[3].Error(), "Only one task may be marked as leader") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[4].Error(), "Task web validation failed") {
		t.Fatalf("err: %s", err)
	}

	// COMPAT: Enable in 0.7.0
	//j.Type = JobTypeBatch
	//err = tg.Validate(j)
	//if !strings.Contains(err.Error(), "does not allow update block") {
	//t.Fatalf("err: %s", err)
	//}
}

func TestTask_Validate(t *testing.T) {
	task := &Task{}
	ephemeralDisk := DefaultEphemeralDisk()
	err := task.Validate(ephemeralDisk)
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "task name") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "task driver") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "task resources") {
		t.Fatalf("err: %s", err)
	}

	task = &Task{Name: "web/foo"}
	err = task.Validate(ephemeralDisk)
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "slashes") {
		t.Fatalf("err: %s", err)
	}

	task = &Task{
		Name:   "web",
		Driver: "docker",
		Resources: &Resources{
			CPU:      100,
			MemoryMB: 100,
			IOPS:     10,
		},
		LogConfig: DefaultLogConfig(),
	}
	ephemeralDisk.SizeMB = 200
	err = task.Validate(ephemeralDisk)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	task.Constraints = append(task.Constraints,
		&Constraint{
			Operand: ConstraintDistinctHosts,
		},
		&Constraint{
			Operand: ConstraintDistinctProperty,
			LTarget: "${meta.rack}",
		})

	err = task.Validate(ephemeralDisk)
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "task level: distinct_hosts") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "task level: distinct_property") {
		t.Fatalf("err: %s", err)
	}
}

func TestTask_Validate_Services(t *testing.T) {
	s1 := &Service{
		Name:      "service-name",
		PortLabel: "bar",
		Checks: []*ServiceCheck{
			{
				Name:     "check-name",
				Type:     ServiceCheckTCP,
				Interval: 0 * time.Second,
			},
			{
				Name:    "check-name",
				Type:    ServiceCheckTCP,
				Timeout: 2 * time.Second,
			},
			{
				Name:     "check-name",
				Type:     ServiceCheckTCP,
				Interval: 1 * time.Second,
			},
		},
	}

	s2 := &Service{
		Name:      "service-name",
		PortLabel: "bar",
	}

	s3 := &Service{
		Name:      "service-A",
		PortLabel: "a",
	}
	s4 := &Service{
		Name:      "service-A",
		PortLabel: "b",
	}

	ephemeralDisk := DefaultEphemeralDisk()
	ephemeralDisk.SizeMB = 200
	task := &Task{
		Name:   "web",
		Driver: "docker",
		Resources: &Resources{
			CPU:      100,
			MemoryMB: 100,
			IOPS:     10,
		},
		Services: []*Service{s1, s2},
	}

	task1 := &Task{
		Name:      "web",
		Driver:    "docker",
		Resources: DefaultResources(),
		Services:  []*Service{s3, s4},
		LogConfig: DefaultLogConfig(),
	}
	task1.Resources.Networks = []*NetworkResource{
		&NetworkResource{
			MBits: 10,
			DynamicPorts: []Port{
				Port{
					Label: "a",
					Value: 1000,
				},
				Port{
					Label: "b",
					Value: 2000,
				},
			},
		},
	}

	err := task.Validate(ephemeralDisk)
	if err == nil {
		t.Fatal("expected an error")
	}

	if !strings.Contains(err.Error(), "service \"service-name\" is duplicate") {
		t.Fatalf("err: %v", err)
	}

	if !strings.Contains(err.Error(), "check \"check-name\" is duplicate") {
		t.Fatalf("err: %v", err)
	}

	if !strings.Contains(err.Error(), "missing required value interval") {
		t.Fatalf("err: %v", err)
	}

	if !strings.Contains(err.Error(), "cannot be less than") {
		t.Fatalf("err: %v", err)
	}

	if err = task1.Validate(ephemeralDisk); err != nil {
		t.Fatalf("err : %v", err)
	}
}

func TestTask_Validate_Service_Check(t *testing.T) {

	invalidCheck := ServiceCheck{
		Name:     "check-name",
		Command:  "/bin/true",
		Type:     ServiceCheckScript,
		Interval: 10 * time.Second,
	}

	err := invalidCheck.validate()
	if err == nil || !strings.Contains(err.Error(), "Timeout cannot be less") {
		t.Fatalf("expected a timeout validation error but received: %q", err)
	}

	check1 := ServiceCheck{
		Name:     "check-name",
		Type:     ServiceCheckTCP,
		Interval: 10 * time.Second,
		Timeout:  2 * time.Second,
	}

	if err := check1.validate(); err != nil {
		t.Fatalf("err: %v", err)
	}

	check1.InitialStatus = "foo"
	err = check1.validate()
	if err == nil {
		t.Fatal("Expected an error")
	}

	if !strings.Contains(err.Error(), "invalid initial check state (foo)") {
		t.Fatalf("err: %v", err)
	}

	check1.InitialStatus = api.HealthCritical
	err = check1.validate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	check1.InitialStatus = api.HealthPassing
	err = check1.validate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	check1.InitialStatus = ""
	err = check1.validate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestTask_Validate_LogConfig(t *testing.T) {
	task := &Task{
		LogConfig: DefaultLogConfig(),
	}
	ephemeralDisk := &EphemeralDisk{
		SizeMB: 1,
	}

	err := task.Validate(ephemeralDisk)
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[3].Error(), "log storage") {
		t.Fatalf("err: %s", err)
	}
}

func TestTask_Validate_Template(t *testing.T) {

	bad := &Template{}
	task := &Task{
		Templates: []*Template{bad},
	}
	ephemeralDisk := &EphemeralDisk{
		SizeMB: 1,
	}

	err := task.Validate(ephemeralDisk)
	if !strings.Contains(err.Error(), "Template 1 validation failed") {
		t.Fatalf("err: %s", err)
	}

	// Have two templates that share the same destination
	good := &Template{
		SourcePath: "foo",
		DestPath:   "local/foo",
		ChangeMode: "noop",
	}

	task.Templates = []*Template{good, good}
	err = task.Validate(ephemeralDisk)
	if !strings.Contains(err.Error(), "same destination as") {
		t.Fatalf("err: %s", err)
	}

	// Env templates can't use signals
	task.Templates = []*Template{
		{
			Envvars:    true,
			ChangeMode: "signal",
		},
	}

	err = task.Validate(ephemeralDisk)
	if err == nil {
		t.Fatalf("expected error from Template.Validate")
	}
	if expected := "cannot use signals"; !strings.Contains(err.Error(), expected) {
		t.Errorf("expected to find %q but found %v", expected, err)
	}
}

func TestTemplate_Validate(t *testing.T) {
	cases := []struct {
		Tmpl         *Template
		Fail         bool
		ContainsErrs []string
	}{
		{
			Tmpl: &Template{},
			Fail: true,
			ContainsErrs: []string{
				"specify a source path",
				"specify a destination",
				TemplateChangeModeInvalidError.Error(),
			},
		},
		{
			Tmpl: &Template{
				Splay: -100,
			},
			Fail: true,
			ContainsErrs: []string{
				"positive splay",
			},
		},
		{
			Tmpl: &Template{
				ChangeMode: "foo",
			},
			Fail: true,
			ContainsErrs: []string{
				TemplateChangeModeInvalidError.Error(),
			},
		},
		{
			Tmpl: &Template{
				ChangeMode: "signal",
			},
			Fail: true,
			ContainsErrs: []string{
				"specify signal value",
			},
		},
		{
			Tmpl: &Template{
				SourcePath: "foo",
				DestPath:   "../../root",
				ChangeMode: "noop",
			},
			Fail: true,
			ContainsErrs: []string{
				"destination escapes",
			},
		},
		{
			Tmpl: &Template{
				SourcePath: "foo",
				DestPath:   "local/foo",
				ChangeMode: "noop",
			},
			Fail: false,
		},
		{
			Tmpl: &Template{
				SourcePath: "foo",
				DestPath:   "local/foo",
				ChangeMode: "noop",
				Perms:      "0444",
			},
			Fail: false,
		},
		{
			Tmpl: &Template{
				SourcePath: "foo",
				DestPath:   "local/foo",
				ChangeMode: "noop",
				Perms:      "zza",
			},
			Fail: true,
			ContainsErrs: []string{
				"as octal",
			},
		},
	}

	for i, c := range cases {
		err := c.Tmpl.Validate()
		if err != nil {
			if !c.Fail {
				t.Fatalf("Case %d: shouldn't have failed: %v", i+1, err)
			}

			e := err.Error()
			for _, exp := range c.ContainsErrs {
				if !strings.Contains(e, exp) {
					t.Fatalf("Cased %d: should have contained error %q: %q", i+1, exp, e)
				}
			}
		} else if c.Fail {
			t.Fatalf("Case %d: should have failed: %v", i+1, err)
		}
	}
}

func TestConstraint_Validate(t *testing.T) {
	c := &Constraint{}
	err := c.Validate()
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "Missing constraint operand") {
		t.Fatalf("err: %s", err)
	}

	c = &Constraint{
		LTarget: "$attr.kernel.name",
		RTarget: "linux",
		Operand: "=",
	}
	err = c.Validate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Perform additional regexp validation
	c.Operand = ConstraintRegex
	c.RTarget = "(foo"
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "missing closing") {
		t.Fatalf("err: %s", err)
	}

	// Perform version validation
	c.Operand = ConstraintVersion
	c.RTarget = "~> foo"
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "Malformed constraint") {
		t.Fatalf("err: %s", err)
	}

	// Perform distinct_property validation
	c.Operand = ConstraintDistinctProperty
	c.RTarget = "0"
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "count of 1 or greater") {
		t.Fatalf("err: %s", err)
	}

	c.RTarget = "-1"
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "to uint64") {
		t.Fatalf("err: %s", err)
	}

	// Perform distinct_hosts validation
	c.Operand = ConstraintDistinctHosts
	c.LTarget = ""
	c.RTarget = ""
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid constraint: %v", err)
	}

	// Perform set_contains validation
	c.Operand = ConstraintSetContains
	c.RTarget = ""
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "requires an RTarget") {
		t.Fatalf("err: %s", err)
	}

	// Perform LTarget validation
	c.Operand = ConstraintRegex
	c.RTarget = "foo"
	c.LTarget = ""
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "No LTarget") {
		t.Fatalf("err: %s", err)
	}

	// Perform constraint type validation
	c.Operand = "foo"
	err = c.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "Unknown constraint type") {
		t.Fatalf("err: %s", err)
	}
}

func TestUpdateStrategy_Validate(t *testing.T) {
	u := &UpdateStrategy{
		MaxParallel:     0,
		HealthCheck:     "foo",
		MinHealthyTime:  -10,
		HealthyDeadline: -15,
		AutoRevert:      false,
		Canary:          -1,
	}

	err := u.Validate()
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "Invalid health check given") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "Max parallel can not be less than one") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[2].Error(), "Canary count can not be less than zero") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[3].Error(), "Minimum healthy time may not be less than zero") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[4].Error(), "Healthy deadline must be greater than zero") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[5].Error(), "Minimum healthy time must be less than healthy deadline") {
		t.Fatalf("err: %s", err)
	}
}

func TestResource_NetIndex(t *testing.T) {
	r := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{Device: "eth0"},
			&NetworkResource{Device: "lo0"},
			&NetworkResource{Device: ""},
		},
	}
	if idx := r.NetIndex(&NetworkResource{Device: "eth0"}); idx != 0 {
		t.Fatalf("Bad: %d", idx)
	}
	if idx := r.NetIndex(&NetworkResource{Device: "lo0"}); idx != 1 {
		t.Fatalf("Bad: %d", idx)
	}
	if idx := r.NetIndex(&NetworkResource{Device: "eth1"}); idx != -1 {
		t.Fatalf("Bad: %d", idx)
	}
}

func TestResource_Superset(t *testing.T) {
	r1 := &Resources{
		CPU:      2000,
		MemoryMB: 2048,
		DiskMB:   10000,
		IOPS:     100,
	}
	r2 := &Resources{
		CPU:      2000,
		MemoryMB: 1024,
		DiskMB:   5000,
		IOPS:     50,
	}

	if s, _ := r1.Superset(r1); !s {
		t.Fatalf("bad")
	}
	if s, _ := r1.Superset(r2); !s {
		t.Fatalf("bad")
	}
	if s, _ := r2.Superset(r1); s {
		t.Fatalf("bad")
	}
	if s, _ := r2.Superset(r2); !s {
		t.Fatalf("bad")
	}
}

func TestResource_Add(t *testing.T) {
	r1 := &Resources{
		CPU:      2000,
		MemoryMB: 2048,
		DiskMB:   10000,
		IOPS:     100,
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:          "10.0.0.0/8",
				MBits:         100,
				ReservedPorts: []Port{{"ssh", 22}},
			},
		},
	}
	r2 := &Resources{
		CPU:      2000,
		MemoryMB: 1024,
		DiskMB:   5000,
		IOPS:     50,
		Networks: []*NetworkResource{
			&NetworkResource{
				IP:            "10.0.0.1",
				MBits:         50,
				ReservedPorts: []Port{{"web", 80}},
			},
		},
	}

	err := r1.Add(r2)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	expect := &Resources{
		CPU:      3000,
		MemoryMB: 3072,
		DiskMB:   15000,
		IOPS:     150,
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:          "10.0.0.0/8",
				MBits:         150,
				ReservedPorts: []Port{{"ssh", 22}, {"web", 80}},
			},
		},
	}

	if !reflect.DeepEqual(expect.Networks, r1.Networks) {
		t.Fatalf("bad: %#v %#v", expect, r1)
	}
}

func TestResource_Add_Network(t *testing.T) {
	r1 := &Resources{}
	r2 := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        50,
				DynamicPorts: []Port{{"http", 0}, {"https", 0}},
			},
		},
	}
	r3 := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        25,
				DynamicPorts: []Port{{"admin", 0}},
			},
		},
	}

	err := r1.Add(r2)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	err = r1.Add(r3)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	expect := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        75,
				DynamicPorts: []Port{{"http", 0}, {"https", 0}, {"admin", 0}},
			},
		},
	}

	if !reflect.DeepEqual(expect.Networks, r1.Networks) {
		t.Fatalf("bad: %#v %#v", expect.Networks[0], r1.Networks[0])
	}
}

func TestEncodeDecode(t *testing.T) {
	type FooRequest struct {
		Foo string
		Bar int
		Baz bool
	}
	arg := &FooRequest{
		Foo: "test",
		Bar: 42,
		Baz: true,
	}
	buf, err := Encode(1, arg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out FooRequest
	err = Decode(buf[1:], &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(arg, &out) {
		t.Fatalf("bad: %#v %#v", arg, out)
	}
}

func BenchmarkEncodeDecode(b *testing.B) {
	job := testJob()

	for i := 0; i < b.N; i++ {
		buf, err := Encode(1, job)
		if err != nil {
			b.Fatalf("err: %v", err)
		}

		var out Job
		err = Decode(buf[1:], &out)
		if err != nil {
			b.Fatalf("err: %v", err)
		}
	}
}

func TestInvalidServiceCheck(t *testing.T) {
	s := Service{
		Name:      "service-name",
		PortLabel: "bar",
		Checks: []*ServiceCheck{
			{
				Name: "check-name",
				Type: "lol",
			},
		},
	}
	if err := s.Validate(); err == nil {
		t.Fatalf("Service should be invalid (invalid type)")
	}

	s = Service{
		Name:      "service.name",
		PortLabel: "bar",
	}
	if err := s.ValidateName(s.Name); err == nil {
		t.Fatalf("Service should be invalid (contains a dot): %v", err)
	}

	s = Service{
		Name:      "-my-service",
		PortLabel: "bar",
	}
	if err := s.Validate(); err == nil {
		t.Fatalf("Service should be invalid (begins with a hyphen): %v", err)
	}

	s = Service{
		Name:      "my-service-${NOMAD_META_FOO}",
		PortLabel: "bar",
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Service should be valid: %v", err)
	}

	s = Service{
		Name:      "abcdef0123456789-abcdef0123456789-abcdef0123456789-abcdef0123456",
		PortLabel: "bar",
	}
	if err := s.ValidateName(s.Name); err == nil {
		t.Fatalf("Service should be invalid (too long): %v", err)
	}

	s = Service{
		Name: "service-name",
		Checks: []*ServiceCheck{
			{
				Name:     "check-tcp",
				Type:     ServiceCheckTCP,
				Interval: 5 * time.Second,
				Timeout:  2 * time.Second,
			},
			{
				Name:     "check-http",
				Type:     ServiceCheckHTTP,
				Path:     "/foo",
				Interval: 5 * time.Second,
				Timeout:  2 * time.Second,
			},
		},
	}
	if err := s.Validate(); err == nil {
		t.Fatalf("service should be invalid (tcp/http checks with no port): %v", err)
	}

	s = Service{
		Name: "service-name",
		Checks: []*ServiceCheck{
			{
				Name:     "check-script",
				Type:     ServiceCheckScript,
				Command:  "/bin/date",
				Interval: 5 * time.Second,
				Timeout:  2 * time.Second,
			},
		},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("un-expected error: %v", err)
	}
}

func TestDistinctCheckID(t *testing.T) {
	c1 := ServiceCheck{
		Name:     "web-health",
		Type:     "http",
		Path:     "/health",
		Interval: 2 * time.Second,
		Timeout:  3 * time.Second,
	}
	c2 := ServiceCheck{
		Name:     "web-health",
		Type:     "http",
		Path:     "/health1",
		Interval: 2 * time.Second,
		Timeout:  3 * time.Second,
	}

	c3 := ServiceCheck{
		Name:     "web-health",
		Type:     "http",
		Path:     "/health",
		Interval: 4 * time.Second,
		Timeout:  3 * time.Second,
	}
	serviceID := "123"
	c1Hash := c1.Hash(serviceID)
	c2Hash := c2.Hash(serviceID)
	c3Hash := c3.Hash(serviceID)

	if c1Hash == c2Hash || c1Hash == c3Hash || c3Hash == c2Hash {
		t.Fatalf("Checks need to be uniq c1: %s, c2: %s, c3: %s", c1Hash, c2Hash, c3Hash)
	}

}

func TestService_Canonicalize(t *testing.T) {
	job := "example"
	taskGroup := "cache"
	task := "redis"

	s := Service{
		Name: "${TASK}-db",
	}

	s.Canonicalize(job, taskGroup, task)
	if s.Name != "redis-db" {
		t.Fatalf("Expected name: %v, Actual: %v", "redis-db", s.Name)
	}

	s.Name = "db"
	s.Canonicalize(job, taskGroup, task)
	if s.Name != "db" {
		t.Fatalf("Expected name: %v, Actual: %v", "redis-db", s.Name)
	}

	s.Name = "${JOB}-${TASKGROUP}-${TASK}-db"
	s.Canonicalize(job, taskGroup, task)
	if s.Name != "example-cache-redis-db" {
		t.Fatalf("Expected name: %v, Actual: %v", "expample-cache-redis-db", s.Name)
	}

	s.Name = "${BASE}-db"
	s.Canonicalize(job, taskGroup, task)
	if s.Name != "example-cache-redis-db" {
		t.Fatalf("Expected name: %v, Actual: %v", "expample-cache-redis-db", s.Name)
	}

}

func TestJob_ExpandServiceNames(t *testing.T) {
	j := &Job{
		Name: "my-job",
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name: "web",
				Tasks: []*Task{
					{
						Name: "frontend",
						Services: []*Service{
							{
								Name: "${BASE}-default",
							},
							{
								Name: "jmx",
							},
						},
					},
				},
			},
			&TaskGroup{
				Name: "admin",
				Tasks: []*Task{
					{
						Name: "admin-web",
					},
				},
			},
		},
	}

	j.Canonicalize()

	service1Name := j.TaskGroups[0].Tasks[0].Services[0].Name
	if service1Name != "my-job-web-frontend-default" {
		t.Fatalf("Expected Service Name: %s, Actual: %s", "my-job-web-frontend-default", service1Name)
	}

	service2Name := j.TaskGroups[0].Tasks[0].Services[1].Name
	if service2Name != "jmx" {
		t.Fatalf("Expected Service Name: %s, Actual: %s", "jmx", service2Name)
	}

}

func TestPeriodicConfig_EnabledInvalid(t *testing.T) {
	// Create a config that is enabled but with no interval specified.
	p := &PeriodicConfig{Enabled: true}
	if err := p.Validate(); err == nil {
		t.Fatal("Enabled PeriodicConfig with no spec or type shouldn't be valid")
	}

	// Create a config that is enabled, with a spec but no type specified.
	p = &PeriodicConfig{Enabled: true, Spec: "foo"}
	if err := p.Validate(); err == nil {
		t.Fatal("Enabled PeriodicConfig with no spec type shouldn't be valid")
	}

	// Create a config that is enabled, with a spec type but no spec specified.
	p = &PeriodicConfig{Enabled: true, SpecType: PeriodicSpecCron}
	if err := p.Validate(); err == nil {
		t.Fatal("Enabled PeriodicConfig with no spec shouldn't be valid")
	}

	// Create a config that is enabled, with a bad time zone.
	p = &PeriodicConfig{Enabled: true, TimeZone: "FOO"}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "time zone") {
		t.Fatalf("Enabled PeriodicConfig with bad time zone shouldn't be valid: %v", err)
	}
}

func TestPeriodicConfig_InvalidCron(t *testing.T) {
	specs := []string{"foo", "* *", "@foo"}
	for _, spec := range specs {
		p := &PeriodicConfig{Enabled: true, SpecType: PeriodicSpecCron, Spec: spec}
		p.Canonicalize()
		if err := p.Validate(); err == nil {
			t.Fatal("Invalid cron spec")
		}
	}
}

func TestPeriodicConfig_ValidCron(t *testing.T) {
	specs := []string{"0 0 29 2 *", "@hourly", "0 0-15 * * *"}
	for _, spec := range specs {
		p := &PeriodicConfig{Enabled: true, SpecType: PeriodicSpecCron, Spec: spec}
		p.Canonicalize()
		if err := p.Validate(); err != nil {
			t.Fatal("Passed valid cron")
		}
	}
}

func TestPeriodicConfig_NextCron(t *testing.T) {
	from := time.Date(2009, time.November, 10, 23, 22, 30, 0, time.UTC)
	specs := []string{"0 0 29 2 * 1980", "*/5 * * * *"}
	expected := []time.Time{time.Time{}, time.Date(2009, time.November, 10, 23, 25, 0, 0, time.UTC)}
	for i, spec := range specs {
		p := &PeriodicConfig{Enabled: true, SpecType: PeriodicSpecCron, Spec: spec}
		p.Canonicalize()
		n := p.Next(from)
		if expected[i] != n {
			t.Fatalf("Next(%v) returned %v; want %v", from, n, expected[i])
		}
	}
}

func TestPeriodicConfig_ValidTimeZone(t *testing.T) {
	zones := []string{"Africa/Abidjan", "America/Chicago", "Europe/Minsk", "UTC"}
	for _, zone := range zones {
		p := &PeriodicConfig{Enabled: true, SpecType: PeriodicSpecCron, Spec: "0 0 29 2 * 1980", TimeZone: zone}
		p.Canonicalize()
		if err := p.Validate(); err != nil {
			t.Fatalf("Valid tz errored: %v", err)
		}
	}
}

func TestPeriodicConfig_DST(t *testing.T) {
	// On Sun, Mar 12, 2:00 am 2017: +1 hour UTC
	p := &PeriodicConfig{
		Enabled:  true,
		SpecType: PeriodicSpecCron,
		Spec:     "0 2 11-12 3 * 2017",
		TimeZone: "America/Los_Angeles",
	}
	p.Canonicalize()

	t1 := time.Date(2017, time.March, 11, 1, 0, 0, 0, p.location)
	t2 := time.Date(2017, time.March, 12, 1, 0, 0, 0, p.location)

	// E1 is an 8 hour adjustment, E2 is a 7 hour adjustment
	e1 := time.Date(2017, time.March, 11, 10, 0, 0, 0, time.UTC)
	e2 := time.Date(2017, time.March, 12, 9, 0, 0, 0, time.UTC)

	n1 := p.Next(t1).UTC()
	n2 := p.Next(t2).UTC()

	if !reflect.DeepEqual(e1, n1) {
		t.Fatalf("Got %v; want %v", n1, e1)
	}
	if !reflect.DeepEqual(e2, n2) {
		t.Fatalf("Got %v; want %v", n1, e1)
	}
}

func TestRestartPolicy_Validate(t *testing.T) {
	// Policy with acceptable restart options passes
	p := &RestartPolicy{
		Mode:     RestartPolicyModeFail,
		Attempts: 0,
		Interval: 5 * time.Second,
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Policy with ambiguous restart options fails
	p = &RestartPolicy{
		Mode:     RestartPolicyModeDelay,
		Attempts: 0,
		Interval: 5 * time.Second,
	}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expect ambiguity error, got: %v", err)
	}

	// Bad policy mode fails
	p = &RestartPolicy{
		Mode:     "nope",
		Attempts: 1,
		Interval: 5 * time.Second,
	}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("expect mode error, got: %v", err)
	}

	// Fails when attempts*delay does not fit inside interval
	p = &RestartPolicy{
		Mode:     RestartPolicyModeDelay,
		Attempts: 3,
		Delay:    5 * time.Second,
		Interval: 5 * time.Second,
	}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "can't restart") {
		t.Fatalf("expect restart interval error, got: %v", err)
	}

	// Fails when interval is to small
	p = &RestartPolicy{
		Mode:     RestartPolicyModeDelay,
		Attempts: 3,
		Delay:    5 * time.Second,
		Interval: 2 * time.Second,
	}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "Interval can not be less than") {
		t.Fatalf("expect interval too small error, got: %v", err)
	}
}

func TestAllocation_Index(t *testing.T) {
	a1 := Allocation{
		Name:      "example.cache[1]",
		TaskGroup: "cache",
		JobID:     "example",
		Job: &Job{
			ID:         "example",
			TaskGroups: []*TaskGroup{{Name: "cache"}}},
	}
	e1 := uint(1)
	a2 := a1.Copy()
	a2.Name = "example.cache[713127]"
	e2 := uint(713127)

	if a1.Index() != e1 || a2.Index() != e2 {
		t.Fatalf("Got %d and %d", a1.Index(), a2.Index())
	}
}

func TestTaskArtifact_Validate_Source(t *testing.T) {
	valid := &TaskArtifact{GetterSource: "google.com"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskArtifact_Validate_Dest(t *testing.T) {
	valid := &TaskArtifact{GetterSource: "google.com"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valid.RelativeDest = "local/"
	if err := valid.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valid.RelativeDest = "local/.."
	if err := valid.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	valid.RelativeDest = "local/../../.."
	if err := valid.Validate(); err == nil {
		t.Fatalf("expected error: %v", err)
	}
}

func TestAllocation_ShouldMigrate(t *testing.T) {
	alloc := Allocation{
		TaskGroup: "foo",
		Job: &Job{
			TaskGroups: []*TaskGroup{
				{
					Name: "foo",
					EphemeralDisk: &EphemeralDisk{
						Migrate: true,
						Sticky:  true,
					},
				},
			},
		},
	}

	if !alloc.ShouldMigrate() {
		t.Fatalf("bad: %v", alloc)
	}

	alloc1 := Allocation{
		TaskGroup: "foo",
		Job: &Job{
			TaskGroups: []*TaskGroup{
				{
					Name:          "foo",
					EphemeralDisk: &EphemeralDisk{},
				},
			},
		},
	}

	if alloc1.ShouldMigrate() {
		t.Fatalf("bad: %v", alloc)
	}

	alloc2 := Allocation{
		TaskGroup: "foo",
		Job: &Job{
			TaskGroups: []*TaskGroup{
				{
					Name: "foo",
					EphemeralDisk: &EphemeralDisk{
						Sticky:  false,
						Migrate: true,
					},
				},
			},
		},
	}

	if alloc2.ShouldMigrate() {
		t.Fatalf("bad: %v", alloc)
	}

	alloc3 := Allocation{
		TaskGroup: "foo",
		Job: &Job{
			TaskGroups: []*TaskGroup{
				{
					Name: "foo",
				},
			},
		},
	}

	if alloc3.ShouldMigrate() {
		t.Fatalf("bad: %v", alloc)
	}
}

func TestTaskArtifact_Validate_Checksum(t *testing.T) {
	cases := []struct {
		Input *TaskArtifact
		Err   bool
	}{
		{
			&TaskArtifact{
				GetterSource: "foo.com",
				GetterOptions: map[string]string{
					"checksum": "no-type",
				},
			},
			true,
		},
		{
			&TaskArtifact{
				GetterSource: "foo.com",
				GetterOptions: map[string]string{
					"checksum": "md5:toosmall",
				},
			},
			true,
		},
		{
			&TaskArtifact{
				GetterSource: "foo.com",
				GetterOptions: map[string]string{
					"checksum": "invalid:type",
				},
			},
			true,
		},
	}

	for i, tc := range cases {
		err := tc.Input.Validate()
		if (err != nil) != tc.Err {
			t.Fatalf("case %d: %v", i, err)
			continue
		}
	}
}

func TestAllocation_Terminated(t *testing.T) {
	type desiredState struct {
		ClientStatus  string
		DesiredStatus string
		Terminated    bool
	}

	harness := []desiredState{
		{
			ClientStatus:  AllocClientStatusPending,
			DesiredStatus: AllocDesiredStatusStop,
			Terminated:    false,
		},
		{
			ClientStatus:  AllocClientStatusRunning,
			DesiredStatus: AllocDesiredStatusStop,
			Terminated:    false,
		},
		{
			ClientStatus:  AllocClientStatusFailed,
			DesiredStatus: AllocDesiredStatusStop,
			Terminated:    true,
		},
		{
			ClientStatus:  AllocClientStatusFailed,
			DesiredStatus: AllocDesiredStatusRun,
			Terminated:    true,
		},
	}

	for _, state := range harness {
		alloc := Allocation{}
		alloc.DesiredStatus = state.DesiredStatus
		alloc.ClientStatus = state.ClientStatus
		if alloc.Terminated() != state.Terminated {
			t.Fatalf("expected: %v, actual: %v", state.Terminated, alloc.Terminated())
		}
	}
}

func TestVault_Validate(t *testing.T) {
	v := &Vault{
		Env:        true,
		ChangeMode: VaultChangeModeNoop,
	}

	if err := v.Validate(); err == nil || !strings.Contains(err.Error(), "Policy list") {
		t.Fatalf("Expected policy list empty error")
	}

	v.Policies = []string{"foo", "root"}
	v.ChangeMode = VaultChangeModeSignal

	err := v.Validate()
	if err == nil {
		t.Fatalf("Expected validation errors")
	}

	if !strings.Contains(err.Error(), "Signal must") {
		t.Fatalf("Expected signal empty error")
	}
	if !strings.Contains(err.Error(), "root") {
		t.Fatalf("Expected root error")
	}
}

func TestParameterizedJobConfig_Validate(t *testing.T) {
	d := &ParameterizedJobConfig{
		Payload: "foo",
	}

	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "payload") {
		t.Fatalf("Expected unknown payload requirement: %v", err)
	}

	d.Payload = DispatchPayloadOptional
	d.MetaOptional = []string{"foo", "bar"}
	d.MetaRequired = []string{"bar", "baz"}

	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "disjoint") {
		t.Fatalf("Expected meta not being disjoint error: %v", err)
	}
}

func TestParameterizedJobConfig_Validate_NonBatch(t *testing.T) {
	job := testJob()
	job.ParameterizedJob = &ParameterizedJobConfig{
		Payload: DispatchPayloadOptional,
	}
	job.Type = JobTypeSystem

	if err := job.Validate(); err == nil || !strings.Contains(err.Error(), "only be used with") {
		t.Fatalf("Expected bad scheduler tpye: %v", err)
	}
}

func TestParameterizedJobConfig_Canonicalize(t *testing.T) {
	d := &ParameterizedJobConfig{}
	d.Canonicalize()
	if d.Payload != DispatchPayloadOptional {
		t.Fatalf("Canonicalize failed")
	}
}

func TestDispatchPayloadConfig_Validate(t *testing.T) {
	d := &DispatchPayloadConfig{
		File: "foo",
	}

	// task/local/haha
	if err := d.Validate(); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// task/haha
	d.File = "../haha"
	if err := d.Validate(); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// ../haha
	d.File = "../../../haha"
	if err := d.Validate(); err == nil {
		t.Fatalf("bad: %v", err)
	}
}

func TestIsRecoverable(t *testing.T) {
	if IsRecoverable(nil) {
		t.Errorf("nil should not be recoverable")
	}
	if IsRecoverable(NewRecoverableError(nil, true)) {
		t.Errorf("NewRecoverableError(nil, true) should not be recoverable")
	}
	if IsRecoverable(fmt.Errorf("i promise im recoverable")) {
		t.Errorf("Custom errors should not be recoverable")
	}
	if IsRecoverable(NewRecoverableError(fmt.Errorf(""), false)) {
		t.Errorf("Explicitly unrecoverable errors should not be recoverable")
	}
	if !IsRecoverable(NewRecoverableError(fmt.Errorf(""), true)) {
		t.Errorf("Explicitly recoverable errors *should* be recoverable")
	}
}
