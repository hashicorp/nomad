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

// withinJitter is a helper that returns whether the returned delay is within
// the jitter.
func withinJitter(expected, actual time.Duration) bool {
	return float64((actual.Nanoseconds()-expected.Nanoseconds())/
		expected.Nanoseconds()) <= jitter
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
		if !withinJitter(p.Delay, when) {
			t.Fatalf("NextRestart() returned %v; want %v+jitter", when, p.Delay)
		}
	}

	// Follow up restarts should cause delay.
	for i := 0; i < 3; i++ {
		actual, when := rt.NextRestart(127)
		if !actual {
			t.Fail()
		}
		if !(when > p.Delay && when <= p.Interval) {
			t.Fatalf("NextRestart() returned %v; want > %v and <= %v", when, p.Delay, p.Interval)
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
		if !withinJitter(p.Delay, when) {
			t.Fatalf("NextRestart() returned %v; want %v+jitter", when, p.Delay)
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

func TestClient_RestartTracker_ZeroAttempts(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0
	rt := newRestartTracker(p)
	if actual, when := rt.NextRestart(1); actual {
		t.Fatalf("expect no restart, got restart/delay: %v", when)
	}
}
