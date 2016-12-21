package client

import (
	"fmt"
	"testing"
	"time"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testPolicy(success bool, mode string) *structs.RestartPolicy {
	return &structs.RestartPolicy{
		Interval: 2 * time.Minute,
		Delay:    1 * time.Second,
		Attempts: 3,
		Mode:     mode,
	}
}

// withinJitter is a helper that returns whether the returned delay is within
// the jitter.
func withinJitter(expected, actual time.Duration) bool {
	return float64((actual.Nanoseconds()-expected.Nanoseconds())/
		expected.Nanoseconds()) <= jitter
}

func testWaitResult(exit int) *cstructs.WaitResult {
	return cstructs.NewWaitResult(exit, 0, nil)
}

func TestClient_RestartTracker_ModeDelay(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeDelay)
	rt := newRestartTracker(p, structs.JobTypeService)
	for i := 0; i < p.Attempts; i++ {
		state, when := rt.SetWaitResult(testWaitResult(127)).GetState()
		if state != structs.TaskRestarting {
			t.Fatalf("NextRestart() returned %v, want %v", state, structs.TaskRestarting)
		}
		if !withinJitter(p.Delay, when) {
			t.Fatalf("NextRestart() returned %v; want %v+jitter", when, p.Delay)
		}
	}

	// Follow up restarts should cause delay.
	for i := 0; i < 3; i++ {
		state, when := rt.SetWaitResult(testWaitResult(127)).GetState()
		if state != structs.TaskRestarting {
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
	rt := newRestartTracker(p, structs.JobTypeSystem)
	for i := 0; i < p.Attempts; i++ {
		state, when := rt.SetWaitResult(testWaitResult(127)).GetState()
		if state != structs.TaskRestarting {
			t.Fatalf("NextRestart() returned %v, want %v", state, structs.TaskRestarting)
		}
		if !withinJitter(p.Delay, when) {
			t.Fatalf("NextRestart() returned %v; want %v+jitter", when, p.Delay)
		}
	}

	// Next restart should cause fail
	if state, _ := rt.SetWaitResult(testWaitResult(127)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("NextRestart() returned %v; want %v", state, structs.TaskNotRestarting)
	}
}

func TestClient_RestartTracker_NoRestartOnSuccess(t *testing.T) {
	t.Parallel()
	p := testPolicy(false, structs.RestartPolicyModeDelay)
	rt := newRestartTracker(p, structs.JobTypeBatch)
	if state, _ := rt.SetWaitResult(testWaitResult(0)).GetState(); state != structs.TaskTerminated {
		t.Fatalf("NextRestart() returned %v, expected: %v", state, structs.TaskTerminated)
	}
}

func TestClient_RestartTracker_ZeroAttempts(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0
	rt := newRestartTracker(p, structs.JobTypeService)
	if state, when := rt.SetWaitResult(testWaitResult(1)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("expect no restart, got restart/delay: %v", when)
	}
}

func TestClient_RestartTracker_RestartTriggered(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0
	rt := newRestartTracker(p, structs.JobTypeService)
	if state, when := rt.SetRestartTriggered().GetState(); state != structs.TaskRestarting && when != 0 {
		t.Fatalf("expect restart immediately, got %v %v", state, when)
	}
}

func TestClient_RestartTracker_StartError_Recoverable_Fail(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	rt := newRestartTracker(p, structs.JobTypeSystem)
	recErr := structs.NewRecoverableError(fmt.Errorf("foo"), true)
	for i := 0; i < p.Attempts; i++ {
		state, when := rt.SetStartError(recErr).GetState()
		if state != structs.TaskRestarting {
			t.Fatalf("NextRestart() returned %v, want %v", state, structs.TaskRestarting)
		}
		if !withinJitter(p.Delay, when) {
			t.Fatalf("NextRestart() returned %v; want %v+jitter", when, p.Delay)
		}
	}

	// Next restart should cause fail
	if state, _ := rt.SetStartError(recErr).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("NextRestart() returned %v; want %v", state, structs.TaskNotRestarting)
	}
}

func TestClient_RestartTracker_StartError_Recoverable_Delay(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeDelay)
	rt := newRestartTracker(p, structs.JobTypeSystem)
	recErr := structs.NewRecoverableError(fmt.Errorf("foo"), true)
	for i := 0; i < p.Attempts; i++ {
		state, when := rt.SetStartError(recErr).GetState()
		if state != structs.TaskRestarting {
			t.Fatalf("NextRestart() returned %v, want %v", state, structs.TaskRestarting)
		}
		if !withinJitter(p.Delay, when) {
			t.Fatalf("NextRestart() returned %v; want %v+jitter", when, p.Delay)
		}
	}

	// Next restart should cause delay
	state, when := rt.SetStartError(recErr).GetState()
	if state != structs.TaskRestarting {
		t.Fatalf("NextRestart() returned %v; want %v", state, structs.TaskRestarting)
	}
	if !(when > p.Delay && when <= p.Interval) {
		t.Fatalf("NextRestart() returned %v; want > %v and <= %v", when, p.Delay, p.Interval)
	}
}
