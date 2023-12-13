// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package serviceregistration

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// composite of allocID + taskName for uniqueness
type key string

type restarter struct {
	allocID   string
	taskName  string
	checkID   string
	checkName string
	taskKey   key

	logger         hclog.Logger
	task           WorkloadRestarter
	grace          time.Duration
	interval       time.Duration
	timeLimit      time.Duration
	ignoreWarnings bool

	// unhealthyState is the time a check first went unhealthy. Set to the
	// zero value if the check passes before timeLimit.
	unhealthyState time.Time

	// graceUntil is when the check's grace period expires and unhealthy
	// checks should be counted.
	graceUntil time.Time
}

// apply restart state for check and restart task if necessary. Current
// timestamp is passed in so all check updates have the same view of time (and
// to ease testing).
//
// Returns true if a restart was triggered in which case this check should be
// removed (checks are added on task startup).
func (r *restarter) apply(ctx context.Context, now time.Time, status string) bool {
	healthy := func() {
		if !r.unhealthyState.IsZero() {
			r.logger.Debug("canceling restart because check became healthy")
			r.unhealthyState = time.Time{}
		}
	}
	switch status {
	case "critical": // consul
	case string(structs.CheckFailure): // nomad
	case string(structs.CheckPending): // nomad
	case "warning": // consul
		if r.ignoreWarnings {
			// Warnings are ignored, reset state and exit
			healthy()
			return false
		}
	default:
		// All other statuses are ok, reset state and exit
		healthy()
		return false
	}

	if now.Before(r.graceUntil) {
		// In grace period, exit
		return false
	}

	if r.unhealthyState.IsZero() {
		// First failure, set restart deadline
		if r.timeLimit != 0 {
			r.logger.Debug("check became unhealthy. Will restart if check doesn't become healthy", "time_limit", r.timeLimit)
		}
		r.unhealthyState = now
	}

	// restart timeLimit after start of this check becoming unhealthy
	restartAt := r.unhealthyState.Add(r.timeLimit)

	// Must test >= because if limit=1, restartAt == first failure
	if now.Equal(restartAt) || now.After(restartAt) {
		// hasn't become healthy by deadline, restart!
		r.logger.Debug("restarting due to unhealthy check")

		// Tell TaskRunner to restart due to failure
		reason := fmt.Sprintf("healthcheck: check %q unhealthy", r.checkName)
		event := structs.NewTaskEvent(structs.TaskRestartSignal).SetRestartReason(reason)
		go asyncRestart(ctx, r.logger, r.task, event)
		return true
	}

	return false
}

// asyncRestart mimics the pre-0.9 TaskRunner.Restart behavior and is intended
// to be called in a goroutine.
func asyncRestart(ctx context.Context, logger hclog.Logger, task WorkloadRestarter, event *structs.TaskEvent) {
	// Check watcher restarts are always failures
	const failure = true

	// Restarting is asynchronous so there's no reason to allow this
	// goroutine to block indefinitely.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := task.Restart(ctx, event, failure); err != nil {
		// Restart errors are not actionable and only relevant when
		// debugging allocation lifecycle management.
		logger.Debug("failed to restart task", "error", err, "event_time", event.Time, "event_type", event.Type)
	}
}

// CheckStatusGetter is implemented per-provider.
type CheckStatusGetter interface {
	// Get returns a map from CheckID -> (minimal) CheckStatus
	Get() (map[string]string, error)
}

// checkWatchUpdates add or remove checks from the watcher
type checkWatchUpdate struct {
	checkID string
	remove  bool
	restart *restarter
}

// A CheckWatcher watches for check failures and restarts tasks according to
// their check_restart policy.
type CheckWatcher interface {
	// Run the CheckWatcher. Maintains a background process to continuously
	// monitor active checks. Must be called before Watch or Unwatch. Must be
	// called as a goroutine.
	Run(ctx context.Context)

	// Watch the given check. If the check status enters a failing state, the
	// task associated with the check will be restarted according to its check_restart
	// policy via wr.
	Watch(allocID, taskName, checkID string, check *structs.ServiceCheck, wr WorkloadRestarter)

	// Unwatch will cause the CheckWatcher to no longer monitor the check of given checkID.
	Unwatch(checkID string)
}

// UniversalCheckWatcher is an implementation of CheckWatcher capable of watching
// checks in the Nomad or Consul service providers.
type UniversalCheckWatcher struct {
	logger hclog.Logger
	getter CheckStatusGetter

	// pollFrequency is how often to poll the checks API
	pollFrequency time.Duration

	// checkUpdateCh sends watches/removals to the main loop
	checkUpdateCh chan checkWatchUpdate

	// done is closed when Run has exited
	done chan struct{}

	// failedPreviousInterval is used to indicate whether something went wrong during
	// the previous poll interval - if so we can silence ongoing errors
	failedPreviousInterval bool
}

