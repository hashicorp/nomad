package client

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

func testPolicy(success bool, mode string) *structs.RestartPolicy {
	return &structs.RestartPolicy{
		Interval:         2 * time.Minute,
		Delay:            1 * time.Second,
		Attempts:         3,
		Mode:             mode,
		RestartOnSuccess: success,
	}
}

func TestClient_RestartTracker_ModeDelay(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeDelay)
	rt := newRestartTracker(p)
	for i := 0; i < p.Attempts; i++ {
		actual, when := rt.NextRestart(127)
		if !actual {
			t.Fatalf("NextRestart() returned %v, want %v", actual, true)
		}
		if when != p.Delay {
			t.Fatalf("NextRestart() returned %v; want %v", when, p.Delay)
		}
	}

	// Follow up restarts should cause delay.
	for i := 0; i < 3; i++ {
		actual, when := rt.NextRestart(127)
		if !actual {
			t.Fail()
		}
		if !(when > p.Delay && when < p.Interval) {
			t.Fatalf("NextRestart() returned %v; want less than %v and more than %v", when, p.Interval, p.Delay)
		}
	}
}

func TestClient_RestartTracker_ModeFail(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	rt := newRestartTracker(p)
	for i := 0; i < p.Attempts; i++ {
		actual, when := rt.NextRestart(127)
		if !actual {
			t.Fatalf("NextRestart() returned %v, want %v", actual, true)
		}
		if when != p.Delay {
			t.Fatalf("NextRestart() returned %v; want %v", when, p.Delay)
		}
	}

	// Next restart should cause fail
	if actual, _ := rt.NextRestart(127); actual {
		t.Fail()
	}
}

func TestClient_RestartTracker_NoRestartOnSuccess(t *testing.T) {
	t.Parallel()
	p := testPolicy(false, structs.RestartPolicyModeDelay)
	rt := newRestartTracker(p)
	if shouldRestart, _ := rt.NextRestart(0); shouldRestart {
		t.Fatalf("NextRestart() returned %v, expected: %v", shouldRestart, false)
	}

}
