package client

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

func newRestartTracker(policy *structs.RestartPolicy) *RestartTracker {
	return &RestartTracker{
		startTime: time.Now(),
		policy:    policy,
	}
}

type RestartTracker struct {
	count     int       // Current number of attempts.
	startTime time.Time // When the interval began
	policy    *structs.RestartPolicy
}

func (r *RestartTracker) NextRestart(exitCode int) (bool, time.Duration) {
	// Check if we have entered a new interval.
	end := r.startTime.Add(r.policy.Interval)
	now := time.Now()
	if now.After(end) {
		r.count = 0
		r.startTime = now
		return true, r.policy.Delay
	}

	r.count++

	// If we are under the attempts, restart with delay.
	if r.count <= r.policy.Attempts {
		return r.shouldRestart(exitCode), r.policy.Delay
	}

	// Don't restart since mode is "fail"
	if r.policy.Mode == structs.RestartPolicyModeFail {
		return false, 0
	}

	// Apply an artifical wait to enter the next interval
	return r.shouldRestart(exitCode), end.Sub(now)
}

// shouldRestart returns whether a restart should occur based on the exit code
// and the RestartOnSuccess configuration.
func (r *RestartTracker) shouldRestart(exitCode int) bool {
	return exitCode != 0 || r.policy.RestartOnSuccess && exitCode == 0
}

// Returns a tracker that never restarts.
func noRestartsTracker() *RestartTracker {
	policy := &structs.RestartPolicy{Attempts: 0, Mode: structs.RestartPolicyModeFail}
	return newRestartTracker(policy)
}
