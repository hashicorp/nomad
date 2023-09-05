package api

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/shoenig/test/must"
)

func assertQueryMeta(t *testing.T, qm *QueryMeta) {
	t.Helper()

	must.NotEq(t, 0, qm.LastIndex, must.Sprint("bad index"))
	must.True(t, qm.KnownLeader, must.Sprint("expected a known leader but gone none"))
}

func assertWriteMeta(t *testing.T, wm *WriteMeta) {
	t.Helper()
	if wm.LastIndex == 0 {
		t.Fatalf("bad index: %d", wm.LastIndex)
	}
}

func testJob() *Job {
	task := NewTask("task1", "raw_exec").
		SetConfig("command", "/bin/sleep").
		Require(&Resources{
			CPU:      pointerOf(100),
			MemoryMB: pointerOf(256),
		}).
		SetLogConfig(&LogConfig{
			MaxFiles:      pointerOf(1),
			MaxFileSizeMB: pointerOf(2),
		})

	group := NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: pointerOf(25),
		})

	job := NewBatchJob("job1", "redis", "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group)

	return job
}

func testServiceJob() *Job {
	// Create a job of type service
	task := NewTask("dummy-task", "exec").SetConfig("command", "/bin/sleep")
	group1 := NewTaskGroup("dummy-group", 1).AddTask(task)
	job := NewServiceJob("dummy-service", "dummy-service", "global", 5).AddTaskGroup(group1)
	return job
}

func testJobWithScalingPolicy() *Job {
	job := testJob()
	job.TaskGroups[0].Scaling = &ScalingPolicy{
		Policy:  map[string]interface{}{},
		Min:     pointerOf(int64(1)),
		Max:     pointerOf(int64(5)),
		Enabled: pointerOf(true),
	}
	return job
}

func testPeriodicJob() *Job {
	job := testJob().AddPeriodicConfig(&PeriodicConfig{
		Enabled:  pointerOf(true),
		Spec:     pointerOf("*/30 * * * *"),
		SpecType: pointerOf("cron"),
	})
	return job
}

func testRecommendation(job *Job) *Recommendation {
	rec := &Recommendation{
		ID:        "",
		Region:    *job.Region,
		Namespace: *job.Namespace,
		JobID:     *job.ID,
		Group:     *job.TaskGroups[0].Name,
		Task:      job.TaskGroups[0].Tasks[0].Name,
		Resource:  "CPU",
		Value:     *job.TaskGroups[0].Tasks[0].Resources.CPU * 2,
		Meta: map[string]interface{}{
			"testing": true,
			"mocked":  "also true",
		},
		Stats: map[string]float64{
			"median": 50.0,
			"mean":   51.0,
			"max":    75.5,
			"99":     73.0,
			"min":    0.0,
		},
		EnforceVersion: false,
	}
	return rec
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
					CPU:      pointerOf(2000),
					MemoryMB: pointerOf(2000),
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
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}
