package client

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// consulCheckLookupInterval is the  interval at which we check if the
	// Consul checks are healthy or unhealthy.
	consulCheckLookupInterval = 500 * time.Millisecond

	// allocHealthEventSource is the source used for emitting task events
	allocHealthEventSource = "Alloc Unhealthy"
)

// watchHealth is responsible for watching an allocation's task status and
// potentially Consul health check status to determine if the allocation is
// healthy or unhealthy.
func (r *AllocRunner) watchHealth(ctx context.Context) {

	// See if we should watch the allocs health
	alloc := r.Alloc()
	if alloc.DeploymentID == "" || alloc.DeploymentStatus.IsHealthy() || alloc.DeploymentStatus.IsUnhealthy() {
		return
	}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		r.logger.Printf("[ERR] client.alloc_watcher: failed to lookup allocation's task group. Exiting watcher")
		return
	} else if tg.Update == nil || tg.Update.HealthCheck == structs.UpdateStrategyHealthCheck_Manual {
		return
	}

	// Get an allocation listener to watch for alloc events
	l := r.allocBroadcast.Listen()
	defer l.Close()

	// Create a new context with the health deadline
	deadline := time.Now().Add(tg.Update.HealthyDeadline)
	healthCtx, healthCtxCancel := context.WithDeadline(ctx, deadline)
	defer healthCtxCancel()
	r.logger.Printf("[DEBUG] client.alloc_watcher: deadline (%v) for alloc %q is at %v", tg.Update.HealthyDeadline, alloc.ID, deadline)

	// Create the health tracker object
	tracker := newAllocHealthTracker(healthCtx, r.logger, alloc, l, r.consulClient)
	tracker.Start()

	allocHealthy := false
	select {
	case <-healthCtx.Done():
		// We were cancelled which means we are no longer needed
		if healthCtx.Err() == context.Canceled {
			return
		}

		// Since the deadline has been reached we are not healthy
	case <-tracker.AllocStoppedCh():
		// The allocation was stopped so nothing to do
		return
	case healthy := <-tracker.HealthyCh():
		allocHealthy = healthy
	}

	r.allocLock.Lock()
	r.allocHealth = helper.BoolToPtr(allocHealthy)
	r.allocLock.Unlock()

	// We are unhealthy so emit task events explaining why
	if !allocHealthy {
		r.taskLock.RLock()
		for task, event := range tracker.TaskEvents() {
			if tr, ok := r.tasks[task]; ok {
				tr.EmitEvent(allocHealthEventSource, event)
			}
		}
		r.taskLock.RUnlock()
	}

	r.syncStatus()
}

// allocHealthTracker tracks the health of an allocation and makes health events
// watchable via channels.
type allocHealthTracker struct {
	// logger is used to log
	logger *log.Logger

	// ctx and cancelFn is used to shutdown the tracker
	ctx      context.Context
	cancelFn context.CancelFunc

	// alloc is the alloc we are tracking
	alloc *structs.Allocation

	// tg is the task group we are tracking
	tg *structs.TaskGroup

	// consulCheckCount is the number of checks the task group will attempt to
	// register
	consulCheckCount int

	// allocUpdates is a listener for retrieving new alloc updates
	allocUpdates *cstructs.AllocListener

	// consulClient is used to look up the state of the task's checks
	consulClient ConsulServiceAPI

	// healthy is used to signal whether we have determined the allocation to be
	// healthy or unhealthy
	healthy chan bool

	// allocStopped is triggered when the allocation is stopped and tracking is
	// not needed
	allocStopped chan struct{}

	// l is used to lock shared fields listed below
	l sync.Mutex

	// tasksHealthy marks whether all the tasks have met their health check
	// (disregards Consul)
	tasksHealthy bool

	// allocFailed marks whether the allocation failed
	allocFailed bool

	// checksHealthy marks whether all the task's Consul checks are healthy
	checksHealthy bool

	// taskHealth contains the health state for each task
	taskHealth map[string]*taskHealthState
}

// newAllocHealthTracker returns a health tracker for the given allocation. An
// alloc listener and consul API object are given so that the watcher can detect
// health changes.
func newAllocHealthTracker(parentCtx context.Context, logger *log.Logger, alloc *structs.Allocation,
	allocUpdates *cstructs.AllocListener, consulClient ConsulServiceAPI) *allocHealthTracker {

	a := &allocHealthTracker{
		logger:       logger,
		healthy:      make(chan bool, 1),
		allocStopped: make(chan struct{}),
		alloc:        alloc,
		tg:           alloc.Job.LookupTaskGroup(alloc.TaskGroup),
		allocUpdates: allocUpdates,
		consulClient: consulClient,
	}

	a.taskHealth = make(map[string]*taskHealthState, len(a.tg.Tasks))
	for _, task := range a.tg.Tasks {
		a.taskHealth[task.Name] = &taskHealthState{task: task}
	}

	for _, task := range a.tg.Tasks {
		for _, s := range task.Services {
			a.consulCheckCount += len(s.Checks)
		}
	}

	a.ctx, a.cancelFn = context.WithCancel(parentCtx)
	return a
}

