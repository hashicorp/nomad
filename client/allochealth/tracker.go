package allochealth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	hclog "github.com/hashicorp/go-hclog"
	cconsul "github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// AllocHealthEventSource is the source used for emitting task events
	AllocHealthEventSource = "Alloc Unhealthy"

	// consulCheckLookupInterval is the  interval at which we check if the
	// Consul checks are healthy or unhealthy.
	consulCheckLookupInterval = 500 * time.Millisecond
)

// Tracker tracks the health of an allocation and makes health events watchable
// via channels.
type Tracker struct {
	// ctx and cancelFn is used to shutdown the tracker
	ctx      context.Context
	cancelFn context.CancelFunc

	// alloc is the alloc we are tracking
	alloc *structs.Allocation

	// tg is the task group we are tracking
	tg *structs.TaskGroup

	// minHealthyTime is the duration an alloc must remain healthy to be
	// considered healthy
	minHealthyTime time.Duration

	// checkLookupInterval is the interval at which we check if the
	// Consul checks are healthy or unhealthy.
	checkLookupInterval time.Duration

	// useChecks specifies whether to use Consul healh checks or not
	useChecks bool

	// consulCheckCount is the number of checks the task group will attempt to
	// register
	consulCheckCount int

	// allocUpdates is a listener for retrieving new alloc updates
	allocUpdates *cstructs.AllocListener

	// consulClient is used to look up the state of the task's checks
	consulClient cconsul.ConsulServiceAPI

	// healthy is used to signal whether we have determined the allocation to be
	// healthy or unhealthy
	healthy chan bool

	// allocStopped is triggered when the allocation is stopped and tracking is
	// not needed
	allocStopped chan struct{}

	// lifecycleTasks is a map of ephemeral tasks and their lifecycle hooks.
	// These tasks may terminate without affecting alloc health
	lifecycleTasks map[string]string

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

	logger hclog.Logger
}

// NewTracker returns a health tracker for the given allocation. An alloc
// listener and consul API object are given so that the watcher can detect
// health changes.
func NewTracker(parentCtx context.Context, logger hclog.Logger, alloc *structs.Allocation,
	allocUpdates *cstructs.AllocListener, consulClient cconsul.ConsulServiceAPI,
	minHealthyTime time.Duration, useChecks bool) *Tracker {

	// Do not create a named sub-logger as the hook controlling
	// this struct should pass in an appropriately named
	// sub-logger.
	t := &Tracker{
		healthy:             make(chan bool, 1),
		allocStopped:        make(chan struct{}),
		alloc:               alloc,
		tg:                  alloc.Job.LookupTaskGroup(alloc.TaskGroup),
		minHealthyTime:      minHealthyTime,
		useChecks:           useChecks,
		allocUpdates:        allocUpdates,
		consulClient:        consulClient,
		checkLookupInterval: consulCheckLookupInterval,
		logger:              logger,
		lifecycleTasks:      map[string]string{},
	}

	t.taskHealth = make(map[string]*taskHealthState, len(t.tg.Tasks))
	for _, task := range t.tg.Tasks {
		t.taskHealth[task.Name] = &taskHealthState{task: task}

		if task.Lifecycle != nil && !task.Lifecycle.Sidecar {
			t.lifecycleTasks[task.Name] = task.Lifecycle.Hook
		}

		for _, s := range task.Services {
			t.consulCheckCount += len(s.Checks)
		}
	}

	for _, s := range t.tg.Services {
		t.consulCheckCount += len(s.Checks)
	}

	t.ctx, t.cancelFn = context.WithCancel(parentCtx)
	return t
}

// Start starts the watcher.
func (t *Tracker) Start() {
	go t.watchTaskEvents()
	if t.useChecks {
		go t.watchConsulEvents()
	}
}

// HealthyCh returns a channel that will emit a boolean indicating the health of
// the allocation.
func (t *Tracker) HealthyCh() <-chan bool {
	return t.healthy
}

// AllocStoppedCh returns a channel that will be fired if the allocation is
// stopped. This means that health will not be set.
func (t *Tracker) AllocStoppedCh() <-chan struct{} {
	return t.allocStopped
}

