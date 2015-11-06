package client

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"testing"
	"time"
)

func TestTaskRunner_ServiceRestartCounter(t *testing.T) {
	interval := 2 * time.Minute
	delay := 1 * time.Second
	attempts := 3
	rt := newRestartTracker(structs.JobTypeService, &structs.RestartPolicy{Attempts: attempts, Interval: interval, Delay: delay})

	for i := 0; i < attempts; i++ {
		actual, when := rt.nextRestart()
		if !actual {
			t.Fatalf("should restart returned %v, actual %v", actual, true)
		}
		if when != delay {
			t.Fatalf("nextRestart() returned %v; want %v", when, delay)
		}
	}

	time.Sleep(1 * time.Second)
	for i := 0; i < 3; i++ {
		actual, when := rt.nextRestart()
		if !actual {
			t.Fail()
		}
		if !(when > delay && when < interval) {
			t.Fatalf("nextRestart() returned %v; want less than %v and more than %v", when, interval, delay)
		}
	}

}

func TestTaskRunner_BatchRestartCounter(t *testing.T) {
	attempts := 2
	interval := 1 * time.Second
	delay := 1 * time.Second
	rt := newRestartTracker(structs.JobTypeBatch,
		&structs.RestartPolicy{Attempts: attempts,
			Interval: interval,
			Delay:    delay,
		},
	)
	for i := 0; i < attempts; i++ {
		shouldRestart, when := rt.nextRestart()
		if !shouldRestart {
			t.Fatalf("should restart returned %v, actual %v", shouldRestart, true)
		}
		if when != delay {
			t.Fatalf("Delay should be %v, actual: %v", delay, when)
		}
	}
	actual, _ := rt.nextRestart()
	if actual {
		t.Fatalf("Expect %v, Actual: %v", false, actual)
	}
}
