package consul

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// defaultPollFreq is the default rate to poll the Consul Checks API
	defaultPollFreq = 900 * time.Millisecond
)

type ConsulChecks interface {
	Checks() (map[string]*api.AgentCheck, error)
}

type TaskRestarter interface {
	RestartDelay() time.Duration
	Restart(source, reason string, failure bool)
}

// checkRestart handles restarting a task if a check is unhealthy.
type checkRestart struct {
	allocID   string
	taskName  string
	checkID   string
	checkName string

	// remove this checkID (if true only checkID will be set)
	remove bool

	task         TaskRestarter
	restartDelay time.Duration
	grace        time.Duration
	interval     time.Duration
	timeLimit    time.Duration
	warning      bool

	// Mutable fields

	// unhealthyStart is the time a check first went unhealthy. Set to the
	// zero value if the check passes before timeLimit.
	unhealthyStart time.Time

	// graceUntil is when the check's grace period expires and unhealthy
	// checks should be counted.
	graceUntil time.Time

	logger *log.Logger
}

// update restart state for check and restart task if necessary. Currrent
// timestamp is passed in so all check updates have the same view of time (and
// to ease testing).
func (c *checkRestart) update(now time.Time, status string) {
	switch status {
	case api.HealthCritical:
	case api.HealthWarning:
		if !c.warning {
			// Warnings are ok, reset state and exit
			c.unhealthyStart = time.Time{}
			return
		}
	default:
		// All other statuses are ok, reset state and exit
		c.unhealthyStart = time.Time{}
		return
	}

	if now.Before(c.graceUntil) {
		// In grace period exit
		return
	}

	if c.unhealthyStart.IsZero() {
		// First failure, set restart deadline
		c.unhealthyStart = now
	}

	// restart timeLimit after start of this check becoming unhealthy
	restartAt := c.unhealthyStart.Add(c.timeLimit)

	// Must test >= because if limit=1, restartAt == first failure
	if now.Equal(restartAt) || now.After(restartAt) {
		// hasn't become healthy by deadline, restart!
		c.logger.Printf("[DEBUG] consul.health: restarting alloc %q task %q due to unhealthy check %q", c.allocID, c.taskName, c.checkName)

		// Tell TaskRunner to restart due to failure
		const failure = true
		c.task.Restart("healthcheck", fmt.Sprintf("check %q unhealthy", c.checkName), failure)

		// Reset grace time to grace + restart.delay + (restart.delay * 25%) (the max jitter)
		c.graceUntil = now.Add(c.grace + c.restartDelay + time.Duration(float64(c.restartDelay)*0.25))
		c.unhealthyStart = time.Time{}
	}
}

// checkWatcher watches Consul checks and restarts tasks when they're
// unhealthy.
type checkWatcher struct {
	consul ConsulChecks

	pollFreq time.Duration

	watchCh chan *checkRestart

	// done is closed when Run has exited
	done chan struct{}

	// lastErr is true if the last Consul call failed. It is used to
	// squelch repeated error messages.
	lastErr bool

	logger *log.Logger
}

// newCheckWatcher creates a new checkWatcher but does not call its Run method.
func newCheckWatcher(logger *log.Logger, consul ConsulChecks) *checkWatcher {
	return &checkWatcher{
		consul:   consul,
		pollFreq: defaultPollFreq,
		watchCh:  make(chan *checkRestart, 8),
		done:     make(chan struct{}),
		logger:   logger,
	}
}

// Run the main Consul checks watching loop to restart tasks when their checks
// fail. Blocks until context is canceled.
func (w *checkWatcher) Run(ctx context.Context) {
	defer close(w.done)

	// map of check IDs to their metadata
	checks := map[string]*checkRestart{}

	// timer for check polling
	checkTimer := time.NewTimer(0)
	defer checkTimer.Stop() // ensure timer is never leaked
	resetTimer := func(d time.Duration) {
		if !checkTimer.Stop() {
			<-checkTimer.C
		}
		checkTimer.Reset(d)
	}

	// Main watch loop
	for {
		// Don't start watching until we actually have checks that
		// trigger restarts.
		for len(checks) == 0 {
			select {
			case c := <-w.watchCh:
				if c.remove {
					// should not happen
					w.logger.Printf("[DEBUG] consul.health: told to stop watching an unwatched check: %q", c.checkID)
				} else {
					checks[c.checkID] = c

					// First check should be after grace period
					resetTimer(c.grace)
				}
			case <-ctx.Done():
				return
			}
		}

		// As long as there are checks to be watched, keep watching
		for len(checks) > 0 {
			select {
			case c := <-w.watchCh:
				if c.remove {
					delete(checks, c.checkID)
				} else {
					checks[c.checkID] = c
					w.logger.Printf("[DEBUG] consul.health: watching alloc %q task %q check %q", c.allocID, c.taskName, c.checkName)
				}
			case <-ctx.Done():
				return
			case <-checkTimer.C:
				checkTimer.Reset(w.pollFreq)

				// Set "now" as the point in time the following check results represent
				now := time.Now()

				results, err := w.consul.Checks()
				if err != nil {
					if !w.lastErr {
						w.lastErr = true
						w.logger.Printf("[ERR] consul.health: error retrieving health checks: %q", err)
					}
					continue
				}

				w.lastErr = false

				// Loop over watched checks and update their status from results
				for cid, check := range checks {
					result, ok := results[cid]
					if !ok {
						// Only warn if outside grace period to avoid races with check registration
						if now.After(check.graceUntil) {
							w.logger.Printf("[WARN] consul.health: watched check %q (%s) not found in Consul", check.checkName, cid)
						}
						continue
					}

					check.update(now, result.Status)
				}
			}
		}
	}
}

// Watch a task and restart it if unhealthy.
func (w *checkWatcher) Watch(allocID, taskName, checkID string, check *structs.ServiceCheck, restarter TaskRestarter) {
	if !check.Watched() {
		// Not watched, noop
		return
	}

	c := checkRestart{
		allocID:      allocID,
		taskName:     taskName,
		checkID:      checkID,
		checkName:    check.Name,
		task:         restarter,
		restartDelay: restarter.RestartDelay(),
		interval:     check.Interval,
		grace:        check.CheckRestart.Grace,
		graceUntil:   time.Now().Add(check.CheckRestart.Grace),
		timeLimit:    check.Interval * time.Duration(check.CheckRestart.Limit-1),
		warning:      check.CheckRestart.OnWarning,
		logger:       w.logger,
	}

	select {
	case w.watchCh <- &c:
		// sent watch
	case <-w.done:
		// exited; nothing to do
	}
}

// Unwatch a task.
func (w *checkWatcher) Unwatch(cid string) {
	c := checkRestart{
		checkID: cid,
		remove:  true,
	}
	select {
	case w.watchCh <- &c:
		// sent remove watch
	case <-w.done:
		// exited; nothing to do
	}
}
