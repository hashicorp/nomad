package restarts

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
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

func testExitResult(exit int) *drivers.ExitResult {
	return &drivers.ExitResult{
		ExitCode: exit,
	}
}

func TestClient_RestartTracker_ModeDelay(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeDelay)
	rt := NewRestartTracker(p, structs.JobTypeService)
	for i := 0; i < p.Attempts; i++ {
		state, when := rt.SetExitResult(testExitResult(127)).GetState()
		if state != structs.TaskRestarting {
			t.Fatalf("NextRestart() returned %v, want %v", state, structs.TaskRestarting)
		}
		if !withinJitter(p.Delay, when) {
			t.Fatalf("NextRestart() returned %v; want %v+jitter", when, p.Delay)
		}
	}

	// Follow up restarts should cause delay.
	for i := 0; i < 3; i++ {
		state, when := rt.SetExitResult(testExitResult(127)).GetState()
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
	rt := NewRestartTracker(p, structs.JobTypeSystem)
	for i := 0; i < p.Attempts; i++ {
		state, when := rt.SetExitResult(testExitResult(127)).GetState()
		if state != structs.TaskRestarting {
			t.Fatalf("NextRestart() returned %v, want %v", state, structs.TaskRestarting)
		}
		if !withinJitter(p.Delay, when) {
			t.Fatalf("NextRestart() returned %v; want %v+jitter", when, p.Delay)
		}
	}

	// Next restart should cause fail
	if state, _ := rt.SetExitResult(testExitResult(127)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("NextRestart() returned %v; want %v", state, structs.TaskNotRestarting)
	}
}

func TestClient_RestartTracker_NoRestartOnSuccess(t *testing.T) {
	t.Parallel()
	p := testPolicy(false, structs.RestartPolicyModeDelay)
	rt := NewRestartTracker(p, structs.JobTypeBatch)
	if state, _ := rt.SetExitResult(testExitResult(0)).GetState(); state != structs.TaskTerminated {
		t.Fatalf("NextRestart() returned %v, expected: %v", state, structs.TaskTerminated)
	}
}

func TestClient_RestartTracker_ZeroAttempts(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0

	// Test with a non-zero exit code
	rt := NewRestartTracker(p, structs.JobTypeService)
	if state, when := rt.SetExitResult(testExitResult(1)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("expect no restart, got restart/delay: %v/%v", state, when)
	}

	// Even with a zero (successful) exit code non-batch jobs should exit
	// with TaskNotRestarting
	rt = NewRestartTracker(p, structs.JobTypeService)
	if state, when := rt.SetExitResult(testExitResult(0)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("expect no restart, got restart/delay: %v/%v", state, when)
	}

	// Batch jobs with a zero exit code and 0 attempts *do* exit cleanly
	// with Terminated
	rt = NewRestartTracker(p, structs.JobTypeBatch)
	if state, when := rt.SetExitResult(testExitResult(0)).GetState(); state != structs.TaskTerminated {
		t.Fatalf("expect terminated, got restart/delay: %v/%v", state, when)
	}

	// Batch jobs with a non-zero exit code and 0 attempts exit with
	// TaskNotRestarting
	rt = NewRestartTracker(p, structs.JobTypeBatch)
	if state, when := rt.SetExitResult(testExitResult(1)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("expect no restart, got restart/delay: %v/%v", state, when)
	}
}

func TestClient_RestartTracker_TaskKilled(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0
	rt := NewRestartTracker(p, structs.JobTypeService)
	if state, when := rt.SetKilled().GetState(); state != structs.TaskKilled && when != 0 {
		t.Fatalf("expect no restart; got %v %v", state, when)
	}
}

func TestClient_RestartTracker_RestartTriggered(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0
	rt := NewRestartTracker(p, structs.JobTypeService)
	if state, when := rt.SetRestartTriggered(false).GetState(); state != structs.TaskRestarting && when != 0 {
		t.Fatalf("expect restart immediately, got %v %v", state, when)
	}
}

func TestClient_RestartTracker_RestartTriggered_Failure(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 1
	rt := NewRestartTracker(p, structs.JobTypeService)
	if state, when := rt.SetRestartTriggered(true).GetState(); state != structs.TaskRestarting || when == 0 {
		t.Fatalf("expect restart got %v %v", state, when)
	}
	if state, when := rt.SetRestartTriggered(true).GetState(); state != structs.TaskNotRestarting || when != 0 {
		t.Fatalf("expect failed got %v %v", state, when)
	}
}

func TestClient_RestartTracker_StartError_Recoverable_Fail(t *testing.T) {
	t.Parallel()
	p := testPolicy(true, structs.RestartPolicyModeFail)
	rt := NewRestartTracker(p, structs.JobTypeSystem)
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
	rt := NewRestartTracker(p, structs.JobTypeSystem)
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
