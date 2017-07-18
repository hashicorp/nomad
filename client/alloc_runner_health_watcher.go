package client

import (
	"context"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// consulCheckLookupInterval is the  interval at which we check if the
	// Consul checks are healthy or unhealthy.
	consulCheckLookupInterval = 500 * time.Millisecond
)

// watchHealth is responsible for watching an allocation's task status and
// potentially consul health check status to determine if the allocation is
// healthy or unhealthy.
func (r *AllocRunner) watchHealth(ctx context.Context) {
	// See if we should watch the allocs health
	alloc := r.Alloc()
	if alloc.DeploymentID == "" {
		r.logger.Printf("[TRACE] client.alloc_watcher: exiting because alloc isn't part of a deployment")
		return
	} else if alloc.DeploymentStatus.IsHealthy() || alloc.DeploymentStatus.IsUnhealthy() {
		r.logger.Printf("[TRACE] client.alloc_watcher: exiting because alloc deployment health already determined")
		return
	}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		r.logger.Printf("[ERR] client.alloc_watcher: failed to lookup allocation's task group. Exiting watcher")
		return
	}

	// Checks marks whether we should be watching for Consul health checks
	desiredChecks := 0
	var checkTicker *time.Ticker
	var checkCh <-chan time.Time

	u := tg.Update
	switch {
	case u == nil:
		r.logger.Printf("[TRACE] client.alloc_watcher: no update block for alloc %q. exiting", alloc.ID)
		return
	case u.HealthCheck == structs.UpdateStrategyHealthCheck_Manual:
		r.logger.Printf("[TRACE] client.alloc_watcher: update block has manual checks for alloc %q. exiting", alloc.ID)
		return
	case u.HealthCheck == structs.UpdateStrategyHealthCheck_Checks:
		for _, task := range tg.Tasks {
			for _, s := range task.Services {
				desiredChecks += len(s.Checks)
			}
		}

		checkTicker = time.NewTicker(consulCheckLookupInterval)
		checkCh = checkTicker.C
	}

	// Get a listener so we know when an allocation is updated.
	l := r.allocBroadcast.Listen()

	// Create a deadline timer for the health
	r.logger.Printf("[DEBUG] client.alloc_watcher: deadline (%v) for alloc %q is at %v", u.HealthyDeadline, alloc.ID, time.Now().Add(u.HealthyDeadline))
	deadline := time.NewTimer(u.HealthyDeadline)

	// Create a healthy timer
	latestTaskHealthy := time.Unix(0, 0)
	latestChecksHealthy := time.Unix(0, 0)
	healthyTimer := time.NewTimer(0)
	healthyTime := time.Time{}
	cancelHealthyTimer := func() {
		if !healthyTimer.Stop() {
			select {
			case <-healthyTimer.C:
			default:
			}
		}
	}
	cancelHealthyTimer()

	// Cleanup function
	defer func() {
		if !deadline.Stop() {
			<-deadline.C
		}
		if !healthyTimer.Stop() {
			<-healthyTimer.C
		}
		if checkTicker != nil {
			checkTicker.Stop()
		}
		l.Close()
	}()

	setHealth := func(h bool) {
		r.allocLock.Lock()
		r.allocHealth = helper.BoolToPtr(h)
		r.allocLock.Unlock()
		r.syncStatus()
	}

	// Store whether the last consul checks call was successful or not
	consulChecksErr := false

	var checks []*api.AgentCheck
	first := true
OUTER:
	for {
		if !first {
			select {
			case <-ctx.Done():
				return
			case newAlloc, ok := <-l.Ch:
				if !ok {
					return
				}

				alloc = newAlloc
				r.logger.Printf("[TRACE] client.alloc_watcher: new alloc version for %q", alloc.ID)
			case <-checkCh:
				newChecks, err := r.consulClient.Checks(alloc)
				if err != nil {
					if !consulChecksErr {
						consulChecksErr = true
						r.logger.Printf("[WARN] client.alloc_watcher: failed to lookup consul checks for allocation %q: %v", alloc.ID, err)
					}
				} else {
					consulChecksErr = false
					checks = newChecks
				}
			case <-deadline.C:
				// We have exceeded our deadline without being healthy.
				r.logger.Printf("[TRACE] client.alloc_watcher: alloc %q hit healthy deadline", alloc.ID)
				setHealth(false)
				return
			case <-healthyTimer.C:
				r.logger.Printf("[TRACE] client.alloc_watcher: alloc %q is healthy", alloc.ID)
				setHealth(true)
				return
			}
		}
		first = false

		// If the alloc is being stopped by the server just exit
		switch alloc.DesiredStatus {
		case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
			r.logger.Printf("[TRACE] client.alloc_watcher: desired status terminal for alloc %q", alloc.ID)
			return
		}

		// If the alloc is marked as failed by the client set the status to
		// unhealthy
		if alloc.ClientStatus == structs.AllocClientStatusFailed {
			r.logger.Printf("[TRACE] client.alloc_watcher: client status failed for alloc %q", alloc.ID)
			setHealth(false)
			return
		}

		if len(alloc.TaskStates) != len(tg.Tasks) {
			r.logger.Printf("[TRACE] client.alloc_watcher: all task runners haven't started")
			continue OUTER
		}

		// If the task is dead or has restarted, fail
		for _, tstate := range alloc.TaskStates {
			if tstate.Failed || !tstate.FinishedAt.IsZero() || tstate.Restarts != 0 {
				r.logger.Printf("[TRACE] client.alloc_watcher: setting health to false for alloc %q", alloc.ID)
				setHealth(false)
				return
			}
		}

		// If we should have checks and they aren't all healthy continue
		if len(checks) != desiredChecks {
			r.logger.Printf("[TRACE] client.alloc_watcher: continuing since all checks (want %d; got %d) haven't been registered for alloc %q", desiredChecks, len(checks), alloc.ID)
			cancelHealthyTimer()
			continue OUTER
		}

		// Check if all the checks are passing
		for _, check := range checks {
			if check.Status != api.HealthPassing {
				r.logger.Printf("[TRACE] client.alloc_watcher: continuing since check %q isn't passing for alloc %q", check.CheckID, alloc.ID)
				latestChecksHealthy = time.Time{}
				cancelHealthyTimer()
				continue OUTER
			}
		}
		if latestChecksHealthy.IsZero() {
			latestChecksHealthy = time.Now()
		}

		// Determine if the allocation is healthy
		for task, tstate := range alloc.TaskStates {
			if tstate.State != structs.TaskStateRunning {
				r.logger.Printf("[TRACE] client.alloc_watcher: continuing since task %q hasn't started for alloc %q", task, alloc.ID)
				continue OUTER
			}

			if tstate.StartedAt.After(latestTaskHealthy) {
				latestTaskHealthy = tstate.StartedAt
			}
		}

		// Determine when we can mark ourselves as healthy.
		totalHealthy := latestTaskHealthy
		if totalHealthy.Before(latestChecksHealthy) {
			totalHealthy = latestChecksHealthy
		}

		// Nothing to do since we are already waiting for the healthy timer to
		// fire at the same time.
		if totalHealthy.Equal(healthyTime) {
			continue OUTER
		}

		healthyTime = totalHealthy
		cancelHealthyTimer()
		d := time.Until(totalHealthy.Add(u.MinHealthyTime))
		healthyTimer.Reset(d)
		r.logger.Printf("[TRACE] client.alloc_watcher: setting healthy timer to %v for alloc %q", d, alloc.ID)
	}
}
