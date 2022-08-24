package consul

import (
	"context"
	"fmt"
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// defaultPollFreq is the default rate to poll the Consul Checks API
	defaultPollFreq = 900 * time.Millisecond
)

// ChecksAPI is the part of the Consul API the checkWatcher requires.
type ChecksAPI interface {
	ChecksWithFilterOpts(filter string, q *api.QueryOptions) (map[string]*api.AgentCheck, error)
}

// WorkloadRestarter allows the checkWatcher to restart tasks or entire task groups.
type WorkloadRestarter interface {
	Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error
}

// checkRestart handles restarting a task if a check is unhealthy.
type checkRestart struct {
	allocID   string
	taskName  string
	checkID   string
	checkName string
	taskKey   string // composite of allocID + taskName for uniqueness

	task           WorkloadRestarter
	grace          time.Duration
	interval       time.Duration
	timeLimit      time.Duration
	ignoreWarnings bool

	// Mutable fields

	// unhealthyState is the time a check first went unhealthy. Set to the
	// zero value if the check passes before timeLimit.
	unhealthyState time.Time

	// graceUntil is when the check's grace period expires and unhealthy
	// checks should be counted.
	graceUntil time.Time

	logger log.Logger
}

// apply restart state for check and restart task if necessary. Current
// timestamp is passed in so all check updates have the same view of time (and
// to ease testing).
//
// Returns true if a restart was triggered in which case this check should be
// removed (checks are added on task startup).
func (c *checkRestart) apply(ctx context.Context, now time.Time, status string) bool {
	healthy := func() {
		if !c.unhealthyState.IsZero() {
			c.logger.Debug("canceling restart because check became healthy")
			c.unhealthyState = time.Time{}
		}
	}
	switch status {
	case api.HealthCritical:
	case api.HealthWarning:
		if c.ignoreWarnings {
			// Warnings are ignored, reset state and exit
			healthy()
			return false
		}
	default:
		// All other statuses are ok, reset state and exit
		healthy()
		return false
	}

	if now.Before(c.graceUntil) {
		// In grace period, exit
		return false
	}

	if c.unhealthyState.IsZero() {
		// First failure, set restart deadline
		if c.timeLimit != 0 {
			c.logger.Debug("check became unhealthy. Will restart if check doesn't become healthy", "time_limit", c.timeLimit)
		}
		c.unhealthyState = now
	}

	// restart timeLimit after start of this check becoming unhealthy
	restartAt := c.unhealthyState.Add(c.timeLimit)

	// Must test >= because if limit=1, restartAt == first failure
	if now.Equal(restartAt) || now.After(restartAt) {
		// hasn't become healthy by deadline, restart!
		c.logger.Debug("restarting due to unhealthy check")

		// Tell TaskRunner to restart due to failure
		reason := fmt.Sprintf("healthcheck: check %q unhealthy", c.checkName)
		event := structs.NewTaskEvent(structs.TaskRestartSignal).SetRestartReason(reason)
		go asyncRestart(ctx, c.logger, c.task, event)
		return true
	}

	return false
}

// asyncRestart mimics the pre-0.9 TaskRunner.Restart behavior and is intended
// to be called in a goroutine.
func asyncRestart(ctx context.Context, logger log.Logger, task WorkloadRestarter, event *structs.TaskEvent) {
	// Check watcher restarts are always failures
	const failure = true

	// Restarting is asynchronous so there's no reason to allow this
	// goroutine to block indefinitely.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := task.Restart(ctx, event, failure); err != nil {
		// Restart errors are not actionable and only relevant when
		// debugging allocation lifecycle management.
		logger.Debug("failed to restart task", "error", err,
			"event_time", event.Time, "event_type", event.Type)
	}
}

// checkWatchUpdates add or remove checks from the watcher
type checkWatchUpdate struct {
	checkID      string
	remove       bool
	checkRestart *checkRestart
}

// checkWatcher watches Consul checks and restarts tasks when they're
// unhealthy.
type checkWatcher struct {
	namespacesClient *NamespacesClient
	checksAPI        ChecksAPI

	// pollFreq is how often to poll the checks API and defaults to
	// defaultPollFreq
	pollFreq time.Duration

	// checkUpdateCh is how watches (and removals) are sent to the main
	// watching loop
	checkUpdateCh chan checkWatchUpdate

	// done is closed when Run has exited
	done chan struct{}

	// lastErr is true if the last Consul call failed. It is used to
	// squelch repeated error messages.
	lastErr bool

	logger log.Logger
}

