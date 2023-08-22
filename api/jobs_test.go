// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/hashicorp/nomad/api/internal/testutil"
)

func TestJobs_Register(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, resp)

	// Create a job and attempt to register it
	job := testJob()
	resp2, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)
	must.UUIDv4(t, resp2.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err := jobs.List(nil)
	assertQueryMeta(t, qm)
	must.Nil(t, err)

	// Check that we got the expected response
	must.Len(t, 1, resp)
	must.Eq(t, *job.ID, resp[0].ID)
}

func TestJobs_Register_PreserveCounts(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, resp)

	// Create a job
	task := NewTask("task", "exec").
		SetConfig("command", "/bin/sleep").
		Require(&Resources{
			CPU:      pointerOf(100),
			MemoryMB: pointerOf(256),
		}).
		SetLogConfig(&LogConfig{
			MaxFiles:      pointerOf(1),
			MaxFileSizeMB: pointerOf(2),
		})

	group1 := NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: pointerOf(25),
		})
	group2 := NewTaskGroup("group2", 2).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: pointerOf(25),
		})

	job := NewBatchJob("job", "redis", "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group1).
		AddTaskGroup(group2)

	// Create a job and register it
	resp2, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)
	must.UUIDv4(t, resp2.EvalID)
	assertWriteMeta(t, wm)

	// Update the job, new groups to test PreserveCounts
	group1.Count = nil
	group2.Count = pointerOf(0)
	group3 := NewTaskGroup("group3", 3).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: pointerOf(25),
		})
	job.AddTaskGroup(group3)

	// Update the job, with PreserveCounts = true
	_, _, err = jobs.RegisterOpts(job, &RegisterOptions{
		PreserveCounts: true,
	}, nil)
	must.NoError(t, err)

	// Query the job scale status
	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	must.NoError(t, err)
	must.Eq(t, 1, status.TaskGroups["group1"].Desired) // present and nil => preserved
	must.Eq(t, 2, status.TaskGroups["group2"].Desired) // present and specified => preserved
	must.Eq(t, 3, status.TaskGroups["group3"].Desired) // new => as specific in job spec
}

func TestJobs_Register_NoPreserveCounts(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, resp)

	// Create a job
	task := NewTask("task", "exec").
		SetConfig("command", "/bin/sleep").
		Require(&Resources{
			CPU:      pointerOf(100),
			MemoryMB: pointerOf(256),
		}).
		SetLogConfig(&LogConfig{
			MaxFiles:      pointerOf(1),
			MaxFileSizeMB: pointerOf(2),
		})

	group1 := NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: pointerOf(25),
		})
	group2 := NewTaskGroup("group2", 2).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: pointerOf(25),
		})

	job := NewBatchJob("job", "redis", "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group1).
		AddTaskGroup(group2)

	// Create a job and register it
	resp2, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)
	must.UUIDv4(t, resp2.EvalID)
	assertWriteMeta(t, wm)

	// Update the job, new groups to test PreserveCounts
	group1.Count = pointerOf(0)
	group2.Count = nil
	group3 := NewTaskGroup("group3", 3).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: pointerOf(25),
		})
	job.AddTaskGroup(group3)

	// Update the job, with PreserveCounts = default [false]
	_, _, err = jobs.Register(job, nil)
	must.NoError(t, err)

	// Query the job scale status
	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	must.NoError(t, err)
	must.Eq(t, "default", status.Namespace)
	must.Eq(t, 0, status.TaskGroups["group1"].Desired) // present => as specified
	must.Eq(t, 1, status.TaskGroups["group2"].Desired) // nil     => default (1)
	must.Eq(t, 3, status.TaskGroups["group3"].Desired) // new     => as specified
}

func TestJobs_Register_EvalPriority(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// Listing jobs before registering returns nothing
	listResp, _, err := c.Jobs().List(nil)
	must.NoError(t, err)
	must.Len(t, 0, listResp)

	// Create a job and register it with an eval priority.
	job := testJob()
	registerResp, wm, err := c.Jobs().RegisterOpts(job, &RegisterOptions{EvalPriority: 99}, nil)
	must.NoError(t, err)
	must.NotNil(t, registerResp)
	must.UUIDv4(t, registerResp.EvalID)
	assertWriteMeta(t, wm)

	// Check the created job evaluation has a priority that matches our desired
	// value.
	evalInfo, _, err := c.Evaluations().Info(registerResp.EvalID, nil)
	must.NoError(t, err)
	must.Eq(t, 99, evalInfo.Priority)
}

func TestJobs_Register_NoEvalPriority(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// Listing jobs before registering returns nothing
	listResp, _, err := c.Jobs().List(nil)
	must.NoError(t, err)
	must.Len(t, 0, listResp)

	// Create a job and register it with an eval priority.
	job := testJob()
	registerResp, wm, err := c.Jobs().RegisterOpts(job, nil, nil)
	must.NoError(t, err)
	must.NotNil(t, registerResp)
	must.UUIDv4(t, registerResp.EvalID)
	assertWriteMeta(t, wm)

	// Check the created job evaluation has a priority that matches the job
	// priority.
	evalInfo, _, err := c.Evaluations().Info(registerResp.EvalID, nil)
	must.NoError(t, err)
	must.Eq(t, *job.Priority, evalInfo.Priority)
}

func TestJobs_Validate(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Create a job and attempt to register it
	job := testJob()
	resp, _, err := jobs.Validate(job, nil)
	must.NoError(t, err)
	must.SliceEmpty(t, resp.ValidationErrors)

	job.ID = nil
	resp1, _, err := jobs.Validate(job, nil)
	must.NoError(t, err)
	must.Positive(t, len(resp1.ValidationErrors))
}