// Start starts the watcher.
func (a *allocHealthTracker) Start() {
	go a.watchTaskEvents()
	if a.tg.Update.HealthCheck == structs.UpdateStrategyHealthCheck_Checks {
		go a.watchConsulEvents()
	}
}

// HealthyCh returns a channel that will emit a boolean indicating the health of
// the allocation.
func (a *allocHealthTracker) HealthyCh() <-chan bool {
	return a.healthy
}

// AllocStoppedCh returns a channel that will be fired if the allocation is
// stopped. This means that health will not be set.
func (a *allocHealthTracker) AllocStoppedCh() <-chan struct{} {
	return a.allocStopped
}

// TaskEvents returns a map of events by task. This should only be called after
// health has been determined. Only tasks that have contributed to the
// allocation being unhealthy will have an event.
func (a *allocHealthTracker) TaskEvents() map[string]string {
	a.l.Lock()
	defer a.l.Unlock()

	// Nothing to do since the failure wasn't task related
	if a.allocFailed {
		return nil
	}

	deadline, _ := a.ctx.Deadline()
	events := make(map[string]string, len(a.tg.Tasks))

	// Go through are task information and build the event map
	for task, state := range a.taskHealth {
		if e, ok := state.event(deadline, a.tg.Update); ok {
			events[task] = e
		}
	}

	return events
}

// setTaskHealth is used to set the tasks health as healthy or unhealthy. If the
// allocation is terminal, health is immediately broadcasted.
func (a *allocHealthTracker) setTaskHealth(healthy, terminal bool) {
	a.l.Lock()
	defer a.l.Unlock()
	a.tasksHealthy = healthy

	// If we are marked healthy but we also require Consul to be healthy and it
	// isn't yet, return, unless the task is terminal
	requireConsul := a.tg.Update.HealthCheck == structs.UpdateStrategyHealthCheck_Checks && a.consulCheckCount > 0
	if !terminal && healthy && requireConsul && !a.checksHealthy {
		return
	}

	select {
	case a.healthy <- healthy:
	default:
	}

	// Shutdown the tracker
	a.cancelFn()
}

// setCheckHealth is used to mark the checks as either healthy or unhealthy.
func (a *allocHealthTracker) setCheckHealth(healthy bool) {
	a.l.Lock()
	defer a.l.Unlock()
	a.checksHealthy = healthy

	// Only signal if we are healthy and so is the tasks
	if !healthy || !a.tasksHealthy {
		return
	}

	select {
	case a.healthy <- healthy:
	default:
	}

	// Shutdown the tracker
	a.cancelFn()
}

// markAllocStopped is used to mark the allocation as having stopped.
func (a *allocHealthTracker) markAllocStopped() {
	close(a.allocStopped)
	a.cancelFn()
}

// watchTaskEvents is a long lived watcher that watches for the health of the
// allocation's tasks.
func (a *allocHealthTracker) watchTaskEvents() {
	alloc := a.alloc
	allStartedTime := time.Time{}
	healthyTimer := time.NewTimer(0)
	if !healthyTimer.Stop() {
		select {
		case <-healthyTimer.C:
		default:
		}
	}

	for {
		// If the alloc is being stopped by the server just exit
		switch alloc.DesiredStatus {
		case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
			a.logger.Printf("[TRACE] client.alloc_watcher: desired status terminal for alloc %q", alloc.ID)
			a.markAllocStopped()
			return
		}

		// Store the task states
		a.l.Lock()
		for task, state := range alloc.TaskStates {
			a.taskHealth[task].state = state
		}
		a.l.Unlock()

		// Detect if the alloc is unhealthy or if all tasks have started yet
		latestStartTime := time.Time{}
		for _, state := range alloc.TaskStates {
			// One of the tasks has failed so we can exit watching
			if state.Failed || !state.FinishedAt.IsZero() {
				a.setTaskHealth(false, true)
				return
			}

			if state.State != structs.TaskStateRunning {
				latestStartTime = time.Time{}
				break
			} else if state.StartedAt.After(latestStartTime) {
				latestStartTime = state.StartedAt
			}
		}

		// If the alloc is marked as failed by the client but none of the
		// individual tasks failed, that means something failed at the alloc
		// level.
		if alloc.ClientStatus == structs.AllocClientStatusFailed {
			a.logger.Printf("[TRACE] client.alloc_watcher: client status failed for alloc %q", alloc.ID)
			a.l.Lock()
			a.allocFailed = true
			a.l.Unlock()
			a.setTaskHealth(false, true)
			return
		}

		if !latestStartTime.Equal(allStartedTime) {
			// Avoid the timer from firing at the old start time
			if !healthyTimer.Stop() {
				select {
				case <-healthyTimer.C:
				default:
				}
			}

			// Set the timer since all tasks are started
			if !latestStartTime.IsZero() {
				allStartedTime = latestStartTime
				healthyTimer.Reset(a.tg.Update.MinHealthyTime)
			}
		}

		select {
		case <-a.ctx.Done():
			return
		case newAlloc, ok := <-a.allocUpdates.Ch:
			if !ok {
				return
			}
			alloc = newAlloc
		case <-healthyTimer.C:
			a.setTaskHealth(true, false)
		}
	}
}