// newCheckWatcher creates a new checkWatcher but does not call its Run method.
func newCheckWatcher(logger log.Logger, checksAPI ChecksAPI, namespacesClient *NamespacesClient) *checkWatcher {
	return &checkWatcher{
		namespacesClient: namespacesClient,
		checksAPI:        checksAPI,
		pollFreq:         defaultPollFreq,
		checkUpdateCh:    make(chan checkWatchUpdate, 8),
		done:             make(chan struct{}),
		logger:           logger.ResetNamed("consul.health"),
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

	stopTimer := func() {
		checkTimer.Stop()
		select {
		case <-checkTimer.C:
		default:
		}
	}

	// disable by default
	stopTimer()

	// Main watch loop
WATCHER:
	for {
		// disable polling if there are no checks
		if len(checks) == 0 {
			stopTimer()
		}

		select {
		case update := <-w.checkUpdateCh:
			if update.remove {
				// Remove a check
				delete(checks, update.checkID)
				continue
			}

			// Add/update a check
			checks[update.checkID] = update.checkRestart
			w.logger.Debug("watching check", "alloc_id", update.checkRestart.allocID,
				"task", update.checkRestart.taskName, "check", update.checkRestart.checkName)

			// if first check was added make sure polling is enabled
			if len(checks) == 1 {
				stopTimer()
				checkTimer.Reset(w.pollFreq)
			}

		case <-ctx.Done():
			return

		case <-checkTimer.C:
			checkTimer.Reset(w.pollFreq)

			// Set "now" as the point in time the following check results represent
			now := time.Now()

			// Get the list of all namespaces so we can iterate them.
			namespaces, err := w.namespacesClient.List()
			if err != nil {
				if !w.lastErr {
					w.lastErr = true
					w.logger.Error("failed retrieving namespaces", "error", err)
				}
				continue WATCHER
			}

			checkResults := make(map[string]*api.AgentCheck)
			for _, namespace := range namespaces {
				nsResults, err := w.checksAPI.ChecksWithFilterOpts("", &api.QueryOptions{Namespace: normalizeNamespace(namespace)})
				if err != nil {
					if !w.lastErr {
						w.lastErr = true
						w.logger.Error("failed retrieving health checks", "error", err)
					}
					continue WATCHER
				} else {
					for k, v := range nsResults {
						checkResults[k] = v
					}
				}
			}

			w.lastErr = false

			// Keep track of tasks restarted this period so they
			// are only restarted once and all of their checks are
			// removed.
			restartedTasks := map[string]struct{}{}

			// Loop over watched checks and update their status from results
			for cid, check := range checks {
				// Shortcircuit if told to exit
				if ctx.Err() != nil {
					return
				}

				if _, ok := restartedTasks[check.taskKey]; ok {
					// Check for this task already restarted; remove and skip check
					delete(checks, cid)
					continue
				}

				result, ok := checkResults[cid]
				if !ok {
					// Only warn if outside grace period to avoid races with check registration
					if now.After(check.graceUntil) {
						w.logger.Warn("watched check not found in Consul", "check", check.checkName, "check_id", cid)
					}
					continue
				}

				restarted := check.apply(ctx, now, result.Status)
				if restarted {
					// Checks are registered+watched on
					// startup, so it's safe to remove them
					// whenever they're restarted
					delete(checks, cid)

					restartedTasks[check.taskKey] = struct{}{}
				}
			}

			// Ensure even passing checks for restartedTasks are removed
			if len(restartedTasks) > 0 {
				for cid, check := range checks {
					if _, ok := restartedTasks[check.taskKey]; ok {
						delete(checks, cid)
					}
				}
			}
		}
	}
}

// Watch a check and restart its task if unhealthy.
func (w *checkWatcher) Watch(allocID, taskName, checkID string, check *structs.ServiceCheck, restarter WorkloadRestarter) {
	if !check.TriggersRestarts() {
		// Not watched, noop
		return
	}

	c := &checkRestart{
		allocID:        allocID,
		taskName:       taskName,
		checkID:        checkID,
		checkName:      check.Name,
		taskKey:        fmt.Sprintf("%s%s", allocID, taskName), // unique task ID
		task:           restarter,
		interval:       check.Interval,
		grace:          check.CheckRestart.Grace,
		graceUntil:     time.Now().Add(check.CheckRestart.Grace),
		timeLimit:      check.Interval * time.Duration(check.CheckRestart.Limit-1),
		ignoreWarnings: check.CheckRestart.IgnoreWarnings,
		logger:         w.logger.With("alloc_id", allocID, "task", taskName, "check", check.Name),
	}

	update := checkWatchUpdate{
		checkID:      checkID,
		checkRestart: c,
	}

	select {
	case w.checkUpdateCh <- update:
		// sent watch
	case <-w.done:
		// exited; nothing to do
	}
}

// Unwatch a check.
func (w *checkWatcher) Unwatch(cid string) {
	c := checkWatchUpdate{
		checkID: cid,
		remove:  true,
	}
	select {
	case w.checkUpdateCh <- c:
		// sent remove watch
	case <-w.done:
		// exited; nothing to do
	}
}
