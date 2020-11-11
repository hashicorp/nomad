package structs

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper/uuid"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	if !strings.Contains(mErr.Errors[3].Error(), "namespace") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[4].Error(), "job type") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[5].Error(), "priority") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[6].Error(), "datacenters") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[7].Error(), "task groups") {
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
		ID:          uuid.Generate(),
		Namespace:   "test",
		Name:        "my-job",
		Type:        JobTypeService,
		Priority:    50,
		Datacenters: []string{"dc1"},
		TaskGroups: []*TaskGroup{
			{
				Name: "web",
				RestartPolicy: &RestartPolicy{
					Interval: 5 * time.Minute,
					Delay:    10 * time.Second,
					Attempts: 10,
				},
			},
			{
				Name: "web",
				RestartPolicy: &RestartPolicy{
					Interval: 5 * time.Minute,
					Delay:    10 * time.Second,
					Attempts: 10,
				},
			},
			{
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

	// test for empty datacenters
	j = &Job{
		Datacenters: []string{""},
	}
	err = j.Validate()
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Error(), "datacenter must be non-empty string") {
		t.Fatalf("err: %s", err)
	}
}

func TestJob_ValidateScaling(t *testing.T) {
	require := require.New(t)

	p := &ScalingPolicy{
		Policy:  nil, // allowed to be nil
		Type:    ScalingPolicyTypeHorizontal,
		Min:     5,
		Max:     5,
		Enabled: true,
	}
	job := testJob()
	job.TaskGroups[0].Scaling = p
	job.TaskGroups[0].Count = 5

	require.NoError(job.Validate())

	// min <= max
	p.Max = 0
	p.Min = 10
	err := job.Validate()
	require.Error(err)
	mErr := err.(*multierror.Error)
	require.Len(mErr.Errors, 1)
	require.Contains(mErr.Errors[0].Error(), "task group count must not be less than minimum count in scaling policy")
	require.Contains(mErr.Errors[0].Error(), "task group count must not be greater than maximum count in scaling policy")

	// count <= max
	p.Max = 0
	p.Min = 5
	job.TaskGroups[0].Count = 5
	err = job.Validate()
	require.Error(err)
	mErr = err.(*multierror.Error)
	require.Len(mErr.Errors, 1)
	require.Contains(mErr.Errors[0].Error(), "task group count must not be greater than maximum count in scaling policy")

	// min <= count
	job.TaskGroups[0].Count = 0
	p.Min = 5
	p.Max = 5
	err = job.Validate()
	require.Error(err)
	mErr = err.(*multierror.Error)
	require.Len(mErr.Errors, 1)
	require.Contains(mErr.Errors[0].Error(), "task group count must not be less than minimum count in scaling policy")
}

