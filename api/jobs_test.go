package api

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func TestJobs_Register(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.List(nil)
	require.Nil(err)
	require.Emptyf(resp, "expected 0 jobs, got: %d", len(resp))

	// Create a job and attempt to register it
	job := testJob()
	resp2, wm, err := jobs.Register(job, nil)
	require.Nil(err)
	require.NotNil(resp2)
	require.NotEmpty(resp2.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	require.Nil(err)

	// Check that we got the expected response
	if len(resp) != 1 || resp[0].ID != *job.ID {
		t.Fatalf("bad: %#v", resp[0])
	}
}

func TestJobs_Register_PreserveCounts(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.List(nil)
	require.Nil(err)
	require.Emptyf(resp, "expected 0 jobs, got: %d", len(resp))

	// Create a job
	task := NewTask("task", "exec").
		SetConfig("command", "/bin/sleep").
		Require(&Resources{
			CPU:      intToPtr(100),
			MemoryMB: intToPtr(256),
		}).
		SetLogConfig(&LogConfig{
			MaxFiles:      intToPtr(1),
			MaxFileSizeMB: intToPtr(2),
		})

	group1 := NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: intToPtr(25),
		})
	group2 := NewTaskGroup("group2", 2).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: intToPtr(25),
		})

	job := NewBatchJob("job", "redis", "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group1).
		AddTaskGroup(group2)

	// Create a job and register it
	resp2, wm, err := jobs.Register(job, nil)
	require.Nil(err)
	require.NotNil(resp2)
	require.NotEmpty(resp2.EvalID)
	assertWriteMeta(t, wm)

	// Update the job, new groups to test PreserveCounts
	group1.Count = nil
	group2.Count = intToPtr(0)
	group3 := NewTaskGroup("group3", 3).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: intToPtr(25),
		})
	job.AddTaskGroup(group3)

	// Update the job, with PreserveCounts = true
	_, _, err = jobs.RegisterOpts(job, &RegisterOptions{
		PreserveCounts: true,
	}, nil)
	require.NoError(err)

	// Query the job scale status
	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	require.NoError(err)
	require.Equal(1, status.TaskGroups["group1"].Desired) // present and nil => preserved
	require.Equal(2, status.TaskGroups["group2"].Desired) // present and specified => preserved
	require.Equal(3, status.TaskGroups["group3"].Desired) // new => as specific in job spec
}

func TestJobs_Register_NoPreserveCounts(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.List(nil)
	require.Nil(err)
	require.Emptyf(resp, "expected 0 jobs, got: %d", len(resp))

	// Create a job
	task := NewTask("task", "exec").
		SetConfig("command", "/bin/sleep").
		Require(&Resources{
			CPU:      intToPtr(100),
			MemoryMB: intToPtr(256),
		}).
		SetLogConfig(&LogConfig{
			MaxFiles:      intToPtr(1),
			MaxFileSizeMB: intToPtr(2),
		})

	group1 := NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: intToPtr(25),
		})
	group2 := NewTaskGroup("group2", 2).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: intToPtr(25),
		})

	job := NewBatchJob("job", "redis", "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group1).
		AddTaskGroup(group2)

	// Create a job and register it
	resp2, wm, err := jobs.Register(job, nil)
	require.Nil(err)
	require.NotNil(resp2)
	require.NotEmpty(resp2.EvalID)
	assertWriteMeta(t, wm)

	// Update the job, new groups to test PreserveCounts
	group1.Count = intToPtr(0)
	group2.Count = nil
	group3 := NewTaskGroup("group3", 3).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: intToPtr(25),
		})
	job.AddTaskGroup(group3)

	// Update the job, with PreserveCounts = default [false]
	_, _, err = jobs.Register(job, nil)
	require.NoError(err)

	// Query the job scale status
	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	require.NoError(err)
	require.Equal("default", status.Namespace)
	require.Equal(0, status.TaskGroups["group1"].Desired) // present => as specified
	require.Equal(1, status.TaskGroups["group2"].Desired) // nil     => default (1)
	require.Equal(3, status.TaskGroups["group3"].Desired) // new     => as specified
}

