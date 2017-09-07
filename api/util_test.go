package api

import (
	"testing"

	"github.com/hashicorp/nomad/helper"
)

func assertQueryMeta(t *testing.T, qm *QueryMeta) {
	if qm.LastIndex == 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if !qm.KnownLeader {
		t.Fatalf("expected known leader, got none")
	}
}

func assertWriteMeta(t *testing.T, wm *WriteMeta) {
	if wm.LastIndex == 0 {
		t.Fatalf("bad index: %d", wm.LastIndex)
	}
}

func testJob() *Job {
	task := NewTask("task1", "exec").
		SetConfig("command", "/bin/sleep").
		Require(&Resources{
			CPU:      helper.IntToPtr(100),
			MemoryMB: helper.IntToPtr(256),
			IOPS:     helper.IntToPtr(10),
		}).
		SetLogConfig(&LogConfig{
			MaxFiles:      helper.IntToPtr(1),
			MaxFileSizeMB: helper.IntToPtr(2),
		})

	group := NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&EphemeralDisk{
			SizeMB: helper.IntToPtr(25),
		})

	job := NewBatchJob("job1", "redis", "region1", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group)

	return job
}

func testPeriodicJob() *Job {
	job := testJob().AddPeriodicConfig(&PeriodicConfig{
		Enabled:  helper.BoolToPtr(true),
		Spec:     helper.StringToPtr("*/30 * * * *"),
		SpecType: helper.StringToPtr("cron"),
	})
	return job
}

func testNamespace() *Namespace {
	return &Namespace{
		Name:        "test-namespace",
		Description: "Testing namespaces",
	}
}
