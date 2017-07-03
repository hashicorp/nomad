package client

import (
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// watchHealth is responsible for watching an allocation's task status and
// potentially consul health check status to determine if the allocation is
// healthy or unhealthy.
func (r *AllocRunner) watchHealth() {
	// Get our alloc and the task group
	alloc := r.Alloc()
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		r.logger.Printf("[ERR] client.alloc_watcher: failed to lookup allocation's task group. Exiting watcher")
		return
	}

	u := tg.Update

	// Checks marks whether we should be watching for Consul health checks
	checks := false
	r.logger.Printf("XXX %v", checks)

	switch {
	case u == nil:
		r.logger.Printf("[TRACE] client.alloc_watcher: no update block for alloc %q. exiting", alloc.ID)
		return
	case u.HealthCheck == structs.UpdateStrategyHealthCheck_Manual:
		r.logger.Printf("[TRACE] client.alloc_watcher: update block has manual checks for alloc %q. exiting", alloc.ID)
		return
	case u.HealthCheck == structs.UpdateStrategyHealthCheck_Checks:
		checks = true
	}

	// Get a listener so we know when an allocation is updated.
	l := r.allocBroadcast.Listen()

	// Create a deadline timer for the health
	deadline := time.NewTimer(u.HealthyDeadline)

	// Create a healthy timer
	latestHealthyTime := time.Unix(0, 0)
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
		l.Close()
	}()

	setHealth := func(h bool) {
		r.allocLock.Lock()
		r.allocHealth = helper.BoolToPtr(h)
		r.allocLock.Unlock()
		r.syncStatus()
	}

	first := true
OUTER:
	for {
		if !first {
			select {
			case <-r.destroyCh:
				return
			case newAlloc, ok := <-l.Ch:
				if !ok {
					return
				}

				alloc = newAlloc
				if alloc.DeploymentID == "" {
					continue OUTER
				}

				r.logger.Printf("[TRACE] client.alloc_watcher: new alloc version for %q", alloc.ID)
			case <-deadline.C:
				// We have exceeded our deadline without being healthy.
				setHealth(false)
				// XXX Think about in-place
				return
			case <-healthyTimer.C:
				r.logger.Printf("[TRACE] client.alloc_watcher: alloc %q is healthy", alloc.ID)
				setHealth(true)
			}
		}

		first = false

		// If the alloc is being stopped by the server just exit
		switch alloc.DesiredStatus {
		case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
			r.logger.Printf("[TRACE] client.alloc_watcher: desired status terminal for alloc %q", alloc.ID)
			return
		}

		// If the task is dead or has restarted, fail
		for _, tstate := range alloc.TaskStates {
			if tstate.Failed || !tstate.FinishedAt.IsZero() || tstate.Restarts != 0 {
				r.logger.Printf("[TRACE] client.alloc_watcher: setting health to false for alloc %q", alloc.ID)
				setHealth(false)
				return
			}
		}

		// Determine if the allocation is healthy
		for task, tstate := range alloc.TaskStates {
			if tstate.State != structs.TaskStateRunning {
				r.logger.Printf("[TRACE] client.alloc_watcher: continuing since task %q hasn't started for alloc %q", task, alloc.ID)
				continue OUTER
			}

			if tstate.StartedAt.After(latestHealthyTime) {
				latestHealthyTime = tstate.StartedAt
			}
		}

		// If we are already healthy we don't set the timer
		healthyThreshold := latestHealthyTime.Add(u.MinHealthyTime)
		if time.Now().After(healthyThreshold) {
			continue OUTER
		}

		// Start the time til we are healthy
		if !healthyTimer.Stop() {
			select {
			case <-healthyTimer.C:
			default:
			}
		}
		d := time.Until(healthyThreshold)
		healthyTimer.Reset(d)
		r.logger.Printf("[TRACE] client.alloc_watcher: setting healthy timer to %v for alloc %q", d, alloc.ID)
	}
}