func TestJob_ValidateNullChar(t *testing.T) {
	assert := assert.New(t)

	// job id should not allow null characters
	job := testJob()
	job.ID = "id_with\000null_character"
	assert.Error(job.Validate(), "null character in job ID should not validate")

	// job name should not allow null characters
	job.ID = "happy_little_job_id"
	job.Name = "my job name with \000 characters"
	assert.Error(job.Validate(), "null character in job name should not validate")

	// task group name should not allow null characters
	job.Name = "my job"
	job.TaskGroups[0].Name = "oh_no_another_\000_char"
	assert.Error(job.Validate(), "null character in task group name should not validate")

	// task name should not allow null characters
	job.TaskGroups[0].Name = "so_much_better"
	job.TaskGroups[0].Tasks[0].Name = "ive_had_it_with_these_\000_chars_in_these_names"
	assert.Error(job.Validate(), "null character in task name should not validate")
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
		{
			Name:     "AutoPromote mixed TaskGroups",
			Expected: []string{"auto_promote must be true for all groups"},
			Job: &Job{
				Type: JobTypeService,
				TaskGroups: []*TaskGroup{
					{
						Update: &UpdateStrategy{
							AutoPromote: true,
						},
					},
					{
						Update: &UpdateStrategy{
							AutoPromote: false,
						},
					},
				},
			},
		},
		{
			Name:     "Template.VaultGrace Deprecated",
			Expected: []string{"VaultGrace has been deprecated as of Nomad 0.11 and ignored since Vault 0.5. Please remove VaultGrace / vault_grace from template stanza."},
			Job: &Job{
				Type: JobTypeService,
				TaskGroups: []*TaskGroup{
					{
						Tasks: []*Task{
							{
								Templates: []*Template{
									{
										VaultGrace: 1,
									},
								},
							},
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
		ID:          uuid.Generate(),
		Namespace:   "test",
		Name:        "my-job",
		Type:        JobTypeService,
		Priority:    50,
		AllAtOnce:   false,
		Datacenters: []string{"dc1"},
		Constraints: []*Constraint{
			{
				LTarget: "$attr.kernel.name",
				RTarget: "linux",
				Operand: "=",
			},
		},
		Periodic: &PeriodicConfig{
			Enabled: false,
		},
		TaskGroups: []*TaskGroup{
			{
				Name:          "web",
				Count:         10,
				EphemeralDisk: DefaultEphemeralDisk(),
				RestartPolicy: &RestartPolicy{
					Mode:     RestartPolicyModeFail,
					Attempts: 3,
					Interval: 10 * time.Minute,
					Delay:    1 * time.Minute,
				},
				ReschedulePolicy: &ReschedulePolicy{
					Interval:      5 * time.Minute,
					Attempts:      10,
					Delay:         5 * time.Second,
					DelayFunction: "constant",
				},
				Tasks: []*Task{
					{
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
								{
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

func TestJob_IsPeriodicActive(t *testing.T) {
	cases := []struct {
		job    *Job
		active bool
	}{
		{
			job: &Job{
				Type: JobTypeService,
				Periodic: &PeriodicConfig{
					Enabled: true,
				},
			},
			active: true,
		},
		{
			job: &Job{
				Type: JobTypeService,
				Periodic: &PeriodicConfig{
					Enabled: false,
				},
			},
			active: false,
		},
		{
			job: &Job{
				Type: JobTypeService,
				Periodic: &PeriodicConfig{
					Enabled: true,
				},
				Stop: true,
			},
			active: false,
		},
		{
			job: &Job{
				Type: JobTypeService,
				Periodic: &PeriodicConfig{
					Enabled: false,
				},
				ParameterizedJob: &ParameterizedJobConfig{},
			},
			active: false,
		},
	}

	for i, c := range cases {
		if act := c.job.IsPeriodicActive(); act != c.active {
			t.Fatalf("case %d failed: got %v; want %v", i, act, c.active)
		}
	}
}

func TestJob_SystemJob_Validate(t *testing.T) {
	j := testJob()
	j.Type = JobTypeSystem
	j.TaskGroups[0].ReschedulePolicy = nil
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

	// Add affinities at job, task group and task level, that should fail validation

	j.Affinities = []*Affinity{{
		Operand: "=",
		LTarget: "${node.datacenter}",
		RTarget: "dc1",
	}}
	j.TaskGroups[0].Affinities = []*Affinity{{
		Operand: "=",
		LTarget: "${meta.rack}",
		RTarget: "r1",
	}}
	j.TaskGroups[0].Tasks[0].Affinities = []*Affinity{{
		Operand: "=",
		LTarget: "${meta.rack}",
		RTarget: "r1",
	}}
	err = j.Validate()
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "System jobs may not have an affinity stanza")

	// Add spread at job and task group level, that should fail validation
	j.Spreads = []*Spread{{
		Attribute: "${node.datacenter}",
		Weight:    100,
	}}
	j.TaskGroups[0].Spreads = []*Spread{{
		Attribute: "${node.datacenter}",
		Weight:    100,
	}}

	err = j.Validate()
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "System jobs may not have a spread stanza")

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
			{
				Name: "foo",
				Tasks: []*Task{
					{
						Name: "t1",
					},
					{
						Name:  "t2",
						Vault: vj1,
					},
				},
			},
			{
				Name: "bar",
				Tasks: []*Task{
					{
						Name:  "t3",
						Vault: vj2,
					},
					{
						Name:  "t4",
						Vault: vj3,
					},
				},
			},
		},
	}

	e1 := map[string]map[string]*Vault{
		"foo": {
			"t2": vj1,
		},
		"bar": {
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

func TestJob_ConnectTasks(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	j0 := &Job{
		TaskGroups: []*TaskGroup{{
			Name: "tg1",
			Tasks: []*Task{{
				Name: "connect-proxy-task1",
				Kind: "connect-proxy:task1",
			}, {
				Name: "task2",
				Kind: "task2",
			}, {
				Name: "connect-proxy-task3",
				Kind: "connect-proxy:task3",
			}},
		}, {
			Name: "tg2",
			Tasks: []*Task{{
				Name: "task1",
				Kind: "task1",
			}, {
				Name: "connect-proxy-task2",
				Kind: "connect-proxy:task2",
			}},
		}, {
			Name: "tg3",
			Tasks: []*Task{{
				Name: "ingress",
				Kind: "connect-ingress:ingress",
			}},
		}, {
			Name: "tg4",
			Tasks: []*Task{{
				Name: "frontend",
				Kind: "connect-native:uuid-fe",
			}, {
				Name: "generator",
				Kind: "connect-native:uuid-api",
			}},
		}},
	}

	connectTasks := j0.ConnectTasks()

	exp := []TaskKind{
		NewTaskKind(ConnectProxyPrefix, "task1"),
		NewTaskKind(ConnectProxyPrefix, "task3"),
		NewTaskKind(ConnectProxyPrefix, "task2"),
		NewTaskKind(ConnectIngressPrefix, "ingress"),
		NewTaskKind(ConnectNativePrefix, "uuid-fe"),
		NewTaskKind(ConnectNativePrefix, "uuid-api"),
	}

	r.Equal(exp, connectTasks)
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
			{
				Name: "foo",
				Tasks: []*Task{
					{
						Name: "t1",
					},
					{
						Name:      "t2",
						Vault:     vj2,
						Templates: []*Template{tj2},
					},
				},
			},
			{
				Name: "bar",
				Tasks: []*Task{
					{
						Name:      "t3",
						Vault:     vj1,
						Templates: []*Template{tj1},
					},
					{
						Name:  "t4",
						Vault: vj2,
					},
				},
			},
		},
	}

	e1 := map[string]map[string][]string{
		"foo": {
			"t2": {"SIGUSR1", "SIGUSR2"},
		},
		"bar": {
			"t4": {"SIGUSR1"},
		},
	}

	j2 := &Job{
		TaskGroups: []*TaskGroup{
			{
				Name: "foo",
				Tasks: []*Task{
					{
						Name:       "t1",
						KillSignal: "SIGQUIT",
					},
				},
			},
		},
	}

	e2 := map[string]map[string][]string{
		"foo": {
			"t1": {"SIGQUIT"},
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
		{
			Job:      j2,
			Expected: e2,
		},
	}

	for i, c := range cases {
		got := c.Job.RequiredSignals()
		if !reflect.DeepEqual(got, c.Expected) {
			t.Fatalf("case %d: got %#v; want %#v", i+1, got, c.Expected)
		}
	}
}

// test new Equal comparisons for components of Jobs
func TestJob_PartEqual(t *testing.T) {
	ns := &Networks{}
	require.True(t, ns.Equals(&Networks{}))

	ns = &Networks{
		&NetworkResource{Device: "eth0"},
	}
	require.True(t, ns.Equals(&Networks{
		&NetworkResource{Device: "eth0"},
	}))

	ns = &Networks{
		&NetworkResource{Device: "eth0"},
		&NetworkResource{Device: "eth1"},
		&NetworkResource{Device: "eth2"},
	}
	require.True(t, ns.Equals(&Networks{
		&NetworkResource{Device: "eth2"},
		&NetworkResource{Device: "eth0"},
		&NetworkResource{Device: "eth1"},
	}))

	cs := &Constraints{
		&Constraint{"left0", "right0", "=", ""},
		&Constraint{"left1", "right1", "=", ""},
		&Constraint{"left2", "right2", "=", ""},
	}
	require.True(t, cs.Equals(&Constraints{
		&Constraint{"left0", "right0", "=", ""},
		&Constraint{"left2", "right2", "=", ""},
		&Constraint{"left1", "right1", "=", ""},
	}))

	as := &Affinities{
		&Affinity{"left0", "right0", "=", 0, ""},
		&Affinity{"left1", "right1", "=", 0, ""},
		&Affinity{"left2", "right2", "=", 0, ""},
	}
	require.True(t, as.Equals(&Affinities{
		&Affinity{"left0", "right0", "=", 0, ""},
		&Affinity{"left2", "right2", "=", 0, ""},
		&Affinity{"left1", "right1", "=", 0, ""},
	}))
}

func TestTask_UsesConnect(t *testing.T) {
	t.Parallel()

	t.Run("normal task", func(t *testing.T) {
		task := testJob().TaskGroups[0].Tasks[0]
		usesConnect := task.UsesConnect()
		require.False(t, usesConnect)
	})

	t.Run("sidecar proxy", func(t *testing.T) {
		task := &Task{
			Name: "connect-proxy-task1",
			Kind: NewTaskKind(ConnectProxyPrefix, "task1"),
		}
		usesConnect := task.UsesConnect()
		require.True(t, usesConnect)
	})

	t.Run("native task", func(t *testing.T) {
		task := &Task{
			Name: "task1",
			Kind: NewTaskKind(ConnectNativePrefix, "task1"),
		}
		usesConnect := task.UsesConnect()
		require.True(t, usesConnect)
	})

	t.Run("ingress gateway", func(t *testing.T) {
		task := &Task{
			Name: "task1",
			Kind: NewTaskKind(ConnectIngressPrefix, "task1"),
		}
		usesConnect := task.UsesConnect()
		require.True(t, usesConnect)
	})
}

func TestTaskGroup_UsesConnect(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, tg *TaskGroup, exp bool) {
		result := tg.UsesConnect()
		require.Equal(t, exp, result)
	}

	t.Run("tg uses native", func(t *testing.T) {
		try(t, &TaskGroup{
			Services: []*Service{
				{Connect: nil},
				{Connect: &ConsulConnect{Native: true}},
			},
		}, true)
	})

	t.Run("tg uses sidecar", func(t *testing.T) {
		try(t, &TaskGroup{
			Services: []*Service{{
				Connect: &ConsulConnect{
					SidecarService: &ConsulSidecarService{
						Port: "9090",
					},
				},
			}},
		}, true)
	})

	t.Run("tg uses gateway", func(t *testing.T) {
		try(t, &TaskGroup{
			Services: []*Service{{
				Connect: &ConsulConnect{
					Gateway: consulIngressGateway1,
				},
			}},
		}, true)
	})

	t.Run("tg does not use connect", func(t *testing.T) {
		try(t, &TaskGroup{
			Services: []*Service{
				{Connect: nil},
			},
		}, false)
	})
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
		ReschedulePolicy: &ReschedulePolicy{
			Interval: 5 * time.Minute,
			Attempts: 5,
			Delay:    5 * time.Second,
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
			{
				Name: "task-a",
				Resources: &Resources{
					Networks: []*NetworkResource{
						{
							ReservedPorts: []Port{{Label: "foo", Value: 123}},
						},
					},
				},
			},
			{
				Name: "task-b",
				Resources: &Resources{
					Networks: []*NetworkResource{
						{
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
			{
				Name: "task-a",
				Resources: &Resources{
					Networks: []*NetworkResource{
						{
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
			{Name: "web", Leader: true},
			{Name: "web", Leader: true},
			{},
		},
		RestartPolicy: &RestartPolicy{
			Interval: 5 * time.Minute,
			Delay:    10 * time.Second,
			Attempts: 10,
			Mode:     RestartPolicyModeDelay,
		},
		ReschedulePolicy: &ReschedulePolicy{
			Interval:      5 * time.Minute,
			Attempts:      10,
			Delay:         5 * time.Second,
			DelayFunction: "constant",
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

	tg = &TaskGroup{
		Name:  "web",
		Count: 1,
		Tasks: []*Task{
			{Name: "web", Leader: true},
		},
		Update: DefaultUpdateStrategy.Copy(),
	}
	j.Type = JobTypeBatch
	err = tg.Validate(j)
	if !strings.Contains(err.Error(), "does not allow update block") {
		t.Fatalf("err: %s", err)
	}

	tg = &TaskGroup{
		Count: -1,
		RestartPolicy: &RestartPolicy{
			Interval: 5 * time.Minute,
			Delay:    10 * time.Second,
			Attempts: 10,
			Mode:     RestartPolicyModeDelay,
		},
		ReschedulePolicy: &ReschedulePolicy{
			Interval: 5 * time.Minute,
			Attempts: 5,
			Delay:    5 * time.Second,
		},
	}
	j.Type = JobTypeSystem
	err = tg.Validate(j)
	if !strings.Contains(err.Error(), "System jobs should not have a reschedule policy") {
		t.Fatalf("err: %s", err)
	}

	tg = &TaskGroup{
		Networks: []*NetworkResource{
			{
				DynamicPorts: []Port{{"http", 0, 80, ""}},
			},
		},
		Tasks: []*Task{
			{
				Resources: &Resources{
					Networks: []*NetworkResource{
						{
							DynamicPorts: []Port{{"http", 0, 80, ""}},
						},
					},
				},
			},
		},
	}
	err = tg.Validate(j)
	require.Contains(t, err.Error(), "Port label http already in use")

	tg = &TaskGroup{
		Volumes: map[string]*VolumeRequest{
			"foo": {
				Type:   "nothost",
				Source: "foo",
			},
		},
		Tasks: []*Task{
			{
				Name:      "task-a",
				Resources: &Resources{},
			},
		},
	}
	err = tg.Validate(&Job{})
	require.Contains(t, err.Error(), `Volume foo has unrecognised type nothost`)

	tg = &TaskGroup{
		Volumes: map[string]*VolumeRequest{
			"foo": {
				Type: "host",
			},
		},
		Tasks: []*Task{
			{
				Name:      "task-a",
				Resources: &Resources{},
			},
		},
	}
	err = tg.Validate(&Job{})
	require.Contains(t, err.Error(), `Volume foo has an empty source`)

	tg = &TaskGroup{
		Volumes: map[string]*VolumeRequest{
			"foo": {
				Type: "host",
			},
		},
		Tasks: []*Task{
			{
				Name:      "task-a",
				Resources: &Resources{},
				VolumeMounts: []*VolumeMount{
					{
						Volume: "",
					},
				},
			},
			{
				Name:      "task-b",
				Resources: &Resources{},
				VolumeMounts: []*VolumeMount{
					{
						Volume: "foob",
					},
				},
			},
		},
	}
	err = tg.Validate(&Job{})
	expected = `Task task-a has a volume mount (0) referencing an empty volume`
	require.Contains(t, err.Error(), expected)

	expected = `Task task-b has a volume mount (0) referencing undefined volume foob`
	require.Contains(t, err.Error(), expected)

	taskA := &Task{Name: "task-a"}
	tg = &TaskGroup{
		Name: "group-a",
		Services: []*Service{
			{
				Name: "service-a",
				Checks: []*ServiceCheck{
					{
						Name:      "check-a",
						Type:      "tcp",
						TaskName:  "task-b",
						PortLabel: "http",
						Interval:  time.Duration(1 * time.Second),
						Timeout:   time.Duration(1 * time.Second),
					},
				},
			},
		},
		Tasks: []*Task{taskA},
	}
	err = tg.Validate(&Job{})
	expected = `Check check-a invalid: refers to non-existent task task-b`
	require.Contains(t, err.Error(), expected)

	expected = `Check check-a invalid: only script and gRPC checks should have tasks`
	require.Contains(t, err.Error(), expected)

}

func TestTaskGroupNetwork_Validate(t *testing.T) {
	cases := []struct {
		TG          *TaskGroup
		ErrContains string
	}{
		{
			TG: &TaskGroup{
				Name: "group-static-value-ok",
				Networks: Networks{
					&NetworkResource{
						ReservedPorts: []Port{
							{
								Label: "ok",
								Value: 65535,
							},
						},
					},
				},
			},
		},
		{
			TG: &TaskGroup{
				Name: "group-dynamic-value-ok",
				Networks: Networks{
					&NetworkResource{
						DynamicPorts: []Port{
							{
								Label: "ok",
								Value: 65535,
							},
						},
					},
				},
			},
		},
		{
			TG: &TaskGroup{
				Name: "group-static-to-ok",
				Networks: Networks{
					&NetworkResource{
						ReservedPorts: []Port{
							{
								Label: "ok",
								To:    65535,
							},
						},
					},
				},
			},
		},
		{
			TG: &TaskGroup{
				Name: "group-dynamic-to-ok",
				Networks: Networks{
					&NetworkResource{
						DynamicPorts: []Port{
							{
								Label: "ok",
								To:    65535,
							},
						},
					},
				},
			},
		},
		{
			TG: &TaskGroup{
				Name: "group-static-value-too-high",
				Networks: Networks{
					&NetworkResource{
						ReservedPorts: []Port{
							{
								Label: "too-high",
								Value: 65536,
							},
						},
					},
				},
			},
			ErrContains: "greater than",
		},
		{
			TG: &TaskGroup{
				Name: "group-dynamic-value-too-high",
				Networks: Networks{
					&NetworkResource{
						DynamicPorts: []Port{
							{
								Label: "too-high",
								Value: 65536,
							},
						},
					},
				},
			},
			ErrContains: "greater than",
		},
		{
			TG: &TaskGroup{
				Name: "group-static-to-too-high",
				Networks: Networks{
					&NetworkResource{
						ReservedPorts: []Port{
							{
								Label: "too-high",
								To:    65536,
							},
						},
					},
				},
			},
			ErrContains: "greater than",
		},
		{
			TG: &TaskGroup{
				Name: "group-dynamic-to-too-high",
				Networks: Networks{
					&NetworkResource{
						DynamicPorts: []Port{
							{
								Label: "too-high",
								To:    65536,
							},
						},
					},
				},
			},
			ErrContains: "greater than",
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.TG.Name, func(t *testing.T) {
			err := tc.TG.validateNetworks()
			t.Logf("%s -> %v", tc.TG.Name, err)
			if tc.ErrContains == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.ErrContains)
		})
	}
}

func TestTask_Validate(t *testing.T) {
	task := &Task{}
	ephemeralDisk := DefaultEphemeralDisk()
	err := task.Validate(ephemeralDisk, JobTypeBatch, nil)
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
	err = task.Validate(ephemeralDisk, JobTypeBatch, nil)
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
		},
		LogConfig: DefaultLogConfig(),
	}
	ephemeralDisk.SizeMB = 200
	err = task.Validate(ephemeralDisk, JobTypeBatch, nil)
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

	err = task.Validate(ephemeralDisk, JobTypeBatch, nil)
	mErr = err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "task level: distinct_hosts") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "task level: distinct_property") {
		t.Fatalf("err: %s", err)
	}
}

func TestTask_Validate_Resources(t *testing.T) {
	cases := []struct {
		name string
		res  *Resources
	}{
		{
			name: "Minimum",
			res:  MinResources(),
		},
		{
			name: "Default",
			res:  DefaultResources(),
		},
		{
			name: "Full",
			res: &Resources{
				CPU:      1000,
				MemoryMB: 1000,
				IOPS:     1000,
				Networks: []*NetworkResource{
					{
						Mode:   "host",
						Device: "localhost",
						CIDR:   "127.0.0.0/8",
						IP:     "127.0.0.1",
						MBits:  1000,
						DNS: &DNSConfig{
							Servers:  []string{"localhost"},
							Searches: []string{"localdomain"},
							Options:  []string{"ndots:5"},
						},
						ReservedPorts: []Port{
							{
								Label:       "reserved",
								Value:       1234,
								To:          1234,
								HostNetwork: "loopback",
							},
						},
						DynamicPorts: []Port{
							{
								Label:       "dynamic",
								Value:       5678,
								To:          5678,
								HostNetwork: "loopback",
							},
						},
					},
				},
			},
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.res.Validate())
		})
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
		{
			MBits: 10,
			DynamicPorts: []Port{
				{
					Label: "a",
					Value: 1000,
				},
				{
					Label: "b",
					Value: 2000,
				},
			},
		},
	}

	err := task.Validate(ephemeralDisk, JobTypeService, nil)
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

	if err = task1.Validate(ephemeralDisk, JobTypeService, nil); err != nil {
		t.Fatalf("err : %v", err)
	}
}

func TestTask_Validate_Service_AddressMode_Ok(t *testing.T) {
	ephemeralDisk := DefaultEphemeralDisk()
	getTask := func(s *Service) *Task {
		task := &Task{
			Name:      "web",
			Driver:    "docker",
			Resources: DefaultResources(),
			Services:  []*Service{s},
			LogConfig: DefaultLogConfig(),
		}
		task.Resources.Networks = []*NetworkResource{
			{
				MBits: 10,
				DynamicPorts: []Port{
					{
						Label: "http",
						Value: 80,
					},
				},
			},
		}
		return task
	}

	cases := []*Service{
		{
			// https://github.com/hashicorp/nomad/issues/3681#issuecomment-357274177
			Name:        "DriverModeWithLabel",
			PortLabel:   "http",
			AddressMode: AddressModeDriver,
		},
		{
			Name:        "DriverModeWithPort",
			PortLabel:   "80",
			AddressMode: AddressModeDriver,
		},
		{
			Name:        "HostModeWithLabel",
			PortLabel:   "http",
			AddressMode: AddressModeHost,
		},
		{
			Name:        "HostModeWithoutLabel",
			AddressMode: AddressModeHost,
		},
		{
			Name:        "DriverModeWithoutLabel",
			AddressMode: AddressModeDriver,
		},
	}

	for _, service := range cases {
		task := getTask(service)
		t.Run(service.Name, func(t *testing.T) {
			if err := task.Validate(ephemeralDisk, JobTypeService, nil); err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
		})
	}
}

func TestTask_Validate_Service_AddressMode_Bad(t *testing.T) {
	ephemeralDisk := DefaultEphemeralDisk()
	getTask := func(s *Service) *Task {
		task := &Task{
			Name:      "web",
			Driver:    "docker",
			Resources: DefaultResources(),
			Services:  []*Service{s},
			LogConfig: DefaultLogConfig(),
		}
		task.Resources.Networks = []*NetworkResource{
			{
				MBits: 10,
				DynamicPorts: []Port{
					{
						Label: "http",
						Value: 80,
					},
				},
			},
		}
		return task
	}

	cases := []*Service{
		{
			// https://github.com/hashicorp/nomad/issues/3681#issuecomment-357274177
			Name:        "DriverModeWithLabel",
			PortLabel:   "asdf",
			AddressMode: AddressModeDriver,
		},
		{
			Name:        "HostModeWithLabel",
			PortLabel:   "asdf",
			AddressMode: AddressModeHost,
		},
		{
			Name:        "HostModeWithPort",
			PortLabel:   "80",
			AddressMode: AddressModeHost,
		},
	}

	for _, service := range cases {
		task := getTask(service)
		t.Run(service.Name, func(t *testing.T) {
			err := task.Validate(ephemeralDisk, JobTypeService, nil)
			if err == nil {
				t.Fatalf("expected an error")
			}
			//t.Logf("err: %v", err)
		})
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

	check2 := ServiceCheck{
		Name:     "check-name-2",
		Type:     ServiceCheckHTTP,
		Interval: 10 * time.Second,
		Timeout:  2 * time.Second,
		Path:     "/foo/bar",
	}

	err = check2.validate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	check2.Path = ""
	err = check2.validate()
	if err == nil {
		t.Fatal("Expected an error")
	}
	if !strings.Contains(err.Error(), "valid http path") {
		t.Fatalf("err: %v", err)
	}

	check2.Path = "http://www.example.com"
	err = check2.validate()
	if err == nil {
		t.Fatal("Expected an error")
	}
	if !strings.Contains(err.Error(), "relative http path") {
		t.Fatalf("err: %v", err)
	}

	t.Run("check expose", func(t *testing.T) {
		t.Run("type http", func(t *testing.T) {
			require.NoError(t, (&ServiceCheck{
				Type:     ServiceCheckHTTP,
				Interval: 1 * time.Second,
				Timeout:  1 * time.Second,
				Path:     "/health",
				Expose:   true,
			}).validate())
		})
		t.Run("type tcp", func(t *testing.T) {
			require.EqualError(t, (&ServiceCheck{
				Type:     ServiceCheckTCP,
				Interval: 1 * time.Second,
				Timeout:  1 * time.Second,
				Expose:   true,
			}).validate(), "expose may only be set on HTTP or gRPC checks")
		})
	})
}

// TestTask_Validate_Service_Check_AddressMode asserts that checks do not
// inherit address mode but do inherit ports.
func TestTask_Validate_Service_Check_AddressMode(t *testing.T) {
	getTask := func(s *Service) *Task {
		return &Task{
			Resources: &Resources{
				Networks: []*NetworkResource{
					{
						DynamicPorts: []Port{
							{
								Label: "http",
								Value: 9999,
							},
						},
					},
				},
			},
			Services: []*Service{s},
		}
	}

	cases := []struct {
		Service     *Service
		ErrContains string
	}{
		{
			Service: &Service{
				Name:        "invalid-driver",
				PortLabel:   "80",
				AddressMode: "host",
			},
			ErrContains: `port label "80" referenced`,
		},
		{
			Service: &Service{
				Name:        "http-driver-fail-1",
				PortLabel:   "80",
				AddressMode: "driver",
				Checks: []*ServiceCheck{
					{
						Name:     "invalid-check-1",
						Type:     "tcp",
						Interval: time.Second,
						Timeout:  time.Second,
					},
				},
			},
			ErrContains: `check "invalid-check-1" cannot use a numeric port`,
		},
		{
			Service: &Service{
				Name:        "http-driver-fail-2",
				PortLabel:   "80",
				AddressMode: "driver",
				Checks: []*ServiceCheck{
					{
						Name:      "invalid-check-2",
						Type:      "tcp",
						PortLabel: "80",
						Interval:  time.Second,
						Timeout:   time.Second,
					},
				},
			},
			ErrContains: `check "invalid-check-2" cannot use a numeric port`,
		},
		{
			Service: &Service{
				Name:        "http-driver-fail-3",
				PortLabel:   "80",
				AddressMode: "driver",
				Checks: []*ServiceCheck{
					{
						Name:      "invalid-check-3",
						Type:      "tcp",
						PortLabel: "missing-port-label",
						Interval:  time.Second,
						Timeout:   time.Second,
					},
				},
			},
			ErrContains: `port label "missing-port-label" referenced`,
		},
		{
			Service: &Service{
				Name:        "http-driver-passes",
				PortLabel:   "80",
				AddressMode: "driver",
				Checks: []*ServiceCheck{
					{
						Name:     "valid-script-check",
						Type:     "script",
						Command:  "ok",
						Interval: time.Second,
						Timeout:  time.Second,
					},
					{
						Name:      "valid-host-check",
						Type:      "tcp",
						PortLabel: "http",
						Interval:  time.Second,
						Timeout:   time.Second,
					},
					{
						Name:        "valid-driver-check",
						Type:        "tcp",
						AddressMode: "driver",
						Interval:    time.Second,
						Timeout:     time.Second,
					},
				},
			},
		},
		{
			Service: &Service{
				Name: "empty-address-3673-passes-1",
				Checks: []*ServiceCheck{
					{
						Name:      "valid-port-label",
						Type:      "tcp",
						PortLabel: "http",
						Interval:  time.Second,
						Timeout:   time.Second,
					},
					{
						Name:     "empty-is-ok",
						Type:     "script",
						Command:  "ok",
						Interval: time.Second,
						Timeout:  time.Second,
					},
				},
			},
		},
		{
			Service: &Service{
				Name: "empty-address-3673-passes-2",
			},
		},
		{
			Service: &Service{
				Name: "empty-address-3673-fails",
				Checks: []*ServiceCheck{
					{
						Name:     "empty-is-not-ok",
						Type:     "tcp",
						Interval: time.Second,
						Timeout:  time.Second,
					},
				},
			},
			ErrContains: `invalid: check requires a port but neither check nor service`,
		},
	}

	for _, tc := range cases {
		tc := tc
		task := getTask(tc.Service)
		t.Run(tc.Service.Name, func(t *testing.T) {
			err := validateServices(task)
			if err == nil && tc.ErrContains == "" {
				// Ok!
				return
			}
			if err == nil {
				t.Fatalf("no error returned. expected: %s", tc.ErrContains)
			}
			if !strings.Contains(err.Error(), tc.ErrContains) {
				t.Fatalf("expected %q but found: %v", tc.ErrContains, err)
			}
		})
	}
}

func TestTask_Validate_Service_Check_GRPC(t *testing.T) {
	t.Parallel()
	// Bad (no port)
	invalidGRPC := &ServiceCheck{
		Type:     ServiceCheckGRPC,
		Interval: time.Second,
		Timeout:  time.Second,
	}
	service := &Service{
		Name:   "test",
		Checks: []*ServiceCheck{invalidGRPC},
	}

	assert.Error(t, service.Validate())

	// Good
	service.Checks[0] = &ServiceCheck{
		Type:      ServiceCheckGRPC,
		Interval:  time.Second,
		Timeout:   time.Second,
		PortLabel: "some-port-label",
	}

	assert.NoError(t, service.Validate())
}

func TestTask_Validate_Service_Check_CheckRestart(t *testing.T) {
	t.Parallel()
	invalidCheckRestart := &CheckRestart{
		Limit: -1,
		Grace: -1,
	}

	err := invalidCheckRestart.Validate()
	assert.NotNil(t, err, "invalidateCheckRestart.Validate()")
	assert.Len(t, err.(*multierror.Error).Errors, 2)

	validCheckRestart := &CheckRestart{}
	assert.Nil(t, validCheckRestart.Validate())

	validCheckRestart.Limit = 1
	validCheckRestart.Grace = 1
	assert.Nil(t, validCheckRestart.Validate())
}

func TestTask_Validate_ConnectProxyKind(t *testing.T) {
	ephemeralDisk := DefaultEphemeralDisk()
	getTask := func(kind TaskKind, leader bool) *Task {
		task := &Task{
			Name:      "web",
			Driver:    "docker",
			Resources: DefaultResources(),
			LogConfig: DefaultLogConfig(),
			Kind:      kind,
			Leader:    leader,
		}
		task.Resources.Networks = []*NetworkResource{
			{
				MBits: 10,
				DynamicPorts: []Port{
					{
						Label: "http",
						Value: 80,
					},
				},
			},
		}
		return task
	}

	cases := []struct {
		Desc        string
		Kind        TaskKind
		Leader      bool
		Service     *Service
		TgService   []*Service
		ErrContains string
	}{
		{
			Desc: "Not connect",
			Kind: "test",
		},
		{
			Desc: "Invalid because of service in task definition",
			Kind: "connect-proxy:redis",
			Service: &Service{
				Name: "redis",
			},
			ErrContains: "Connect proxy task must not have a service stanza",
		},
		{
			Desc:   "Leader should not be set",
			Kind:   "connect-proxy:redis",
			Leader: true,
			Service: &Service{
				Name: "redis",
			},
			ErrContains: "Connect proxy task must not have leader set",
		},
		{
			Desc: "Service name invalid",
			Kind: "connect-proxy:redis:test",
			Service: &Service{
				Name: "redis",
			},
			ErrContains: `No Connect services in task group with Connect proxy ("redis:test")`,
		},
		{
			Desc:        "Service name not found in group",
			Kind:        "connect-proxy:redis",
			ErrContains: `No Connect services in task group with Connect proxy ("redis")`,
		},
		{
			Desc: "Connect stanza not configured in group",
			Kind: "connect-proxy:redis",
			TgService: []*Service{{
				Name: "redis",
			}},
			ErrContains: `No Connect services in task group with Connect proxy ("redis")`,
		},
		{
			Desc: "Valid connect proxy kind",
			Kind: "connect-proxy:redis",
			TgService: []*Service{{
				Name: "redis",
				Connect: &ConsulConnect{
					SidecarService: &ConsulSidecarService{
						Port: "db",
					},
				},
			}},
		},
	}

	for _, tc := range cases {
		tc := tc
		task := getTask(tc.Kind, tc.Leader)
		if tc.Service != nil {
			task.Services = []*Service{tc.Service}
		}
		t.Run(tc.Desc, func(t *testing.T) {
			err := task.Validate(ephemeralDisk, "service", tc.TgService)
			if err == nil && tc.ErrContains == "" {
				// Ok!
				return
			}
			require.Errorf(t, err, "no error returned. expected: %s", tc.ErrContains)
			require.Containsf(t, err.Error(), tc.ErrContains, "expected %q but found: %v", tc.ErrContains, err)
		})
	}

}
func TestTask_Validate_LogConfig(t *testing.T) {
	task := &Task{
		LogConfig: DefaultLogConfig(),
	}
	ephemeralDisk := &EphemeralDisk{
		SizeMB: 1,
	}

	err := task.Validate(ephemeralDisk, JobTypeService, nil)
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[3].Error(), "log storage") {
		t.Fatalf("err: %s", err)
	}
}

func TestLogConfig_Equals(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		a := (*LogConfig)(nil)
		b := (*LogConfig)(nil)
		require.True(t, a.Equals(b))
	})

	t.Run("one nil", func(t *testing.T) {
		a := new(LogConfig)
		b := (*LogConfig)(nil)
		require.False(t, a.Equals(b))
	})

	t.Run("max files", func(t *testing.T) {
		a := &LogConfig{MaxFiles: 1, MaxFileSizeMB: 200}
		b := &LogConfig{MaxFiles: 2, MaxFileSizeMB: 200}
		require.False(t, a.Equals(b))
	})

	t.Run("max file size", func(t *testing.T) {
		a := &LogConfig{MaxFiles: 1, MaxFileSizeMB: 100}
		b := &LogConfig{MaxFiles: 1, MaxFileSizeMB: 200}
		require.False(t, a.Equals(b))
	})

	t.Run("same", func(t *testing.T) {
		a := &LogConfig{MaxFiles: 1, MaxFileSizeMB: 200}
		b := &LogConfig{MaxFiles: 1, MaxFileSizeMB: 200}
		require.True(t, a.Equals(b))
	})
}

func TestTask_Validate_CSIPluginConfig(t *testing.T) {
	table := []struct {
		name        string
		pc          *TaskCSIPluginConfig
		expectedErr string
	}{
		{
			name: "no errors when not specified",
			pc:   nil,
		},
		{
			name:        "requires non-empty plugin id",
			pc:          &TaskCSIPluginConfig{},
			expectedErr: "CSIPluginConfig must have a non-empty PluginID",
		},
		{
			name: "requires valid plugin type",
			pc: &TaskCSIPluginConfig{
				ID:   "com.hashicorp.csi",
				Type: "nonsense",
			},
			expectedErr: "CSIPluginConfig PluginType must be one of 'node', 'controller', or 'monolith', got: \"nonsense\"",
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				CSIPluginConfig: tt.pc,
			}
			ephemeralDisk := &EphemeralDisk{
				SizeMB: 1,
			}

			err := task.Validate(ephemeralDisk, JobTypeService, nil)
			mErr := err.(*multierror.Error)
			if tt.expectedErr != "" {
				if !strings.Contains(mErr.Errors[4].Error(), tt.expectedErr) {
					t.Fatalf("err: %s", err)
				}
			} else {
				if len(mErr.Errors) != 4 {
					t.Fatalf("unexpected err: %s", mErr.Errors[4])
				}
			}
		})
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

	err := task.Validate(ephemeralDisk, JobTypeService, nil)
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
	err = task.Validate(ephemeralDisk, JobTypeService, nil)
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

	err = task.Validate(ephemeralDisk, JobTypeService, nil)
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
	require.NoError(t, err)

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

	// Perform semver validation
	c.Operand = ConstraintSemver
	err = c.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "Malformed constraint")

	c.RTarget = ">= 0.6.1"
	require.NoError(t, c.Validate())

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

	// Perform set_contains* validation
	c.RTarget = ""
	for _, o := range []string{ConstraintSetContains, ConstraintSetContainsAll, ConstraintSetContainsAny} {
		c.Operand = o
		err = c.Validate()
		mErr = err.(*multierror.Error)
		if !strings.Contains(mErr.Errors[0].Error(), "requires an RTarget") {
			t.Fatalf("err: %s", err)
		}
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

func TestAffinity_Validate(t *testing.T) {

	type tc struct {
		affinity *Affinity
		err      error
		name     string
	}

	testCases := []tc{
		{
			affinity: &Affinity{},
			err:      fmt.Errorf("Missing affinity operand"),
		},
		{
			affinity: &Affinity{
				Operand: "foo",
				LTarget: "${meta.node_class}",
				Weight:  10,
			},
			err: fmt.Errorf("Unknown affinity operator \"foo\""),
		},
		{
			affinity: &Affinity{
				Operand: "=",
				LTarget: "${meta.node_class}",
				Weight:  10,
			},
			err: fmt.Errorf("Operator \"=\" requires an RTarget"),
		},
		{
			affinity: &Affinity{
				Operand: "=",
				LTarget: "${meta.node_class}",
				RTarget: "c4",
				Weight:  0,
			},
			err: fmt.Errorf("Affinity weight cannot be zero"),
		},
		{
			affinity: &Affinity{
				Operand: "=",
				LTarget: "${meta.node_class}",
				RTarget: "c4",
				Weight:  110,
			},
			err: fmt.Errorf("Affinity weight must be within the range [-100,100]"),
		},
		{
			affinity: &Affinity{
				Operand: "=",
				LTarget: "${node.class}",
				Weight:  10,
			},
			err: fmt.Errorf("Operator \"=\" requires an RTarget"),
		},
		{
			affinity: &Affinity{
				Operand: "version",
				LTarget: "${meta.os}",
				RTarget: ">>2.0",
				Weight:  110,
			},
			err: fmt.Errorf("Version affinity is invalid"),
		},
		{
			affinity: &Affinity{
				Operand: "regexp",
				LTarget: "${meta.os}",
				RTarget: "\\K2.0",
				Weight:  100,
			},
			err: fmt.Errorf("Regular expression failed to compile"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.affinity.Validate()
			if tc.err != nil {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tc.err.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestUpdateStrategy_Validate(t *testing.T) {
	u := &UpdateStrategy{
		MaxParallel:      -1,
		HealthCheck:      "foo",
		MinHealthyTime:   -10,
		HealthyDeadline:  -15,
		ProgressDeadline: -25,
		AutoRevert:       false,
		Canary:           -1,
	}

	err := u.Validate()
	mErr := err.(*multierror.Error)
	if !strings.Contains(mErr.Errors[0].Error(), "Invalid health check given") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[1].Error(), "Max parallel can not be less than zero") {
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
	if !strings.Contains(mErr.Errors[5].Error(), "Progress deadline must be zero or greater") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[6].Error(), "Minimum healthy time must be less than healthy deadline") {
		t.Fatalf("err: %s", err)
	}
	if !strings.Contains(mErr.Errors[7].Error(), "Healthy deadline must be less than progress deadline") {
		t.Fatalf("err: %s", err)
	}
}

func TestResource_NetIndex(t *testing.T) {
	r := &Resources{
		Networks: []*NetworkResource{
			{Device: "eth0"},
			{Device: "lo0"},
			{Device: ""},
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
	}
	r2 := &Resources{
		CPU:      2000,
		MemoryMB: 1024,
		DiskMB:   5000,
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
		Networks: []*NetworkResource{
			{
				CIDR:          "10.0.0.0/8",
				MBits:         100,
				ReservedPorts: []Port{{"ssh", 22, 0, ""}},
			},
		},
	}
	r2 := &Resources{
		CPU:      2000,
		MemoryMB: 1024,
		DiskMB:   5000,
		Networks: []*NetworkResource{
			{
				IP:            "10.0.0.1",
				MBits:         50,
				ReservedPorts: []Port{{"web", 80, 0, ""}},
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
		Networks: []*NetworkResource{
			{
				CIDR:          "10.0.0.0/8",
				MBits:         150,
				ReservedPorts: []Port{{"ssh", 22, 0, ""}, {"web", 80, 0, ""}},
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
			{
				MBits:        50,
				DynamicPorts: []Port{{"http", 0, 80, ""}, {"https", 0, 443, ""}},
			},
		},
	}
	r3 := &Resources{
		Networks: []*NetworkResource{
			{
				MBits:        25,
				DynamicPorts: []Port{{"admin", 0, 8080, ""}},
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
			{
				MBits:        75,
				DynamicPorts: []Port{{"http", 0, 80, ""}, {"https", 0, 443, ""}, {"admin", 0, 8080, ""}},
			},
		},
	}

	if !reflect.DeepEqual(expect.Networks, r1.Networks) {
		t.Fatalf("bad: %#v %#v", expect.Networks[0], r1.Networks[0])
	}
}

func TestComparableResources_Subtract(t *testing.T) {
	r1 := &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares: 2000,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB: 2048,
			},
			Networks: []*NetworkResource{
				{
					CIDR:          "10.0.0.0/8",
					MBits:         100,
					ReservedPorts: []Port{{"ssh", 22, 0, ""}},
				},
			},
		},
		Shared: AllocatedSharedResources{
			DiskMB: 10000,
		},
	}

	r2 := &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares: 1000,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB: 1024,
			},
			Networks: []*NetworkResource{
				{
					CIDR:          "10.0.0.0/8",
					MBits:         20,
					ReservedPorts: []Port{{"ssh", 22, 0, ""}},
				},
			},
		},
		Shared: AllocatedSharedResources{
			DiskMB: 5000,
		},
	}
	r1.Subtract(r2)

	expect := &ComparableResources{
		Flattened: AllocatedTaskResources{
			Cpu: AllocatedCpuResources{
				CpuShares: 1000,
			},
			Memory: AllocatedMemoryResources{
				MemoryMB: 1024,
			},
			Networks: []*NetworkResource{
				{
					CIDR:          "10.0.0.0/8",
					MBits:         100,
					ReservedPorts: []Port{{"ssh", 22, 0, ""}},
				},
			},
		},
		Shared: AllocatedSharedResources{
			DiskMB: 5000,
		},
	}

	require := require.New(t)
	require.Equal(expect, r1)
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
		Name:      "my_service-${NOMAD_META_FOO}",
		PortLabel: "bar",
	}
	if err := s.Validate(); err == nil {
		t.Fatalf("Service should be invalid (contains underscore but not in a variable name): %v", err)
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

	s = Service{
		Name: "service-name",
		Checks: []*ServiceCheck{
			{
				Name:     "tcp-check",
				Type:     ServiceCheckTCP,
				Interval: 5 * time.Second,
				Timeout:  2 * time.Second,
			},
		},
		Connect: &ConsulConnect{
			SidecarService: &ConsulSidecarService{},
		},
	}
	require.Error(t, s.Validate())
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
		t.Fatalf("Expected name: %v, Actual: %v", "example-cache-redis-db", s.Name)
	}

	s.Name = "${BASE}-db"
	s.Canonicalize(job, taskGroup, task)
	if s.Name != "example-cache-redis-db" {
		t.Fatalf("Expected name: %v, Actual: %v", "example-cache-redis-db", s.Name)
	}

}

func TestService_Validate(t *testing.T) {
	s := Service{
		Name: "testservice",
	}

	s.Canonicalize("testjob", "testgroup", "testtask")

	// Base service should be valid
	require.NoError(t, s.Validate())

	// Native Connect requires task name on service
	s.Connect = &ConsulConnect{
		Native: true,
	}
	require.Error(t, s.Validate())

	// Native Connect should work with task name on service set
	s.TaskName = "testtask"
	require.NoError(t, s.Validate())

	// Native Connect + Sidecar should be invalid
	s.Connect.SidecarService = &ConsulSidecarService{}
	require.Error(t, s.Validate())
}

func TestService_Equals(t *testing.T) {
	s := Service{
		Name: "testservice",
	}

	s.Canonicalize("testjob", "testgroup", "testtask")

	o := s.Copy()

	// Base service should be equal to copy of itself
	require.True(t, s.Equals(o))

	// create a helper to assert a diff and reset the struct
	assertDiff := func() {
		require.False(t, s.Equals(o))
		o = s.Copy()
		require.True(t, s.Equals(o), "bug in copy")
	}

	// Changing any field should cause inequality
	o.Name = "diff"
	assertDiff()

	o.PortLabel = "diff"
	assertDiff()

	o.AddressMode = AddressModeDriver
	assertDiff()

	o.Tags = []string{"diff"}
	assertDiff()

	o.CanaryTags = []string{"diff"}
	assertDiff()

	o.Checks = []*ServiceCheck{{Name: "diff"}}
	assertDiff()

	o.Connect = &ConsulConnect{Native: true}
	assertDiff()

	o.EnableTagOverride = true
	assertDiff()
}

func TestJob_ExpandServiceNames(t *testing.T) {
	j := &Job{
		Name: "my-job",
		TaskGroups: []*TaskGroup{
			{
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
			{
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

func TestJob_CombinedTaskMeta(t *testing.T) {
	j := &Job{
		Meta: map[string]string{
			"job_test":   "job",
			"group_test": "job",
			"task_test":  "job",
		},
		TaskGroups: []*TaskGroup{
			{
				Name: "group",
				Meta: map[string]string{
					"group_test": "group",
					"task_test":  "group",
				},
				Tasks: []*Task{
					{
						Name: "task",
						Meta: map[string]string{
							"task_test": "task",
						},
					},
				},
			},
		},
	}

	require := require.New(t)
	require.EqualValues(map[string]string{
		"job_test":   "job",
		"group_test": "group",
		"task_test":  "task",
	}, j.CombinedTaskMeta("group", "task"))
	require.EqualValues(map[string]string{
		"job_test":   "job",
		"group_test": "group",
		"task_test":  "group",
	}, j.CombinedTaskMeta("group", ""))
	require.EqualValues(map[string]string{
		"job_test":   "job",
		"group_test": "job",
		"task_test":  "job",
	}, j.CombinedTaskMeta("", "task"))

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

	cases := []struct {
		spec     string
		nextTime time.Time
		errorMsg string
	}{
		{
			spec:     "0 0 29 2 * 1980",
			nextTime: time.Time{},
		},
		{
			spec:     "*/5 * * * *",
			nextTime: time.Date(2009, time.November, 10, 23, 25, 0, 0, time.UTC),
		},
		{
			spec:     "1 15-0 *",
			nextTime: time.Time{},
			errorMsg: "failed parsing cron expression",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case: %d: %s", i, c.spec), func(t *testing.T) {
			p := &PeriodicConfig{Enabled: true, SpecType: PeriodicSpecCron, Spec: c.spec}
			p.Canonicalize()
			n, err := p.Next(from)

			require.Equal(t, c.nextTime, n)
			if c.errorMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.errorMsg)
			}
		})
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
	require := require.New(t)

	// On Sun, Mar 12, 2:00 am 2017: +1 hour UTC
	p := &PeriodicConfig{
		Enabled:  true,
		SpecType: PeriodicSpecCron,
		Spec:     "0 2 11-13 3 * 2017",
		TimeZone: "America/Los_Angeles",
	}
	p.Canonicalize()

	t1 := time.Date(2017, time.March, 11, 1, 0, 0, 0, p.location)
	t2 := time.Date(2017, time.March, 12, 1, 0, 0, 0, p.location)

	// E1 is an 8 hour adjustment, E2 is a 7 hour adjustment
	e1 := time.Date(2017, time.March, 11, 10, 0, 0, 0, time.UTC)
	e2 := time.Date(2017, time.March, 13, 9, 0, 0, 0, time.UTC)

	n1, err := p.Next(t1)
	require.Nil(err)

	n2, err := p.Next(t2)
	require.Nil(err)

	require.Equal(e1, n1.UTC())
	require.Equal(e2, n2.UTC())
}

func TestTaskLifecycleConfig_Validate(t *testing.T) {
	testCases := []struct {
		name string
		tlc  *TaskLifecycleConfig
		err  error
	}{
		{
			name: "prestart completed",
			tlc: &TaskLifecycleConfig{
				Hook:    "prestart",
				Sidecar: false,
			},
			err: nil,
		},
		{
			name: "prestart running",
			tlc: &TaskLifecycleConfig{
				Hook:    "prestart",
				Sidecar: true,
			},
			err: nil,
		},
		{
			name: "no hook",
			tlc: &TaskLifecycleConfig{
				Sidecar: true,
			},
			err: fmt.Errorf("no lifecycle hook provided"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tlc.Validate()
			if tc.err != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.err.Error())
			} else {
				require.Nil(t, err)
			}
		})

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

func TestReschedulePolicy_Validate(t *testing.T) {
	type testCase struct {
		desc             string
		ReschedulePolicy *ReschedulePolicy
		errors           []error
	}

	testCases := []testCase{
		{
			desc: "Nil",
		},
		{
			desc: "Disabled",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts: 0,
				Interval: 0 * time.Second},
		},
		{
			desc: "Disabled",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts: -1,
				Interval: 5 * time.Minute},
		},
		{
			desc: "Valid Linear Delay",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      1,
				Interval:      5 * time.Minute,
				Delay:         10 * time.Second,
				DelayFunction: "constant"},
		},
		{
			desc: "Valid Exponential Delay",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      5,
				Interval:      1 * time.Hour,
				Delay:         30 * time.Second,
				MaxDelay:      5 * time.Minute,
				DelayFunction: "exponential"},
		},
		{
			desc: "Valid Fibonacci Delay",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      5,
				Interval:      15 * time.Minute,
				Delay:         10 * time.Second,
				MaxDelay:      5 * time.Minute,
				DelayFunction: "fibonacci"},
		},
		{
			desc: "Invalid delay function",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      1,
				Interval:      1 * time.Second,
				DelayFunction: "blah"},
			errors: []error{
				fmt.Errorf("Interval cannot be less than %v (got %v)", ReschedulePolicyMinInterval, time.Second),
				fmt.Errorf("Delay cannot be less than %v (got %v)", ReschedulePolicyMinDelay, 0*time.Second),
				fmt.Errorf("Invalid delay function %q, must be one of %q", "blah", RescheduleDelayFunctions),
			},
		},
		{
			desc: "Invalid delay ceiling",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      1,
				Interval:      8 * time.Second,
				DelayFunction: "exponential",
				Delay:         15 * time.Second,
				MaxDelay:      5 * time.Second},
			errors: []error{
				fmt.Errorf("Max Delay cannot be less than Delay %v (got %v)",
					15*time.Second, 5*time.Second),
			},
		},
		{
			desc: "Invalid delay and interval",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      1,
				Interval:      1 * time.Second,
				DelayFunction: "constant"},
			errors: []error{
				fmt.Errorf("Interval cannot be less than %v (got %v)", ReschedulePolicyMinInterval, time.Second),
				fmt.Errorf("Delay cannot be less than %v (got %v)", ReschedulePolicyMinDelay, 0*time.Second),
			},
		}, {
			// Should suggest 2h40m as the interval
			desc: "Invalid Attempts - linear delay",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      10,
				Interval:      1 * time.Hour,
				Delay:         20 * time.Minute,
				DelayFunction: "constant",
			},
			errors: []error{
				fmt.Errorf("Nomad can only make %v attempts in %v with initial delay %v and"+
					" delay function %q", 3, time.Hour, 20*time.Minute, "constant"),
				fmt.Errorf("Set the interval to at least %v to accommodate %v attempts",
					200*time.Minute, 10),
			},
		},
		{
			// Should suggest 4h40m as the interval
			// Delay progression in minutes {5, 10, 20, 40, 40, 40, 40, 40, 40, 40}
			desc: "Invalid Attempts - exponential delay",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      10,
				Interval:      30 * time.Minute,
				Delay:         5 * time.Minute,
				MaxDelay:      40 * time.Minute,
				DelayFunction: "exponential",
			},
			errors: []error{
				fmt.Errorf("Nomad can only make %v attempts in %v with initial delay %v, "+
					"delay function %q, and delay ceiling %v", 3, 30*time.Minute, 5*time.Minute,
					"exponential", 40*time.Minute),
				fmt.Errorf("Set the interval to at least %v to accommodate %v attempts",
					280*time.Minute, 10),
			},
		},
		{
			// Should suggest 8h as the interval
			// Delay progression in minutes {20, 20, 40, 60, 80, 80, 80, 80, 80, 80}
			desc: "Invalid Attempts - fibonacci delay",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      10,
				Interval:      1 * time.Hour,
				Delay:         20 * time.Minute,
				MaxDelay:      80 * time.Minute,
				DelayFunction: "fibonacci",
			},
			errors: []error{
				fmt.Errorf("Nomad can only make %v attempts in %v with initial delay %v, "+
					"delay function %q, and delay ceiling %v", 4, 1*time.Hour, 20*time.Minute,
					"fibonacci", 80*time.Minute),
				fmt.Errorf("Set the interval to at least %v to accommodate %v attempts",
					480*time.Minute, 10),
			},
		},
		{
			desc: "Ambiguous Unlimited config, has both attempts and unlimited set",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      1,
				Unlimited:     true,
				DelayFunction: "exponential",
				Delay:         5 * time.Minute,
				MaxDelay:      1 * time.Hour,
			},
			errors: []error{
				fmt.Errorf("Interval must be a non zero value if Attempts > 0"),
				fmt.Errorf("Reschedule Policy with Attempts = %v, Interval = %v, and Unlimited = %v is ambiguous", 1, time.Duration(0), true),
			},
		},
		{
			desc: "Invalid Unlimited config",
			ReschedulePolicy: &ReschedulePolicy{
				Attempts:      1,
				Interval:      1 * time.Second,
				Unlimited:     true,
				DelayFunction: "exponential",
			},
			errors: []error{
				fmt.Errorf("Delay cannot be less than %v (got %v)", ReschedulePolicyMinDelay, 0*time.Second),
				fmt.Errorf("Max Delay cannot be less than %v (got %v)", ReschedulePolicyMinDelay, 0*time.Second),
			},
		},
		{
			desc: "Valid Unlimited config",
			ReschedulePolicy: &ReschedulePolicy{
				Unlimited:     true,
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      1 * time.Hour,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require := require.New(t)
			gotErr := tc.ReschedulePolicy.Validate()
			if tc.errors != nil {
				// Validate all errors
				for _, err := range tc.errors {
					require.Contains(gotErr.Error(), err.Error())
				}
			} else {
				require.Nil(gotErr)
			}
		})
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

// TestTaskArtifact_Hash asserts an artifact's hash changes when any of the
// fields change.
func TestTaskArtifact_Hash(t *testing.T) {
	t.Parallel()

	cases := []TaskArtifact{
		{},
		{
			GetterSource: "a",
		},
		{
			GetterSource: "b",
		},
		{
			GetterSource:  "b",
			GetterOptions: map[string]string{"c": "c"},
		},
		{
			GetterSource: "b",
			GetterOptions: map[string]string{
				"c": "c",
				"d": "d",
			},
		},
		{
			GetterSource: "b",
			GetterOptions: map[string]string{
				"c": "c",
				"d": "e",
			},
		},
		{
			GetterSource: "b",
			GetterOptions: map[string]string{
				"c": "c",
				"d": "e",
			},
			GetterMode: "f",
		},
		{
			GetterSource: "b",
			GetterOptions: map[string]string{
				"c": "c",
				"d": "e",
			},
			GetterMode: "g",
		},
		{
			GetterSource: "b",
			GetterOptions: map[string]string{
				"c": "c",
				"d": "e",
			},
			GetterMode:   "g",
			RelativeDest: "h",
		},
		{
			GetterSource: "b",
			GetterOptions: map[string]string{
				"c": "c",
				"d": "e",
			},
			GetterMode:   "g",
			RelativeDest: "i",
		},
	}

	// Map of hash to source
	hashes := make(map[string]TaskArtifact, len(cases))
	for _, tc := range cases {
		h := tc.Hash()

		// Hash should be deterministic
		require.Equal(t, h, tc.Hash())

		// Hash should be unique
		if orig, ok := hashes[h]; ok {
			require.Failf(t, "hashes match", "artifact 1: %s\n\n artifact 2: %s\n",
				pretty.Sprint(tc), pretty.Sprint(orig),
			)
		}
		hashes[h] = tc
	}

	require.Len(t, hashes, len(cases))
}

func TestAllocation_ShouldMigrate(t *testing.T) {
	alloc := Allocation{
		PreviousAllocation: "123",
		TaskGroup:          "foo",
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
		PreviousAllocation: "123",
		TaskGroup:          "foo",
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
		PreviousAllocation: "123",
		TaskGroup:          "foo",
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
		PreviousAllocation: "123",
		TaskGroup:          "foo",
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

	// No previous
	alloc4 := Allocation{
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

	if alloc4.ShouldMigrate() {
		t.Fatalf("bad: %v", alloc4)
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
		{
			&TaskArtifact{
				GetterSource: "foo.com",
				GetterOptions: map[string]string{
					"checksum": "md5:${ARTIFACT_CHECKSUM}",
				},
			},
			false,
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

func TestPlan_NormalizeAllocations(t *testing.T) {
	t.Parallel()
	plan := &Plan{
		NodeUpdate:      make(map[string][]*Allocation),
		NodePreemptions: make(map[string][]*Allocation),
	}
	stoppedAlloc := MockAlloc()
	desiredDesc := "Desired desc"
	plan.AppendStoppedAlloc(stoppedAlloc, desiredDesc, AllocClientStatusLost, "followup-eval-id")
	preemptedAlloc := MockAlloc()
	preemptingAllocID := uuid.Generate()
	plan.AppendPreemptedAlloc(preemptedAlloc, preemptingAllocID)

	plan.NormalizeAllocations()

	actualStoppedAlloc := plan.NodeUpdate[stoppedAlloc.NodeID][0]
	expectedStoppedAlloc := &Allocation{
		ID:                 stoppedAlloc.ID,
		DesiredDescription: desiredDesc,
		ClientStatus:       AllocClientStatusLost,
		FollowupEvalID:     "followup-eval-id",
	}
	assert.Equal(t, expectedStoppedAlloc, actualStoppedAlloc)
	actualPreemptedAlloc := plan.NodePreemptions[preemptedAlloc.NodeID][0]
	expectedPreemptedAlloc := &Allocation{
		ID:                    preemptedAlloc.ID,
		PreemptedByAllocation: preemptingAllocID,
	}
	assert.Equal(t, expectedPreemptedAlloc, actualPreemptedAlloc)
}

func TestPlan_AppendStoppedAllocAppendsAllocWithUpdatedAttrs(t *testing.T) {
	t.Parallel()
	plan := &Plan{
		NodeUpdate: make(map[string][]*Allocation),
	}
	alloc := MockAlloc()
	desiredDesc := "Desired desc"

	plan.AppendStoppedAlloc(alloc, desiredDesc, AllocClientStatusLost, "")

	expectedAlloc := new(Allocation)
	*expectedAlloc = *alloc
	expectedAlloc.DesiredDescription = desiredDesc
	expectedAlloc.DesiredStatus = AllocDesiredStatusStop
	expectedAlloc.ClientStatus = AllocClientStatusLost
	expectedAlloc.Job = nil
	expectedAlloc.AllocStates = []*AllocState{{
		Field: AllocStateFieldClientStatus,
		Value: "lost",
	}}

	// This value is set to time.Now() in AppendStoppedAlloc, so clear it
	appendedAlloc := plan.NodeUpdate[alloc.NodeID][0]
	appendedAlloc.AllocStates[0].Time = time.Time{}

	assert.Equal(t, expectedAlloc, appendedAlloc)
	assert.Equal(t, alloc.Job, plan.Job)
}

func TestPlan_AppendPreemptedAllocAppendsAllocWithUpdatedAttrs(t *testing.T) {
	t.Parallel()
	plan := &Plan{
		NodePreemptions: make(map[string][]*Allocation),
	}
	alloc := MockAlloc()
	preemptingAllocID := uuid.Generate()

	plan.AppendPreemptedAlloc(alloc, preemptingAllocID)

	appendedAlloc := plan.NodePreemptions[alloc.NodeID][0]
	expectedAlloc := &Allocation{
		ID:                    alloc.ID,
		PreemptedByAllocation: preemptingAllocID,
		JobID:                 alloc.JobID,
		Namespace:             alloc.Namespace,
		DesiredStatus:         AllocDesiredStatusEvict,
		DesiredDescription:    fmt.Sprintf("Preempted by alloc ID %v", preemptingAllocID),
		AllocatedResources:    alloc.AllocatedResources,
		TaskResources:         alloc.TaskResources,
		SharedResources:       alloc.SharedResources,
	}
	assert.Equal(t, expectedAlloc, appendedAlloc)
}

func TestAllocation_MsgPackTags(t *testing.T) {
	t.Parallel()
	planType := reflect.TypeOf(Allocation{})

	msgPackTags, _ := planType.FieldByName("_struct")

	assert.Equal(t, msgPackTags.Tag, reflect.StructTag(`codec:",omitempty"`))
}

func TestEvaluation_MsgPackTags(t *testing.T) {
	t.Parallel()
	planType := reflect.TypeOf(Evaluation{})

	msgPackTags, _ := planType.FieldByName("_struct")

	assert.Equal(t, msgPackTags.Tag, reflect.StructTag(`codec:",omitempty"`))
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

func TestAllocation_ShouldReschedule(t *testing.T) {
	type testCase struct {
		Desc               string
		FailTime           time.Time
		ClientStatus       string
		DesiredStatus      string
		ReschedulePolicy   *ReschedulePolicy
		RescheduleTrackers []*RescheduleEvent
		ShouldReschedule   bool
	}

	fail := time.Now()

	harness := []testCase{
		{
			Desc:             "Reschedule when desired state is stop",
			ClientStatus:     AllocClientStatusPending,
			DesiredStatus:    AllocDesiredStatusStop,
			FailTime:         fail,
			ReschedulePolicy: nil,
			ShouldReschedule: false,
		},
		{
			Desc:             "Disabled rescheduling",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 0, Interval: 1 * time.Minute},
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule when client status is complete",
			ClientStatus:     AllocClientStatusComplete,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: nil,
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule with nil reschedule policy",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: nil,
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule with unlimited and attempts >0",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 1, Unlimited: true},
			ShouldReschedule: true,
		},
		{
			Desc:             "Reschedule when client status is complete",
			ClientStatus:     AllocClientStatusComplete,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: nil,
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule with policy when client status complete",
			ClientStatus:     AllocClientStatusComplete,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 1, Interval: 1 * time.Minute},
			ShouldReschedule: false,
		},
		{
			Desc:             "Reschedule with no previous attempts",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 1, Interval: 1 * time.Minute},
			ShouldReschedule: true,
		},
		{
			Desc:             "Reschedule with leftover attempts",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 2, Interval: 5 * time.Minute},
			FailTime:         fail,
			RescheduleTrackers: []*RescheduleEvent{
				{
					RescheduleTime: fail.Add(-1 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldReschedule: true,
		},
		{
			Desc:             "Reschedule with too old previous attempts",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 1, Interval: 5 * time.Minute},
			RescheduleTrackers: []*RescheduleEvent{
				{
					RescheduleTime: fail.Add(-6 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldReschedule: true,
		},
		{
			Desc:             "Reschedule with no leftover attempts",
			ClientStatus:     AllocClientStatusFailed,
			DesiredStatus:    AllocDesiredStatusRun,
			FailTime:         fail,
			ReschedulePolicy: &ReschedulePolicy{Attempts: 2, Interval: 5 * time.Minute},
			RescheduleTrackers: []*RescheduleEvent{
				{
					RescheduleTime: fail.Add(-3 * time.Minute).UTC().UnixNano(),
				},
				{
					RescheduleTime: fail.Add(-4 * time.Minute).UTC().UnixNano(),
				},
			},
			ShouldReschedule: false,
		},
	}

	for _, state := range harness {
		alloc := Allocation{}
		alloc.DesiredStatus = state.DesiredStatus
		alloc.ClientStatus = state.ClientStatus
		alloc.RescheduleTracker = &RescheduleTracker{state.RescheduleTrackers}

		t.Run(state.Desc, func(t *testing.T) {
			if got := alloc.ShouldReschedule(state.ReschedulePolicy, state.FailTime); got != state.ShouldReschedule {
				t.Fatalf("expected %v but got %v", state.ShouldReschedule, got)
			}
		})

	}
}

func TestAllocation_LastEventTime(t *testing.T) {
	type testCase struct {
		desc                  string
		taskState             map[string]*TaskState
		expectedLastEventTime time.Time
	}

	t1 := time.Now().UTC()

	testCases := []testCase{
		{
			desc:                  "nil task state",
			expectedLastEventTime: t1,
		},
		{
			desc:                  "empty task state",
			taskState:             make(map[string]*TaskState),
			expectedLastEventTime: t1,
		},
		{
			desc: "Finished At not set",
			taskState: map[string]*TaskState{"foo": {State: "start",
				StartedAt: t1.Add(-2 * time.Hour)}},
			expectedLastEventTime: t1,
		},
		{
			desc: "One finished ",
			taskState: map[string]*TaskState{"foo": {State: "start",
				StartedAt:  t1.Add(-2 * time.Hour),
				FinishedAt: t1.Add(-1 * time.Hour)}},
			expectedLastEventTime: t1.Add(-1 * time.Hour),
		},
		{
			desc: "Multiple task groups",
			taskState: map[string]*TaskState{"foo": {State: "start",
				StartedAt:  t1.Add(-2 * time.Hour),
				FinishedAt: t1.Add(-1 * time.Hour)},
				"bar": {State: "start",
					StartedAt:  t1.Add(-2 * time.Hour),
					FinishedAt: t1.Add(-40 * time.Minute)}},
			expectedLastEventTime: t1.Add(-40 * time.Minute),
		},
		{
			desc: "No finishedAt set, one task event, should use modify time",
			taskState: map[string]*TaskState{"foo": {
				State:     "run",
				StartedAt: t1.Add(-2 * time.Hour),
				Events: []*TaskEvent{
					{Type: "start", Time: t1.Add(-20 * time.Minute).UnixNano()},
				}},
			},
			expectedLastEventTime: t1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			alloc := &Allocation{CreateTime: t1.UnixNano(), ModifyTime: t1.UnixNano()}
			alloc.TaskStates = tc.taskState
			require.Equal(t, tc.expectedLastEventTime, alloc.LastEventTime())
		})
	}
}

func TestAllocation_NextDelay(t *testing.T) {
	type testCase struct {
		desc                       string
		reschedulePolicy           *ReschedulePolicy
		alloc                      *Allocation
		expectedRescheduleTime     time.Time
		expectedRescheduleEligible bool
	}
	now := time.Now()
	testCases := []testCase{
		{
			desc: "Allocation hasn't failed yet",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
			},
			alloc:                      &Allocation{},
			expectedRescheduleTime:     time.Time{},
			expectedRescheduleEligible: false,
		},
		{
			desc:                       "Allocation has no reschedule policy",
			alloc:                      &Allocation{},
			expectedRescheduleTime:     time.Time{},
			expectedRescheduleEligible: false,
		},
		{
			desc: "Allocation lacks task state",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
				Unlimited:     true,
			},
			alloc:                      &Allocation{ClientStatus: AllocClientStatusFailed, ModifyTime: now.UnixNano()},
			expectedRescheduleTime:     now.UTC().Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "linear delay, unlimited restarts, no reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "dead",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
			},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "linear delay with reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
				Interval:      10 * time.Minute,
				Attempts:      2,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{{
						RescheduleTime: now.Add(-2 * time.Minute).UTC().UnixNano(),
						Delay:          5 * time.Second,
					}},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "linear delay with reschedule tracker, attempts exhausted",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "constant",
				Delay:         5 * time.Second,
				Interval:      10 * time.Minute,
				Attempts:      2,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-3 * time.Minute).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-2 * time.Minute).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: false,
		},
		{
			desc: "exponential delay - no reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
			},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "exponential delay with reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          20 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(40 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "exponential delay with delay ceiling reached",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-15 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          20 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
						{
							RescheduleTime: now.Add(-40 * time.Second).UTC().UnixNano(),
							Delay:          80 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-15 * time.Second).Add(90 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			// Test case where most recent reschedule ran longer than delay ceiling
			desc: "exponential delay, delay ceiling reset condition met",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "exponential",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-15 * time.Minute)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          20 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          80 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          90 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          90 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-15 * time.Minute).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay - no reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with reschedule tracker",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-5 * time.Second).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(10 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with more events",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      90 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-2 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          15 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          25 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-2 * time.Second).Add(40 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with delay ceiling reached",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      50 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-15 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          15 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          25 * time.Second,
						},
						{
							RescheduleTime: now.Add(-40 * time.Second).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-15 * time.Second).Add(50 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with delay reset condition met",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      50 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-5 * time.Minute)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          15 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          25 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-5 * time.Minute).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
		{
			desc: "fibonacci delay with the most recent event that reset delay value",
			reschedulePolicy: &ReschedulePolicy{
				DelayFunction: "fibonacci",
				Delay:         5 * time.Second,
				MaxDelay:      50 * time.Second,
				Unlimited:     true,
			},
			alloc: &Allocation{
				ClientStatus: AllocClientStatusFailed,
				TaskStates: map[string]*TaskState{"foo": {State: "start",
					StartedAt:  now.Add(-1 * time.Hour),
					FinishedAt: now.Add(-5 * time.Second)}},
				RescheduleTracker: &RescheduleTracker{
					Events: []*RescheduleEvent{
						{
							RescheduleTime: now.Add(-2 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          10 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          15 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          25 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          40 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
							Delay:          50 * time.Second,
						},
						{
							RescheduleTime: now.Add(-1 * time.Minute).UTC().UnixNano(),
							Delay:          5 * time.Second,
						},
					},
				}},
			expectedRescheduleTime:     now.Add(-5 * time.Second).Add(5 * time.Second),
			expectedRescheduleEligible: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require := require.New(t)
			j := testJob()
			if tc.reschedulePolicy != nil {
				j.TaskGroups[0].ReschedulePolicy = tc.reschedulePolicy
			}
			tc.alloc.Job = j
			tc.alloc.TaskGroup = j.TaskGroups[0].Name
			reschedTime, allowed := tc.alloc.NextRescheduleTime()
			require.Equal(tc.expectedRescheduleEligible, allowed)
			require.Equal(tc.expectedRescheduleTime, reschedTime)
		})
	}

}

func TestAllocation_WaitClientStop(t *testing.T) {
	type testCase struct {
		desc                   string
		stop                   time.Duration
		status                 string
		expectedShould         bool
		expectedRescheduleTime time.Time
	}
	now := time.Now().UTC()
	testCases := []testCase{
		{
			desc:           "running",
			stop:           2 * time.Second,
			status:         AllocClientStatusRunning,
			expectedShould: true,
		},
		{
			desc:           "no stop_after_client_disconnect",
			status:         AllocClientStatusLost,
			expectedShould: false,
		},
		{
			desc:                   "stop",
			status:                 AllocClientStatusLost,
			stop:                   2 * time.Second,
			expectedShould:         true,
			expectedRescheduleTime: now.Add((2 + 5) * time.Second),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			j := testJob()
			a := &Allocation{
				ClientStatus: tc.status,
				Job:          j,
				TaskStates:   map[string]*TaskState{},
			}

			if tc.status == AllocClientStatusLost {
				a.AppendState(AllocStateFieldClientStatus, AllocClientStatusLost)
			}

			j.TaskGroups[0].StopAfterClientDisconnect = &tc.stop
			a.TaskGroup = j.TaskGroups[0].Name

			require.Equal(t, tc.expectedShould, a.ShouldClientStop())

			if !tc.expectedShould || tc.status != AllocClientStatusLost {
				return
			}

			// the reschedTime is close to the expectedRescheduleTime
			reschedTime := a.WaitClientStop()
			e := reschedTime.Unix() - tc.expectedRescheduleTime.Unix()
			require.Less(t, e, int64(2))
		})
	}
}

func TestAllocation_Canonicalize_Old(t *testing.T) {
	alloc := MockAlloc()
	alloc.AllocatedResources = nil
	alloc.TaskResources = map[string]*Resources{
		"web": {
			CPU:      500,
			MemoryMB: 256,
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []Port{{Label: "admin", Value: 5000}},
					MBits:         50,
					DynamicPorts:  []Port{{Label: "http", Value: 9876}},
				},
			},
		},
	}
	alloc.SharedResources = &Resources{
		DiskMB: 150,
	}
	alloc.Canonicalize()

	expected := &AllocatedResources{
		Tasks: map[string]*AllocatedTaskResources{
			"web": {
				Cpu: AllocatedCpuResources{
					CpuShares: 500,
				},
				Memory: AllocatedMemoryResources{
					MemoryMB: 256,
				},
				Networks: []*NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []Port{{Label: "admin", Value: 5000}},
						MBits:         50,
						DynamicPorts:  []Port{{Label: "http", Value: 9876}},
					},
				},
			},
		},
		Shared: AllocatedSharedResources{
			DiskMB: 150,
		},
	}

	require.Equal(t, expected, alloc.AllocatedResources)
}

