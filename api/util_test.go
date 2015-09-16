package api

import (
	"testing"
)

func assertQueryMeta(t *testing.T, qm *QueryMeta) {
	if qm.LastIndex == 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if qm.RequestTime == 0 {
		t.Fatalf("bad request time: %d", qm.RequestTime)
	}
	if !qm.KnownLeader {
		t.Fatalf("expected known leader, got none")
	}
}

func assertWriteMeta(t *testing.T, wm *WriteMeta) {
	if wm.LastIndex == 0 {
		t.Fatalf("bad index: %d", wm.LastIndex)
	}
	if wm.RequestTime == 0 {
		t.Fatalf("bad request time: %d", wm.RequestTime)
	}
}

func testJob() *Job {
	return &Job{
		Region:      "region1",
		ID:          "job1",
		Name:        "redis",
		Type:        JobTypeService,
		Datacenters: []string{"dc1"},
		TaskGroups: []*TaskGroup{
			&TaskGroup{
				Name:  "group1",
				Count: 1,
				Tasks: []*Task{
					&Task{
						Name:   "task1",
						Driver: "exec",
						Resources: &Resources{
							MemoryMB: 256,
						},
					},
				},
			},
		},
		Priority: 1,
	}
}
