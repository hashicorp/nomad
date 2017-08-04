package api

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
)

func TestJobs_Register(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, qm, err := jobs.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(resp); n != 0 {
		t.Fatalf("expected 0 jobs, got: %d", n)
	}

	// Create a job and attempt to register it
	job := testJob()
	resp2, wm, err := jobs.Register(job, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp2 == nil || resp2.EvalID == "" {
		t.Fatalf("missing eval id")
	}
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err = jobs.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that we got the expected response
	if len(resp) != 1 || resp[0].ID != *job.ID {
		t.Fatalf("bad: %#v", resp[0])
	}
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
				ID:                helper.StringToPtr(""),
				Name:              helper.StringToPtr(""),
				Region:            helper.StringToPtr("global"),
				Type:              helper.StringToPtr("service"),
				ParentID:          helper.StringToPtr(""),
				Priority:          helper.IntToPtr(50),
				AllAtOnce:         helper.BoolToPtr(false),
				VaultToken:        helper.StringToPtr(""),
				Status:            helper.StringToPtr(""),
				StatusDescription: helper.StringToPtr(""),
				Stop:              helper.BoolToPtr(false),
				Stable:            helper.BoolToPtr(false),
				Version:           helper.Uint64ToPtr(0),
				CreateIndex:       helper.Uint64ToPtr(0),
				ModifyIndex:       helper.Uint64ToPtr(0),
				JobModifyIndex:    helper.Uint64ToPtr(0),
				TaskGroups: []*TaskGroup{
					{
						Name:  helper.StringToPtr(""),
						Count: helper.IntToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  helper.BoolToPtr(false),
							Migrate: helper.BoolToPtr(false),
							SizeMB:  helper.IntToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    helper.TimeToPtr(15 * time.Second),
							Attempts: helper.IntToPtr(2),
							Interval: helper.TimeToPtr(1 * time.Minute),
							Mode:     helper.StringToPtr("delay"),
						},
						Tasks: []*Task{
							{
								KillTimeout: helper.TimeToPtr(5 * time.Second),
								LogConfig:   DefaultLogConfig(),
								Resources:   MinResources(),
							},
						},
					},
				},
			},
		},
		{
			name: "partial",
			input: &Job{
				Name:     helper.StringToPtr("foo"),
				ID:       helper.StringToPtr("bar"),
				ParentID: helper.StringToPtr("lol"),
				TaskGroups: []*TaskGroup{
					{
						Name: helper.StringToPtr("bar"),
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
				},
			},
			expected: &Job{
				ID:                helper.StringToPtr("bar"),
				Name:              helper.StringToPtr("foo"),
				Region:            helper.StringToPtr("global"),
				Type:              helper.StringToPtr("service"),
				ParentID:          helper.StringToPtr("lol"),
				Priority:          helper.IntToPtr(50),
				AllAtOnce:         helper.BoolToPtr(false),
				VaultToken:        helper.StringToPtr(""),
				Stop:              helper.BoolToPtr(false),
				Stable:            helper.BoolToPtr(false),
				Version:           helper.Uint64ToPtr(0),
				Status:            helper.StringToPtr(""),
				StatusDescription: helper.StringToPtr(""),
				CreateIndex:       helper.Uint64ToPtr(0),
				ModifyIndex:       helper.Uint64ToPtr(0),
				JobModifyIndex:    helper.Uint64ToPtr(0),
				TaskGroups: []*TaskGroup{
					{
						Name:  helper.StringToPtr("bar"),
						Count: helper.IntToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  helper.BoolToPtr(false),
							Migrate: helper.BoolToPtr(false),
							SizeMB:  helper.IntToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    helper.TimeToPtr(15 * time.Second),
							Attempts: helper.IntToPtr(2),
							Interval: helper.TimeToPtr(1 * time.Minute),
							Mode:     helper.StringToPtr("delay"),
						},
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   MinResources(),
								KillTimeout: helper.TimeToPtr(5 * time.Second),
							},
						},
					},
				},
			},
		},
		{
			name: "example_template",
			input: &Job{
				ID:          helper.StringToPtr("example_template"),
				Name:        helper.StringToPtr("example_template"),
				Datacenters: []string{"dc1"},
				Type:        helper.StringToPtr("service"),
				Update: &UpdateStrategy{
					MaxParallel: helper.IntToPtr(1),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  helper.StringToPtr("cache"),
						Count: helper.IntToPtr(1),
						RestartPolicy: &RestartPolicy{
							Interval: helper.TimeToPtr(5 * time.Minute),
							Attempts: helper.IntToPtr(10),
							Delay:    helper.TimeToPtr(25 * time.Second),
							Mode:     helper.StringToPtr("delay"),
						},
						EphemeralDisk: &EphemeralDisk{
							SizeMB: helper.IntToPtr(300),
						},
						Tasks: []*Task{
							{
								Name:   "redis",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "redis:3.2",
									"port_map": map[string]int{
										"db": 6379,
									},
								},
								Resources: &Resources{
									CPU:      helper.IntToPtr(500),
									MemoryMB: helper.IntToPtr(256),
									Networks: []*NetworkResource{
										{
											MBits: helper.IntToPtr(10),
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
										Name:      "global-redis-check",
										Tags:      []string{"global", "cache"},
										PortLabel: "db",
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
										EmbeddedTmpl: helper.StringToPtr("---"),
										DestPath:     helper.StringToPtr("local/file.yml"),
									},
									{
										EmbeddedTmpl: helper.StringToPtr("FOO=bar\n"),
										DestPath:     helper.StringToPtr("local/file.env"),
										Envvars:      helper.BoolToPtr(true),
									},
								},
							},
						},
					},
				},
			},
			expected: &Job{
				ID:                helper.StringToPtr("example_template"),
				Name:              helper.StringToPtr("example_template"),
				ParentID:          helper.StringToPtr(""),
				Priority:          helper.IntToPtr(50),
				Region:            helper.StringToPtr("global"),
				Type:              helper.StringToPtr("service"),
				AllAtOnce:         helper.BoolToPtr(false),
				VaultToken:        helper.StringToPtr(""),
				Stop:              helper.BoolToPtr(false),
				Stable:            helper.BoolToPtr(false),
				Version:           helper.Uint64ToPtr(0),
				Status:            helper.StringToPtr(""),
				StatusDescription: helper.StringToPtr(""),
				CreateIndex:       helper.Uint64ToPtr(0),
				ModifyIndex:       helper.Uint64ToPtr(0),
				JobModifyIndex:    helper.Uint64ToPtr(0),
				Datacenters:       []string{"dc1"},
				Update: &UpdateStrategy{
					Stagger:         helper.TimeToPtr(30 * time.Second),
					MaxParallel:     helper.IntToPtr(1),
					HealthCheck:     helper.StringToPtr("checks"),
					MinHealthyTime:  helper.TimeToPtr(10 * time.Second),
					HealthyDeadline: helper.TimeToPtr(5 * time.Minute),
					AutoRevert:      helper.BoolToPtr(false),
					Canary:          helper.IntToPtr(0),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  helper.StringToPtr("cache"),
						Count: helper.IntToPtr(1),
						RestartPolicy: &RestartPolicy{
							Interval: helper.TimeToPtr(5 * time.Minute),
							Attempts: helper.IntToPtr(10),
							Delay:    helper.TimeToPtr(25 * time.Second),
							Mode:     helper.StringToPtr("delay"),
						},
						EphemeralDisk: &EphemeralDisk{
							Sticky:  helper.BoolToPtr(false),
							Migrate: helper.BoolToPtr(false),
							SizeMB:  helper.IntToPtr(300),
						},

						Update: &UpdateStrategy{
							Stagger:         helper.TimeToPtr(30 * time.Second),
							MaxParallel:     helper.IntToPtr(1),
							HealthCheck:     helper.StringToPtr("checks"),
							MinHealthyTime:  helper.TimeToPtr(10 * time.Second),
							HealthyDeadline: helper.TimeToPtr(5 * time.Minute),
							AutoRevert:      helper.BoolToPtr(false),
							Canary:          helper.IntToPtr(0),
						},
						Tasks: []*Task{
							{
								Name:   "redis",
								Driver: "docker",
								Config: map[string]interface{}{
									"image": "redis:3.2",
									"port_map": map[string]int{
										"db": 6379,
									},
								},
								Resources: &Resources{
									CPU:      helper.IntToPtr(500),
									MemoryMB: helper.IntToPtr(256),
									IOPS:     helper.IntToPtr(0),
									Networks: []*NetworkResource{
										{
											MBits: helper.IntToPtr(10),
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
										Name:        "global-redis-check",
										Tags:        []string{"global", "cache"},
										PortLabel:   "db",
										AddressMode: "auto",
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
								KillTimeout: helper.TimeToPtr(5 * time.Second),
								LogConfig:   DefaultLogConfig(),
								Templates: []*Template{
									{
										SourcePath:   helper.StringToPtr(""),
										DestPath:     helper.StringToPtr("local/file.yml"),
										EmbeddedTmpl: helper.StringToPtr("---"),
										ChangeMode:   helper.StringToPtr("restart"),
										ChangeSignal: helper.StringToPtr(""),
										Splay:        helper.TimeToPtr(5 * time.Second),
										Perms:        helper.StringToPtr("0644"),
										LeftDelim:    helper.StringToPtr("{{"),
										RightDelim:   helper.StringToPtr("}}"),
										Envvars:      helper.BoolToPtr(false),
									},
									{
										SourcePath:   helper.StringToPtr(""),
										DestPath:     helper.StringToPtr("local/file.env"),
										EmbeddedTmpl: helper.StringToPtr("FOO=bar\n"),
										ChangeMode:   helper.StringToPtr("restart"),
										ChangeSignal: helper.StringToPtr(""),
										Splay:        helper.TimeToPtr(5 * time.Second),
										Perms:        helper.StringToPtr("0644"),
										LeftDelim:    helper.StringToPtr("{{"),
										RightDelim:   helper.StringToPtr("}}"),
										Envvars:      helper.BoolToPtr(true),
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
				ID:       helper.StringToPtr("bar"),
				Periodic: &PeriodicConfig{},
			},
			expected: &Job{
				ID:                helper.StringToPtr("bar"),
				ParentID:          helper.StringToPtr(""),
				Name:              helper.StringToPtr("bar"),
				Region:            helper.StringToPtr("global"),
				Type:              helper.StringToPtr("service"),
				Priority:          helper.IntToPtr(50),
				AllAtOnce:         helper.BoolToPtr(false),
				VaultToken:        helper.StringToPtr(""),
				Stop:              helper.BoolToPtr(false),
				Stable:            helper.BoolToPtr(false),
				Version:           helper.Uint64ToPtr(0),
				Status:            helper.StringToPtr(""),
				StatusDescription: helper.StringToPtr(""),
				CreateIndex:       helper.Uint64ToPtr(0),
				ModifyIndex:       helper.Uint64ToPtr(0),
				JobModifyIndex:    helper.Uint64ToPtr(0),
				Periodic: &PeriodicConfig{
					Enabled:         helper.BoolToPtr(true),
					Spec:            helper.StringToPtr(""),
					SpecType:        helper.StringToPtr(PeriodicSpecCron),
					ProhibitOverlap: helper.BoolToPtr(false),
					TimeZone:        helper.StringToPtr("UTC"),
				},
			},
		},

		{
			name: "update_merge",
			input: &Job{
				Name:     helper.StringToPtr("foo"),
				ID:       helper.StringToPtr("bar"),
				ParentID: helper.StringToPtr("lol"),
				Update: &UpdateStrategy{
					Stagger:         helper.TimeToPtr(1 * time.Second),
					MaxParallel:     helper.IntToPtr(1),
					HealthCheck:     helper.StringToPtr("checks"),
					MinHealthyTime:  helper.TimeToPtr(10 * time.Second),
					HealthyDeadline: helper.TimeToPtr(6 * time.Minute),
					AutoRevert:      helper.BoolToPtr(false),
					Canary:          helper.IntToPtr(0),
				},
				TaskGroups: []*TaskGroup{
					{
						Name: helper.StringToPtr("bar"),
						Update: &UpdateStrategy{
							Stagger:        helper.TimeToPtr(2 * time.Second),
							MaxParallel:    helper.IntToPtr(2),
							HealthCheck:    helper.StringToPtr("manual"),
							MinHealthyTime: helper.TimeToPtr(1 * time.Second),
							AutoRevert:     helper.BoolToPtr(true),
							Canary:         helper.IntToPtr(1),
						},
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
					{
						Name: helper.StringToPtr("baz"),
						Tasks: []*Task{
							{
								Name: "task1",
							},
						},
					},
				},
			},
			expected: &Job{
				ID:                helper.StringToPtr("bar"),
				Name:              helper.StringToPtr("foo"),
				Region:            helper.StringToPtr("global"),
				Type:              helper.StringToPtr("service"),
				ParentID:          helper.StringToPtr("lol"),
				Priority:          helper.IntToPtr(50),
				AllAtOnce:         helper.BoolToPtr(false),
				VaultToken:        helper.StringToPtr(""),
				Stop:              helper.BoolToPtr(false),
				Stable:            helper.BoolToPtr(false),
				Version:           helper.Uint64ToPtr(0),
				Status:            helper.StringToPtr(""),
				StatusDescription: helper.StringToPtr(""),
				CreateIndex:       helper.Uint64ToPtr(0),
				ModifyIndex:       helper.Uint64ToPtr(0),
				JobModifyIndex:    helper.Uint64ToPtr(0),
				Update: &UpdateStrategy{
					Stagger:         helper.TimeToPtr(1 * time.Second),
					MaxParallel:     helper.IntToPtr(1),
					HealthCheck:     helper.StringToPtr("checks"),
					MinHealthyTime:  helper.TimeToPtr(10 * time.Second),
					HealthyDeadline: helper.TimeToPtr(6 * time.Minute),
					AutoRevert:      helper.BoolToPtr(false),
					Canary:          helper.IntToPtr(0),
				},
				TaskGroups: []*TaskGroup{
					{
						Name:  helper.StringToPtr("bar"),
						Count: helper.IntToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  helper.BoolToPtr(false),
							Migrate: helper.BoolToPtr(false),
							SizeMB:  helper.IntToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    helper.TimeToPtr(15 * time.Second),
							Attempts: helper.IntToPtr(2),
							Interval: helper.TimeToPtr(1 * time.Minute),
							Mode:     helper.StringToPtr("delay"),
						},
						Update: &UpdateStrategy{
							Stagger:         helper.TimeToPtr(2 * time.Second),
							MaxParallel:     helper.IntToPtr(2),
							HealthCheck:     helper.StringToPtr("manual"),
							MinHealthyTime:  helper.TimeToPtr(1 * time.Second),
							HealthyDeadline: helper.TimeToPtr(6 * time.Minute),
							AutoRevert:      helper.BoolToPtr(true),
							Canary:          helper.IntToPtr(1),
						},
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   MinResources(),
								KillTimeout: helper.TimeToPtr(5 * time.Second),
							},
						},
					},
					{
						Name:  helper.StringToPtr("baz"),
						Count: helper.IntToPtr(1),
						EphemeralDisk: &EphemeralDisk{
							Sticky:  helper.BoolToPtr(false),
							Migrate: helper.BoolToPtr(false),
							SizeMB:  helper.IntToPtr(300),
						},
						RestartPolicy: &RestartPolicy{
							Delay:    helper.TimeToPtr(15 * time.Second),
							Attempts: helper.IntToPtr(2),
							Interval: helper.TimeToPtr(1 * time.Minute),
							Mode:     helper.StringToPtr("delay"),
						},
						Update: &UpdateStrategy{
							Stagger:         helper.TimeToPtr(1 * time.Second),
							MaxParallel:     helper.IntToPtr(1),
							HealthCheck:     helper.StringToPtr("checks"),
							MinHealthyTime:  helper.TimeToPtr(10 * time.Second),
							HealthyDeadline: helper.TimeToPtr(6 * time.Minute),
							AutoRevert:      helper.BoolToPtr(false),
							Canary:          helper.IntToPtr(0),
						},
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   MinResources(),
								KillTimeout: helper.TimeToPtr(5 * time.Second),
							},
						},
					},
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
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	// Listing jobs before registering returns nothing
	resp, qm, err := jobs.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(resp); n != 0 {
		t.Fatalf("expected 0 jobs, got: %d", n)
	}

	// Create a job and attempt to register it with an incorrect index.
	job := testJob()
	resp2, wm, err := jobs.EnforceRegister(job, 10, nil)
	if err == nil || !strings.Contains(err.Error(), RegisterEnforceIndexErrPrefix) {
		t.Fatalf("expected enforcement error: %v", err)
	}

	// Register
	resp2, wm, err = jobs.EnforceRegister(job, 0, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp2 == nil || resp2.EvalID == "" {
		t.Fatalf("missing eval id")
	}
	assertWriteMeta(t, wm)

	// Query the jobs back out again
	resp, qm, err = jobs.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that we got the expected response
	if len(resp) != 1 {
		t.Fatalf("bad length: %d", len(resp))
	}

	if resp[0].ID != *job.ID {
		t.Fatalf("bad: %#v", resp[0])
	}
	curIndex := resp[0].JobModifyIndex

	// Fail at incorrect index
	resp2, wm, err = jobs.EnforceRegister(job, 123456, nil)
	if err == nil || !strings.Contains(err.Error(), RegisterEnforceIndexErrPrefix) {
		t.Fatalf("expected enforcement error: %v", err)
	}

	// Works at correct index
	resp3, wm, err := jobs.EnforceRegister(job, curIndex, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if resp3 == nil || resp3.EvalID == "" {
		t.Fatalf("missing eval id")
	}
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
	_, wm, err = jobs.Revert(*job.ID, 0, helper.Uint64ToPtr(10), nil)
	if err == nil || !strings.Contains(err.Error(), "enforcing version") {
		t.Fatalf("expected enforcement error: %v", err)
	}

	// Works at correct index
	revertResp, wm, err := jobs.Revert(*job.ID, 0, helper.Uint64ToPtr(1), nil)
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
	_, _, err := jobs.Info("job1", nil)
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
	result, qm, err := jobs.Info("job1", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	if result == nil || *result.ID != *job.ID {
		t.Fatalf("expect: %#v, got: %#v", job, result)
	}
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
	results, qm, err := jobs.PrefixList("dummy")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
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
	results, qm, err = jobs.PrefixList((*job.ID)[:1])
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
	results, qm, err := jobs.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
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
	results, qm, err = jobs.List(nil)
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

	// Looking up by a non-existent job returns nothing
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

	// Looking up by a non-existent job ID returns nothing
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

	// Force-eval on a non-existent job fails
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
	job := NewBatchJob("job1", "myjob", "region1", 5)
	expect := &Job{
		Region:   helper.StringToPtr("region1"),
		ID:       helper.StringToPtr("job1"),
		Name:     helper.StringToPtr("myjob"),
		Type:     helper.StringToPtr(JobTypeBatch),
		Priority: helper.IntToPtr(5),
	}
	if !reflect.DeepEqual(job, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job)
	}
}

func TestJobs_NewServiceJob(t *testing.T) {
	t.Parallel()
	job := NewServiceJob("job1", "myjob", "region1", 5)
	expect := &Job{
		Region:   helper.StringToPtr("region1"),
		ID:       helper.StringToPtr("job1"),
		Name:     helper.StringToPtr("myjob"),
		Type:     helper.StringToPtr(JobTypeService),
		Priority: helper.IntToPtr(5),
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
	if !reflect.DeepEqual(job.Constraints, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, job.Constraints)
	}
}

func TestJobs_Sort(t *testing.T) {
	t.Parallel()
	jobs := []*JobListStub{
		&JobListStub{ID: "job2"},
		&JobListStub{ID: "job0"},
		&JobListStub{ID: "job1"},
	}
	sort.Sort(JobIDSort(jobs))

	expect := []*JobListStub{
		&JobListStub{ID: "job0"},
		&JobListStub{ID: "job1"},
		&JobListStub{ID: "job2"},
	}
	if !reflect.DeepEqual(jobs, expect) {
		t.Fatalf("\n\n%#v\n\n%#v", jobs, expect)
	}
}