// TestAllocation_Canonicalize_New asserts that an alloc with latest
// schema isn't modified with Canonicalize
func TestAllocation_Canonicalize_New(t *testing.T) {
	alloc := MockAlloc()
	copy := alloc.Copy()

	alloc.Canonicalize()
	require.Equal(t, copy, alloc)
}

func TestRescheduleTracker_Copy(t *testing.T) {
	type testCase struct {
		original *RescheduleTracker
		expected *RescheduleTracker
	}

	cases := []testCase{
		{nil, nil},
		{&RescheduleTracker{Events: []*RescheduleEvent{
			{RescheduleTime: 2,
				PrevAllocID: "12",
				PrevNodeID:  "12",
				Delay:       30 * time.Second},
		}}, &RescheduleTracker{Events: []*RescheduleEvent{
			{RescheduleTime: 2,
				PrevAllocID: "12",
				PrevNodeID:  "12",
				Delay:       30 * time.Second},
		}}},
	}

	for _, tc := range cases {
		if got := tc.original.Copy(); !reflect.DeepEqual(got, tc.expected) {
			t.Fatalf("expected %v but got %v", *tc.expected, *got)
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

func TestJobConfig_Validate_StopAferClientDisconnect(t *testing.T) {
	// Setup a system Job with stop_after_client_disconnect set, which is invalid
	job := testJob()
	job.Type = JobTypeSystem
	stop := 1 * time.Minute
	job.TaskGroups[0].StopAfterClientDisconnect = &stop

	err := job.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "stop_after_client_disconnect can only be set in batch and service jobs")

	// Modify the job to a batch job with an invalid stop_after_client_disconnect value
	job.Type = JobTypeBatch
	invalid := -1 * time.Minute
	job.TaskGroups[0].StopAfterClientDisconnect = &invalid

	err = job.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "stop_after_client_disconnect must be a positive value")

	// Modify the job to a batch job with a valid stop_after_client_disconnect value
	job.Type = JobTypeBatch
	job.TaskGroups[0].StopAfterClientDisconnect = &stop
	err = job.Validate()
	require.NoError(t, err)
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

func TestScalingPolicy_Canonicalize(t *testing.T) {
	cases := []struct {
		name     string
		input    *ScalingPolicy
		expected *ScalingPolicy
	}{
		{
			name:     "empty policy",
			input:    &ScalingPolicy{},
			expected: &ScalingPolicy{Type: ScalingPolicyTypeHorizontal},
		},
		{
			name:     "policy with type",
			input:    &ScalingPolicy{Type: "other-type"},
			expected: &ScalingPolicy{Type: "other-type"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require := require.New(t)

			c.input.Canonicalize()
			require.Equal(c.expected, c.input)
		})
	}
}

func TestScalingPolicy_Validate(t *testing.T) {
	type testCase struct {
		name        string
		input       *ScalingPolicy
		expectedErr string
	}

	cases := []testCase{
		{
			name: "full horizontal policy",
			input: &ScalingPolicy{
				Policy: map[string]interface{}{
					"key": "value",
				},
				Type:    ScalingPolicyTypeHorizontal,
				Min:     5,
				Max:     5,
				Enabled: true,
				Target: map[string]string{
					ScalingTargetNamespace: "my-namespace",
					ScalingTargetJob:       "my-job",
					ScalingTargetGroup:     "my-task-group",
				},
			},
		},
		{
			name:        "missing type",
			input:       &ScalingPolicy{},
			expectedErr: "missing scaling policy type",
		},
		{
			name: "invalid type",
			input: &ScalingPolicy{
				Type: "not valid",
			},
			expectedErr: `scaling policy type "not valid" is not valid`,
		},
		{
			name: "min < 0",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Min:  -1,
				Max:  5,
			},
			expectedErr: "minimum count must be specified and non-negative",
		},
		{
			name: "max < 0",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Min:  5,
				Max:  -1,
			},
			expectedErr: "maximum count must be specified and non-negative",
		},
		{
			name: "min > max",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Min:  10,
				Max:  0,
			},
			expectedErr: "maximum count must not be less than minimum count",
		},
		{
			name: "min == max",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Min:  10,
				Max:  10,
			},
		},
		{
			name: "min == 0",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Min:  0,
				Max:  10,
			},
		},
		{
			name: "max == 0",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Min:  0,
				Max:  0,
			},
		},
		{
			name: "horizontal missing namespace",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Target: map[string]string{
					ScalingTargetJob:   "my-job",
					ScalingTargetGroup: "my-group",
				},
			},
			expectedErr: "missing target namespace",
		},
		{
			name: "horizontal missing job",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Target: map[string]string{
					ScalingTargetNamespace: "my-namespace",
					ScalingTargetGroup:     "my-group",
				},
			},
			expectedErr: "missing target job",
		},
		{
			name: "horizontal missing group",
			input: &ScalingPolicy{
				Type: ScalingPolicyTypeHorizontal,
				Target: map[string]string{
					ScalingTargetNamespace: "my-namespace",
					ScalingTargetJob:       "my-job",
				},
			},
			expectedErr: "missing target group",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require := require.New(t)

			err := c.input.Validate()

			if len(c.expectedErr) > 0 {
				require.Error(err)
				mErr := err.(*multierror.Error)
				require.Len(mErr.Errors, 1)
				require.Contains(mErr.Errors[0].Error(), c.expectedErr)
			} else {
				require.NoError(err)
			}
		})
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