// watchConsulEvents iis a long lived watcher that watches for the health of the
// allocation's Consul checks.
func (a *allocHealthTracker) watchConsulEvents() {
	// checkTicker is the ticker that triggers us to look at the checks in
	// Consul
	checkTicker := time.NewTicker(consulCheckLookupInterval)
	defer checkTicker.Stop()

	// healthyTimer fires when the checks have been healthy for the
	// MinHealthyTime
	healthyTimer := time.NewTimer(0)
	if !healthyTimer.Stop() {
		select {
		case <-healthyTimer.C:
		default:
		}
	}

	// primed marks whether the healthy timer has been set
	primed := false

	// Store whether the last Consul checks call was successful or not
	consulChecksErr := false

	// allocReg are the registered objects in Consul for the allocation
	var allocReg *consul.AllocRegistration

OUTER:
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-checkTicker.C:
			newAllocReg, err := a.consulClient.AllocRegistrations(a.alloc.ID)
			if err != nil {
				if !consulChecksErr {
					consulChecksErr = true
					a.logger.Printf("[WARN] client.alloc_watcher: failed to lookup Consul registrations for allocation %q: %v", a.alloc.ID, err)
				}
				continue OUTER
			} else {
				consulChecksErr = false
				allocReg = newAllocReg
			}
		case <-healthyTimer.C:
			a.setCheckHealth(true)
		}

		if allocReg == nil {
			continue
		}

		// Store the task registrations
		a.l.Lock()
		for task, reg := range allocReg.Tasks {
			a.taskHealth[task].taskRegistrations = reg
		}
		a.l.Unlock()

		// Detect if all the checks are passing
		passed := true

	CHECKS:
		for _, treg := range allocReg.Tasks {
			for _, sreg := range treg.Services {
				for _, check := range sreg.Checks {
					if check.Status == api.HealthPassing {
						continue
					}

					passed = false
					a.setCheckHealth(false)
					break CHECKS
				}
			}
		}

		if !passed {
			// Reset the timer since we have transistioned back to unhealthy
			if primed {
				if !healthyTimer.Stop() {
					select {
					case <-healthyTimer.C:
					default:
					}
				}
				primed = false
			}
		} else if !primed {
			// Reset the timer to fire after MinHealthyTime
			if !healthyTimer.Stop() {
				select {
				case <-healthyTimer.C:
				default:
				}
			}

			primed = true
			healthyTimer.Reset(a.tg.Update.MinHealthyTime)
		}
	}
}

// taskHealthState captures all known health information about a task. It is
// largely used to determine if the task has contributed to the allocation being
// unhealthy.
type taskHealthState struct {
	task              *structs.Task
	state             *structs.TaskState
	taskRegistrations *consul.TaskRegistration
}

// event takes the deadline time for the allocation to be healthy and the update
// strategy of the group. It returns true if the task has contributed to the
// allocation being unhealthy and if so, an event description of why.
func (t *taskHealthState) event(deadline time.Time, update *structs.UpdateStrategy) (string, bool) {
	requireChecks := false
	desiredChecks := 0
	for _, s := range t.task.Services {
		if nc := len(s.Checks); nc > 0 {
			requireChecks = true
			desiredChecks += nc
		}
	}
	requireChecks = requireChecks && update.HealthCheck == structs.UpdateStrategyHealthCheck_Checks

	if t.state != nil {
		if t.state.Failed {
			return "Unhealthy because of failed task", true
		}
		if t.state.State != structs.TaskStateRunning {
			return "Task not running by deadline", true
		}

		// We are running so check if we have been running long enough
		if t.state.StartedAt.Add(update.MinHealthyTime).After(deadline) {
			return fmt.Sprintf("Task not running for min_healthy_time of %v by deadline", update.MinHealthyTime), true
		}
	}

	if t.taskRegistrations != nil {
		var notPassing []string
		passing := 0

	OUTER:
		for _, sreg := range t.taskRegistrations.Services {
			for _, check := range sreg.Checks {
				if check.Status != api.HealthPassing {
					notPassing = append(notPassing, sreg.Service.Service)
					continue OUTER
				} else {
					passing++
				}
			}
		}

		if len(notPassing) != 0 {
			return fmt.Sprintf("Services not healthy by deadline: %s", strings.Join(notPassing, ", ")), true
		}

		if passing != desiredChecks {
			return fmt.Sprintf("Only %d out of %d checks registered and passing", passing, desiredChecks), true
		}

	} else if requireChecks {
		return "Service checks not registered", true
	}

	return "", false
}