func TestJobs_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

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
				ID:                pointerOf(""),
				Name:              pointerOf(""),
				Region:            pointerOf("global"),
				Namespace:         pointerOf(DefaultNamespace),
				Type:              pointerOf("service"),
				ParentID:          pointerOf(""),
				Priority:          pointerOf(JobDefaultPriority),
				NodePool:          pointerOf(""),
				AllAtOnce:         pointerOf(false),
				ConsulToken:       pointerOf(""),
				ConsulNamespace:   pointerOf(""),
				VaultToken:        pointerOf(""),
				VaultNamespace:    pointerOf(""),
				NomadTokenID:      pointerOf(""),
				Status:            pointerOf(""),
				StatusDescription: pointerOf(""),
				Stop:              pointerOf(false),
				Stable:            pointerOf(false),
				Version:           pointerOf(uint64(0)),
				CreateIndex:       pointerOf(uint64(0)),
				ModifyIndex:       pointerOf(uint64(0)),
				JobModifyIndex:    pointerOf(uint64(0)),
				Update: &UpdateStrategy{
					Stagger:          pointerOf(30 * time.Second),
					MaxParallel:      pointerOf(1),
					HealthCheck:      pointerOf("checks"),
					MinHealthyTime:   pointerOf(10 * time.Second),
					HealthyDeadline:  pointerOf(5 * time.Minute),
					ProgressDeadline: pointerOf(10 * time.Minute),
					AutoRevert:       pointerOf(false),
					Canary:           pointerOf(0),
					AutoPromote:      pointerOf(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  pointerOf(""),
						Count: pointerOf(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  pointerOf(false),
							Migrate: pointerOf(false),
							SizeMB:  pointerOf(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:           pointerOf(15 * time.Second),
							Attempts:        pointerOf(2),
							Interval:        pointerOf(30 * time.Minute),
							Mode:            pointerOf("fail"),
							RenderTemplates: pointerOf(false),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      pointerOf(0),
							Interval:      pointerOf(time.Duration(0)),
							DelayFunction: pointerOf("exponential"),
							Delay:         pointerOf(30 * time.Second),
							MaxDelay:      pointerOf(1 * time.Hour),
							Unlimited:     pointerOf(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          pointerOf(30 * time.Second),
							MaxParallel:      pointerOf(1),
							HealthCheck:      pointerOf("checks"),
							MinHealthyTime:   pointerOf(10 * time.Second),
							HealthyDeadline:  pointerOf(5 * time.Minute),
							ProgressDeadline: pointerOf(10 * time.Minute),
							AutoRevert:       pointerOf(false),
							Canary:           pointerOf(0),
							AutoPromote:      pointerOf(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								KillTimeout:   pointerOf(5 * time.Second),
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
				Type: pointerOf("batch"),
				TaskGroups: []*TaskGroup{
					{
						Tasks: []*Task{
							{},
						},
					},
				},
			},
			expected: &Job{
				ID:                pointerOf(""),
				Name:              pointerOf(""),
				Region:            pointerOf("global"),
				Namespace:         pointerOf(DefaultNamespace),
				Type:              pointerOf("batch"),
				ParentID:          pointerOf(""),
				Priority:          pointerOf(JobDefaultPriority),
				NodePool:          pointerOf(""),
				AllAtOnce:         pointerOf(false),
				ConsulToken:       pointerOf(""),
				ConsulNamespace:   pointerOf(""),
				VaultToken:        pointerOf(""),
				VaultNamespace:    pointerOf(""),
				NomadTokenID:      pointerOf(""),
				Status:            pointerOf(""),
				StatusDescription: pointerOf(""),
				Stop:              pointerOf(false),
				Stable:            pointerOf(false),
				Version:           pointerOf(uint64(0)),
				CreateIndex:       pointerOf(uint64(0)),
				ModifyIndex:       pointerOf(uint64(0)),
				JobModifyIndex:    pointerOf(uint64(0)),
				TaskGroups: []*TaskGroup{
					{
						Name:  pointerOf(""),
						Count: pointerOf(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  pointerOf(false),
							Migrate: pointerOf(false),
							SizeMB:  pointerOf(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:           pointerOf(15 * time.Second),
							Attempts:        pointerOf(3),
							Interval:        pointerOf(24 * time.Hour),
							Mode:            pointerOf("fail"),
							RenderTemplates: pointerOf(false),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      pointerOf(1),
							Interval:      pointerOf(24 * time.Hour),
							DelayFunction: pointerOf("constant"),
							Delay:         pointerOf(5 * time.Second),
							MaxDelay:      pointerOf(time.Duration(0)),
							Unlimited:     pointerOf(false),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Tasks: []*Task{
							{
								KillTimeout:   pointerOf(5 * time.Second),
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
				Name:      pointerOf("foo"),
				Namespace: pointerOf("bar"),
				ID:        pointerOf("bar"),
				ParentID:  pointerOf("lol"),
				TaskGroups: []*TaskGroup{
					{
						Name: pointerOf("bar"),
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
				},
			},
			expected: &Job{
				Namespace:         pointerOf("bar"),
				ID:                pointerOf("bar"),
				Name:              pointerOf("foo"),
				Region:            pointerOf("global"),
				Type:              pointerOf("service"),
				ParentID:          pointerOf("lol"),
				Priority:          pointerOf(JobDefaultPriority),
				NodePool:          pointerOf(""),
				AllAtOnce:         pointerOf(false),
				ConsulToken:       pointerOf(""),
				ConsulNamespace:   pointerOf(""),
				VaultToken:        pointerOf(""),
				VaultNamespace:    pointerOf(""),
				NomadTokenID:      pointerOf(""),
				Stop:              pointerOf(false),
				Stable:            pointerOf(false),
				Version:           pointerOf(uint64(0)),
				Status:            pointerOf(""),
				StatusDescription: pointerOf(""),
				CreateIndex:       pointerOf(uint64(0)),
				ModifyIndex:       pointerOf(uint64(0)),
				JobModifyIndex:    pointerOf(uint64(0)),
				Update: &UpdateStrategy{
					Stagger:          pointerOf(30 * time.Second),
					MaxParallel:      pointerOf(1),
					HealthCheck:      pointerOf("checks"),
					MinHealthyTime:   pointerOf(10 * time.Second),
					HealthyDeadline:  pointerOf(5 * time.Minute),
					ProgressDeadline: pointerOf(10 * time.Minute),
					AutoRevert:       pointerOf(false),
					Canary:           pointerOf(0),
					AutoPromote:      pointerOf(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  pointerOf("bar"),
						Count: pointerOf(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  pointerOf(false),
							Migrate: pointerOf(false),
							SizeMB:  pointerOf(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:           pointerOf(15 * time.Second),
							Attempts:        pointerOf(2),
							Interval:        pointerOf(30 * time.Minute),
							Mode:            pointerOf("fail"),
							RenderTemplates: pointerOf(false),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      pointerOf(0),
							Interval:      pointerOf(time.Duration(0)),
							DelayFunction: pointerOf("exponential"),
							Delay:         pointerOf(30 * time.Second),
							MaxDelay:      pointerOf(1 * time.Hour),
							Unlimited:     pointerOf(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          pointerOf(30 * time.Second),
							MaxParallel:      pointerOf(1),
							HealthCheck:      pointerOf("checks"),
							MinHealthyTime:   pointerOf(10 * time.Second),
							HealthyDeadline:  pointerOf(5 * time.Minute),
							ProgressDeadline: pointerOf(10 * time.Minute),
							AutoRevert:       pointerOf(false),
							Canary:           pointerOf(0),
							AutoPromote:      pointerOf(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:          "task1",
								LogConfig:     DefaultLogConfig(),
								Resources:     DefaultResources(),
								KillTimeout:   pointerOf(5 * time.Second),
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
				ID:          pointerOf("example_template"),
				Name:        pointerOf("example_template"),
				Datacenters: []string{"dc1"},
				Type:        pointerOf("service"),
				Update: &UpdateStrategy{
					MaxParallel: pointerOf(1),
					AutoPromote: pointerOf(true),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  pointerOf("cache"),
						Count: pointerOf(1),
						RestartPolicy: &RestartPolicy{
							Interval: pointerOf(5 * time.Minute),
							Attempts: pointerOf(10),
							Delay:    pointerOf(25 * time.Second),
							Mode:     pointerOf("delay"),
						},
						Update: &UpdateStrategy{
							AutoRevert: pointerOf(true),
						},
						EphemeralDisk: &EphemeralDisk{
							SizeMB: pointerOf(300),
						},
						Tasks: []*Task{
							{
								Name:   "redis",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "redis:7",
									"port_map": []map[string]int{{
										"db": 6379,
									}},
								},
								RestartPolicy: &RestartPolicy{
									// inherit other values from TG
									Attempts: pointerOf(20),
								},
								Resources: &Resources{
									CPU:      pointerOf(500),
									MemoryMB: pointerOf(256),
									Networks: []*NetworkResource{
										{
											MBits: pointerOf(10),
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
										EmbeddedTmpl: pointerOf("---"),
										DestPath:     pointerOf("local/file.yml"),
									},
									{
										EmbeddedTmpl: pointerOf("FOO=bar\n"),
										DestPath:     pointerOf("local/file.env"),
										Envvars:      pointerOf(true),
									},
								},
							},
						},
					},
				},
			},
			expected: &Job{
				Namespace:         pointerOf(DefaultNamespace),
				ID:                pointerOf("example_template"),
				Name:              pointerOf("example_template"),
				ParentID:          pointerOf(""),
				Priority:          pointerOf(JobDefaultPriority),
				NodePool:          pointerOf(""),
				Region:            pointerOf("global"),
				Type:              pointerOf("service"),
				AllAtOnce:         pointerOf(false),
				ConsulToken:       pointerOf(""),
				ConsulNamespace:   pointerOf(""),
				VaultToken:        pointerOf(""),
				VaultNamespace:    pointerOf(""),
				NomadTokenID:      pointerOf(""),
				Stop:              pointerOf(false),
				Stable:            pointerOf(false),
				Version:           pointerOf(uint64(0)),
				Status:            pointerOf(""),
				StatusDescription: pointerOf(""),
				CreateIndex:       pointerOf(uint64(0)),
				ModifyIndex:       pointerOf(uint64(0)),
				JobModifyIndex:    pointerOf(uint64(0)),
				Datacenters:       []string{"dc1"},
				Update: &UpdateStrategy{
					Stagger:          pointerOf(30 * time.Second),
					MaxParallel:      pointerOf(1),
					HealthCheck:      pointerOf("checks"),
					MinHealthyTime:   pointerOf(10 * time.Second),
					HealthyDeadline:  pointerOf(5 * time.Minute),
					ProgressDeadline: pointerOf(10 * time.Minute),
					AutoRevert:       pointerOf(false),
					Canary:           pointerOf(0),
					AutoPromote:      pointerOf(true),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  pointerOf("cache"),
						Count: pointerOf(1),
						RestartPolicy: &RestartPolicy{
							Interval:        pointerOf(5 * time.Minute),
							Attempts:        pointerOf(10),
							Delay:           pointerOf(25 * time.Second),
							Mode:            pointerOf("delay"),
							RenderTemplates: pointerOf(false),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      pointerOf(0),
							Interval:      pointerOf(time.Duration(0)),
							DelayFunction: pointerOf("exponential"),
							Delay:         pointerOf(30 * time.Second),
							MaxDelay:      pointerOf(1 * time.Hour),
							Unlimited:     pointerOf(true),
						},
						EphemeralDisk: &EphemeralDisk{
							Sticky:  pointerOf(false),
							Migrate: pointerOf(false),
							SizeMB:  pointerOf(300),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          pointerOf(30 * time.Second),
							MaxParallel:      pointerOf(1),
							HealthCheck:      pointerOf("checks"),
							MinHealthyTime:   pointerOf(10 * time.Second),
							HealthyDeadline:  pointerOf(5 * time.Minute),
							ProgressDeadline: pointerOf(10 * time.Minute),
							AutoRevert:       pointerOf(true),
							Canary:           pointerOf(0),
							AutoPromote:      pointerOf(true),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:   "redis",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "redis:7",
									"port_map": []map[string]int{{
										"db": 6379,
									}},
								},
								RestartPolicy: &RestartPolicy{
									Interval:        pointerOf(5 * time.Minute),
									Attempts:        pointerOf(20),
									Delay:           pointerOf(25 * time.Second),
									Mode:            pointerOf("delay"),
									RenderTemplates: pointerOf(false),
								},
								Resources: &Resources{
									CPU:      pointerOf(500),
									Cores:    pointerOf(0),
									MemoryMB: pointerOf(256),
									Networks: []*NetworkResource{
										{
											MBits: pointerOf(10),
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
										Provider:    "consul",
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
								KillTimeout: pointerOf(5 * time.Second),
								LogConfig:   DefaultLogConfig(),
								Templates: []*Template{
									{
										SourcePath:    pointerOf(""),
										DestPath:      pointerOf("local/file.yml"),
										EmbeddedTmpl:  pointerOf("---"),
										ChangeMode:    pointerOf("restart"),
										ChangeSignal:  pointerOf(""),
										Splay:         pointerOf(5 * time.Second),
										Perms:         pointerOf("0644"),
										LeftDelim:     pointerOf("{{"),
										RightDelim:    pointerOf("}}"),
										Envvars:       pointerOf(false),
										VaultGrace:    pointerOf(time.Duration(0)),
										ErrMissingKey: pointerOf(false),
									},
									{
										SourcePath:    pointerOf(""),
										DestPath:      pointerOf("local/file.env"),
										EmbeddedTmpl:  pointerOf("FOO=bar\n"),
										ChangeMode:    pointerOf("restart"),
										ChangeSignal:  pointerOf(""),
										Splay:         pointerOf(5 * time.Second),
										Perms:         pointerOf("0644"),
										LeftDelim:     pointerOf("{{"),
										RightDelim:    pointerOf("}}"),
										Envvars:       pointerOf(true),
										VaultGrace:    pointerOf(time.Duration(0)),
										ErrMissingKey: pointerOf(false),
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
				ID:       pointerOf("bar"),
				Periodic: &PeriodicConfig{},
			},
			expected: &Job{
				Namespace:         pointerOf(DefaultNamespace),
				ID:                pointerOf("bar"),
				ParentID:          pointerOf(""),
				Name:              pointerOf("bar"),
				Region:            pointerOf("global"),
				Type:              pointerOf("service"),
				Priority:          pointerOf(JobDefaultPriority),
				NodePool:          pointerOf(""),
				AllAtOnce:         pointerOf(false),
				ConsulToken:       pointerOf(""),
				ConsulNamespace:   pointerOf(""),
				VaultToken:        pointerOf(""),
				VaultNamespace:    pointerOf(""),
				NomadTokenID:      pointerOf(""),
				Stop:              pointerOf(false),
				Stable:            pointerOf(false),
				Version:           pointerOf(uint64(0)),
				Status:            pointerOf(""),
				StatusDescription: pointerOf(""),
				CreateIndex:       pointerOf(uint64(0)),
				ModifyIndex:       pointerOf(uint64(0)),
				JobModifyIndex:    pointerOf(uint64(0)),
				Update: &UpdateStrategy{
					Stagger:          pointerOf(30 * time.Second),
					MaxParallel:      pointerOf(1),
					HealthCheck:      pointerOf("checks"),
					MinHealthyTime:   pointerOf(10 * time.Second),
					HealthyDeadline:  pointerOf(5 * time.Minute),
					ProgressDeadline: pointerOf(10 * time.Minute),
					AutoRevert:       pointerOf(false),
					Canary:           pointerOf(0),
					AutoPromote:      pointerOf(false),
				},
				Periodic: &PeriodicConfig{
					Enabled:         pointerOf(true),
					Spec:            pointerOf(""),
					Specs:           []string{},
					SpecType:        pointerOf(PeriodicSpecCron),
					ProhibitOverlap: pointerOf(false),
					TimeZone:        pointerOf("UTC"),
				},
			},
		},
		{
			name: "update_merge",
			input: &Job{
				Name:     pointerOf("foo"),
				ID:       pointerOf("bar"),
				ParentID: pointerOf("lol"),
				Update: &UpdateStrategy{
					Stagger:          pointerOf(1 * time.Second),
					MaxParallel:      pointerOf(1),
					HealthCheck:      pointerOf("checks"),
					MinHealthyTime:   pointerOf(10 * time.Second),
					HealthyDeadline:  pointerOf(6 * time.Minute),
					ProgressDeadline: pointerOf(7 * time.Minute),
					AutoRevert:       pointerOf(false),
					Canary:           pointerOf(0),
					AutoPromote:      pointerOf(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name: pointerOf("bar"),
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:        pointerOf(2 * time.Second),
							MaxParallel:    pointerOf(2),
							HealthCheck:    pointerOf("manual"),
							MinHealthyTime: pointerOf(1 * time.Second),
							AutoRevert:     pointerOf(true),
							Canary:         pointerOf(1),
							AutoPromote:    pointerOf(true),
						},
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
					{
						Name: pointerOf("baz"),
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
				},
			},
			expected: &Job{
				Namespace:         pointerOf(DefaultNamespace),
				ID:                pointerOf("bar"),
				Name:              pointerOf("foo"),
				Region:            pointerOf("global"),
				Type:              pointerOf("service"),
				ParentID:          pointerOf("lol"),
				Priority:          pointerOf(JobDefaultPriority),
				NodePool:          pointerOf(""),
				AllAtOnce:         pointerOf(false),
				ConsulToken:       pointerOf(""),
				ConsulNamespace:   pointerOf(""),
				VaultToken:        pointerOf(""),
				VaultNamespace:    pointerOf(""),
				NomadTokenID:      pointerOf(""),
				Stop:              pointerOf(false),
				Stable:            pointerOf(false),
				Version:           pointerOf(uint64(0)),
				Status:            pointerOf(""),
				StatusDescription: pointerOf(""),
				CreateIndex:       pointerOf(uint64(0)),
				ModifyIndex:       pointerOf(uint64(0)),
				JobModifyIndex:    pointerOf(uint64(0)),
				Update: &UpdateStrategy{
					Stagger:          pointerOf(1 * time.Second),
					MaxParallel:      pointerOf(1),
					HealthCheck:      pointerOf("checks"),
					MinHealthyTime:   pointerOf(10 * time.Second),
					HealthyDeadline:  pointerOf(6 * time.Minute),
					ProgressDeadline: pointerOf(7 * time.Minute),
					AutoRevert:       pointerOf(false),
					Canary:           pointerOf(0),
					AutoPromote:      pointerOf(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  pointerOf("bar"),
						Count: pointerOf(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  pointerOf(false),
							Migrate: pointerOf(false),
							SizeMB:  pointerOf(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:           pointerOf(15 * time.Second),
							Attempts:        pointerOf(2),
							Interval:        pointerOf(30 * time.Minute),
							Mode:            pointerOf("fail"),
							RenderTemplates: pointerOf(false),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      pointerOf(0),
							Interval:      pointerOf(time.Duration(0)),
							DelayFunction: pointerOf("exponential"),
							Delay:         pointerOf(30 * time.Second),
							MaxDelay:      pointerOf(1 * time.Hour),
							Unlimited:     pointerOf(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          pointerOf(2 * time.Second),
							MaxParallel:      pointerOf(2),
							HealthCheck:      pointerOf("manual"),
							MinHealthyTime:   pointerOf(1 * time.Second),
							HealthyDeadline:  pointerOf(6 * time.Minute),
							ProgressDeadline: pointerOf(7 * time.Minute),
							AutoRevert:       pointerOf(true),
							Canary:           pointerOf(1),
							AutoPromote:      pointerOf(true),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:          "task1",
								LogConfig:     DefaultLogConfig(),
								Resources:     DefaultResources(),
								KillTimeout:   pointerOf(5 * time.Second),
								RestartPolicy: defaultServiceJobRestartPolicy(),
							},
						},
					},
					{
						Name:  pointerOf("baz"),
						Count: pointerOf(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  pointerOf(false),
							Migrate: pointerOf(false),
							SizeMB:  pointerOf(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:           pointerOf(15 * time.Second),
							Attempts:        pointerOf(2),
							Interval:        pointerOf(30 * time.Minute),
							Mode:            pointerOf("fail"),
							RenderTemplates: pointerOf(false),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      pointerOf(0),
							Interval:      pointerOf(time.Duration(0)),
							DelayFunction: pointerOf("exponential"),
							Delay:         pointerOf(30 * time.Second),
							MaxDelay:      pointerOf(1 * time.Hour),
							Unlimited:     pointerOf(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          pointerOf(1 * time.Second),
							MaxParallel:      pointerOf(1),
							HealthCheck:      pointerOf("checks"),
							MinHealthyTime:   pointerOf(10 * time.Second),
							HealthyDeadline:  pointerOf(6 * time.Minute),
							ProgressDeadline: pointerOf(7 * time.Minute),
							AutoRevert:       pointerOf(false),
							Canary:           pointerOf(0),
							AutoPromote:      pointerOf(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:          "task1",
								LogConfig:     DefaultLogConfig(),
								Resources:     DefaultResources(),
								KillTimeout:   pointerOf(5 * time.Second),
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
				Name:     pointerOf("foo"),
				ID:       pointerOf("bar"),
				ParentID: pointerOf("lol"),
				TaskGroups: []*TaskGroup{
					{
						Name: pointerOf("bar"),
						RestartPolicy: &RestartPolicy{
							Delay:    pointerOf(15 * time.Second),
							Attempts: pointerOf(2),
							Interval: pointerOf(30 * time.Minute),
							Mode:     pointerOf("fail"),
						},
						Tasks: []*Task{
							{
								Name: "task1",
								RestartPolicy: &RestartPolicy{
									Attempts:        pointerOf(5),
									Delay:           pointerOf(1 * time.Second),
									RenderTemplates: pointerOf(true),
								},
							},
						},
					},
					{
						Name: pointerOf("baz"),
						RestartPolicy: &RestartPolicy{
							Delay:    pointerOf(20 * time.Second),
							Attempts: pointerOf(2),
							Interval: pointerOf(30 * time.Minute),
							Mode:     pointerOf("fail"),
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
				Namespace:         pointerOf(DefaultNamespace),
				ID:                pointerOf("bar"),
				Name:              pointerOf("foo"),
				Region:            pointerOf("global"),
				Type:              pointerOf("service"),
				ParentID:          pointerOf("lol"),
				NodePool:          pointerOf(""),
				Priority:          pointerOf(JobDefaultPriority),
				AllAtOnce:         pointerOf(false),
				ConsulToken:       pointerOf(""),
				ConsulNamespace:   pointerOf(""),
				VaultToken:        pointerOf(""),
				VaultNamespace:    pointerOf(""),
				NomadTokenID:      pointerOf(""),
				Stop:              pointerOf(false),
				Stable:            pointerOf(false),
				Version:           pointerOf(uint64(0)),
				Status:            pointerOf(""),
				StatusDescription: pointerOf(""),
				CreateIndex:       pointerOf(uint64(0)),
				ModifyIndex:       pointerOf(uint64(0)),
				JobModifyIndex:    pointerOf(uint64(0)),
				Update: &UpdateStrategy{
					Stagger:          pointerOf(30 * time.Second),
					MaxParallel:      pointerOf(1),
					HealthCheck:      pointerOf("checks"),
					MinHealthyTime:   pointerOf(10 * time.Second),
					HealthyDeadline:  pointerOf(5 * time.Minute),
					ProgressDeadline: pointerOf(10 * time.Minute),
					AutoRevert:       pointerOf(false),
					Canary:           pointerOf(0),
					AutoPromote:      pointerOf(false),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  pointerOf("bar"),
						Count: pointerOf(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  pointerOf(false),
							Migrate: pointerOf(false),
							SizeMB:  pointerOf(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:           pointerOf(15 * time.Second),
							Attempts:        pointerOf(2),
							Interval:        pointerOf(30 * time.Minute),
							Mode:            pointerOf("fail"),
							RenderTemplates: pointerOf(false),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      pointerOf(0),
							Interval:      pointerOf(time.Duration(0)),
							DelayFunction: pointerOf("exponential"),
							Delay:         pointerOf(30 * time.Second),
							MaxDelay:      pointerOf(1 * time.Hour),
							Unlimited:     pointerOf(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          pointerOf(30 * time.Second),
							MaxParallel:      pointerOf(1),
							HealthCheck:      pointerOf("checks"),
							MinHealthyTime:   pointerOf(10 * time.Second),
							HealthyDeadline:  pointerOf(5 * time.Minute),
							ProgressDeadline: pointerOf(10 * time.Minute),
							AutoRevert:       pointerOf(false),
							Canary:           pointerOf(0),
							AutoPromote:      pointerOf(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   DefaultResources(),
								KillTimeout: pointerOf(5 * time.Second),
								RestartPolicy: &RestartPolicy{
									Attempts:        pointerOf(5),
									Delay:           pointerOf(1 * time.Second),
									Interval:        pointerOf(30 * time.Minute),
									Mode:            pointerOf("fail"),
									RenderTemplates: pointerOf(true),
								},
							},
						},
					},
					{
						Name:  pointerOf("baz"),
						Count: pointerOf(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  pointerOf(false),
							Migrate: pointerOf(false),
							SizeMB:  pointerOf(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:           pointerOf(20 * time.Second),
							Attempts:        pointerOf(2),
							Interval:        pointerOf(30 * time.Minute),
							Mode:            pointerOf("fail"),
							RenderTemplates: pointerOf(false),
						},
						ReschedulePolicy: &ReschedulePolicy{
							Attempts:      pointerOf(0),
							Interval:      pointerOf(time.Duration(0)),
							DelayFunction: pointerOf("exponential"),
							Delay:         pointerOf(30 * time.Second),
							MaxDelay:      pointerOf(1 * time.Hour),
							Unlimited:     pointerOf(true),
						},
						Consul: &Consul{
							Namespace: "",
						},
						Update: &UpdateStrategy{
							Stagger:          pointerOf(30 * time.Second),
							MaxParallel:      pointerOf(1),
							HealthCheck:      pointerOf("checks"),
							MinHealthyTime:   pointerOf(10 * time.Second),
							HealthyDeadline:  pointerOf(5 * time.Minute),
							ProgressDeadline: pointerOf(10 * time.Minute),
							AutoRevert:       pointerOf(false),
							Canary:           pointerOf(0),
							AutoPromote:      pointerOf(false),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   DefaultResources(),
								KillTimeout: pointerOf(5 * time.Second),
								RestartPolicy: &RestartPolicy{
									Delay:           pointerOf(20 * time.Second),
									Attempts:        pointerOf(2),
									Interval:        pointerOf(30 * time.Minute),
									Mode:            pointerOf("fail"),
									RenderTemplates: pointerOf(false),
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
				Name:     pointerOf("foo"),
				ID:       pointerOf("bar"),
				ParentID: pointerOf("lol"),
				Multiregion: &Multiregion{
					Regions: []*MultiregionRegion{
						{
							Name:  "west",
							Count: pointerOf(1),
						},
					},
				},
			},
			expected: &Job{
				Multiregion: &Multiregion{
					Strategy: &MultiregionStrategy{
						MaxParallel: pointerOf(0),
						OnFailure:   pointerOf(""),
					},
					Regions: []*MultiregionRegion{
						{
							Name:        "west",
							Count:       pointerOf(1),
							Datacenters: []string{},
							Meta:        map[string]string{},
						},
					},
				},
				Namespace:         pointerOf(DefaultNamespace),
				ID:                pointerOf("bar"),
				Name:              pointerOf("foo"),
				Region:            pointerOf("global"),
				Type:              pointerOf("service"),
				ParentID:          pointerOf("lol"),
				Priority:          pointerOf(JobDefaultPriority),
				NodePool:          pointerOf(""),
				AllAtOnce:         pointerOf(false),
				ConsulToken:       pointerOf(""),
				ConsulNamespace:   pointerOf(""),
				VaultToken:        pointerOf(""),
				VaultNamespace:    pointerOf(""),
				NomadTokenID:      pointerOf(""),
				Stop:              pointerOf(false),
				Stable:            pointerOf(false),
				Version:           pointerOf(uint64(0)),
				Status:            pointerOf(""),
				StatusDescription: pointerOf(""),
				CreateIndex:       pointerOf(uint64(0)),
				ModifyIndex:       pointerOf(uint64(0)),
				JobModifyIndex:    pointerOf(uint64(0)),
				Update: &UpdateStrategy{
					Stagger:          pointerOf(30 * time.Second),
					MaxParallel:      pointerOf(1),
					HealthCheck:      pointerOf("checks"),
					MinHealthyTime:   pointerOf(10 * time.Second),
					HealthyDeadline:  pointerOf(5 * time.Minute),
					ProgressDeadline: pointerOf(10 * time.Minute),
					AutoRevert:       pointerOf(false),
					Canary:           pointerOf(0),
					AutoPromote:      pointerOf(false),
				},
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

func TestJobs_EnforceRegister(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, _, err := jobs.List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, resp)

	// Create a job and attempt to register it with an incorrect index.
	job := testJob()
	resp2, _, err := jobs.EnforceRegister(job, 10, nil)
	must.ErrorContains(t, err, RegisterEnforceIndexErrPrefix)

	// Register
	resp2, wm, err := jobs.EnforceRegister(job, 0, nil)
	must.NoError(t, err)
	must.NotNil(t, resp2)
	must.UUIDv4(t, resp2.EvalID)
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err := jobs.List(nil)
	must.NoError(t, err)
	must.Len(t, 1, resp)
	must.Eq(t, *job.ID, resp[0].ID)
	assertQueryMeta(t, qm)

	// Fail at incorrect index
	curIndex := resp[0].JobModifyIndex
	resp2, _, err = jobs.EnforceRegister(job, 123456, nil)
	must.ErrorContains(t, err, RegisterEnforceIndexErrPrefix)

	// Works at correct index
	resp3, wm, err := jobs.EnforceRegister(job, curIndex, nil)
	must.NoError(t, err)
	must.NotNil(t, resp3)
	must.UUIDv4(t, resp3.EvalID)
	assertWriteMeta(t, wm)
}

func TestJobs_Revert(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Register twice
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	job.Meta = map[string]string{"foo": "new"}
	resp, wm, err = jobs.Register(job, nil)
	must.NoError(t, err)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// Fail revert at incorrect enforce
	_, _, err = jobs.Revert(*job.ID, 0, pointerOf(uint64(10)), nil, "", "")
	must.ErrorContains(t, err, "enforcing version")

	// Works at correct index
	revertResp, wm, err := jobs.Revert(*job.ID, 0, pointerOf(uint64(1)), nil, "", "")
	must.NoError(t, err)
	must.UUIDv4(t, revertResp.EvalID)
	must.Positive(t, revertResp.EvalCreateIndex)
	must.Positive(t, revertResp.JobModifyIndex)
	assertWriteMeta(t, wm)
}

func TestJobs_Info(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a job by ID before it exists
	// returns an error
	id := "job-id/with\\troublesome:characters\n?&字"
	_, _, err := jobs.Info(id, nil)
	must.ErrorContains(t, err, "not found")

	// Register the job
	job := testJob()
	job.ID = &id
	_, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	result, qm, err := jobs.Info(id, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	must.Eq(t, *result.ID, *job.ID)
}

func TestJobs_ScaleInvalidAction(t *testing.T) {
	testutil.Parallel(t)

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
		must.ErrorContains(t, err, test.want)
	}

	// Register test job
	job := testJob()
	job.ID = pointerOf("TestJobs_Scale")
	_, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Perform a scaling action with bad group name, verify error
	_, _, err = jobs.Scale(*job.ID, "incorrect-group-name", pointerOf(2),
		"because", false, nil, nil)
	must.ErrorContains(t, err, "does not exist")
}

func TestJobs_Versions(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a job by ID before it exists returns an error
	_, _, _, err := jobs.Versions("job1", false, nil)
	must.ErrorContains(t, err, "not found")

	// Register the job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	result, _, qm, err := jobs.Versions("job1", false, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	must.Eq(t, *job.ID, *result[0].ID)
}

func TestJobs_JobSubmission_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		var js *JobSubmission
		js.Canonicalize()
		must.Nil(t, js)
	})

	t.Run("empty variable flags", func(t *testing.T) {
		js := &JobSubmission{
			Source:        "abc123",
			Format:        "hcl2",
			VariableFlags: make(map[string]string),
		}
		js.Canonicalize()
		must.Nil(t, js.VariableFlags)
	})
}

func TestJobs_JobSubmission_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		var js *JobSubmission
		c := js.Copy()
		must.Nil(t, c)
	})

	t.Run("copy", func(t *testing.T) {
		js := &JobSubmission{
			Source:        "source",
			Format:        "format",
			VariableFlags: map[string]string{"foo": "bar"},
			Variables:     "variables",
		}
		c := js.Copy()
		c.Source = "source2"
		c.Format = "format2"
		c.VariableFlags["foo"] = "baz"
		c.Variables = "variables2"
		must.Eq(t, &JobSubmission{
			Source:        "source",
			Format:        "format",
			VariableFlags: map[string]string{"foo": "bar"},
			Variables:     "variables",
		}, js)
	})
}

func TestJobs_Submission_versions(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) { c.DevMode = true })
	t.Cleanup(s.Stop)

	jobs := c.Jobs()

	job := testJob()
	jobID := *job.ID                       // job1
	job.TaskGroups[0].Count = pointerOf(0) // no need to actually run

	// trying to retrieve a version before job is submitted returns a Not Found
	_, _, nfErr := jobs.Submission(jobID, 0, nil)
	must.ErrorContains(t, nfErr, "job source not found")

	// register our test job at version 0
	job.Meta = map[string]string{"v": "0"}
	_, wm, regErr := jobs.RegisterOpts(job, &RegisterOptions{
		Submission: &JobSubmission{
			Source:        "the job source v0",
			Format:        "hcl2",
			VariableFlags: map[string]string{"X": "x", "Y": "42", "Z": "true"},
			Variables:     "var file content",
		},
	}, nil)
	must.NoError(t, regErr)
	assertWriteMeta(t, wm)

	expectSubmission := func(sub *JobSubmission, format, source, vars string, flags map[string]string) {
		must.NotNil(t, sub, must.Sprintf("expected a non-nil job submission for job %s @ version %d", jobID, 0))
		must.Eq(t, format, sub.Format)
		must.Eq(t, source, sub.Source)
		must.Eq(t, vars, sub.Variables)
		must.MapEq(t, flags, sub.VariableFlags)
	}

	// we should have a version 0 now
	sub, _, err := jobs.Submission(jobID, 0, nil)
	must.NoError(t, err)
	expectSubmission(sub, "hcl2", "the job source v0", "var file content", map[string]string{"X": "x", "Y": "42", "Z": "true"})

	// register our test job at version 1
	job.Meta = map[string]string{"v": "1"}
	_, wm, regErr = jobs.RegisterOpts(job, &RegisterOptions{
		Submission: &JobSubmission{
			Source:        "the job source v1",
			Format:        "hcl2",
			VariableFlags: nil,
			Variables:     "different var content",
		},
	}, nil)
	must.NoError(t, regErr)
	assertWriteMeta(t, wm)

	// we should have a version 1 now
	sub, _, err = jobs.Submission(jobID, 1, nil)
	must.NoError(t, err)
	expectSubmission(sub, "hcl2", "the job source v1", "different var content", nil)

	// if we query for version 0 we should still have it
	sub, _, err = jobs.Submission(jobID, 0, nil)
	must.NoError(t, err)
	expectSubmission(sub, "hcl2", "the job source v0", "var file content", map[string]string{"X": "x", "Y": "42", "Z": "true"})

	// deregister (and purge) the job
	_, _, err = jobs.Deregister(jobID, true, &WriteOptions{Namespace: "default"})
	must.NoError(t, err)

	// now if we query for a submission of v0 it will be gone
	sub, _, err = jobs.Submission(jobID, 0, nil)
	must.ErrorContains(t, err, "job source not found")
	must.Nil(t, sub)

	// same for the v1 submission
	sub, _, err = jobs.Submission(jobID, 1, nil)
	must.ErrorContains(t, err, "job source not found")
	must.Nil(t, sub)
}

func TestJobs_Submission_namespaces(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) { c.DevMode = true })
	t.Cleanup(s.Stop)

	first := &Namespace{
		Name:        "first",
		Description: "first namespace",
	}

	second := &Namespace{
		Name:        "second",
		Description: "second namespace",
	}

	// create two namespaces
	namespaces := c.Namespaces()
	_, err := namespaces.Register(first, nil)
	must.NoError(t, err)
	_, err = namespaces.Register(second, nil)
	must.NoError(t, err)

	jobs := c.Jobs()

	// use the same jobID to prove we can query submissions of the same ID but
	// in different namespaces
	commonJobID := "common"

	job := testJob()
	job.ID = pointerOf(commonJobID)
	job.TaskGroups[0].Count = pointerOf(0)

	// register our test job into first namespace
	_, wm, err := jobs.RegisterOpts(job, &RegisterOptions{
		Submission: &JobSubmission{
			Source: "the job source",
			Format: "hcl2",
		},
	}, &WriteOptions{Namespace: "first"})
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// if we query in the default namespace the submission should not exist
	sub, _, err := jobs.Submission(commonJobID, 0, nil)
	must.ErrorContains(t, err, "not found")
	must.Nil(t, sub)

	// if we query in the first namespace we expect to get the submission
	sub, _, err = jobs.Submission(commonJobID, 0, &QueryOptions{Namespace: "first"})
	must.NoError(t, err)
	must.Eq(t, "the job source", sub.Source)

	// if we query in the second namespace we expect the submission should not exist
	sub, _, err = jobs.Submission(commonJobID, 0, &QueryOptions{Namespace: "second"})
	must.ErrorContains(t, err, "not found")
	must.Nil(t, sub)

	// create a second test job for our second namespace
	job2 := testJob()
	job2.ID = pointerOf(commonJobID)
	// keep job name redis to prove we write to correct namespace
	job.TaskGroups[0].Count = pointerOf(0)

	// register our second job into the second namespace
	_, wm, err = jobs.RegisterOpts(job2, &RegisterOptions{
		Submission: &JobSubmission{
			Source: "second job source",
			Format: "hcl1",
		},
	}, &WriteOptions{Namespace: "second"})
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// if we query in the default namespace the submission should not exist
	sub, _, err = jobs.Submission(commonJobID, 0, nil)
	must.ErrorContains(t, err, "not found")
	must.Nil(t, sub)

	// if we query in the first namespace we expect to get the first job submission
	sub, _, err = jobs.Submission(commonJobID, 0, &QueryOptions{Namespace: "first"})
	must.NoError(t, err)
	must.Eq(t, "the job source", sub.Source)

	// if we query in the second namespace we expect the second job submission
	sub, _, err = jobs.Submission(commonJobID, 0, &QueryOptions{Namespace: "second"})
	must.NoError(t, err)
	must.Eq(t, "second job source", sub.Source)

	// if we query v1 in the first namespace we expect nothing
	sub, _, err = jobs.Submission(commonJobID, 1, &QueryOptions{Namespace: "first"})
	must.ErrorContains(t, err, "not found")
	must.Nil(t, sub)

	// if we query v1 in the second namespace we expect nothing
	sub, _, err = jobs.Submission(commonJobID, 1, &QueryOptions{Namespace: "second"})
	must.ErrorContains(t, err, "not found")
	must.Nil(t, sub)
}

func TestJobs_Submission_delete(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) { c.DevMode = true })
	t.Cleanup(s.Stop)

	first := &Namespace{
		Name:        "first",
		Description: "first namespace",
	}

	namespaces := c.Namespaces()
	_, err := namespaces.Register(first, nil)
	must.NoError(t, err)

	jobs := c.Jobs()
	job := testJob()
	jobID := *job.ID
	job.TaskGroups[0].Count = pointerOf(0)
	job.Meta = map[string]string{"version": "0"}

	// register our test job into first namespace
	_, wm, err := jobs.RegisterOpts(job, &RegisterOptions{
		Submission: &JobSubmission{
			Source: "the job source v0",
			Format: "hcl2",
		},
	}, &WriteOptions{Namespace: "first"})
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// modify the job and register it again
	job.Meta["version"] = "1"
	_, wm, err = jobs.RegisterOpts(job, &RegisterOptions{
		Submission: &JobSubmission{
			Source: "the job source v1",
			Format: "hcl2",
		},
	}, &WriteOptions{Namespace: "first"})
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// ensure we have our submissions for both versions
	sub, _, err := jobs.Submission(jobID, 0, &QueryOptions{Namespace: "first"})
	must.NoError(t, err)
	must.Eq(t, "the job source v0", sub.Source)

	sub, _, err = jobs.Submission(jobID, 1, &QueryOptions{Namespace: "first"})
	must.NoError(t, err)
	must.Eq(t, "the job source v1", sub.Source)

	// deregister (and purge) the job
	_, _, err = jobs.Deregister(jobID, true, &WriteOptions{Namespace: "first"})
	must.NoError(t, err)

	// ensure all submissions for the job are gone
	sub, _, err = jobs.Submission(jobID, 0, &QueryOptions{Namespace: "first"})
	must.ErrorContains(t, err, "job source not found")
	must.Nil(t, sub)

	sub, _, err = jobs.Submission(jobID, 1, &QueryOptions{Namespace: "first"})
	must.ErrorContains(t, err, "job source not found")
	must.Nil(t, sub)
}

func TestJobs_PrefixList(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing when nothing exists returns empty
	results, _, err := jobs.PrefixList("dummy")
	must.NoError(t, err)
	must.SliceEmpty(t, results)

	// Register the job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	// Listing when nothing exists returns empty
	results, _, err = jobs.PrefixList((*job.ID)[:1])
	must.NoError(t, err)

	// Check if we have the right list
	must.Len(t, 1, results)
	must.Eq(t, *job.ID, results[0].ID)
}

func TestJobs_List(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing when nothing exists returns empty
	results, _, err := jobs.List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, results)

	// Register the job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the job again and ensure it exists
	// Listing when nothing exists returns empty
	results, _, err = jobs.List(nil)
	must.NoError(t, err)

	// Check if we have the right list
	must.Len(t, 1, results)
	must.Eq(t, *job.ID, results[0].ID)
}

func TestJobs_Allocations(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Looking up by a nonexistent job returns nothing
	allocs, qm, err := jobs.Allocations("job1", true, nil)
	must.NoError(t, err)
	must.Zero(t, qm.LastIndex)
	must.SliceEmpty(t, allocs)

	// TODO: do something here to create some allocations for
	// an existing job, lookup again.
}

func TestJobs_Evaluations(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Looking up by a nonexistent job ID returns nothing
	evals, qm, err := jobs.Evaluations("job1", nil)
	must.NoError(t, err)
	must.Zero(t, qm.LastIndex)
	must.SliceEmpty(t, evals)

	// Insert a job. This also creates an evaluation so we should
	// be able to query that out after.
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Look up the evaluations again.
	evals, qm, err = jobs.Evaluations("job1", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check that we got the evals back, evals are in order most recent to least recent
	// so the last eval is the original registered eval
	idx := len(evals) - 1
	must.Positive(t, len(evals))
	must.Eq(t, resp.EvalID, evals[idx].ID)
}

func TestJobs_Deregister(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Register a new job
	job := testJob()
	_, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Attempting delete on non-existing job does not return an error
	_, _, err = jobs.Deregister("nope", false, nil)
	must.NoError(t, err)

	// Do a soft deregister of an existing job
	evalID, wm3, err := jobs.Deregister("job1", false, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm3)
	must.UUIDv4(t, evalID)

	// Check that the job is still queryable
	out, qm1, err := jobs.Info("job1", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm1)
	must.NotNil(t, out)

	// Do a purge deregister of an existing job
	evalID, wm4, err := jobs.Deregister("job1", true, nil)
	must.NoError(t, err)

	assertWriteMeta(t, wm4)
	must.UUIDv4(t, evalID)

	// Check that the job is really gone
	result, qm, err := jobs.List(nil)
	must.NoError(t, err)

	assertQueryMeta(t, qm)
	must.SliceEmpty(t, result)
}

func TestJobs_Deregister_EvalPriority(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// Listing jobs before registering returns nothing
	listResp, _, err := c.Jobs().List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, listResp)

	// Create a job and register it.
	job := testJob()
	registerResp, wm, err := c.Jobs().Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, registerResp)
	must.UUIDv4(t, registerResp.EvalID)
	assertWriteMeta(t, wm)

	// Deregister the job with an eval priority.
	evalID, _, err := c.Jobs().DeregisterOpts(*job.ID, &DeregisterOptions{EvalPriority: 97}, nil)
	must.NoError(t, err)
	must.UUIDv4(t, evalID)

	// Lookup the eval and check the priority on it.
	evalInfo, _, err := c.Evaluations().Info(evalID, nil)
	must.NoError(t, err)
	must.Eq(t, 97, evalInfo.Priority)
}

func TestJobs_Deregister_NoEvalPriority(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// Listing jobs before registering returns nothing
	listResp, _, err := c.Jobs().List(nil)
	must.NoError(t, err)
	must.SliceEmpty(t, listResp)

	// Create a job and register it.
	job := testJob()
	registerResp, wm, err := c.Jobs().Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, registerResp)
	must.UUIDv4(t, registerResp.EvalID)
	assertWriteMeta(t, wm)

	// Deregister the job with an eval priority.
	evalID, _, err := c.Jobs().DeregisterOpts(*job.ID, &DeregisterOptions{}, nil)
	must.NoError(t, err)
	must.UUIDv4(t, evalID)

	// Lookup the eval and check the priority on it.
	evalInfo, _, err := c.Evaluations().Info(evalID, nil)
	must.NoError(t, err)
	must.Eq(t, *job.Priority, evalInfo.Priority)
}

func TestJobs_ForceEvaluate(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Force-eval on a non-existent job fails
	_, _, err := jobs.ForceEvaluate("job1", nil)
	must.ErrorContains(t, err, "not found")

	// Create a new job
	_, wm, err := jobs.Register(testJob(), nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Try force-eval again
	evalID, wm, err := jobs.ForceEvaluate("job1", nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Retrieve the evals and see if we get a matching one
	evals, qm, err := jobs.Evaluations("job1", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// todo(shoenig) fix must.SliceContainsFunc and use that
	// https://github.com/shoenig/test/issues/88
	for _, eval := range evals {
		if eval.ID == evalID {
			return
		}
	}
	t.Fatalf("evaluation %q missing", evalID)
}

func TestJobs_PeriodicForce(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	jobs := c.Jobs()

	// Force-eval on a nonexistent job fails
	_, _, err := jobs.PeriodicForce("job1", nil)
	must.ErrorContains(t, err, "not found")

	// Create a new job
	job := testPeriodicJob()
	_, _, err = jobs.Register(job, nil)
	must.NoError(t, err)

	f := func() error {
		out, _, err := jobs.Info(*job.ID, nil)
		if err != nil {
			return fmt.Errorf("failed to get jobs info: %w", err)
		}
		if out == nil {
			return fmt.Errorf("jobs info response is nil")
		}
		if *out.ID != *job.ID {
			return fmt.Errorf("expected job ids to match, out: %s, job: %s", *out.ID, *job.ID)
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(10*time.Second),
		wait.Gap(1*time.Second),
	))

	// Try force again
	evalID, wm, err := jobs.PeriodicForce(*job.ID, nil)
	must.NoError(t, err)

	assertWriteMeta(t, wm)

	must.NotEq(t, "", evalID)

	// Retrieve the eval
	evaluations := c.Evaluations()
	eval, qm, err := evaluations.Info(evalID, nil)
	must.NoError(t, err)

	assertQueryMeta(t, qm)
	must.Eq(t, eval.ID, evalID)
}

func TestJobs_Plan(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Create a job and attempt to register it
	job := testJob()
	resp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// Check that passing a nil job fails
	_, _, err = jobs.Plan(nil, true, nil)
	must.Error(t, err)

	// Check that passing a nil job ID fails
	invalidJob := testJob()
	invalidJob.ID = nil
	_, _, err = jobs.Plan(invalidJob, true, nil)
	must.Error(t, err)

	// Make a plan request
	planResp, wm, err := jobs.Plan(job, true, nil)
	must.NoError(t, err)
	must.NotNil(t, planResp)
	must.Positive(t, planResp.JobModifyIndex)
	must.NotNil(t, planResp.Diff)
	must.NotNil(t, planResp.Annotations)
	must.SliceNotEmpty(t, planResp.CreatedEvals)
	assertWriteMeta(t, wm)

	// Make a plan request w/o the diff
	planResp, wm, err = jobs.Plan(job, false, nil)
	must.NoError(t, err)
	must.NotNil(t, planResp)
	assertWriteMeta(t, wm)
	must.Positive(t, planResp.JobModifyIndex)
	must.Nil(t, planResp.Diff)
	must.NotNil(t, planResp.Annotations)
	must.SliceNotEmpty(t, planResp.CreatedEvals)
}

func TestJobs_JobSummary(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a job summary before the job exists
	// returns an error
	_, _, err := jobs.Summary("job1", nil)
	must.ErrorContains(t, err, "not found")

	// Register the job
	job := testJob()
	taskName := job.TaskGroups[0].Name
	_, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the job summary again and ensure it exists
	result, qm, err := jobs.Summary("job1", nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	must.Eq(t, *job.ID, result.JobID)

	_, ok := result.Summary[*taskName]
	must.True(t, ok)
}

func TestJobs_NewBatchJob(t *testing.T) {
	testutil.Parallel(t)

	job := NewBatchJob("job1", "myjob", "global", 5)
	expect := &Job{
		Region:   pointerOf("global"),
		ID:       pointerOf("job1"),
		Name:     pointerOf("myjob"),
		Type:     pointerOf(JobTypeBatch),
		Priority: pointerOf(5),
	}
	must.Eq(t, expect, job)
}

func TestJobs_NewServiceJob(t *testing.T) {
	testutil.Parallel(t)

	job := NewServiceJob("job1", "myjob", "global", 5)
	expect := &Job{
		Region:   pointerOf("global"),
		ID:       pointerOf("job1"),
		Name:     pointerOf("myjob"),
		Type:     pointerOf(JobTypeService),
		Priority: pointerOf(5),
	}
	must.Eq(t, expect, job)
}

func TestJobs_NewSystemJob(t *testing.T) {
	testutil.Parallel(t)

	job := NewSystemJob("job1", "myjob", "global", 5)
	expect := &Job{
		Region:   pointerOf("global"),
		ID:       pointerOf("job1"),
		Name:     pointerOf("myjob"),
		Type:     pointerOf(JobTypeSystem),
		Priority: pointerOf(5),
	}
	must.Eq(t, expect, job)
}

func TestJobs_NewSysbatchJob(t *testing.T) {
	testutil.Parallel(t)

	job := NewSysbatchJob("job1", "myjob", "global", 5)
	expect := &Job{
		Region:   pointerOf("global"),
		ID:       pointerOf("job1"),
		Name:     pointerOf("myjob"),
		Type:     pointerOf(JobTypeSysbatch),
		Priority: pointerOf(5),
	}
	must.Eq(t, expect, job)
}

func TestJobs_SetMeta(t *testing.T) {
	testutil.Parallel(t)
	job := &Job{Meta: nil}

	// Initializes a nil map
	out := job.SetMeta("foo", "bar")
	must.NotNil(t, job.Meta)

	// Check that the job was returned
	must.Eq(t, out, job)

	// Setting another pair is additive
	job.SetMeta("baz", "zip")
	expect := map[string]string{"foo": "bar", "baz": "zip"}
	must.Eq(t, expect, job.Meta)
}

func TestJobs_Constrain(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{Constraints: nil}

	// Create and add a constraint
	out := job.Constrain(NewConstraint("kernel.name", "=", "darwin"))
	must.Len(t, 1, job.Constraints)

	// Check that the job was returned
	must.Eq(t, job, out)

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
	must.Eq(t, expect, job.Constraints)
}

func TestJobs_AddAffinity(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{Affinities: nil}

	// Create and add an affinity
	out := job.AddAffinity(NewAffinity("kernel.version", "=", "4.6", 100))
	must.Len(t, 1, job.Affinities)

	// Check that the job was returned
	must.Eq(t, job, out)

	// Adding another affinity preserves the original
	job.AddAffinity(NewAffinity("${node.datacenter}", "=", "dc2", 50))
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
	must.Eq(t, expect, job.Affinities)
}

func TestJobs_Sort(t *testing.T) {
	testutil.Parallel(t)

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
	must.Eq(t, expect, jobs)
}

func TestJobs_AddSpread(t *testing.T) {
	testutil.Parallel(t)

	job := &Job{Spreads: nil}

	// Create and add a Spread
	spreadTarget := NewSpreadTarget("r1", 50)

	spread := NewSpread("${meta.rack}", 100, []*SpreadTarget{spreadTarget})
	out := job.AddSpread(spread)
	must.Len(t, 1, job.Spreads)

	// Check that the job was returned
	must.Eq(t, job, out)

	// Adding another spread preserves the original
	spreadTarget2 := NewSpreadTarget("dc1", 100)

	spread2 := NewSpread("${node.datacenter}", 100, []*SpreadTarget{spreadTarget2})
	job.AddSpread(spread2)

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
	must.Eq(t, expect, job.Spreads)
}

// TestJobs_ScaleAction tests the scale target for task group count
func TestJobs_ScaleAction(t *testing.T) {
	testutil.Parallel(t)

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
	_, _, err := jobs.Scale(id, "missing", pointerOf(newCount), "this won't work", false, nil, nil)
	must.ErrorContains(t, err, "not found")

	// Register the job
	regResp, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Perform scaling action
	scalingResp, wm, err := jobs.Scale(id, groupName,
		pointerOf(newCount), "need more instances", false,
		map[string]interface{}{
			"meta": "data",
		}, nil)

	must.NoError(t, err)
	must.NotNil(t, scalingResp)
	must.UUIDv4(t, scalingResp.EvalID)
	must.Positive(t, scalingResp.EvalCreateIndex)
	must.Greater(t, regResp.JobModifyIndex, scalingResp.JobModifyIndex)
	assertWriteMeta(t, wm)

	// Query the job again
	resp, _, err := jobs.Info(*job.ID, nil)
	must.NoError(t, err)
	must.Eq(t, *resp.TaskGroups[0].Count, newCount)

	// Check for the scaling event
	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	must.NoError(t, err)
	must.Len(t, 1, status.TaskGroups[groupName].Events)
	scalingEvent := status.TaskGroups[groupName].Events[0]
	must.False(t, scalingEvent.Error)
	must.Eq(t, "need more instances", scalingEvent.Message)
	must.MapEq(t, map[string]interface{}{"meta": "data"}, scalingEvent.Meta)
	must.Positive(t, scalingEvent.Time)
	must.UUIDv4(t, *scalingEvent.EvalID)
	must.Eq(t, scalingResp.EvalID, *scalingEvent.EvalID)
	must.Eq(t, int64(origCount), scalingEvent.PreviousCount)
}

func TestJobs_ScaleAction_Error(t *testing.T) {
	testutil.Parallel(t)

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
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Perform scaling action
	scaleResp, wm, err := jobs.Scale(id, groupName, nil, "something bad happened", true,
		map[string]interface{}{
			"meta": "data",
		}, nil)

	must.NoError(t, err)
	must.NotNil(t, scaleResp)
	must.Eq(t, "", scaleResp.EvalID)
	must.Zero(t, scaleResp.EvalCreateIndex)
	assertWriteMeta(t, wm)

	// Query the job again
	resp, _, err := jobs.Info(*job.ID, nil)
	must.NoError(t, err)
	must.Eq(t, *resp.TaskGroups[0].Count, prevCount)
	must.Eq(t, regResp.JobModifyIndex, scaleResp.JobModifyIndex)
	must.Zero(t, scaleResp.EvalCreateIndex)
	must.Eq(t, "", scaleResp.EvalID)

	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	must.NoError(t, err)
	must.Len(t, 1, status.TaskGroups[groupName].Events)
	errEvent := status.TaskGroups[groupName].Events[0]
	must.True(t, errEvent.Error)
	must.Eq(t, "something bad happened", errEvent.Message)
	must.Eq(t, map[string]interface{}{"meta": "data"}, errEvent.Meta)
	must.Positive(t, errEvent.Time)
	must.Nil(t, errEvent.EvalID)
}

func TestJobs_ScaleAction_Noop(t *testing.T) {
	testutil.Parallel(t)

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
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Perform scaling action
	scaleResp, wm, err := jobs.Scale(id, groupName, nil, "no count, just informative",
		false, map[string]interface{}{
			"meta": "data",
		}, nil)

	must.NoError(t, err)
	must.NotNil(t, scaleResp)
	must.Eq(t, "", scaleResp.EvalID)
	must.Zero(t, scaleResp.EvalCreateIndex)
	assertWriteMeta(t, wm)

	// Query the job again
	resp, _, err := jobs.Info(*job.ID, nil)
	must.NoError(t, err)
	must.Eq(t, *resp.TaskGroups[0].Count, prevCount)
	must.Eq(t, regResp.JobModifyIndex, scaleResp.JobModifyIndex)
	must.Zero(t, scaleResp.EvalCreateIndex)
	must.NotNil(t, scaleResp.EvalID)

	status, _, err := jobs.ScaleStatus(*job.ID, nil)
	must.NoError(t, err)
	must.Len(t, 1, status.TaskGroups[groupName].Events)
	noopEvent := status.TaskGroups[groupName].Events[0]
	must.False(t, noopEvent.Error)
	must.Eq(t, "no count, just informative", noopEvent.Message)
	must.MapEq(t, map[string]interface{}{"meta": "data"}, noopEvent.Meta)
	must.Positive(t, noopEvent.Time)
	must.Nil(t, noopEvent.EvalID)
}

// TestJobs_ScaleStatus tests the /scale status endpoint for task group count
func TestJobs_ScaleStatus(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Trying to retrieve a status before it exists returns an error
	id := "job-id/with\\troublesome:characters\n?&字"
	_, _, err := jobs.ScaleStatus(id, nil)
	must.ErrorContains(t, err, "not found")

	// Register the job
	job := testJob()
	job.ID = &id
	groupName := *job.TaskGroups[0].Name
	groupCount := *job.TaskGroups[0].Count
	_, wm, err := jobs.Register(job, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Query the scaling endpoint and verify success
	result, qm, err := jobs.ScaleStatus(id, nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	must.Eq(t, groupCount, result.TaskGroups[groupName].Desired)
}

func TestJobs_Services(t *testing.T) {
	// TODO(jrasell) add tests once registration process is in place.
}

// TestJobs_Parse asserts ParseHCL and ParseHCLOpts use the API to parse HCL.
func TestJobs_Parse(t *testing.T) {
	testutil.Parallel(t)

	jobspec := `job "example" {}`

	// Assert ParseHCL returns an error if Nomad is not running to ensure
	// that parsing is done server-side and not via the jobspec package.
	{
		c, err := NewClient(DefaultConfig())
		must.NoError(t, err)

		_, err = c.Jobs().ParseHCL(jobspec, false)
		must.ErrorContains(t, err, "Put")
	}

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// Test ParseHCL
	job1, err := c.Jobs().ParseHCL(jobspec, false)
	must.NoError(t, err)
	must.Eq(t, "example", *job1.Name)
	must.Nil(t, job1.Namespace)

	job1Canonicalized, err := c.Jobs().ParseHCL(jobspec, true)
	must.NoError(t, err)
	must.Eq(t, "example", *job1Canonicalized.Name)
	must.Eq(t, "default", *job1Canonicalized.Namespace)
	must.NotEq(t, job1, job1Canonicalized)

	// Test ParseHCLOpts
	req := &JobsParseRequest{
		JobHCL:       jobspec,
		HCLv1:        false,
		Canonicalize: false,
	}

	job2, err := c.Jobs().ParseHCLOpts(req)
	must.NoError(t, err)
	must.Eq(t, job1, job2)

	// Test ParseHCLOpts with Canonicalize=true
	req = &JobsParseRequest{
		JobHCL:       jobspec,
		HCLv1:        false,
		Canonicalize: true,
	}
	job2Canonicalized, err := c.Jobs().ParseHCLOpts(req)
	must.NoError(t, err)
	must.Eq(t, job1Canonicalized, job2Canonicalized)

	// Test ParseHCLOpts with HCLv1=true
	req = &JobsParseRequest{
		JobHCL:       jobspec,
		HCLv1:        true,
		Canonicalize: false,
	}

	job3, err := c.Jobs().ParseHCLOpts(req)
	must.NoError(t, err)
	must.Eq(t, job1, job3)

	// Test ParseHCLOpts with HCLv1=true and Canonicalize=true
	req = &JobsParseRequest{
		JobHCL:       jobspec,
		HCLv1:        true,
		Canonicalize: true,
	}
	job3Canonicalized, err := c.Jobs().ParseHCLOpts(req)
	must.NoError(t, err)
	must.Eq(t, job1Canonicalized, job3Canonicalized)
}