func TestACLTokenValidate(t *testing.T) {
	tk := &ACLToken{}

	// Missing a type
	err := tk.Validate()
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "client or management") {
		t.Fatalf("bad: %v", err)
	}

	// Missing policies
	tk.Type = ACLClientToken
	err = tk.Validate()
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "missing policies") {
		t.Fatalf("bad: %v", err)
	}

	// Invalid policies
	tk.Type = ACLManagementToken
	tk.Policies = []string{"foo"}
	err = tk.Validate()
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "associated with policies") {
		t.Fatalf("bad: %v", err)
	}

	// Name too long policies
	tk.Name = ""
	for i := 0; i < 8; i++ {
		tk.Name += uuid.Generate()
	}
	tk.Policies = nil
	err = tk.Validate()
	assert.NotNil(t, err)
	if !strings.Contains(err.Error(), "too long") {
		t.Fatalf("bad: %v", err)
	}

	// Make it valid
	tk.Name = "foo"
	err = tk.Validate()
	assert.Nil(t, err)
}

func TestACLTokenPolicySubset(t *testing.T) {
	tk := &ACLToken{
		Type:     ACLClientToken,
		Policies: []string{"foo", "bar", "baz"},
	}

	assert.Equal(t, true, tk.PolicySubset([]string{"foo", "bar", "baz"}))
	assert.Equal(t, true, tk.PolicySubset([]string{"foo", "bar"}))
	assert.Equal(t, true, tk.PolicySubset([]string{"foo"}))
	assert.Equal(t, true, tk.PolicySubset([]string{}))
	assert.Equal(t, false, tk.PolicySubset([]string{"foo", "bar", "new"}))
	assert.Equal(t, false, tk.PolicySubset([]string{"new"}))

	tk = &ACLToken{
		Type: ACLManagementToken,
	}

	assert.Equal(t, true, tk.PolicySubset([]string{"foo", "bar", "baz"}))
	assert.Equal(t, true, tk.PolicySubset([]string{"foo", "bar"}))
	assert.Equal(t, true, tk.PolicySubset([]string{"foo"}))
	assert.Equal(t, true, tk.PolicySubset([]string{}))
	assert.Equal(t, true, tk.PolicySubset([]string{"foo", "bar", "new"}))
	assert.Equal(t, true, tk.PolicySubset([]string{"new"}))
}