// TaskEvents returns a map of events by task. This should only be called after
// health has been determined. Only tasks that have contributed to the
// allocation being unhealthy will have an event.
func (t *Tracker) TaskEvents() map[string]*structs.TaskEvent {
	t.l.Lock()
	defer t.l.Unlock()

	// Nothing to do since the failure wasn't task related
	if t.allocFailed {
		return nil
	}

	deadline, _ := t.ctx.Deadline()
	events := make(map[string]*structs.TaskEvent, len(t.tg.Tasks))

	// Go through are task information and build the event map
	for task, state := range t.taskHealth {
		useChecks := t.tg.Update.HealthCheck == structs.UpdateStrategyHealthCheck_Checks
		if e, ok := state.event(deadline, t.tg.Update.MinHealthyTime, useChecks); ok {
			events[task] = structs.NewTaskEvent(AllocHealthEventSource).SetMessage(e)
		}
	}

	return events
}

// setTaskHealth is used to set the tasks health as healthy or unhealthy. If the
// allocation is terminal, health is immediately broadcasted.
func (t *Tracker) setTaskHealth(healthy, terminal bool) {
	t.l.Lock()
	defer t.l.Unlock()
	t.tasksHealthy = healthy

	// if unhealthy, force waiting for new checks health status
	if !terminal && !healthy {
		t.checksHealthy = false
		return
	}

	// If we are marked healthy but we also require Consul to be healthy and it
	// isn't yet, return, unless the task is terminal
	requireConsul := t.useChecks && t.consulCheckCount > 0
	if !terminal && healthy && requireConsul && !t.checksHealthy {
		return
	}

	select {
	case t.healthy <- healthy:
	default:
	}

	// Shutdown the tracker
	t.cancelFn()
}

// setCheckHealth is used to mark the checks as either healthy or unhealthy.
// returns true if health is propagated and no more health monitoring is needed
func (t *Tracker) setCheckHealth(healthy bool) bool {
	t.l.Lock()
	defer t.l.Unlock()

	// check health should always be false if tasks are unhealthy
	// as checks might be missing from unhealthy tasks
	t.checksHealthy = healthy && t.tasksHealthy

	// Only signal if we are healthy and so is the tasks
	if !t.checksHealthy {
		return false
	}

	select {
	case t.healthy <- healthy:
	default:
	}

	// Shutdown the tracker
	t.cancelFn()
	return true
}

// markAllocStopped is used to mark the allocation as having stopped.
func (t *Tracker) markAllocStopped() {
	close(t.allocStopped)
	t.cancelFn()
}

// watchTaskEvents is a long lived watcher that watches for the health of the
// allocation's tasks.
func (t *Tracker) watchTaskEvents() {
	alloc := t.alloc
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
			t.logger.Trace("desired status is terminal for alloc", "alloc_id", alloc.ID, "desired_status", alloc.DesiredStatus)
			t.markAllocStopped()
			return
		}

		// Store the task states
		t.l.Lock()
		for task, state := range alloc.TaskStates {
			//TODO(schmichael) for now skip unknown tasks as
			//they're task group services which don't currently
			//support checks anyway
			if v, ok := t.taskHealth[task]; ok {
				v.state = state
			}
		}
		t.l.Unlock()

		// Detect if the alloc is unhealthy or if all tasks have started yet
		latestStartTime := time.Time{}
		for taskName, state := range alloc.TaskStates {
			// If the task is a poststop task we do not want to evaluate it
			// since it will remain pending until the main task has finished
			// or exited.
			if t.lifecycleTasks[taskName] == structs.TaskLifecycleHookPoststop {
				continue
			}

			// One of the tasks has failed so we can exit watching
			if state.Failed || (!state.FinishedAt.IsZero() && t.lifecycleTasks[taskName] != structs.TaskLifecycleHookPrestart) {
				t.setTaskHealth(false, true)
				return
			}

			if state.State == structs.TaskStatePending {
				latestStartTime = time.Time{}
				break
			} else if state.StartedAt.After(latestStartTime) {
				// task is either running or exited successfully
				latestStartTime = state.StartedAt
			}
		}

		// If the alloc is marked as failed by the client but none of the
		// individual tasks failed, that means something failed at the alloc
		// level.
		if alloc.ClientStatus == structs.AllocClientStatusFailed {
			t.l.Lock()
			t.allocFailed = true
			t.l.Unlock()

			t.setTaskHealth(false, true)
			return
		}

		if !latestStartTime.Equal(allStartedTime) {
			// reset task health
			t.setTaskHealth(false, false)

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
				healthyTimer.Reset(t.minHealthyTime)
			}
		}

		select {
		case <-t.ctx.Done():
			return
		case newAlloc, ok := <-t.allocUpdates.Ch():
			if !ok {
				return
			}
			alloc = newAlloc
		case <-healthyTimer.C:
			t.setTaskHealth(true, false)
		}
	}
}

