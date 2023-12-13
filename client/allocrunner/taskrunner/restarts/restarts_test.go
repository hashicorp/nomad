// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package restarts

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
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
	ci.Parallel(t)
	p := testPolicy(true, structs.RestartPolicyModeDelay)
	rt := NewRestartTracker(p, structs.JobTypeService, nil)
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
	ci.Parallel(t)
	p := testPolicy(true, structs.RestartPolicyModeFail)
	rt := NewRestartTracker(p, structs.JobTypeSystem, nil)
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
	ci.Parallel(t)
	p := testPolicy(false, structs.RestartPolicyModeDelay)
	rt := NewRestartTracker(p, structs.JobTypeBatch, nil)
	if state, _ := rt.SetExitResult(testExitResult(0)).GetState(); state != structs.TaskTerminated {
		t.Fatalf("NextRestart() returned %v, expected: %v", state, structs.TaskTerminated)
	}
}

func TestClient_RestartTracker_ZeroAttempts(t *testing.T) {
	ci.Parallel(t)
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0

	// Test with a non-zero exit code
	rt := NewRestartTracker(p, structs.JobTypeService, nil)
	if state, when := rt.SetExitResult(testExitResult(1)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("expect no restart, got restart/delay: %v/%v", state, when)
	}

	// Even with a zero (successful) exit code non-batch jobs should exit
	// with TaskNotRestarting
	rt = NewRestartTracker(p, structs.JobTypeService, nil)
	if state, when := rt.SetExitResult(testExitResult(0)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("expect no restart, got restart/delay: %v/%v", state, when)
	}

	// Batch jobs with a zero exit code and 0 attempts *do* exit cleanly
	// with Terminated
	rt = NewRestartTracker(p, structs.JobTypeBatch, nil)
	if state, when := rt.SetExitResult(testExitResult(0)).GetState(); state != structs.TaskTerminated {
		t.Fatalf("expect terminated, got restart/delay: %v/%v", state, when)
	}

	// Batch jobs with a non-zero exit code and 0 attempts exit with
	// TaskNotRestarting
	rt = NewRestartTracker(p, structs.JobTypeBatch, nil)
	if state, when := rt.SetExitResult(testExitResult(1)).GetState(); state != structs.TaskNotRestarting {
		t.Fatalf("expect no restart, got restart/delay: %v/%v", state, when)
	}
}

func TestClient_RestartTracker_TaskKilled(t *testing.T) {
	ci.Parallel(t)
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0
	rt := NewRestartTracker(p, structs.JobTypeService, nil)
	if state, when := rt.SetKilled().GetState(); state != structs.TaskKilled && when != 0 {
		t.Fatalf("expect no restart; got %v %v", state, when)
	}
}

func TestClient_RestartTracker_RestartTriggered(t *testing.T) {
	ci.Parallel(t)
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 0
	rt := NewRestartTracker(p, structs.JobTypeService, nil)
	if state, when := rt.SetRestartTriggered(false).GetState(); state != structs.TaskRestarting && when != 0 {
		t.Fatalf("expect restart immediately, got %v %v", state, when)
	}
}

func TestClient_RestartTracker_RestartTriggered_Failure(t *testing.T) {
	ci.Parallel(t)
	p := testPolicy(true, structs.RestartPolicyModeFail)
	p.Attempts = 1
	rt := NewRestartTracker(p, structs.JobTypeService, nil)
	if state, when := rt.SetRestartTriggered(true).GetState(); state != structs.TaskRestarting || when == 0 {
		t.Fatalf("expect restart got %v %v", state, when)
	}
	if state, when := rt.SetRestartTriggered(true).GetState(); state != structs.TaskNotRestarting || when != 0 {
		t.Fatalf("expect failed got %v %v", state, when)
	}
}