func TestACLTokenSetHash(t *testing.T) {
	tk := &ACLToken{
		Name:     "foo",
		Type:     ACLClientToken,
		Policies: []string{"foo", "bar"},
		Global:   false,
	}
	out1 := tk.SetHash()
	assert.NotNil(t, out1)
	assert.NotNil(t, tk.Hash)
	assert.Equal(t, out1, tk.Hash)

	tk.Policies = []string{"foo"}
	out2 := tk.SetHash()
	assert.NotNil(t, out2)
	assert.NotNil(t, tk.Hash)
	assert.Equal(t, out2, tk.Hash)
	assert.NotEqual(t, out1, out2)
}

func TestACLPolicySetHash(t *testing.T) {
	ap := &ACLPolicy{
		Name:        "foo",
		Description: "great policy",
		Rules:       "node { policy = \"read\" }",
	}
	out1 := ap.SetHash()
	assert.NotNil(t, out1)
	assert.NotNil(t, ap.Hash)
	assert.Equal(t, out1, ap.Hash)

	ap.Rules = "node { policy = \"write\" }"
	out2 := ap.SetHash()
	assert.NotNil(t, out2)
	assert.NotNil(t, ap.Hash)
	assert.Equal(t, out2, ap.Hash)
	assert.NotEqual(t, out1, out2)
}