func NewCheckWatcher(logger hclog.Logger, getter CheckStatusGetter) *UniversalCheckWatcher {
	return &UniversalCheckWatcher{
		logger:        logger.ResetNamed("watch.checks"),
		getter:        getter,
		pollFrequency: 1 * time.Second,
		checkUpdateCh: make(chan checkWatchUpdate, 8),
		done:          make(chan struct{}),
	}
}

// Watch a check and restart its task if unhealthy.
func (w *UniversalCheckWatcher) Watch(allocID, taskName, checkID string, check *structs.ServiceCheck, wr WorkloadRestarter) {
	if !check.TriggersRestarts() {
		return // check_restart not set; no-op
	}

	c := &restarter{
		allocID:        allocID,
		taskName:       taskName,
		checkID:        checkID,
		checkName:      check.Name,
		taskKey:        key(allocID + taskName),
		task:           wr,
		interval:       check.Interval,
		grace:          check.CheckRestart.Grace,
		graceUntil:     time.Now().Add(check.CheckRestart.Grace),
		timeLimit:      check.Interval * time.Duration(check.CheckRestart.Limit-1),
		ignoreWarnings: check.CheckRestart.IgnoreWarnings,
		logger:         w.logger.With("alloc_id", allocID, "task", taskName, "check", check.Name),
	}

	select {
	case w.checkUpdateCh <- checkWatchUpdate{
		checkID: checkID,
		restart: c,
	}: // activate watch
	case <-w.done: // exited; nothing to do
	}
}

// Unwatch a check.
func (w *UniversalCheckWatcher) Unwatch(checkID string) {
	select {
	case w.checkUpdateCh <- checkWatchUpdate{
		checkID: checkID,
		remove:  true,
	}: // deactivate watch
	case <-w.done: // exited; nothing to do
	}
}

func (w *UniversalCheckWatcher) Run(ctx context.Context) {
	defer close(w.done)

	// map of checkID to their restarter handle (contains only checks we are watching)
	watched := make(map[string]*restarter)

	checkTimer, cleanupCheckTimer := helper.NewSafeTimer(0)
	defer cleanupCheckTimer()

	stopCheckTimer := func() { // todo: refactor using that other pattern
		checkTimer.Stop()
		select {
		case <-checkTimer.C:
		default:
		}
	}

	// initialize with checkTimer disabled
	stopCheckTimer()

	for {
		// disable polling if there are no checks
		if len(watched) == 0 {
			stopCheckTimer()
		}

		select {
		// caller cancelled us; goodbye
		case <-ctx.Done():
			return

		// received an update; add or remove check
		case update := <-w.checkUpdateCh:
			if update.remove {
				delete(watched, update.checkID)
				continue
			}

			watched[update.checkID] = update.restart
			allocID := update.restart.allocID
			taskName := update.restart.taskName
			checkName := update.restart.checkName
			w.logger.Trace("now watching check", "alloc_i", allocID, "task", taskName, "check", checkName)

			// turn on the timer if we are now active
			if len(watched) == 1 {
				stopCheckTimer()
				checkTimer.Reset(w.pollFrequency)
			}

		// poll time; refresh check statuses
		case now := <-checkTimer.C:
			w.interval(ctx, now, watched)
			checkTimer.Reset(w.pollFrequency)
		}
	}
}

func (w *UniversalCheckWatcher) interval(ctx context.Context, now time.Time, watched map[string]*restarter) {
	statuses, err := w.getter.Get()
	if err != nil && !w.failedPreviousInterval {
		w.failedPreviousInterval = true
		w.logger.Error("failed to retrieve check statuses", "error", err)
		return
	}
	w.failedPreviousInterval = false

	// keep track of tasks restarted this interval
	restarts := set.New[key](len(statuses))

	// iterate over status of all checks, and update the status of checks
	// we care about watching
	for checkID, checkRestarter := range watched {
		if ctx.Err() != nil {
			return //  short circuit; caller cancelled us
		}

		if restarts.Contains(checkRestarter.taskKey) {
			// skip; task is already being restarted
			delete(watched, checkID)
			continue
		}

		status, exists := statuses[checkID]
		if !exists {
			// warn only if outside grace period; avoiding race with check registration
			if now.After(checkRestarter.graceUntil) {
				w.logger.Warn("watched check not found", "check_id", checkID)
			}
			continue
		}

		if checkRestarter.apply(ctx, now, status) {
			// check will be re-registered & re-watched on startup
			delete(watched, checkID)
			restarts.Insert(checkRestarter.taskKey)
		}
	}

	// purge passing checks of tasks that are being restarted
	if restarts.Size() > 0 {
		for checkID, checkRestarter := range watched {
			if restarts.Contains(checkRestarter.taskKey) {
				delete(watched, checkID)
			}
		}
	}
}
