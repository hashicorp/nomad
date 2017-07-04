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
	deadline := time.NewTimer(u.HealthyDeadline)

	// Create a healthy timer
	latestTaskHealthy := time.Unix(0, 0)
	latestChecksHealthy := time.Unix(0, 0)
	healthyTimer := time.NewTimer(0)
	if !healthyTimer.Stop() {
		<-healthyTimer.C
	}

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
					r.logger.Printf("[TRACE] client.alloc_watcher: failed to lookup consul checks for allocation %q: %v", alloc.ID, err)
				}

				checks = newChecks
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
			continue OUTER
		}

		// Check if all the checks are passing
		for _, check := range checks {
			if check.Status != api.HealthPassing {
				r.logger.Printf("[TRACE] client.alloc_watcher: continuing since check %q isn't passing for alloc %q", check.CheckID, alloc.ID)
				latestChecksHealthy = time.Time{}
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

		// Don't need to set the timer if we are healthy and have marked
		// ourselves healthy.
		if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Healthy != nil && *alloc.DeploymentStatus.Healthy {
			continue OUTER
		}

		// Determine when we can mark ourselves as healthy.
		totalHealthy := latestTaskHealthy
		if totalHealthy.Before(latestChecksHealthy) {
			totalHealthy = latestChecksHealthy
		}
		d := time.Until(totalHealthy.Add(u.MinHealthyTime))

		if !healthyTimer.Stop() {
			select {
			case <-healthyTimer.C:
			default:
			}
		}

		healthyTimer.Reset(d)
		r.logger.Printf("[TRACE] client.alloc_watcher: setting healthy timer to %v for alloc %q", d, alloc.ID)
	}
}