func TestTaskEventPopulate(t *testing.T) {
	prepopulatedEvent := NewTaskEvent(TaskSetup)
	prepopulatedEvent.DisplayMessage = "Hola"
	testcases := []struct {
		event       *TaskEvent
		expectedMsg string
	}{
		{nil, ""},
		{prepopulatedEvent, "Hola"},
		{NewTaskEvent(TaskSetup).SetMessage("Setup"), "Setup"},
		{NewTaskEvent(TaskStarted), "Task started by client"},
		{NewTaskEvent(TaskReceived), "Task received by client"},
		{NewTaskEvent(TaskFailedValidation), "Validation of task failed"},
		{NewTaskEvent(TaskFailedValidation).SetValidationError(fmt.Errorf("task failed validation")), "task failed validation"},
		{NewTaskEvent(TaskSetupFailure), "Task setup failed"},
		{NewTaskEvent(TaskSetupFailure).SetSetupError(fmt.Errorf("task failed setup")), "task failed setup"},
		{NewTaskEvent(TaskDriverFailure), "Failed to start task"},
		{NewTaskEvent(TaskDownloadingArtifacts), "Client is downloading artifacts"},
		{NewTaskEvent(TaskArtifactDownloadFailed), "Failed to download artifacts"},
		{NewTaskEvent(TaskArtifactDownloadFailed).SetDownloadError(fmt.Errorf("connection reset by peer")), "connection reset by peer"},
		{NewTaskEvent(TaskRestarting).SetRestartDelay(2 * time.Second).SetRestartReason(ReasonWithinPolicy), "Task restarting in 2s"},
		{NewTaskEvent(TaskRestarting).SetRestartReason("Chaos Monkey did it"), "Chaos Monkey did it - Task restarting in 0s"},
		{NewTaskEvent(TaskKilling), "Sent interrupt"},
		{NewTaskEvent(TaskKilling).SetKillReason("Its time for you to die"), "Its time for you to die"},
		{NewTaskEvent(TaskKilling).SetKillTimeout(1 * time.Second), "Sent interrupt. Waiting 1s before force killing"},
		{NewTaskEvent(TaskTerminated).SetExitCode(-1).SetSignal(3), "Exit Code: -1, Signal: 3"},
		{NewTaskEvent(TaskTerminated).SetMessage("Goodbye"), "Exit Code: 0, Exit Message: \"Goodbye\""},
		{NewTaskEvent(TaskKilled), "Task successfully killed"},
		{NewTaskEvent(TaskKilled).SetKillError(fmt.Errorf("undead creatures can't be killed")), "undead creatures can't be killed"},
		{NewTaskEvent(TaskNotRestarting).SetRestartReason("Chaos Monkey did it"), "Chaos Monkey did it"},
		{NewTaskEvent(TaskNotRestarting), "Task exceeded restart policy"},
		{NewTaskEvent(TaskLeaderDead), "Leader Task in Group dead"},
		{NewTaskEvent(TaskSiblingFailed), "Task's sibling failed"},
		{NewTaskEvent(TaskSiblingFailed).SetFailedSibling("patient zero"), "Task's sibling \"patient zero\" failed"},
		{NewTaskEvent(TaskSignaling), "Task being sent a signal"},
		{NewTaskEvent(TaskSignaling).SetTaskSignal(os.Interrupt), "Task being sent signal interrupt"},
		{NewTaskEvent(TaskSignaling).SetTaskSignal(os.Interrupt).SetTaskSignalReason("process interrupted"), "Task being sent signal interrupt: process interrupted"},
		{NewTaskEvent(TaskRestartSignal), "Task signaled to restart"},
		{NewTaskEvent(TaskRestartSignal).SetRestartReason("Chaos Monkey restarted it"), "Chaos Monkey restarted it"},
		{NewTaskEvent(TaskDriverMessage).SetDriverMessage("YOLO"), "YOLO"},
		{NewTaskEvent("Unknown Type, No message"), ""},
		{NewTaskEvent("Unknown Type").SetMessage("Hello world"), "Hello world"},
	}

	for _, tc := range testcases {
		tc.event.PopulateEventDisplayMessage()
		if tc.event != nil && tc.event.DisplayMessage != tc.expectedMsg {
			t.Fatalf("Expected %v but got %v", tc.expectedMsg, tc.event.DisplayMessage)
		}
	}
}

