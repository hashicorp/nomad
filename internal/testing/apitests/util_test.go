// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/pointer"
)

func assertQueryMeta(t *testing.T, qm *api.QueryMeta) {
	t.Helper()
	if qm.LastIndex == 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if !qm.KnownLeader {
		t.Fatalf("expected known leader, got none")
	}
}

func assertWriteMeta(t *testing.T, wm *api.WriteMeta) {
	t.Helper()
	if wm.LastIndex == 0 {
		t.Fatalf("bad index: %d", wm.LastIndex)
	}
}

func testJob() *api.Job {
	task := api.NewTask("task1", "exec").
		SetConfig("command", "/bin/sleep").
		Require(&api.Resources{
			CPU:      pointer.Of(100),
			MemoryMB: pointer.Of(256),
		}).
		SetLogConfig(&api.LogConfig{
			MaxFiles:      pointer.Of(1),
			MaxFileSizeMB: pointer.Of(2),
		})

	group := api.NewTaskGroup("group1", 1).
		AddTask(task).
		RequireDisk(&api.EphemeralDisk{
			SizeMB: pointer.Of(25),
		})

	job := api.NewBatchJob("job1", "redis", "global", 1).
		AddDatacenter("dc1").
		AddTaskGroup(group)

	return job
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
