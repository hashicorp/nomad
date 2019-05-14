package api

import (
	"testing"
)

func assertQueryMeta(t *testing.T, qm *QueryMeta) {
	t.Helper()
	if qm.LastIndex == 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if !qm.KnownLeader {
		t.Fatalf("expected known leader, got none")
	}
}

func assertWriteMeta(t *testing.T, wm *WriteMeta) {
	t.Helper()
	if wm.LastIndex == 0 {
		t.Fatalf("bad index: %d", wm.LastIndex)
	}
}

func testJob() *Job {
	task := NewTask("task1", "exec").
		SetConfig("command", "/bin/sleep").
		Require(&Resources{
			CPU:      intToPtr(100),
			MemoryMB: intToPtr(256),
		}).
		SetLogConfig(&LogConfig{
			MaxFiles:      intToPtr(1),
			MaxFileSizeMB: intToPtr(2),
		})

	group := NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: intToPtr(25),
		})

	job := NewBatchJob("job1", "redis", "region1", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group)

	return job
}

func testPeriodicJob() *Job {
	job := testJob().AddPeriodicConfig(&PeriodicConfig{
		Enabled:  boolToPtr(true),
		Spec:     stringToPtr("*/30 * * * *"),
		SpecType: stringToPtr("cron"),
	})
	return job
}

func testNamespace() *Namespace {
	return &Namespace{
		Name:        "test-namespace",
		Description: "Testing namespaces",
	}
}

func testQuotaSpec() *QuotaSpec {
	return &QuotaSpec{
		Name:        "test-namespace",
		Description: "Testing namespaces",
		Limits: []*QuotaLimit{
			{
				Region: "global",
				RegionLimit: &Resources{
					CPU:      intToPtr(2000),
					MemoryMB: intToPtr(2000),
				},
			},
		},
	}
}

// conversions utils only used for testing
// added here to avoid linter warning

// int64ToPtr returns the pointer to an int
func int64ToPtr(i int64) *int64 {
	return &i
}

// float64ToPtr returns the pointer to an float64
func float64ToPtr(f float64) *float64 {
	return &f
}