func TestNetworkResourcesEquals(t *testing.T) {
	require := require.New(t)
	var networkResourcesTest = []struct {
		input    []*NetworkResource
		expected bool
		errorMsg string
	}{
		{
			[]*NetworkResource{
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
			},
			true,
			"Equal network resources should return true",
		},
		{
			[]*NetworkResource{
				{
					IP:            "10.0.0.0",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
			},
			false,
			"Different IP addresses should return false",
		},
		{
			[]*NetworkResource{
				{
					IP:            "10.0.0.1",
					MBits:         40,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
			},
			false,
			"Different MBits values should return false",
		},
		{
			[]*NetworkResource{
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}, {"web", 80, 0, ""}},
				},
			},
			false,
			"Different ReservedPorts lengths should return false",
		},
		{
			[]*NetworkResource{
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{},
				},
			},
			false,
			"Empty and non empty ReservedPorts values should return false",
		},
		{
			[]*NetworkResource{
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"notweb", 80, 0, ""}},
				},
			},
			false,
			"Different valued ReservedPorts values should return false",
		},
		{
			[]*NetworkResource{
				{
					IP:           "10.0.0.1",
					MBits:        50,
					DynamicPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:           "10.0.0.1",
					MBits:        50,
					DynamicPorts: []Port{{"web", 80, 0, ""}, {"web", 80, 0, ""}},
				},
			},
			false,
			"Different DynamicPorts lengths should return false",
		},
		{
			[]*NetworkResource{
				{
					IP:           "10.0.0.1",
					MBits:        50,
					DynamicPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:           "10.0.0.1",
					MBits:        50,
					DynamicPorts: []Port{},
				},
			},
			false,
			"Empty and non empty DynamicPorts values should return false",
		},
		{
			[]*NetworkResource{
				{
					IP:           "10.0.0.1",
					MBits:        50,
					DynamicPorts: []Port{{"web", 80, 0, ""}},
				},
				{
					IP:           "10.0.0.1",
					MBits:        50,
					DynamicPorts: []Port{{"notweb", 80, 0, ""}},
				},
			},
			false,
			"Different valued DynamicPorts values should return false",
		},
	}
	for _, testCase := range networkResourcesTest {
		first := testCase.input[0]
		second := testCase.input[1]
		require.Equal(testCase.expected, first.Equals(second), testCase.errorMsg)
	}
}

