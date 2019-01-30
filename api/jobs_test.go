package api

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
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

func TestJobs_Parse(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	jobs := c.Jobs()

	checkJob := func(job *Job, expectedRegion string) {
		if job == nil {
			t.Fatal("job should not be nil")
		}

		region := job.Region

		if region == nil {
			if expectedRegion != "" {
				t.Fatalf("expected job region to be '%s' but was unset", expectedRegion)
			}
		} else {
			if expectedRegion != *region {
				t.Fatalf("expected job region '%s', but got '%s'", expectedRegion, *region)
			}
		}
	}
	job, err := jobs.ParseHCL(mock.HCL(), true)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	checkJob(job, "global")

	job, err = jobs.ParseHCL(mock.HCL(), false)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	checkJob(job, "")
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
				VaultToken:        stringToPtr(""),
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
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								KillTimeout: timeToPtr(5 * time.Second),
								LogConfig:   DefaultLogConfig(),
								Resources:   DefaultResources(),
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
				VaultToken:        stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
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
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   DefaultResources(),
								KillTimeout: timeToPtr(5 * time.Second),
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
										VaultGrace:   timeToPtr(3 * time.Second),
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
				VaultToken:        stringToPtr(""),
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

						Update: &UpdateStrategy{
							Stagger:          timeToPtr(30 * time.Second),
							MaxParallel:      intToPtr(1),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(10 * time.Second),
							HealthyDeadline:  timeToPtr(5 * time.Minute),
							ProgressDeadline: timeToPtr(10 * time.Minute),
							AutoRevert:       boolToPtr(false),
							Canary:           intToPtr(0),
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
										Name:        "redis-cache",
										Tags:        []string{"global", "cache"},
										CanaryTags:  []string{"canary", "global", "cache"},
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
										VaultGrace:   timeToPtr(15 * time.Second),
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
										VaultGrace:   timeToPtr(3 * time.Second),
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
				VaultToken:        stringToPtr(""),
				Stop:              boolToPtr(false),
				Stable:            boolToPtr(false),
				Version:           uint64ToPtr(0),
				Status:            stringToPtr(""),
				StatusDescription: stringToPtr(""),
				CreateIndex:       uint64ToPtr(0),
				ModifyIndex:       uint64ToPtr(0),
				JobModifyIndex:    uint64ToPtr(0),
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
				},
				TaskGroups: []*TaskGroup{
					{
						Name: stringToPtr("bar"),
						Update: &UpdateStrategy{
							Stagger:        timeToPtr(2 * time.Second),
							MaxParallel:    intToPtr(2),
							HealthCheck:    stringToPtr("manual"),
							MinHealthyTime: timeToPtr(1 * time.Second),
							AutoRevert:     boolToPtr(true),
							Canary:         intToPtr(1),
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
				VaultToken:        stringToPtr(""),
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
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(2 * time.Second),
							MaxParallel:      intToPtr(2),
							HealthCheck:      stringToPtr("manual"),
							MinHealthyTime:   timeToPtr(1 * time.Second),
							HealthyDeadline:  timeToPtr(6 * time.Minute),
							ProgressDeadline: timeToPtr(7 * time.Minute),
							AutoRevert:       boolToPtr(true),
							Canary:           intToPtr(1),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   DefaultResources(),
								KillTimeout: timeToPtr(5 * time.Second),
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
						Update: &UpdateStrategy{
							Stagger:          timeToPtr(1 * time.Second),
							MaxParallel:      intToPtr(1),
							HealthCheck:      stringToPtr("checks"),
							MinHealthyTime:   timeToPtr(10 * time.Second),
							HealthyDeadline:  timeToPtr(6 * time.Minute),
							ProgressDeadline: timeToPtr(7 * time.Minute),
							AutoRevert:       boolToPtr(false),
							Canary:           intToPtr(0),
						},
						Migrate: DefaultMigrateStrategy(),
						Tasks: []*Task{
							{
								Name:        "task1",
								LogConfig:   DefaultLogConfig(),
								Resources:   DefaultResources(),
								KillTimeout: timeToPtr(5 * time.Second),
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
	_, _, err = jobs.Revert(*job.ID, 0, uint64ToPtr(10), nil)
	if err == nil || !strings.Contains(err.Error(), "enforcing version") {
		t.Fatalf("expected enforcement error: %v", err)
	}

	// Works at correct index
	revertResp, wm, err := jobs.Revert(*job.ID, 0, uint64ToPtr(1), nil)
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
	job := NewBatchJob("job1", "myjob", "region1", 5)
	expect := &Job{
		Region:   stringToPtr("region1"),
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
	job := NewServiceJob("job1", "myjob", "region1", 5)
	expect := &Job{
		Region:   stringToPtr("region1"),
		ID:       stringToPtr("job1"),
		Name:     stringToPtr("myjob"),
		Type:     stringToPtr(JobTypeService),
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

func TestJobs_Summary_WithACL(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	c, s, root := makeACLClient(t, nil, nil)
	defer s.Stop()
	jobs := c.Jobs()

	invalidToken := mock.ACLToken()

	// Registering with an invalid  token should fail
	c.SetSecretID(invalidToken.SecretID)
	job := testJob()
	_, _, err := jobs.Register(job, nil)
	assert.NotNil(err)

	// Register with token should succeed
	c.SetSecretID(root.SecretID)
	resp2, wm, err := jobs.Register(job, nil)
	assert.Nil(err)
	assert.NotNil(resp2)
	assert.NotEqual("", resp2.EvalID)
	assertWriteMeta(t, wm)

	// Query the job summary with an invalid token should fail
	c.SetSecretID(invalidToken.SecretID)
	result, _, err := jobs.Summary(*job.ID, nil)
	assert.NotNil(err)

	// Query the job summary with a valid token should succeed
	c.SetSecretID(root.SecretID)
	result, qm, err := jobs.Summary(*job.ID, nil)
	assert.Nil(err)
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	assert.Equal(*job.ID, result.JobID)
}
