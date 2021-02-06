package allocrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allochealth"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/consul"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// healthMutator is able to set/clear alloc health.
type healthSetter interface {
	// HasHealth returns true if health is already set.
	HasHealth() bool

	// Set health via the mutator
	SetHealth(healthy, isDeploy bool, taskEvents map[string]*structs.TaskEvent)

	// Clear health when the deployment ID changes
	ClearHealth()
}

// allocHealthWatcherHook is responsible for watching an allocation's task
// status and (optionally) Consul health check status to determine if the
// allocation is health or unhealthy. Used by deployments and migrations.
type allocHealthWatcherHook struct {
	healthSetter healthSetter

	// consul client used to monitor health checks
	consul consul.ConsulServiceAPI

	// listener is given to trackers to listen for alloc updates and closed
	// when the alloc is destroyed.
	listener *cstructs.AllocListener

	// hookLock is held by hook methods to prevent concurrent access by
	// Update and synchronous hooks.
	hookLock sync.Mutex

	// watchDone is created before calling watchHealth and is closed when
	// watchHealth exits. Must be passed into watchHealth to avoid races.
	// Initialized already closed as Update may be called before Prerun.
	watchDone chan struct{}

	// ranOnce is set once Prerun or Update have run at least once. This
	// prevents Prerun from running if an Update has already been
	// processed. Must hold hookLock to access.
	ranOnce bool

	// cancelFn stops the health watching/setting goroutine. Wait on
	// watchLock to block until the watcher exits.
	cancelFn context.CancelFunc

	// alloc set by new func or Update. Must hold hookLock to access.
	alloc *structs.Allocation

	// isDeploy is true if monitoring a deployment. Set in init(). Must
	// hold hookLock to access.
	isDeploy bool

	logger log.Logger
}

func newAllocHealthWatcherHook(logger log.Logger, alloc *structs.Allocation, hs healthSetter,
	listener *cstructs.AllocListener, consul consul.ConsulServiceAPI) interfaces.RunnerHook {

	// Neither deployments nor migrations care about the health of
	// non-service jobs so never watch their health
	if alloc.Job.Type != structs.JobTypeService {
		return noopAllocHealthWatcherHook{}
	}

	// Initialize watchDone with a closed chan in case Update runs before Prerun
	closedDone := make(chan struct{})
	close(closedDone)

	h := &allocHealthWatcherHook{
		alloc:        alloc,
		cancelFn:     func() {}, // initialize to prevent nil func panics
		watchDone:    closedDone,
		consul:       consul,
		healthSetter: hs,
		listener:     listener,
	}

	h.logger = logger.Named(h.Name())
	return h
}

func (h *allocHealthWatcherHook) Name() string {
	return "alloc_health_watcher"
}

// init starts the allochealth.Tracker and watchHealth goroutine on either
// Prerun or Update. Caller must set/update alloc and logger fields.
//
// Not threadsafe so the caller should lock since Updates occur concurrently.
func (h *allocHealthWatcherHook) init() error {
	// No need to watch health as it's already set
	if h.healthSetter.HasHealth() {
		h.logger.Trace("not watching; already has health set")
		return nil
	}

	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	if tg == nil {
		return fmt.Errorf("task group %q does not exist in job %q", h.alloc.TaskGroup, h.alloc.Job.ID)
	}

	h.isDeploy = h.alloc.DeploymentID != ""

	// No need to watch allocs for deployments that rely on operators
	// manually setting health
	if h.isDeploy && (tg.Update.IsEmpty() || tg.Update.HealthCheck == structs.UpdateStrategyHealthCheck_Manual) {
		return nil
	}

	// Define the deadline, health method, min healthy time from the
	// deployment if this is a deployment; otherwise from the migration
	// strategy.
	deadline, useChecks, minHealthyTime := getHealthParams(time.Now(), tg, h.isDeploy)

	// Create a context that is canceled when the tracker should shutdown.
	ctx := context.Background()
	ctx, h.cancelFn = context.WithCancel(ctx)

	h.logger.Trace("watching", "deadline", deadline, "checks", useChecks, "min_healthy_time", minHealthyTime)
	// Create a new tracker, start it, and watch for health results.
	tracker := allochealth.NewTracker(ctx, h.logger, h.alloc,
		h.listener, h.consul, minHealthyTime, useChecks)
	tracker.Start()

	// Create a new done chan and start watching for health updates
	h.watchDone = make(chan struct{})
	go h.watchHealth(ctx, deadline, tracker, h.watchDone)
	return nil
}

