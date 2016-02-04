package client

import (
	"math/rand"
	"sync"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// jitter is the percent of jitter added to restart delays.
const jitter = 0.25

func newRestartTracker(policy *structs.RestartPolicy, jobType string) *RestartTracker {
	onSuccess := true
	if jobType == structs.JobTypeBatch {
		onSuccess = false
	}
	return &RestartTracker{
		startTime: time.Now(),
		onSuccess: onSuccess,
		policy:    policy,
		rand:      rand.New(rand.NewSource(time.Now().Unix())),
	}
}

type RestartTracker struct {
	count     int       // Current number of attempts.
	onSuccess bool      // Whether to restart on successful exit code.
	startTime time.Time // When the interval began
	policy    *structs.RestartPolicy
	rand      *rand.Rand
	lock      sync.Mutex
}

// SetPolicy updates the policy used to determine restarts.
func (r *RestartTracker) SetPolicy(policy *structs.RestartPolicy) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.policy = policy
}

// NextRestart takes the exit code from the last attempt and returns whether the
// task should be restarted and the duration to wait.
func (r *RestartTracker) NextRestart(exitCode int) (bool, time.Duration) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Hot path if no attempts are expected
	if r.policy.Attempts == 0 {
		return false, 0
	}

	// Check if we have entered a new interval.
	end := r.startTime.Add(r.policy.Interval)
	now := time.Now()
	if now.After(end) {
		r.count = 0
		r.startTime = now
		return r.shouldRestart(exitCode), r.jitter()
	}

	r.count++

	// If we are under the attempts, restart with delay.
	if r.count <= r.policy.Attempts {
		return r.shouldRestart(exitCode), r.jitter()
	}

	// Don't restart since mode is "fail"
	if r.policy.Mode == structs.RestartPolicyModeFail {
		return false, 0
	}

	// Apply an artifical wait to enter the next interval
	return r.shouldRestart(exitCode), end.Sub(now)
}

// shouldRestart returns whether a restart should occur based on the exit code
// and job type.
func (r *RestartTracker) shouldRestart(exitCode int) bool {
	return exitCode != 0 || r.onSuccess
}

// jitter returns the delay time plus a jitter.
func (r *RestartTracker) jitter() time.Duration {
	// Get the delay and ensure it is valid.
	d := r.policy.Delay.Nanoseconds()
	if d == 0 {
		d = 1
	}

	j := float64(r.rand.Int63n(d)) * jitter
	return time.Duration(d + int64(j))
}

// Returns a tracker that never restarts.
func noRestartsTracker() *RestartTracker {
	policy := &structs.RestartPolicy{Attempts: 0, Mode: structs.RestartPolicyModeFail}
	return newRestartTracker(policy, structs.JobTypeBatch)
}