func TestNode_Canonicalize(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Make sure the eligiblity is set properly
	node := &Node{}
	node.Canonicalize()
	require.Equal(NodeSchedulingEligible, node.SchedulingEligibility)

	node = &Node{
		Drain: true,
	}
	node.Canonicalize()
	require.Equal(NodeSchedulingIneligible, node.SchedulingEligibility)
}

func TestNode_Copy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	node := &Node{
		ID:         uuid.Generate(),
		SecretID:   uuid.Generate(),
		Datacenter: "dc1",
		Name:       "foobar",
		Attributes: map[string]string{
			"kernel.name":        "linux",
			"arch":               "x86",
			"nomad.version":      "0.5.0",
			"driver.exec":        "1",
			"driver.mock_driver": "1",
		},
		Resources: &Resources{
			CPU:      4000,
			MemoryMB: 8192,
			DiskMB:   100 * 1024,
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		},
		Reserved: &Resources{
			CPU:      100,
			MemoryMB: 256,
			DiskMB:   4 * 1024,
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []Port{{Label: "ssh", Value: 22}},
					MBits:         1,
				},
			},
		},
		NodeResources: &NodeResources{
			Cpu: NodeCpuResources{
				CpuShares: 4000,
			},
			Memory: NodeMemoryResources{
				MemoryMB: 8192,
			},
			Disk: NodeDiskResources{
				DiskMB: 100 * 1024,
			},
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Cpu: NodeReservedCpuResources{
				CpuShares: 100,
			},
			Memory: NodeReservedMemoryResources{
				MemoryMB: 256,
			},
			Disk: NodeReservedDiskResources{
				DiskMB: 4 * 1024,
			},
			Networks: NodeReservedNetworkResources{
				ReservedHostPorts: "22",
			},
		},
		Links: map[string]string{
			"consul": "foobar.dc1",
		},
		Meta: map[string]string{
			"pci-dss":  "true",
			"database": "mysql",
			"version":  "5.6",
		},
		NodeClass:             "linux-medium-pci",
		Status:                NodeStatusReady,
		SchedulingEligibility: NodeSchedulingEligible,
		Drivers: map[string]*DriverInfo{
			"mock_driver": {
				Attributes:        map[string]string{"running": "1"},
				Detected:          true,
				Healthy:           true,
				HealthDescription: "Currently active",
				UpdateTime:        time.Now(),
			},
		},
	}
	node.ComputeClass()

	node2 := node.Copy()

	require.Equal(node.Attributes, node2.Attributes)
	require.Equal(node.Resources, node2.Resources)
	require.Equal(node.Reserved, node2.Reserved)
	require.Equal(node.Links, node2.Links)
	require.Equal(node.Meta, node2.Meta)
	require.Equal(node.Events, node2.Events)
	require.Equal(node.DrainStrategy, node2.DrainStrategy)
	require.Equal(node.Drivers, node2.Drivers)
}

func TestSpread_Validate(t *testing.T) {
	type tc struct {
		spread *Spread
		err    error
		name   string
	}

	testCases := []tc{
		{
			spread: &Spread{},
			err:    fmt.Errorf("Missing spread attribute"),
			name:   "empty spread",
		},
		{
			spread: &Spread{
				Attribute: "${node.datacenter}",
				Weight:    -1,
			},
			err:  fmt.Errorf("Spread stanza must have a positive weight from 0 to 100"),
			name: "Invalid weight",
		},
		{
			spread: &Spread{
				Attribute: "${node.datacenter}",
				Weight:    110,
			},
			err:  fmt.Errorf("Spread stanza must have a positive weight from 0 to 100"),
			name: "Invalid weight",
		},
		{
			spread: &Spread{
				Attribute: "${node.datacenter}",
				Weight:    50,
				SpreadTarget: []*SpreadTarget{
					{
						Value:   "dc1",
						Percent: 25,
					},
					{
						Value:   "dc2",
						Percent: 150,
					},
				},
			},
			err:  fmt.Errorf("Spread target percentage for value \"dc2\" must be between 0 and 100"),
			name: "Invalid percentages",
		},
		{
			spread: &Spread{
				Attribute: "${node.datacenter}",
				Weight:    50,
				SpreadTarget: []*SpreadTarget{
					{
						Value:   "dc1",
						Percent: 75,
					},
					{
						Value:   "dc2",
						Percent: 75,
					},
				},
			},
			err:  fmt.Errorf("Sum of spread target percentages must not be greater than 100%%; got %d%%", 150),
			name: "Invalid percentages",
		},
		{
			spread: &Spread{
				Attribute: "${node.datacenter}",
				Weight:    50,
				SpreadTarget: []*SpreadTarget{
					{
						Value:   "dc1",
						Percent: 25,
					},
					{
						Value:   "dc1",
						Percent: 50,
					},
				},
			},
			err:  fmt.Errorf("Spread target value \"dc1\" already defined"),
			name: "No spread targets",
		},
		{
			spread: &Spread{
				Attribute: "${node.datacenter}",
				Weight:    50,
				SpreadTarget: []*SpreadTarget{
					{
						Value:   "dc1",
						Percent: 25,
					},
					{
						Value:   "dc2",
						Percent: 50,
					},
				},
			},
			err:  nil,
			name: "Valid spread",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.spread.Validate()
			if tc.err != nil {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tc.err.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestNodeReservedNetworkResources_ParseReserved(t *testing.T) {
	require := require.New(t)
	cases := []struct {
		Input  string
		Parsed []uint64
		Err    bool
	}{
		{
			"1,2,3",
			[]uint64{1, 2, 3},
			false,
		},
		{
			"3,1,2,1,2,3,1-3",
			[]uint64{1, 2, 3},
			false,
		},
		{
			"3-1",
			nil,
			true,
		},
		{
			"1-3,2-4",
			[]uint64{1, 2, 3, 4},
			false,
		},
		{
			"1-3,4,5-5,6,7,8-10",
			[]uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			false,
		},
	}

	for i, tc := range cases {
		r := &NodeReservedNetworkResources{ReservedHostPorts: tc.Input}
		out, err := r.ParseReservedHostPorts()
		if (err != nil) != tc.Err {
			t.Fatalf("test case %d: %v", i, err)
			continue
		}

		require.Equal(out, tc.Parsed)
	}
}

func TestMultiregion_CopyCanonicalize(t *testing.T) {
	require := require.New(t)

	emptyOld := &Multiregion{}
	expected := &Multiregion{
		Strategy: &MultiregionStrategy{},
		Regions:  []*MultiregionRegion{},
	}

	old := emptyOld.Copy()
	old.Canonicalize()
	require.Equal(old, expected)
	require.False(old.Diff(expected))

	nonEmptyOld := &Multiregion{
		Strategy: &MultiregionStrategy{
			MaxParallel: 2,
			OnFailure:   "fail_all",
		},
		Regions: []*MultiregionRegion{
			{
				Name:        "west",
				Count:       2,
				Datacenters: []string{"west-1", "west-2"},
				Meta:        map[string]string{},
			},
			{
				Name:        "east",
				Count:       1,
				Datacenters: []string{"east-1"},
				Meta:        map[string]string{},
			},
		},
	}

	old = nonEmptyOld.Copy()
	old.Canonicalize()
	require.Equal(old, nonEmptyOld)
	require.False(old.Diff(nonEmptyOld))
}

func TestNodeResources_Merge(t *testing.T) {
	res := &NodeResources{
		Cpu: NodeCpuResources{
			CpuShares: int64(32000),
		},
		Memory: NodeMemoryResources{
			MemoryMB: int64(64000),
		},
		Networks: Networks{
			{
				Device: "foo",
			},
		},
	}

	res.Merge(&NodeResources{
		Memory: NodeMemoryResources{
			MemoryMB: int64(100000),
		},
		Networks: Networks{
			{
				Mode: "foo/bar",
			},
		},
	})

	require.Exactly(t, &NodeResources{
		Cpu: NodeCpuResources{
			CpuShares: int64(32000),
		},
		Memory: NodeMemoryResources{
			MemoryMB: int64(100000),
		},
		Networks: Networks{
			{
				Device: "foo",
			},
			{
				Mode: "foo/bar",
			},
		},
	}, res)
}

func TestAllocatedResources_Canonicalize(t *testing.T) {
	cases := map[string]struct {
		input    *AllocatedResources
		expected *AllocatedResources
	}{
		"base": {
			input: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"task": {
						Networks: Networks{
							{
								IP:           "127.0.0.1",
								DynamicPorts: []Port{{"admin", 8080, 0, "default"}},
							},
						},
					},
				},
			},
			expected: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"task": {
						Networks: Networks{
							{
								IP:           "127.0.0.1",
								DynamicPorts: []Port{{"admin", 8080, 0, "default"}},
							},
						},
					},
				},
				Shared: AllocatedSharedResources{
					Ports: AllocatedPorts{
						{
							Label:  "admin",
							Value:  8080,
							To:     0,
							HostIP: "127.0.0.1",
						},
					},
				},
			},
		},
		"base with existing": {
			input: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"task": {
						Networks: Networks{
							{
								IP:           "127.0.0.1",
								DynamicPorts: []Port{{"admin", 8080, 0, "default"}},
							},
						},
					},
				},
				Shared: AllocatedSharedResources{
					Ports: AllocatedPorts{
						{
							Label:  "http",
							Value:  80,
							To:     8080,
							HostIP: "127.0.0.1",
						},
					},
				},
			},
			expected: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"task": {
						Networks: Networks{
							{
								IP:           "127.0.0.1",
								DynamicPorts: []Port{{"admin", 8080, 0, "default"}},
							},
						},
					},
				},
				Shared: AllocatedSharedResources{
					Ports: AllocatedPorts{
						{
							Label:  "http",
							Value:  80,
							To:     8080,
							HostIP: "127.0.0.1",
						},
						{
							Label:  "admin",
							Value:  8080,
							To:     0,
							HostIP: "127.0.0.1",
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		tc.input.Canonicalize()
		require.Exactly(t, tc.expected, tc.input, "case %s did not match", name)
	}
}

func TestAllocatedSharedResources_Canonicalize(t *testing.T) {
	a := &AllocatedSharedResources{
		Networks: []*NetworkResource{
			{
				IP: "127.0.0.1",
				DynamicPorts: []Port{
					{
						Label: "http",
						Value: 22222,
						To:    8080,
					},
				},
				ReservedPorts: []Port{
					{
						Label: "redis",
						Value: 6783,
						To:    6783,
					},
				},
			},
		},
	}

	a.Canonicalize()
	require.Exactly(t, AllocatedPorts{
		{
			Label:  "http",
			Value:  22222,
			To:     8080,
			HostIP: "127.0.0.1",
		},
		{
			Label:  "redis",
			Value:  6783,
			To:     6783,
			HostIP: "127.0.0.1",
		},
	}, a.Ports)
}

func TestTaskGroup_validateScriptChecksInGroupServices(t *testing.T) {
	t.Run("service task not set", func(t *testing.T) {
		tg := &TaskGroup{
			Name: "group1",
			Services: []*Service{{
				Name:     "service1",
				TaskName: "", // unset
				Checks: []*ServiceCheck{{
					Name:     "check1",
					Type:     "script",
					TaskName: "", // unset
				}, {
					Name: "check2",
					Type: "ttl", // not script
				}, {
					Name:     "check3",
					Type:     "script",
					TaskName: "", // unset
				}},
			}, {
				Name: "service2",
				Checks: []*ServiceCheck{{
					Type:     "script",
					TaskName: "task1", // set
				}},
			}, {
				Name:     "service3",
				TaskName: "", // unset
				Checks: []*ServiceCheck{{
					Name:     "check1",
					Type:     "script",
					TaskName: "", // unset
				}},
			}},
		}

		errStr := tg.validateScriptChecksInGroupServices().Error()
		require.Contains(t, errStr, "Service [group1]->service1 or Check check1 must specify task parameter")
		require.Contains(t, errStr, "Service [group1]->service1 or Check check3 must specify task parameter")
		require.Contains(t, errStr, "Service [group1]->service3 or Check check1 must specify task parameter")
	})

	t.Run("service task set", func(t *testing.T) {
		tgOK := &TaskGroup{
			Name: "group1",
			Services: []*Service{{
				Name:     "service1",
				TaskName: "task1",
				Checks: []*ServiceCheck{{
					Name: "check1",
					Type: "script",
				}, {
					Name: "check2",
					Type: "ttl",
				}, {
					Name: "check3",
					Type: "script",
				}},
			}},
		}

		mErrOK := tgOK.validateScriptChecksInGroupServices()
		require.Nil(t, mErrOK)
	})
}