func TestJobs_Validate(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Create a job and attempt to register it
	job := testJob()
	resp, _, err := jobs.Validate(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(resp.ValidationErrors) != 0 {
		t.Fatalf("bad %v", resp)
	}

	job.ID = nil
	resp1, _, err := jobs.Validate(job, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(resp1.ValidationErrors) == 0 {
		t.Fatalf("bad %v", resp1)
	}
}

func TestJobs_Canonicalize(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		expected *Job
		input    *Job
	}{
		{
			name: "empty",
			input: &Job{
				TaskGroups: []*TaskGroup{
					{
						Tasks: []*Task{
							{},
						},
					},
				},
			},
			expected: &Job{
				ID:                stringToPtr(""),
				Name:              stringToPtr(""),
				Region:            stringToPtr("global"),
				Namespace:         stringToPtr(DefaultNamespace),
				Type:              stringToPtr("service"),
				ParentID:          stringToPtr(""),
				Priority:          intToPtr(50),
				AllAtOnce:         boolToPtr(false),
				ConsulToken:       stringToPtr(""),
				ConsulNamespace:   stringToPtr(""),
				VaultToken:        stringToPtr(""),
				VaultNamespace:    stringToPtr(""),
				NomadTokenID:      stringToPtr(""),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
				Update: &UpdateStrategy{
					Stagger:          timeToPtr(30 * time.Second),
					MaxParallel:      intToPtr(1),
					HealthCheck:      stringToPtr("checks"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(5 * time.Minute),
					ProgressDeadline: timeToPtr(10 * time.Minute),
					AutoRevert:       boolToPtr(false),
					Canary:           intToPtr(0),
					AutoPromote:      boolToPtr(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  stringToPtr(""),
						Count: intToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  boolToPtr(false),
							Migrate: boolToPtr(false),
							SizeMB:  intToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(15 * time.Second),
							Attempts: intToPtr(2),
							Interval: timeToPtr(30 * time.Minute),
							Mode:     stringToPtr("fail"),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      intToPtr(0),
							Interval:      timeToPtr(0),
							DelayFunction: stringToPtr("exponential"),
							Delay:         timeToPtr(30 * time.Second),
							MaxDelay:      timeToPtr(1 * time.Hour),
							Unlimited:     boolToPtr(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(30 * time.Second),
							MaxParallel:      intToPtr(1),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(10 * time.Second),
							HealthyDeadline:  timeToPtr(5 * time.Minute),
							ProgressDeadline: timeToPtr(10 * time.Minute),
							AutoRevert:       boolToPtr(false),
							Canary:           intToPtr(0),
							AutoPromote:      boolToPtr(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								KillTimeout:   timeToPtr(5 * time.Second),
								LogConfig:     DefaultLogConfig(),
								Resources:     DefaultResources(),
								RestartPolicy: defaultServiceJobRestartPolicy(),
							},
						},
					},
				},
			},
		},
		{
			name: "batch",
			input: &Job{
				Type: stringToPtr("batch"),
				TaskGroups: []*TaskGroup{
					{
						Tasks: []*Task{
							{},
						},
					},
				},
			},
			expected: &Job{
				ID:                stringToPtr(""),
				Name:              stringToPtr(""),
				Region:            stringToPtr("global"),
				Namespace:         stringToPtr(DefaultNamespace),
				Type:              stringToPtr("batch"),
				ParentID:          stringToPtr(""),
				Priority:          intToPtr(50),
				AllAtOnce:         boolToPtr(false),
				ConsulToken:       stringToPtr(""),
				ConsulNamespace:   stringToPtr(""),
				VaultToken:        stringToPtr(""),
				VaultNamespace:    stringToPtr(""),
				NomadTokenID:      stringToPtr(""),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
				TaskGroups: []*TaskGroup{
					{
						Name:  stringToPtr(""),
						Count: intToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  boolToPtr(false),
							Migrate: boolToPtr(false),
							SizeMB:  intToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(15 * time.Second),
							Attempts: intToPtr(3),
							Interval: timeToPtr(24 * time.Hour),
							Mode:     stringToPtr("fail"),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      intToPtr(1),
							Interval:      timeToPtr(24 * time.Hour),
							DelayFunction: stringToPtr("constant"),
							Delay:         timeToPtr(5 * time.Second),
							MaxDelay:      timeToPtr(0),
							Unlimited:     boolToPtr(false),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Tasks: []*Task{
							{
								KillTimeout:   timeToPtr(5 * time.Second),
								LogConfig:     DefaultLogConfig(),
								Resources:     DefaultResources(),
								RestartPolicy: defaultBatchJobRestartPolicy(),
							},
						},
					},
				},
			},
		},
		{
			name: "partial",
			input: &Job{
				Name:      stringToPtr("foo"),
				Namespace: stringToPtr("bar"),
				ID:        stringToPtr("bar"),
				ParentID:  stringToPtr("lol"),
				TaskGroups: []*TaskGroup{
					{
						Name: stringToPtr("bar"),
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
				},
			},
			expected: &Job{
				Namespace:         stringToPtr("bar"),
				ID:                stringToPtr("bar"),
				Name:              stringToPtr("foo"),
				Region:            stringToPtr("global"),
				Type:              stringToPtr("service"),
				ParentID:          stringToPtr("lol"),
				Priority:          intToPtr(50),
				AllAtOnce:         boolToPtr(false),
				ConsulToken:       stringToPtr(""),
				ConsulNamespace:   stringToPtr(""),
				VaultToken:        stringToPtr(""),
				VaultNamespace:    stringToPtr(""),
				NomadTokenID:      stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
				Update: &UpdateStrategy{
					Stagger:          timeToPtr(30 * time.Second),
					MaxParallel:      intToPtr(1),
					HealthCheck:      stringToPtr("checks"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(5 * time.Minute),
					ProgressDeadline: timeToPtr(10 * time.Minute),
					AutoRevert:       boolToPtr(false),
					Canary:           intToPtr(0),
					AutoPromote:      boolToPtr(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  stringToPtr("bar"),
						Count: intToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  boolToPtr(false),
							Migrate: boolToPtr(false),
							SizeMB:  intToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(15 * time.Second),
							Attempts: intToPtr(2),
							Interval: timeToPtr(30 * time.Minute),
							Mode:     stringToPtr("fail"),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      intToPtr(0),
							Interval:      timeToPtr(0),
							DelayFunction: stringToPtr("exponential"),
							Delay:         timeToPtr(30 * time.Second),
							MaxDelay:      timeToPtr(1 * time.Hour),
							Unlimited:     boolToPtr(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(30 * time.Second),
							MaxParallel:      intToPtr(1),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(10 * time.Second),
							HealthyDeadline:  timeToPtr(5 * time.Minute),
							ProgressDeadline: timeToPtr(10 * time.Minute),
							AutoRevert:       boolToPtr(false),
							Canary:           intToPtr(0),
							AutoPromote:      boolToPtr(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:          "task1",
								LogConfig:     DefaultLogConfig(),
								Resources:     DefaultResources(),
								KillTimeout:   timeToPtr(5 * time.Second),
								RestartPolicy: defaultServiceJobRestartPolicy(),
							},
						},
					},
				},
			},
		},
		{
			name: "example_template",
			input: &Job{
				ID:          stringToPtr("example_template"),
				Name:        stringToPtr("example_template"),
				Datacenters: []string{"dc1"},
				Type:        stringToPtr("service"),
				Update: &UpdateStrategy{
					MaxParallel: intToPtr(1),
					AutoPromote: boolToPtr(true),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  stringToPtr("cache"),
						Count: intToPtr(1),
						RestartPolicy: &RestartPolicy{
							Interval: timeToPtr(5 * time.Minute),
							Attempts: intToPtr(10),
							Delay:    timeToPtr(25 * time.Second),
							Mode:     stringToPtr("delay"),
						},
						Update: &UpdateStrategy{
							AutoRevert: boolToPtr(true),
						},
						EphemeralDisk: &EphemeralDisk{
							SizeMB: intToPtr(300),
						},
						Tasks: []*Task{
							{
								Name:   "redis",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "redis:3.2",
									"port_map": []map[string]int{{
										"db": 6379,
									}},
								},
								RestartPolicy: &RestartPolicy{
									// inherit other values from TG
									Attempts: intToPtr(20),
								},
								Resources: &Resources{
									CPU:      intToPtr(500),
									MemoryMB: intToPtr(256),
									Networks: []*NetworkResource{
										{
											MBits: intToPtr(10),
											DynamicPorts: []Port{
												{
													Label: "db",
												},
											},
										},
									},
								},
								Services: []*Service{
									{
										Name:       "redis-cache",
										Tags:       []string{"global", "cache"},
										CanaryTags: []string{"canary", "global", "cache"},
										PortLabel:  "db",
										Checks: []ServiceCheck{
											{
												Name:     "alive",
												Type:     "tcp",
												Interval: 10 * time.Second,
												Timeout:  2 * time.Second,
											},
										},
									},
								},
								Templates: []*Template{
									{
										EmbeddedTmpl: stringToPtr("---"),
										DestPath:     stringToPtr("local/file.yml"),
									},
									{
										EmbeddedTmpl: stringToPtr("FOO=bar\n"),
										DestPath:     stringToPtr("local/file.env"),
										Envvars:      boolToPtr(true),
									},
								},
							},
						},
					},
				},
			},
			expected: &Job{
				Namespace:         stringToPtr(DefaultNamespace),
				ID:                stringToPtr("example_template"),
				Name:              stringToPtr("example_template"),
				ParentID:          stringToPtr(""),
				Priority:          intToPtr(50),
				Region:            stringToPtr("global"),
				Type:              stringToPtr("service"),
				AllAtOnce:         boolToPtr(false),
				ConsulToken:       stringToPtr(""),
				ConsulNamespace:   stringToPtr(""),
				VaultToken:        stringToPtr(""),
				VaultNamespace:    stringToPtr(""),
				NomadTokenID:      stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
				Datacenters:       []string{"dc1"},
				Update: &UpdateStrategy{
					Stagger:          timeToPtr(30 * time.Second),
					MaxParallel:      intToPtr(1),
					HealthCheck:      stringToPtr("checks"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(5 * time.Minute),
					ProgressDeadline: timeToPtr(10 * time.Minute),
					AutoRevert:       boolToPtr(false),
					Canary:           intToPtr(0),
					AutoPromote:      boolToPtr(true),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  stringToPtr("cache"),
						Count: intToPtr(1),
						RestartPolicy: &RestartPolicy{
							Interval: timeToPtr(5 * time.Minute),
							Attempts: intToPtr(10),
							Delay:    timeToPtr(25 * time.Second),
							Mode:     stringToPtr("delay"),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      intToPtr(0),
							Interval:      timeToPtr(0),
							DelayFunction: stringToPtr("exponential"),
							Delay:         timeToPtr(30 * time.Second),
							MaxDelay:      timeToPtr(1 * time.Hour),
							Unlimited:     boolToPtr(true),
						},
						EphemeralDisk: &EphemeralDisk{
							Sticky:  boolToPtr(false),
							Migrate: boolToPtr(false),
							SizeMB:  intToPtr(300),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(30 * time.Second),
							MaxParallel:      intToPtr(1),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(10 * time.Second),
							HealthyDeadline:  timeToPtr(5 * time.Minute),
							ProgressDeadline: timeToPtr(10 * time.Minute),
							AutoRevert:       boolToPtr(true),
							Canary:           intToPtr(0),
							AutoPromote:      boolToPtr(true),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:   "redis",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "redis:3.2",
									"port_map": []map[string]int{{
										"db": 6379,
									}},
								},
								RestartPolicy: &RestartPolicy{
									Interval: timeToPtr(5 * time.Minute),
									Attempts: intToPtr(20),
									Delay:    timeToPtr(25 * time.Second),
									Mode:     stringToPtr("delay"),
								},
								Resources: &Resources{
									CPU:      intToPtr(500),
									Cores:    intToPtr(0),
									MemoryMB: intToPtr(256),
									Networks: []*NetworkResource{
										{
											MBits: intToPtr(10),
											DynamicPorts: []Port{
												{
													Label: "db",
												},
											},
										},
									},
								},
								Services: []*Service{
									{
										Name:        "redis-cache",
										Tags:        []string{"global", "cache"},
										CanaryTags:  []string{"canary", "global", "cache"},
										PortLabel:   "db",
										AddressMode: "auto",
										OnUpdate:    "require_healthy",
										Checks: []ServiceCheck{
											{
												Name:     "alive",
												Type:     "tcp",
												Interval: 10 * time.Second,
												Timeout:  2 * time.Second,
												OnUpdate: "require_healthy",
											},
										},
									},
								},
								KillTimeout: timeToPtr(5 * time.Second),
								LogConfig:   DefaultLogConfig(),
								Templates: []*Template{
									{
										SourcePath:   stringToPtr(""),
										DestPath:     stringToPtr("local/file.yml"),
										EmbeddedTmpl: stringToPtr("---"),
										ChangeMode:   stringToPtr("restart"),
										ChangeSignal: stringToPtr(""),
										Splay:        timeToPtr(5 * time.Second),
										Perms:        stringToPtr("0644"),
										LeftDelim:    stringToPtr("{{"),
										RightDelim:   stringToPtr("}}"),
										Envvars:      boolToPtr(false),
										VaultGrace:   timeToPtr(0),
									},
									{
										SourcePath:   stringToPtr(""),
										DestPath:     stringToPtr("local/file.env"),
										EmbeddedTmpl: stringToPtr("FOO=bar\n"),
										ChangeMode:   stringToPtr("restart"),
										ChangeSignal: stringToPtr(""),
										Splay:        timeToPtr(5 * time.Second),
										Perms:        stringToPtr("0644"),
										LeftDelim:    stringToPtr("{{"),
										RightDelim:   stringToPtr("}}"),
										Envvars:      boolToPtr(true),
										VaultGrace:   timeToPtr(0),
									},
								},
							},
						},
					},
				},
			},
		},

		{
			name: "periodic",
			input: &Job{
				ID:       stringToPtr("bar"),
				Periodic: &PeriodicConfig{},
			},
			expected: &Job{
				Namespace:         stringToPtr(DefaultNamespace),
				ID:                stringToPtr("bar"),
				ParentID:          stringToPtr(""),
				Name:              stringToPtr("bar"),
				Region:            stringToPtr("global"),
				Type:              stringToPtr("service"),
				Priority:          intToPtr(50),
				AllAtOnce:         boolToPtr(false),
				ConsulToken:       stringToPtr(""),
				ConsulNamespace:   stringToPtr(""),
				VaultToken:        stringToPtr(""),
				VaultNamespace:    stringToPtr(""),
				NomadTokenID:      stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
				Update: &UpdateStrategy{
					Stagger:          timeToPtr(30 * time.Second),
					MaxParallel:      intToPtr(1),
					HealthCheck:      stringToPtr("checks"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(5 * time.Minute),
					ProgressDeadline: timeToPtr(10 * time.Minute),
					AutoRevert:       boolToPtr(false),
					Canary:           intToPtr(0),
					AutoPromote:      boolToPtr(false),
				},
				Periodic: &PeriodicConfig{
					Enabled:         boolToPtr(true),
					Spec:            stringToPtr(""),
					SpecType:        stringToPtr(PeriodicSpecCron),
					ProhibitOverlap: boolToPtr(false),
					TimeZone:        stringToPtr("UTC"),
				},
			},
		},

		{
			name: "update_merge",
			input: &Job{
				Name:     stringToPtr("foo"),
				ID:       stringToPtr("bar"),
				ParentID: stringToPtr("lol"),
				Update: &UpdateStrategy{
					Stagger:          timeToPtr(1 * time.Second),
					MaxParallel:      intToPtr(1),
					HealthCheck:      stringToPtr("checks"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(6 * time.Minute),
					ProgressDeadline: timeToPtr(7 * time.Minute),
					AutoRevert:       boolToPtr(false),
					Canary:           intToPtr(0),
					AutoPromote:      boolToPtr(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name: stringToPtr("bar"),
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:        timeToPtr(2 * time.Second),
							MaxParallel:    intToPtr(2),
							HealthCheck:    stringToPtr("manual"),
							MinHealthyTime: timeToPtr(1 * time.Second),
							AutoRevert:     boolToPtr(true),
							Canary:         intToPtr(1),
							AutoPromote:    boolToPtr(true),
						},
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
					{
						Name: stringToPtr("baz"),
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
				},
			},
			expected: &Job{
				Namespace:         stringToPtr(DefaultNamespace),
				ID:                stringToPtr("bar"),
				Name:              stringToPtr("foo"),
				Region:            stringToPtr("global"),
				Type:              stringToPtr("service"),
				ParentID:          stringToPtr("lol"),
				Priority:          intToPtr(50),
				AllAtOnce:         boolToPtr(false),
				ConsulToken:       stringToPtr(""),
				ConsulNamespace:   stringToPtr(""),
				VaultToken:        stringToPtr(""),
				VaultNamespace:    stringToPtr(""),
				NomadTokenID:      stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
				Update: &UpdateStrategy{
					Stagger:          timeToPtr(1 * time.Second),
					MaxParallel:      intToPtr(1),
					HealthCheck:      stringToPtr("checks"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(6 * time.Minute),
					ProgressDeadline: timeToPtr(7 * time.Minute),
					AutoRevert:       boolToPtr(false),
					Canary:           intToPtr(0),
					AutoPromote:      boolToPtr(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  stringToPtr("bar"),
						Count: intToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  boolToPtr(false),
							Migrate: boolToPtr(false),
							SizeMB:  intToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(15 * time.Second),
							Attempts: intToPtr(2),
							Interval: timeToPtr(30 * time.Minute),
							Mode:     stringToPtr("fail"),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      intToPtr(0),
							Interval:      timeToPtr(0),
							DelayFunction: stringToPtr("exponential"),
							Delay:         timeToPtr(30 * time.Second),
							MaxDelay:      timeToPtr(1 * time.Hour),
							Unlimited:     boolToPtr(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(2 * time.Second),
							MaxParallel:      intToPtr(2),
							HealthCheck:      stringToPtr("manual"),
							MinHealthyTime:   timeToPtr(1 * time.Second),
							HealthyDeadline:  timeToPtr(6 * time.Minute),
							ProgressDeadline: timeToPtr(7 * time.Minute),
							AutoRevert:       boolToPtr(true),
							Canary:           intToPtr(1),
							AutoPromote:      boolToPtr(true),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:          "task1",
								LogConfig:     DefaultLogConfig(),
								Resources:     DefaultResources(),
								KillTimeout:   timeToPtr(5 * time.Second),
								RestartPolicy: defaultServiceJobRestartPolicy(),
							},
						},
					},
					{
						Name:  stringToPtr("baz"),
						Count: intToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  boolToPtr(false),
							Migrate: boolToPtr(false),
							SizeMB:  intToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(15 * time.Second),
							Attempts: intToPtr(2),
							Interval: timeToPtr(30 * time.Minute),
							Mode:     stringToPtr("fail"),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      intToPtr(0),
							Interval:      timeToPtr(0),
							DelayFunction: stringToPtr("exponential"),
							Delay:         timeToPtr(30 * time.Second),
							MaxDelay:      timeToPtr(1 * time.Hour),
							Unlimited:     boolToPtr(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(1 * time.Second),
							MaxParallel:      intToPtr(1),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(10 * time.Second),
							HealthyDeadline:  timeToPtr(6 * time.Minute),
							ProgressDeadline: timeToPtr(7 * time.Minute),
							AutoRevert:       boolToPtr(false),
							Canary:           intToPtr(0),
							AutoPromote:      boolToPtr(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:          "task1",
								LogConfig:     DefaultLogConfig(),
								Resources:     DefaultResources(),
								KillTimeout:   timeToPtr(5 * time.Second),
								RestartPolicy: defaultServiceJobRestartPolicy(),
							},
						},
					},
				},
			},
		},

		{
			name: "restart_merge",
			input: &Job{
				Name:     stringToPtr("foo"),
				ID:       stringToPtr("bar"),
				ParentID: stringToPtr("lol"),
				TaskGroups: []*TaskGroup{
					{
						Name: stringToPtr("bar"),
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(15 * time.Second),
							Attempts: intToPtr(2),
							Interval: timeToPtr(30 * time.Minute),
							Mode:     stringToPtr("fail"),
						},
						Tasks: []*Task{
							{
								Name: "task1",
								RestartPolicy: &RestartPolicy{
									Attempts: intToPtr(5),
									Delay:    timeToPtr(1 * time.Second),
								},
							},
						},
					},
					{
						Name: stringToPtr("baz"),
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(20 * time.Second),
							Attempts: intToPtr(2),
							Interval: timeToPtr(30 * time.Minute),
							Mode:     stringToPtr("fail"),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
				},
			},
			expected: &Job{
				Namespace:         stringToPtr(DefaultNamespace),
				ID:                stringToPtr("bar"),
				Name:              stringToPtr("foo"),
				Region:            stringToPtr("global"),
				Type:              stringToPtr("service"),
				ParentID:          stringToPtr("lol"),
				Priority:          intToPtr(50),
				AllAtOnce:         boolToPtr(false),
				ConsulToken:       stringToPtr(""),
				ConsulNamespace:   stringToPtr(""),
				VaultToken:        stringToPtr(""),
				VaultNamespace:    stringToPtr(""),
				NomadTokenID:      stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
				Update: &UpdateStrategy{
					Stagger:          timeToPtr(30 * time.Second),
					MaxParallel:      intToPtr(1),
					HealthCheck:      stringToPtr("checks"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(5 * time.Minute),
					ProgressDeadline: timeToPtr(10 * time.Minute),
					AutoRevert:       boolToPtr(false),
					Canary:           intToPtr(0),
					AutoPromote:      boolToPtr(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  stringToPtr("bar"),
						Count: intToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  boolToPtr(false),
							Migrate: boolToPtr(false),
							SizeMB:  intToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(15 * time.Second),
							Attempts: intToPtr(2),
							Interval: timeToPtr(30 * time.Minute),
							Mode:     stringToPtr("fail"),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      intToPtr(0),
							Interval:      timeToPtr(0),
							DelayFunction: stringToPtr("exponential"),
							Delay:         timeToPtr(30 * time.Second),
							MaxDelay:      timeToPtr(1 * time.Hour),
							Unlimited:     boolToPtr(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(30 * time.Second),
							MaxParallel:      intToPtr(1),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(10 * time.Second),
							HealthyDeadline:  timeToPtr(5 * time.Minute),
							ProgressDeadline: timeToPtr(10 * time.Minute),
							AutoRevert:       boolToPtr(false),
							Canary:           intToPtr(0),
							AutoPromote:      boolToPtr(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   DefaultResources(),
								KillTimeout: timeToPtr(5 * time.Second),
								RestartPolicy: &RestartPolicy{
									Attempts: intToPtr(5),
									Delay:    timeToPtr(1 * time.Second),
									Interval: timeToPtr(30 * time.Minute),
									Mode:     stringToPtr("fail"),
								},
							},
						},
					},
					{
						Name:  stringToPtr("baz"),
						Count: intToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  boolToPtr(false),
							Migrate: boolToPtr(false),
							SizeMB:  intToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    timeToPtr(20 * time.Second),
							Attempts: intToPtr(2),
							Interval: timeToPtr(30 * time.Minute),
							Mode:     stringToPtr("fail"),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      intToPtr(0),
							Interval:      timeToPtr(0),
							DelayFunction: stringToPtr("exponential"),
							Delay:         timeToPtr(30 * time.Second),
							MaxDelay:      timeToPtr(1 * time.Hour),
							Unlimited:     boolToPtr(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(30 * time.Second),
							MaxParallel:      intToPtr(1),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(10 * time.Second),
							HealthyDeadline:  timeToPtr(5 * time.Minute),
							ProgressDeadline: timeToPtr(10 * time.Minute),
							AutoRevert:       boolToPtr(false),
							Canary:           intToPtr(0),
							AutoPromote:      boolToPtr(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   DefaultResources(),
								KillTimeout: timeToPtr(5 * time.Second),
								RestartPolicy: &RestartPolicy{
									Delay:    timeToPtr(20 * time.Second),
									Attempts: intToPtr(2),
									Interval: timeToPtr(30 * time.Minute),
									Mode:     stringToPtr("fail"),
								},
							},
						},
					},
				},
			},
		},

		{
			name: "multiregion",
			input: &Job{
				Name:     stringToPtr("foo"),
				ID:       stringToPtr("bar"),
				ParentID: stringToPtr("lol"),
				Multiregion: &Multiregion{
					Regions: []*MultiregionRegion{
						{
							Name:  "west",
							Count: intToPtr(1),
						},
					},
				},
			},
			expected: &Job{
				Multiregion: &Multiregion{
					Strategy: &MultiregionStrategy{
						MaxParallel: intToPtr(0),
						OnFailure:   stringToPtr(""),
					},
					Regions: []*MultiregionRegion{
						{
							Name:        "west",
							Count:       intToPtr(1),
							Datacenters: []string{},
							Meta:        map[string]string{},
						},
					},
				},
				Namespace:         stringToPtr(DefaultNamespace),
				ID:                stringToPtr("bar"),
				Name:              stringToPtr("foo"),
				Region:            stringToPtr("global"),
				Type:              stringToPtr("service"),
				ParentID:          stringToPtr("lol"),
				Priority:          intToPtr(50),
				AllAtOnce:         boolToPtr(false),
				ConsulToken:       stringToPtr(""),
				ConsulNamespace:   stringToPtr(""),
				VaultToken:        stringToPtr(""),
				VaultNamespace:    stringToPtr(""),
				NomadTokenID:      stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
				Update: &UpdateStrategy{
					Stagger:          timeToPtr(30 * time.Second),
					MaxParallel:      intToPtr(1),
					HealthCheck:      stringToPtr("checks"),
					MinHealthyTime:   timeToPtr(10 * time.Second),
					HealthyDeadline:  timeToPtr(5 * time.Minute),
					ProgressDeadline: timeToPtr(10 * time.Minute),
					AutoRevert:       boolToPtr(false),
					Canary:           intToPtr(0),
					AutoPromote:      boolToPtr(false),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.input.Canonicalize()
			if !reflect.DeepEqual(tc.input, tc.expected) {
				t.Fatalf("Name: %v, Diffs:\n%v", tc.name, pretty.Diff(tc.expected, tc.input))
			}
		})
	}
}

func TestJobs_EnforceRegister(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.List(nil)
	require.Nil(err)
	require.Empty(resp)

	// Create a job and attempt to register it with an incorrect index.
	job := testJob()
	resp2, _, err := jobs.EnforceRegister(job, 10, nil)
	require.NotNil(err)
	require.Contains(err.Error(), RegisterEnforceIndexErrPrefix)

	// Register
	resp2, wm, err := jobs.EnforceRegister(job, 0, nil)
	require.Nil(err)
	require.NotNil(resp2)
	require.NotZero(resp2.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err := jobs.List(nil)
	require.Nil(err)
	require.Len(resp, 1)
	require.Equal(*job.ID, resp[0].ID)
	assertQueryMeta(t, qm)

	// Fail at incorrect index
	curIndex := resp[0].JobModifyIndex
	resp2, _, err = jobs.EnforceRegister(job, 123456, nil)
	require.NotNil(err)
	require.Contains(err.Error(), RegisterEnforceIndexErrPrefix)

	// Works at correct index
	resp3, wm, err := jobs.EnforceRegister(job, curIndex, nil)
	require.Nil(err)
	require.NotNil(resp3)
	require.NotZero(resp3.EvalID)
	assertWriteMeta(t, wm)
}

func TestJobs_Revert(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Register twice
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp == nil || resp.EvalID == "" {
		t.Fatalf("missing eval id")
	}
	assertWriteMeta(t, wm)

	job.Meta = map[string]string{"foo": "new"}
	resp, wm, err = jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp == nil || resp.EvalID == "" {
		t.Fatalf("missing eval id")
	}
	assertWriteMeta(t, wm)

	// Fail revert at incorrect enforce
	_, _, err = jobs.Revert(*job.ID, 0, uint64ToPtr(10), nil, "", "")
	if err == nil || !strings.Contains(err.Error(), "enforcing version") {
		t.Fatalf("expected enforcement error: %v", err)
	}

	// Works at correct index
	revertResp, wm, err := jobs.Revert(*job.ID, 0, uint64ToPtr(1), nil, "", "")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if revertResp.EvalID == "" {
		t.Fatalf("missing eval id")
	}
	if revertResp.EvalCreateIndex == 0 {
		t.Fatalf("bad eval create index")
	}
	if revertResp.JobModifyIndex == 0 {
		t.Fatalf("bad job modify index")
	}
	assertWriteMeta(t, wm)
}

func TestJobs_Info(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a job by ID before it exists
	// returns an error
	id := "job-id/with\\troublesome:characters\n?&字"
	_, _, err := jobs.Info(id, nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Register the job
	job := testJob()
	job.ID = &id
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	result, qm, err := jobs.Info(id, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	if result == nil || *result.ID != *job.ID {
		t.Fatalf("expect: %#v, got: %#v", job, result)
	}
}

func TestJobs_ScaleInvalidAction(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Check if invalid inputs fail
	tests := []struct {
		jobID string
		group string
		value int
		want  string
	}{
		{"", "", 1, "404"},
		{"i-dont-exist", "", 1, "400"},
		{"", "i-dont-exist", 1, "404"},
		{"i-dont-exist", "me-neither", 1, "404"},
	}
	for _, test := range tests {
		_, _, err := jobs.Scale(test.jobID, test.group, &test.value, "reason", false, nil, nil)
		require.Errorf(err, "expected jobs.Scale(%s, %s) to fail", test.jobID, test.group)
		require.Containsf(err.Error(), test.want, "jobs.Scale(%s, %s) error doesn't contain %s, got: %s", test.jobID, test.group, test.want, err)
	}

	// Register test job
	job := testJob()
	job.ID = stringToPtr("TestJobs_Scale")
	_, wm, err := jobs.Register(job, nil)
	require.NoError(err)
	assertWriteMeta(t, wm)

	// Perform a scaling action with bad group name, verify error
	_, _, err = jobs.Scale(*job.ID, "incorrect-group-name", intToPtr(2),
		"because", false, nil, nil)
	require.Error(err)
	require.Contains(err.Error(), "does not exist")
}

func TestJobs_Versions(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a job by ID before it exists returns an error
	_, _, _, err := jobs.Versions("job1", false, nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Register the job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	result, _, qm, err := jobs.Versions("job1", false, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	if len(result) == 0 || *result[0].ID != *job.ID {
		t.Fatalf("expect: %#v, got: %#v", job, result)
	}
}

func TestJobs_PrefixList(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing when nothing exists returns empty
	results, _, err := jobs.PrefixList("dummy")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n := len(results); n != 0 {
		t.Fatalf("expected 0 jobs, got: %d", n)
	}

	// Register the job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	// Listing when nothing exists returns empty
	results, _, err = jobs.PrefixList((*job.ID)[:1])
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check if we have the right list
	if len(results) != 1 || results[0].ID != *job.ID {
		t.Fatalf("bad: %#v", results)
	}
}

func TestJobs_List(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing when nothing exists returns empty
	results, _, err := jobs.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n := len(results); n != 0 {
		t.Fatalf("expected 0 jobs, got: %d", n)
	}

	// Register the job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	// Listing when nothing exists returns empty
	results, _, err = jobs.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check if we have the right list
	if len(results) != 1 || results[0].ID != *job.ID {
		t.Fatalf("bad: %#v", results)
	}
}

func TestJobs_Allocations(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Looking up by a nonexistent job returns nothing
	allocs, qm, err := jobs.Allocations("job1", true, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}

	// TODO: do something here to create some allocations for
	// an existing job, lookup again.
}

func TestJobs_Evaluations(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Looking up by a nonexistent job ID returns nothing
	evals, qm, err := jobs.Evaluations("job1", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(evals); n != 0 {
		t.Fatalf("expected 0 evals, got: %d", n)
	}

	// Insert a job. This also creates an evaluation so we should
	// be able to query that out after.
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Look up the evaluations again.
	evals, qm, err = jobs.Evaluations("job1", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that we got the evals back, evals are in order most recent to least recent
	// so the last eval is the original registered eval
	idx := len(evals) - 1
	if n := len(evals); n == 0 || evals[idx].ID != resp.EvalID {
		t.Fatalf("expected >= 1 eval (%s), got: %#v", resp.EvalID, evals[idx])
	}
}

func TestJobs_Deregister(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Register a new job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Attempting delete on non-existing job returns an error
	if _, _, err = jobs.Deregister("nope", false, nil); err != nil {
		t.Fatalf("unexpected error deregistering job: %v", err)
	}

	// Do a soft deregister of an existing job
	evalID, wm3, err := jobs.Deregister("job1", false, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm3)
	if evalID == "" {
		t.Fatalf("missing eval ID")
	}

	// Check that the job is still queryable
	out, qm1, err := jobs.Info("job1", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm1)
	if out == nil {
		t.Fatalf("missing job")
	}

	// Do a purge deregister of an existing job
	evalID, wm4, err := jobs.Deregister("job1", true, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm4)
	if evalID == "" {
		t.Fatalf("missing eval ID")
	}

	// Check that the job is really gone
	result, qm, err := jobs.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)
	if n := len(result); n != 0 {
		t.Fatalf("expected 0 jobs, got: %d", n)
	}
}

func TestJobs_ForceEvaluate(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Force-eval on a non-existent job fails
	_, _, err := jobs.ForceEvaluate("job1", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Create a new job
	_, wm, err := jobs.Register(testJob(), nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Try force-eval again
	evalID, wm, err := jobs.ForceEvaluate("job1", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Retrieve the evals and see if we get a matching one
	evals, qm, err := jobs.Evaluations("job1", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)
	for _, eval := range evals {
		if eval.ID == evalID {
			return
		}
	}
	t.Fatalf("evaluation %q missing", evalID)
}

func TestJobs_PeriodicForce(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Force-eval on a nonexistent job fails
	_, _, err := jobs.PeriodicForce("job1", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Create a new job
	job := testPeriodicJob()
	_, _, err = jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	testutil.WaitForResult(func() (bool, error) {
		out, _, err := jobs.Info(*job.ID, nil)
		if err != nil || out == nil || *out.ID != *job.ID {
			return false, err
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Try force again
	evalID, wm, err := jobs.PeriodicForce(*job.ID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	if evalID == "" {
		t.Fatalf("empty evalID")
	}

	// Retrieve the eval
	evals := c.Evaluations()
	eval, qm, err := evals.Info(evalID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)
	if eval.ID == evalID {
		return
	}
	t.Fatalf("evaluation %q missing", evalID)
}

func TestJobs_Plan(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Create a job and attempt to register it
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp == nil || resp.EvalID == "" {
		t.Fatalf("missing eval id")
	}
	assertWriteMeta(t, wm)

	// Check that passing a nil job fails
	if _, _, err := jobs.Plan(nil, true, nil); err == nil {
		t.Fatalf("expect an error when job isn't provided")
	}

	// Make a plan request
	planResp, wm, err := jobs.Plan(job, true, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if planResp == nil {
		t.Fatalf("nil response")
	}

	if planResp.JobModifyIndex == 0 {
		t.Fatalf("bad JobModifyIndex value: %#v", planResp)
	}
	if planResp.Diff == nil {
		t.Fatalf("got nil diff: %#v", planResp)
	}
	if planResp.Annotations == nil {
		t.Fatalf("got nil annotations: %#v", planResp)
	}
	// Can make this assertion because there are no clients.
	if len(planResp.CreatedEvals) == 0 {
		t.Fatalf("got no CreatedEvals: %#v", planResp)
	}
	assertWriteMeta(t, wm)

	// Make a plan request w/o the diff
	planResp, wm, err = jobs.Plan(job, false, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	if planResp == nil {
		t.Fatalf("nil response")
	}

	if planResp.JobModifyIndex == 0 {
		t.Fatalf("bad JobModifyIndex value: %d", planResp.JobModifyIndex)
	}
	if planResp.Diff != nil {
		t.Fatalf("got non-nil diff: %#v", planResp)
	}
	if planResp.Annotations == nil {
		t.Fatalf("got nil annotations: %#v", planResp)
	}
	// Can make this assertion because there are no clients.
	if len(planResp.CreatedEvals) == 0 {
		t.Fatalf("got no CreatedEvals: %#v", planResp)
	}
}

func TestJobs_JobSummary(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a job summary before the job exists
	// returns an error
	_, _, err := jobs.Summary("job1", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Register the job
	job := testJob()
	taskName := job.TaskGroups[0].Name
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Query the job summary again and ensure it exists
	result, qm, err := jobs.Summary("job1", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	if *job.ID != result.JobID {
		t.Fatalf("err: expected job id of %s saw %s", *job.ID, result.JobID)
	}
	if _, ok := result.Summary[*taskName]; !ok {
		t.Fatalf("err: unable to find %s key in job summary", *taskName)
	}
}

func TestJobs_NewBatchJob(t *testing.T) {
	t.Parallel()
	job := NewBatchJob("job1", "myjob", "global", 5)
	expect := &Job{
		Region:   stringToPtr("global"),
		ID:       stringToPtr("job1"),
		Name:     stringToPtr("myjob"),
		Type:     stringToPtr(JobTypeBatch),
		Priority: intToPtr(5),
	}
	if !reflect.DeepEqual(job, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job)
	}
}

func TestJobs_NewServiceJob(t *testing.T) {
	t.Parallel()
	job := NewServiceJob("job1", "myjob", "global", 5)
	expect := &Job{
		Region:   stringToPtr("global"),
		ID:       stringToPtr("job1"),
		Name:     stringToPtr("myjob"),
		Type:     stringToPtr(JobTypeService),
		Priority: intToPtr(5),
	}
	if !reflect.DeepEqual(job, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job)
	}
}

func TestJobs_NewSystemJob(t *testing.T) {
	t.Parallel()
	job := NewSystemJob("job1", "myjob", "global", 5)
	expect := &Job{
		Region:   stringToPtr("global"),
		ID:       stringToPtr("job1"),
		Name:     stringToPtr("myjob"),
		Type:     stringToPtr(JobTypeSystem),
		Priority: intToPtr(5),
	}
	if !reflect.DeepEqual(job, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job)
	}
}

func TestJobs_SetMeta(t *testing.T) {
	t.Parallel()
	job := &Job{Meta: nil}

	// Initializes a nil map
	out := job.SetMeta("foo", "bar")
	if job.Meta == nil {
		t.Fatalf("should initialize metadata")
	}

	// Check that the job was returned
	if job != out {
		t.Fatalf("expect: %#v, got: %#v", job, out)
	}

	// Setting another pair is additive
	job.SetMeta("baz", "zip")
	expect := map[string]string{"foo": "bar", "baz": "zip"}
	if !reflect.DeepEqual(job.Meta, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job.Meta)
	}
}

func TestJobs_Constrain(t *testing.T) {
	t.Parallel()
	job := &Job{Constraints: nil}

	// Create and add a constraint
	out := job.Constrain(NewConstraint("kernel.name", "=", "darwin"))
	if n := len(job.Constraints); n != 1 {
		t.Fatalf("expected 1 constraint, got: %d", n)
	}

	// Check that the job was returned
	if job != out {
		t.Fatalf("expect: %#v, got: %#v", job, out)
	}

	// Adding another constraint preserves the original
	job.Constrain(NewConstraint("memory.totalbytes", ">=", "128000000"))
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
	if !reflect.DeepEqual(job.Constraints, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job.Constraints)
	}
}

func TestJobs_AddAffinity(t *testing.T) {
	t.Parallel()
	job := &Job{Affinities: nil}

	// Create and add an affinity
	out := job.AddAffinity(NewAffinity("kernel.version", "=", "4.6", 100))
	if n := len(job.Affinities); n != 1 {
		t.Fatalf("expected 1 affinity, got: %d", n)
	}

	// Check that the job was returned
	if job != out {
		t.Fatalf("expect: %#v, got: %#v", job, out)
	}

	// Adding another affinity preserves the original
	job.AddAffinity(NewAffinity("${node.datacenter}", "=", "dc2", 50))
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
	if !reflect.DeepEqual(job.Affinities, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job.Affinities)
	}
}

func TestJobs_Sort(t *testing.T) {
	t.Parallel()
	jobs := []*JobListStub{
		{ID: "job2"},
		{ID: "job0"},
		{ID: "job1"},
	}
	sort.Sort(JobIDSort(jobs))

	expect := []*JobListStub{
		{ID: "job0"},
		{ID: "job1"},
		{ID: "job2"},
	}
	if !reflect.DeepEqual(jobs, expect) {
		t.Fatalf("\n\n%#v\n\n%#v", jobs, expect)
	}
}

func TestJobs_AddSpread(t *testing.T) {
	t.Parallel()
	job := &Job{Spreads: nil}

	// Create and add a Spread
	spreadTarget := NewSpreadTarget("r1", 50)

	spread := NewSpread("${meta.rack}", 100, []*SpreadTarget{spreadTarget})
	out := job.AddSpread(spread)
	if n := len(job.Spreads); n != 1 {
		t.Fatalf("expected 1 spread, got: %d", n)
	}

	// Check that the job was returned
	if job != out {
		t.Fatalf("expect: %#v, got: %#v", job, out)
	}

	// Adding another spread preserves the original
	spreadTarget2 := NewSpreadTarget("dc1", 100)

	spread2 := NewSpread("${node.datacenter}", 100, []*SpreadTarget{spreadTarget2})
	job.AddSpread(spread2)

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
	if !reflect.DeepEqual(job.Spreads, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job.Spreads)
	}
}

// TestJobs_ScaleAction tests the scale target for task group count
func TestJobs_ScaleAction(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	id := "job-id/with\\troublesome:characters\n?&字"
	job := testJobWithScalingPolicy()
	job.ID = &id
	groupName := *job.TaskGroups[0].Name
	origCount := *job.TaskGroups[0].Count
	newCount := origCount + 1

	// Trying to scale against a target before it exists returns an error
	_, _, err := jobs.Scale(id, "missing", intToPtr(newCount), "this won't work",
		false, nil, nil)
	require.Error(err)
	require.Contains(err.Error(), "not found")

	// Register the job
	regResp, wm, err := jobs.Register(job, nil)
	require.NoError(err)
	assertWriteMeta(t, wm)

	// Perform scaling action
	scalingResp, wm, err := jobs.Scale(id, groupName,
		intToPtr(newCount), "need more instances", false,
		map[string]interface{}{
			"meta": "data",
		}, nil)

	require.NoError(err)
	require.NotNil(scalingResp)
	require.NotEmpty(scalingResp.EvalID)
	require.NotEmpty(scalingResp.EvalCreateIndex)
	require.Greater(scalingResp.JobModifyIndex, regResp.JobModifyIndex)
	assertWriteMeta(t, wm)

	// Query the job again
	resp, _, err := jobs.Info(*job.ID, nil)
	require.NoError(err)
	require.Equal(*resp.TaskGroups[0].Count, newCount)

	// Check for the scaling event
	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	require.NoError(err)
	require.Len(status.TaskGroups[groupName].Events, 1)
	scalingEvent := status.TaskGroups[groupName].Events[0]
	require.False(scalingEvent.Error)
	require.Equal("need more instances", scalingEvent.Message)
	require.Equal(map[string]interface{}{
		"meta": "data",
	}, scalingEvent.Meta)
	require.Greater(scalingEvent.Time, uint64(0))
	require.NotNil(scalingEvent.EvalID)
	require.Equal(scalingResp.EvalID, *scalingEvent.EvalID)
	require.Equal(int64(origCount), scalingEvent.PreviousCount)
}

func TestJobs_ScaleAction_Error(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	id := "job-id/with\\troublesome:characters\n?&字"
	job := testJobWithScalingPolicy()
	job.ID = &id
	groupName := *job.TaskGroups[0].Name
	prevCount := *job.TaskGroups[0].Count

	// Register the job
	regResp, wm, err := jobs.Register(job, nil)
	require.NoError(err)
	assertWriteMeta(t, wm)

	// Perform scaling action
	scaleResp, wm, err := jobs.Scale(id, groupName, nil, "something bad happened", true,
		map[string]interface{}{
			"meta": "data",
		}, nil)

	require.NoError(err)
	require.NotNil(scaleResp)
	require.Empty(scaleResp.EvalID)
	require.Empty(scaleResp.EvalCreateIndex)
	assertWriteMeta(t, wm)

	// Query the job again
	resp, _, err := jobs.Info(*job.ID, nil)
	require.NoError(err)
	require.Equal(*resp.TaskGroups[0].Count, prevCount)
	require.Equal(regResp.JobModifyIndex, scaleResp.JobModifyIndex)
	require.Empty(scaleResp.EvalCreateIndex)
	require.Empty(scaleResp.EvalID)

	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	require.NoError(err)
	require.Len(status.TaskGroups[groupName].Events, 1)
	errEvent := status.TaskGroups[groupName].Events[0]
	require.True(errEvent.Error)
	require.Equal("something bad happened", errEvent.Message)
	require.Equal(map[string]interface{}{
		"meta": "data",
	}, errEvent.Meta)
	require.Greater(errEvent.Time, uint64(0))
	require.Nil(errEvent.EvalID)
}

func TestJobs_ScaleAction_Noop(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	id := "job-id/with\\troublesome:characters\n?&字"
	job := testJobWithScalingPolicy()
	job.ID = &id
	groupName := *job.TaskGroups[0].Name
	prevCount := *job.TaskGroups[0].Count

	// Register the job
	regResp, wm, err := jobs.Register(job, nil)
	require.NoError(err)
	assertWriteMeta(t, wm)

	// Perform scaling action
	scaleResp, wm, err := jobs.Scale(id, groupName, nil, "no count, just informative",
		false, map[string]interface{}{
			"meta": "data",
		}, nil)

	require.NoError(err)
	require.NotNil(scaleResp)
	require.Empty(scaleResp.EvalID)
	require.Empty(scaleResp.EvalCreateIndex)
	assertWriteMeta(t, wm)

	// Query the job again
	resp, _, err := jobs.Info(*job.ID, nil)
	require.NoError(err)
	require.Equal(*resp.TaskGroups[0].Count, prevCount)
	require.Equal(regResp.JobModifyIndex, scaleResp.JobModifyIndex)
	require.Empty(scaleResp.EvalCreateIndex)
	require.Empty(scaleResp.EvalID)

	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	require.NoError(err)
	require.Len(status.TaskGroups[groupName].Events, 1)
	noopEvent := status.TaskGroups[groupName].Events[0]
	require.False(noopEvent.Error)
	require.Equal("no count, just informative", noopEvent.Message)
	require.Equal(map[string]interface{}{
		"meta": "data",
	}, noopEvent.Meta)
	require.Greater(noopEvent.Time, uint64(0))
	require.Nil(noopEvent.EvalID)
}

// TestJobs_ScaleStatus tests the /scale status endpoint for task group count
func TestJobs_ScaleStatus(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a status before it exists returns an error
	id := "job-id/with\\troublesome:characters\n?&字"
	_, _, err := jobs.ScaleStatus(id, nil)
	require.Error(err)
	require.Contains(err.Error(), "not found")

	// Register the job
	job := testJob()
	job.ID = &id
	groupName := *job.TaskGroups[0].Name
	groupCount := *job.TaskGroups[0].Count
	_, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Query the scaling endpoint and verify success
	result, qm, err := jobs.ScaleStatus(id, nil)
	require.NoError(err)
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	require.Equal(groupCount, result.TaskGroups[groupName].Desired)
}