func (h *allocHealthWatcherHook) Prerun() error {
	h.hookLock.Lock()
	defer h.hookLock.Unlock()

	if h.ranOnce {
		// An Update beat Prerun to running the watcher; noop
		return nil
	}

	h.ranOnce = true
	return h.init()
}

func (h *allocHealthWatcherHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.hookLock.Lock()
	defer h.hookLock.Unlock()

	// Prevent Prerun from running after an Update
	h.ranOnce = true

	// Cancel the old watcher and create a new one
	h.cancelFn()

	// Wait until the watcher exits
	<-h.watchDone

	// Deployment has changed, reset status
	if req.Alloc.DeploymentID != h.alloc.DeploymentID {
		h.healthSetter.ClearHealth()
	}

	// Update alloc
	h.alloc = req.Alloc

	return h.init()
}

func (h *allocHealthWatcherHook) Postrun() error {
	h.hookLock.Lock()
	defer h.hookLock.Unlock()

	h.cancelFn()
	h.listener.Close()

	// Wait until the watcher exits
	<-h.watchDone

	return nil
}

func (h *allocHealthWatcherHook) Shutdown() {
	// Same as Postrun
	h.Postrun()
}

// watchHealth watches alloc health until it is set, the alloc is stopped, the
// deadline is reached, or the context is canceled. watchHealth will be
// canceled and restarted on Updates so calls are serialized with a lock.
func (h *allocHealthWatcherHook) watchHealth(ctx context.Context, deadline time.Time, tracker *allochealth.Tracker, done chan<- struct{}) {
	defer close(done)

	// Default to unhealthy for the deadline reached case
	healthy := false

	select {
	case <-ctx.Done():
		// Graceful shutdown
		return

	case <-tracker.AllocStoppedCh():
		// Allocation has stopped so no need to set health
		return

	case <-time.After(time.Until(deadline)):
		// Time is up! Fallthrough to set unhealthy.
		h.logger.Trace("deadline reached; setting unhealthy", "deadline", deadline)

	case healthy = <-tracker.HealthyCh():
		// Health received. Fallthrough to set it.
	}

	h.logger.Trace("health set", "healthy", healthy)

	// If this is an unhealthy deployment emit events for tasks
	var taskEvents map[string]*structs.TaskEvent
	if !healthy && h.isDeploy {
		taskEvents = tracker.TaskEvents()
	}

	h.healthSetter.SetHealth(healthy, h.isDeploy, taskEvents)
}

// getHealthParams returns the health watcher parameters which vary based on
// whether this allocation is in a deployment or migration.
func getHealthParams(now time.Time, tg *structs.TaskGroup, isDeploy bool) (deadline time.Time, useChecks bool, minHealthyTime time.Duration) {
	if isDeploy {
		deadline = now.Add(tg.Update.HealthyDeadline)
		minHealthyTime = tg.Update.MinHealthyTime
		useChecks = tg.Update.HealthCheck == structs.UpdateStrategyHealthCheck_Checks
	} else {
		strategy := tg.Migrate
		if strategy == nil {
			// For backwards compat with pre-0.8 allocations that
			// don't have a migrate strategy set.
			strategy = structs.DefaultMigrateStrategy()
		}

		deadline = now.Add(strategy.HealthyDeadline)
		minHealthyTime = strategy.MinHealthyTime
		useChecks = strategy.HealthCheck == structs.MigrateStrategyHealthChecks
	}
	return
}

// noopAllocHealthWatcherHook is an empty hook implementation returned by
// newAllocHealthWatcherHook when an allocation will never need its health
// monitored.
type noopAllocHealthWatcherHook struct{}

func (noopAllocHealthWatcherHook) Name() string {
	return "alloc_health_watcher"
}
