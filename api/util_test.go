package api

import (
	crand "crypto/rand"
	"fmt"
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

	job := NewBatchJob("job1", "redis", "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group)

	return job
}

func testJobWithScalingPolicy() *Job {
	job := testJob()
	job.TaskGroups[0].Scaling = &ScalingPolicy{
		Policy:  map[string]interface{}{},
		Min:     int64ToPtr(1),
		Max:     int64ToPtr(1),
		Enabled: boolToPtr(true),
	}
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

// float64ToPtr returns the pointer to an float64
func float64ToPtr(f float64) *float64 {
	return &f
}

// generateUUID generates a uuid useful for testing only
func generateUUID() string {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}