// watchConsulEvents is a  watcher for the health of the allocation's Consul
// checks. If all checks report healthy the watcher will exit after the
// MinHealthyTime has been reached, Otherwise the watcher will continue to
// check unhealthy checks until the ctx is cancelled
func (t *Tracker) watchConsulEvents() {
	// checkTicker is the ticker that triggers us to look at the checks in
	// Consul
	checkTicker := time.NewTicker(t.checkLookupInterval)
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
		case <-t.ctx.Done():
			return
		case <-checkTicker.C:
			newAllocReg, err := t.consulClient.AllocRegistrations(t.alloc.ID)
			if err != nil {
				if !consulChecksErr {
					consulChecksErr = true
					t.logger.Warn("error looking up Consul registrations for allocation", "error", err, "alloc_id", t.alloc.ID)
				}
				continue OUTER
			} else {
				consulChecksErr = false
				allocReg = newAllocReg
			}
		case <-healthyTimer.C:
			if t.setCheckHealth(true) {
				// final health set and propagated
				return
			}
			// tasks are unhealthy, reset and wait until all is healthy
			primed = false
		}

		if allocReg == nil {
			continue
		}

		// Store the task registrations
		t.l.Lock()
		for task, reg := range allocReg.Tasks {
			//TODO(schmichael) for now skip unknown tasks as
			//they're task group services which don't currently
			//support checks anyway
			if v, ok := t.taskHealth[task]; ok {
				v.taskRegistrations = reg
			}
		}
		t.l.Unlock()

		// Detect if all the checks are passing
		passed := true

	CHECKS:
		for _, treg := range allocReg.Tasks {
			for _, sreg := range treg.Services {
				for _, check := range sreg.Checks {
					onupdate := sreg.CheckOnUpdate[check.CheckID]
					switch check.Status {
					case api.HealthPassing:
						continue
					case api.HealthWarning:
						if onupdate == structs.OnUpdateIgnoreWarn || onupdate == structs.OnUpdateIgnore {
							continue
						}
					case api.HealthCritical:
						if onupdate == structs.OnUpdateIgnore {
							continue
						}
					default:
					}

					passed = false
					t.setCheckHealth(false)
					break CHECKS
				}
			}
		}

		if !passed {
			// Reset the timer since we have transitioned back to unhealthy
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
			healthyTimer.Reset(t.minHealthyTime)
		}
	}
}

// taskHealthState captures all known health information about a task. It is
// largely used to determine if the task has contributed to the allocation being
// unhealthy.
type taskHealthState struct {
	task              *structs.Task
	state             *structs.TaskState
	taskRegistrations *consul.ServiceRegistrations
}

// event takes the deadline time for the allocation to be healthy and the update
// strategy of the group. It returns true if the task has contributed to the
// allocation being unhealthy and if so, an event description of why.
func (t *taskHealthState) event(deadline time.Time, minHealthyTime time.Duration, useChecks bool) (string, bool) {
	requireChecks := false
	desiredChecks := 0
	for _, s := range t.task.Services {
		if nc := len(s.Checks); nc > 0 {
			requireChecks = true
			desiredChecks += nc
		}
	}
	requireChecks = requireChecks && useChecks

	if t.state != nil {
		if t.state.Failed {
			return "Unhealthy because of failed task", true
		}

		switch t.state.State {
		case structs.TaskStatePending:
			return "Task not running by deadline", true
		case structs.TaskStateDead:
			// hook tasks are healthy when dead successfully
			if t.task.Lifecycle == nil || t.task.Lifecycle.Sidecar {
				return "Unhealthy because of dead task", true
			}
		case structs.TaskStateRunning:
			// We are running so check if we have been running long enough
			if t.state.StartedAt.Add(minHealthyTime).After(deadline) {
				return fmt.Sprintf("Task not running for min_healthy_time of %v by deadline", minHealthyTime), true
			}
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