func TestClient_RestartTracker_StartError_Recoverable_Fail(t *testing.T) {
	ci.Parallel(t)
	p := testPolicy(true, structs.RestartPolicyModeFail)
	rt := NewRestartTracker(p, structs.JobTypeSystem, nil)
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
	ci.Parallel(t)
	p := testPolicy(true, structs.RestartPolicyModeDelay)
	rt := NewRestartTracker(p, structs.JobTypeSystem, nil)
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

func TestClient_RestartTracker_Lifecycle(t *testing.T) {
	ci.Parallel(t)

	testCase := []struct {
		name                   string
		taskLifecycleConfig    *structs.TaskLifecycleConfig
		jobType                string
		shouldRestartOnSuccess bool
		shouldRestartOnFailure bool
	}{
		{
			name:                   "system job no lifecycle",
			taskLifecycleConfig:    nil,
			jobType:                structs.JobTypeSystem,
			shouldRestartOnSuccess: true,
			shouldRestartOnFailure: true,
		},
		{
			name:                   "service job no lifecycle",
			taskLifecycleConfig:    nil,
			jobType:                structs.JobTypeService,
			shouldRestartOnSuccess: true,
			shouldRestartOnFailure: true,
		},
		{
			name:                   "batch job no lifecycle",
			taskLifecycleConfig:    nil,
			jobType:                structs.JobTypeBatch,
			shouldRestartOnSuccess: false,
			shouldRestartOnFailure: true,
		},
		{
			name: "system job w/ ephemeral prestart hook",
			taskLifecycleConfig: &structs.TaskLifecycleConfig{
				Hook:    structs.TaskLifecycleHookPrestart,
				Sidecar: false,
			},
			jobType:                structs.JobTypeSystem,
			shouldRestartOnSuccess: false,
			shouldRestartOnFailure: true,
		},
		{
			name: "system job w/ sidecar prestart hook",
			taskLifecycleConfig: &structs.TaskLifecycleConfig{
				Hook:    structs.TaskLifecycleHookPrestart,
				Sidecar: true,
			},
			jobType:                structs.JobTypeSystem,
			shouldRestartOnSuccess: true,
			shouldRestartOnFailure: true,
		},
		{
			name: "service job w/ ephemeral prestart hook",
			taskLifecycleConfig: &structs.TaskLifecycleConfig{
				Hook:    structs.TaskLifecycleHookPrestart,
				Sidecar: false,
			},
			jobType:                structs.JobTypeService,
			shouldRestartOnSuccess: false,
			shouldRestartOnFailure: true,
		},
		{
			name: "service job w/ sidecar prestart hook",
			taskLifecycleConfig: &structs.TaskLifecycleConfig{
				Hook:    structs.TaskLifecycleHookPrestart,
				Sidecar: true,
			},
			jobType:                structs.JobTypeService,
			shouldRestartOnSuccess: true,
			shouldRestartOnFailure: true,
		},
		{
			name: "batch job w/ ephemeral prestart hook",
			taskLifecycleConfig: &structs.TaskLifecycleConfig{
				Hook:    structs.TaskLifecycleHookPrestart,
				Sidecar: false,
			},
			jobType:                structs.JobTypeService,
			shouldRestartOnSuccess: false,
			shouldRestartOnFailure: true,
		},
		{
			name: "batch job w/ sidecar prestart hook",
			taskLifecycleConfig: &structs.TaskLifecycleConfig{
				Hook:    structs.TaskLifecycleHookPrestart,
				Sidecar: true,
			},
			jobType:                structs.JobTypeBatch,
			shouldRestartOnSuccess: true,
			shouldRestartOnFailure: true,
		},
	}

	for _, testCase := range testCase {
		t.Run(testCase.name, func(t *testing.T) {
			restartPolicy := testPolicy(true, testCase.jobType)
			restartTracker := NewRestartTracker(restartPolicy, testCase.jobType, testCase.taskLifecycleConfig)

			state, _ := restartTracker.SetExitResult(testExitResult(0)).GetState()
			if !testCase.shouldRestartOnSuccess {
				require.Equal(t, structs.TaskTerminated, state)
			} else {
				require.Equal(t, structs.TaskRestarting, state)
			}

			state, _ = restartTracker.SetExitResult(testExitResult(127)).GetState()
			if !testCase.shouldRestartOnFailure {
				require.Equal(t, structs.TaskTerminated, state)
			} else {
				require.Equal(t, structs.TaskRestarting, state)
			}
		})
	}
}
