package client

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"testing"
	"time"
)

func TestTaskRunner_ServiceRestartCounter(t *testing.T) {
	rt := newRestartTracker(structs.JobTypeService, &structs.RestartPolicy{Attempts: 2, Interval: 2 * time.Minute, Delay: 1 * time.Second})
	rt.increment()
	rt.increment()
	rt.increment()
	rt.increment()
	rt.increment()
	actual, _ := rt.nextRestart()
	if !actual {
		t.Fatalf("Expect %v, Actual: %v", true, actual)
	}
}

func TestTaskRunner_BatchRestartCounter(t *testing.T) {
	rt := newRestartTracker(structs.JobTypeBatch, &structs.RestartPolicy{Attempts: 2, Interval: 1 * time.Second, Delay: 1 * time.Second})
	rt.increment()
	rt.increment()
	rt.increment()
	rt.increment()
	rt.increment()
	actual, _ := rt.nextRestart()
	if actual {
		t.Fatalf("Expect %v, Actual: %v", false, actual)
	}

	time.Sleep(1 * time.Second)
	actual, _ = rt.nextRestart()
	if actual {
		t.Fatalf("Expect %v, Actual: %v", false, actual)
	}
}
