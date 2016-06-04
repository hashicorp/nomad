package client

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/nomad/client/driver/env"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

const (
	// killBackoffBaseline is the baseline time for exponential backoff while
	// killing a task.
	killBackoffBaseline = 5 * time.Second

	// killBackoffLimit is the the limit of the exponential backoff for killing
	// the task.
	killBackoffLimit = 2 * time.Minute

	// killFailureLimit is how many times we will attempt to kill a task before
	// giving up and potentially leaking resources.
	killFailureLimit = 5
)

// TaskStatsReporter exposes APIs to query resource usage of a Task
type TaskStatsReporter interface {
	// ResourceUsage returns the latest resource usage data point collected for
	// the task
	ResourceUsage() []*cstructs.TaskResourceUsage

	// ResourceUsageTS returns all the resource usage data points since a given
	// time
	ResourceUsageTS(since int64) []*cstructs.TaskResourceUsage
}

// TaskRunner is used to wrap a task within an allocation and provide the execution context.
type TaskRunner struct {
	config         *config.Config
	updater        TaskStateUpdater
	logger         *log.Logger
	ctx            *driver.ExecContext
	alloc          *structs.Allocation
	restartTracker *RestartTracker

	resourceUsage     *stats.RingBuff
	resourceUsageLock sync.RWMutex

	task       *structs.Task
	taskEnv    *env.TaskEnvironment
	updateCh   chan *structs.Allocation
	handle     driver.DriverHandle
	handleLock sync.Mutex

	// artifactsDownloaded tracks whether the tasks artifacts have been
	// downloaded
	artifactsDownloaded bool

	destroy      bool
	destroyCh    chan struct{}
	destroyLock  sync.Mutex
	destroyEvent string
	waitCh       chan struct{}
}

// taskRunnerState is used to snapshot the state of the task runner
type taskRunnerState struct {
	Version            string
	Task               *structs.Task
	HandleID           string
	ArtifactDownloaded bool
}

// TaskStateUpdater is used to signal that tasks state has changed.
type TaskStateUpdater func(taskName, state string, event *structs.TaskEvent)

// NewTaskRunner is used to create a new task context
func NewTaskRunner(logger *log.Logger, config *config.Config,
	updater TaskStateUpdater, ctx *driver.ExecContext,
	alloc *structs.Allocation, task *structs.Task) *TaskRunner {

	// Merge in the task resources
	task.Resources = alloc.TaskResources[task.Name]

	// Build the restart tracker.
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		logger.Printf("[ERR] client: alloc '%s' for missing task group '%s'", alloc.ID, alloc.TaskGroup)
		return nil
	}
	restartTracker := newRestartTracker(tg.RestartPolicy, alloc.Job.Type)

	resourceUsage, err := stats.NewRingBuff(config.StatsDataPoints)
	if err != nil {
		logger.Printf("[ERR] client: can't create resource usage buffer: %v", err)
		return nil
	}

	tc := &TaskRunner{
		config:         config,
		updater:        updater,
		logger:         logger,
		restartTracker: restartTracker,
		resourceUsage:  resourceUsage,
		ctx:            ctx,
		alloc:          alloc,
		task:           task,
		updateCh:       make(chan *structs.Allocation, 64),
		destroyCh:      make(chan struct{}),
		destroyEvent:   structs.TaskKilled,
		waitCh:         make(chan struct{}),
	}

	return tc
}

// MarkReceived marks the task as received.
func (r *TaskRunner) MarkReceived() {
	r.updater(r.task.Name, structs.TaskStatePending, structs.NewTaskEvent(structs.TaskReceived))
}

// WaitCh returns a channel to wait for termination
func (r *TaskRunner) WaitCh() <-chan struct{} {
	return r.waitCh
}

// stateFilePath returns the path to our state file
func (r *TaskRunner) stateFilePath() string {
	// Get the MD5 of the task name
	hashVal := md5.Sum([]byte(r.task.Name))
	hashHex := hex.EncodeToString(hashVal[:])
	dirName := fmt.Sprintf("task-%s", hashHex)

	// Generate the path
	path := filepath.Join(r.config.StateDir, "alloc", r.alloc.ID,
		dirName, "state.json")
	return path
}

// RestoreState is used to restore our state
func (r *TaskRunner) RestoreState() error {
	// Load the snapshot
	var snap taskRunnerState
	if err := restoreState(r.stateFilePath(), &snap); err != nil {
		return err
	}

	// Restore fields
	r.task = snap.Task
	r.artifactsDownloaded = snap.ArtifactDownloaded

	if err := r.setTaskEnv(); err != nil {
		err := fmt.Errorf("failed to create task environment for task %q in allocation %q: %v",
			r.task.Name, r.alloc.ID, err)
		r.logger.Printf("[ERR] client: %s", err)
		return err
	}

	// Restore the driver
	if snap.HandleID != "" {
		driver, err := r.createDriver()
		if err != nil {
			return err
		}

		handle, err := driver.Open(r.ctx, snap.HandleID)

		// In the case it fails, we relaunch the task in the Run() method.
		if err != nil {
			r.logger.Printf("[ERR] client: failed to open handle to task '%s' for alloc '%s': %v",
				r.task.Name, r.alloc.ID, err)
			return nil
		}
		r.handleLock.Lock()
		r.handle = handle
		r.handleLock.Unlock()
	}
	return nil
}

// SaveState is used to snapshot our state
func (r *TaskRunner) SaveState() error {
	snap := taskRunnerState{
		Task:               r.task,
		Version:            r.config.Version,
		ArtifactDownloaded: r.artifactsDownloaded,
	}
	r.handleLock.Lock()
	if r.handle != nil {
		snap.HandleID = r.handle.ID()
	}
	r.handleLock.Unlock()
	return persistState(r.stateFilePath(), &snap)
}

// DestroyState is used to cleanup after ourselves
func (r *TaskRunner) DestroyState() error {
	return os.RemoveAll(r.stateFilePath())
}

// setState is used to update the state of the task runner
func (r *TaskRunner) setState(state string, event *structs.TaskEvent) {
	// Persist our state to disk.
	if err := r.SaveState(); err != nil {
		r.logger.Printf("[ERR] client: failed to save state of Task Runner: %v", r.task.Name)
	}

	// Indicate the task has been updated.
	r.updater(r.task.Name, state, event)
}

// setTaskEnv sets the task environment. It returns an error if it could not be
// created.
func (r *TaskRunner) setTaskEnv() error {
	taskEnv, err := driver.GetTaskEnv(r.ctx.AllocDir, r.config.Node, r.task.Copy(), r.alloc)
	if err != nil {
		return err
	}
	r.taskEnv = taskEnv
	return nil
}

// createDriver makes a driver for the task
func (r *TaskRunner) createDriver() (driver.Driver, error) {
	if r.taskEnv == nil {
		err := fmt.Errorf("task environment not made for task %q in allocation %q", r.task.Name, r.alloc.ID)
		return nil, err
	}

	driverCtx := driver.NewDriverContext(r.task.Name, r.config, r.config.Node, r.logger, r.taskEnv)
	driver, err := driver.NewDriver(r.task.Driver, driverCtx)
	if err != nil {
		err = fmt.Errorf("failed to create driver '%s' for alloc %s: %v",
			r.task.Driver, r.alloc.ID, err)
		r.logger.Printf("[ERR] client: %s", err)
		return nil, err
	}
	return driver, err
}

// Run is a long running routine used to manage the task
func (r *TaskRunner) Run() {
	defer close(r.waitCh)
	r.logger.Printf("[DEBUG] client: starting task context for '%s' (alloc '%s')",
		r.task.Name, r.alloc.ID)

	if err := r.validateTask(); err != nil {
		r.setState(
			structs.TaskStateDead,
			structs.NewTaskEvent(structs.TaskFailedValidation).SetValidationError(err))
		return
	}

	if err := r.setTaskEnv(); err != nil {
		r.setState(
			structs.TaskStateDead,
			structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(err))
		return
	}

	r.run()
	return
}

// validateTask validates the fields of the task and returns an error if the
// task is invalid.
func (r *TaskRunner) validateTask() error {
	var mErr multierror.Error

	// Validate the user.
	unallowedUsers := r.config.ReadStringListToMapDefault("user.blacklist", config.DefaultUserBlacklist)
	checkDrivers := r.config.ReadStringListToMapDefault("user.checked_drivers", config.DefaultUserCheckedDrivers)
	if _, driverMatch := checkDrivers[r.task.Driver]; driverMatch {
		if _, unallowed := unallowedUsers[r.task.User]; unallowed {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("running as user %q is disallowed", r.task.User))
		}
	}

	// Validate the artifacts
	for i, artifact := range r.task.Artifacts {
		// Verify the artifact doesn't escape the task directory.
		if err := artifact.Validate(); err != nil {
			// If this error occurs there is potentially a server bug or
			// mallicious, server spoofing.
			r.logger.Printf("[ERR] client: allocation %q, task %v, artifact %#v (%v) fails validation: %v",
				r.alloc.ID, r.task.Name, artifact, i, err)
			mErr.Errors = append(mErr.Errors, fmt.Errorf("artifact (%d) failed validation: %v", i, err))
		}
	}

	if len(mErr.Errors) == 1 {
		return mErr.Errors[0]
	}
	return mErr.ErrorOrNil()
}

func (r *TaskRunner) run() {
	// Predeclare things so we an jump to the RESTART
	var handleEmpty bool
	var stopCollection chan struct{}

	for {
		// Download the task's artifacts
		if !r.artifactsDownloaded && len(r.task.Artifacts) > 0 {
			r.setState(structs.TaskStatePending, structs.NewTaskEvent(structs.TaskDownloadingArtifacts))
			taskDir, ok := r.ctx.AllocDir.TaskDirs[r.task.Name]
			if !ok {
				err := fmt.Errorf("task directory couldn't be found")
				r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(err))
				r.logger.Printf("[ERR] client: task directory for alloc %q task %q couldn't be found", r.alloc.ID, r.task.Name)
				r.restartTracker.SetStartError(err)
				goto RESTART
			}

			for _, artifact := range r.task.Artifacts {
				if err := getter.GetArtifact(r.taskEnv, artifact, taskDir, r.logger); err != nil {
					r.setState(structs.TaskStateDead,
						structs.NewTaskEvent(structs.TaskArtifactDownloadFailed).SetDownloadError(err))
					r.restartTracker.SetStartError(cstructs.NewRecoverableError(err, true))
					goto RESTART
				}
			}

			r.artifactsDownloaded = true
		}

		// Start the task if not yet started or it is being forced. This logic
		// is necessary because in the case of a restore the handle already
		// exists.
		r.handleLock.Lock()
		handleEmpty = r.handle == nil
		r.handleLock.Unlock()

		if handleEmpty {
			stopCollection = make(chan struct{})
			startErr := r.startTask(stopCollection)
			r.restartTracker.SetStartError(startErr)
			if startErr != nil {
				r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(startErr))
				goto RESTART
			}
		}

		// Mark the task as started
		r.setState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))

		// Wait for updates
	WAIT:
		for {
			select {
			case waitRes := <-r.handle.WaitCh():
				if waitRes == nil {
					panic("nil wait")
				}

				// Stop collection the task's resource usage
				close(stopCollection)

				// Log whether the task was successful or not.
				r.restartTracker.SetWaitResult(waitRes)
				r.setState(structs.TaskStateDead, r.waitErrorToEvent(waitRes))
				if !waitRes.Successful() {
					r.logger.Printf("[INFO] client: task %q for alloc %q failed: %v", r.task.Name, r.alloc.ID, waitRes)
				} else {
					r.logger.Printf("[INFO] client: task %q for alloc %q completed successfully", r.task.Name, r.alloc.ID)
				}

				break WAIT
			case update := <-r.updateCh:
				if err := r.handleUpdate(update); err != nil {
					r.logger.Printf("[ERR] client: update to task %q failed: %v", r.task.Name, err)
				}
			case <-r.destroyCh:
				// Kill the task using an exponential backoff in-case of failures.
				destroySuccess, err := r.handleDestroy()
				if !destroySuccess {
					// We couldn't successfully destroy the resource created.
					r.logger.Printf("[ERR] client: failed to kill task %q. Resources may have been leaked: %v", r.task.Name, err)
				}

				// Store that the task has been destroyed and any associated error.
				r.setState(structs.TaskStateDead, structs.NewTaskEvent(r.destroyEvent).SetKillError(err))
				return
			}
		}

	RESTART:
		state, when := r.restartTracker.GetState()
		r.restartTracker.SetStartError(nil).SetWaitResult(nil)
		reason := r.restartTracker.GetReason()
		switch state {
		case structs.TaskNotRestarting, structs.TaskTerminated:
			r.logger.Printf("[INFO] client: Not restarting task: %v for alloc: %v ", r.task.Name, r.alloc.ID)
			if state == structs.TaskNotRestarting {
				r.setState(structs.TaskStateDead,
					structs.NewTaskEvent(structs.TaskNotRestarting).
						SetRestartReason(reason))
			}
			return
		case structs.TaskRestarting:
			r.logger.Printf("[INFO] client: Restarting task %q for alloc %q in %v", r.task.Name, r.alloc.ID, when)
			r.setState(structs.TaskStatePending,
				structs.NewTaskEvent(structs.TaskRestarting).
					SetRestartDelay(when).
					SetRestartReason(reason))
		default:
			r.logger.Printf("[ERR] client: restart tracker returned unknown state: %q", state)
			return
		}

		// Sleep but watch for destroy events.
		select {
		case <-time.After(when):
		case <-r.destroyCh:
		}

		// Destroyed while we were waiting to restart, so abort.
		r.destroyLock.Lock()
		destroyed := r.destroy
		r.destroyLock.Unlock()
		if destroyed {
			r.logger.Printf("[DEBUG] client: Not restarting task: %v because it's destroyed by user", r.task.Name)
			r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled))
			return
		}

		// Clear the handle so a new driver will be created.
		r.handleLock.Lock()
		r.handle = nil
		r.handleLock.Unlock()
	}
}

// startTask creates the driver and start the task. Resource usage is also
// collect in a launched goroutine. Collection ends when the passed channel is
// closed
func (r *TaskRunner) startTask(stopCollection <-chan struct{}) error {
	// Create a driver
	driver, err := r.createDriver()
	if err != nil {
		r.logger.Printf("[ERR] client: failed to create driver of task '%s' for alloc '%s': %v",
			r.task.Name, r.alloc.ID, err)
		return err
	}

	// Start the job
	handle, err := driver.Start(r.ctx, r.task)
	if err != nil {
		r.logger.Printf("[ERR] client: failed to start task '%s' for alloc '%s': %v",
			r.task.Name, r.alloc.ID, err)
		return err
	}

	r.handleLock.Lock()
	r.handle = handle
	r.handleLock.Unlock()
	go r.collectResourceUsageStats(stopCollection)
	return nil
}

// collectResourceUsageStats starts collecting resource usage stats of a Task.
// Collection ends when the passed channel is closed
func (r *TaskRunner) collectResourceUsageStats(stopCollection <-chan struct{}) {
	// start collecting the stats right away and then start collecting every
	// collection interval
	next := time.NewTimer(0)
	defer next.Stop()
	for {
		select {
		case <-next.C:
			ru, err := r.handle.Stats()
			next.Reset(r.config.StatsCollectionInterval)

			if err != nil {
				// We do not log when the plugin is shutdown as this is simply a
				// race between the stopCollection channel being closed and calling
				// Stats on the handle.
				if !strings.Contains(err.Error(), "connection is shut down") {
					r.logger.Printf("[WARN] client: error fetching stats of task %v: %v", r.task.Name, err)
				}
				continue
			}

			r.resourceUsageLock.Lock()
			r.resourceUsage.Enqueue(ru)
			r.resourceUsageLock.Unlock()
			r.emitStats(ru)
		case <-stopCollection:
			return
		}
	}
}

// TaskStatsReporter returns the stats reporter of the task
func (r *TaskRunner) StatsReporter() TaskStatsReporter {
	return r
}

// ResourceUsage returns the last resource utilization datapoint collected
func (r *TaskRunner) ResourceUsage() []*cstructs.TaskResourceUsage {
	r.resourceUsageLock.RLock()
	defer r.resourceUsageLock.RUnlock()
	val := r.resourceUsage.Peek()
	ru, _ := val.(*cstructs.TaskResourceUsage)
	return []*cstructs.TaskResourceUsage{ru}
}

// ResourceUsageTS returns the list of all the resource utilization datapoints
// collected
func (r *TaskRunner) ResourceUsageTS(since int64) []*cstructs.TaskResourceUsage {
	r.resourceUsageLock.RLock()
	defer r.resourceUsageLock.RUnlock()

	values := r.resourceUsage.Values()
	low := 0
	high := len(values) - 1
	var idx int

	for {
		mid := (low + high) / 2
		midVal, _ := values[mid].(*cstructs.TaskResourceUsage)
		if midVal.Timestamp < since {
			low = mid + 1
		} else if midVal.Timestamp > since {
			high = mid - 1
		} else if midVal.Timestamp == since {
			idx = mid
			break
		}
		if low > high {
			idx = low
			break
		}
	}
	values = values[idx:]
	ts := make([]*cstructs.TaskResourceUsage, len(values))
	for index, val := range values {
		ru, _ := val.(*cstructs.TaskResourceUsage)
		ts[index] = ru
	}
	return ts
}

// handleUpdate takes an updated allocation and updates internal state to
// reflect the new config for the task.
func (r *TaskRunner) handleUpdate(update *structs.Allocation) error {
	// Extract the task group from the alloc.
	tg := update.Job.LookupTaskGroup(update.TaskGroup)
	if tg == nil {
		return fmt.Errorf("alloc '%s' missing task group '%s'", update.ID, update.TaskGroup)
	}

	// Extract the task.
	var updatedTask *structs.Task
	for _, t := range tg.Tasks {
		if t.Name == r.task.Name {
			updatedTask = t
		}
	}
	if updatedTask == nil {
		return fmt.Errorf("task group %q doesn't contain task %q", tg.Name, r.task.Name)
	}

	// Merge in the task resources
	updatedTask.Resources = update.TaskResources[updatedTask.Name]

	// Update will update resources and store the new kill timeout.
	var mErr multierror.Error
	r.handleLock.Lock()
	if r.handle != nil {
		if err := r.handle.Update(updatedTask); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("updating task resources failed: %v", err))
		}
	}
	r.handleLock.Unlock()

	// Update the restart policy.
	if r.restartTracker != nil {
		r.restartTracker.SetPolicy(tg.RestartPolicy)
	}

	// Store the updated alloc.
	r.alloc = update
	r.task = updatedTask
	return mErr.ErrorOrNil()
}

// handleDestroy kills the task handle. In the case that killing fails,
// handleDestroy will retry with an exponential backoff and will give up at a
// given limit. It returns whether the task was destroyed and the error
// associated with the last kill attempt.
func (r *TaskRunner) handleDestroy() (destroyed bool, err error) {
	// Cap the number of times we attempt to kill the task.
	for i := 0; i < killFailureLimit; i++ {
		if err = r.handle.Kill(); err != nil {
			// Calculate the new backoff
			backoff := (1 << (2 * uint64(i))) * killBackoffBaseline
			if backoff > killBackoffLimit {
				backoff = killBackoffLimit
			}

			r.logger.Printf("[ERR] client: failed to kill task '%s' for alloc %q. Retrying in %v: %v",
				r.task.Name, r.alloc.ID, backoff, err)
			time.Sleep(time.Duration(backoff))
		} else {
			// Kill was successful
			return true, nil
		}
	}
	return
}

// Helper function for converting a WaitResult into a TaskTerminated event.
func (r *TaskRunner) waitErrorToEvent(res *cstructs.WaitResult) *structs.TaskEvent {
	return structs.NewTaskEvent(structs.TaskTerminated).
		SetExitCode(res.ExitCode).
		SetSignal(res.Signal).
		SetExitMessage(res.Err)
}

// Update is used to update the task of the context
func (r *TaskRunner) Update(update *structs.Allocation) {
	select {
	case r.updateCh <- update:
	default:
		r.logger.Printf("[ERR] client: dropping task update '%s' (alloc '%s')",
			r.task.Name, r.alloc.ID)
	}
}

func (r *TaskRunner) SetDestroyEvent(event string) {
	r.destroyEvent = event
}

// Destroy is used to indicate that the task context should be destroyed
func (r *TaskRunner) Destroy() {
	r.destroyLock.Lock()
	defer r.destroyLock.Unlock()

	if r.destroy {
		return
	}
	r.destroy = true
	close(r.destroyCh)
}

// emitStats emits resource usage stats of tasks to remote metrics collector
// sinks
func (r *TaskRunner) emitStats(ru *cstructs.TaskResourceUsage) {
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "memory", "rss"}, float32(ru.ResourceUsage.MemoryStats.RSS))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "memory", "cache"}, float32(ru.ResourceUsage.MemoryStats.Cache))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "memory", "swap"}, float32(ru.ResourceUsage.MemoryStats.Swap))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "memory", "max_usage"}, float32(ru.ResourceUsage.MemoryStats.MaxUsage))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "memory", "kernel_usage"}, float32(ru.ResourceUsage.MemoryStats.KernelUsage))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "memory", "kernel_max_usage"}, float32(ru.ResourceUsage.MemoryStats.KernelMaxUsage))

	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "cpu", "percent"}, float32(ru.ResourceUsage.CpuStats.Percent))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "cpu", "system"}, float32(ru.ResourceUsage.CpuStats.SystemMode))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "cpu", "user"}, float32(ru.ResourceUsage.CpuStats.UserMode))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "cpu", "throttled_time"}, float32(ru.ResourceUsage.CpuStats.ThrottledTime))
	metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, "cpu", "throttled_periods"}, float32(ru.ResourceUsage.CpuStats.ThrottledPeriods))

	for pid, pidStats := range ru.Pids {
		// Not emitting max, kernel usages since we never get them on a per-pid
		// basis
		metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, pid, "memory", "rss"}, float32(pidStats.MemoryStats.RSS))
		metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, pid, "memory", "cache"}, float32(pidStats.MemoryStats.Cache))
		metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, pid, "memory", "swap"}, float32(pidStats.MemoryStats.Swap))

		// Not emitting throttled time and periods since we never get them on a
		// per pid basis
		metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, pid, "cpu", "percent"}, float32(pidStats.CpuStats.Percent))
		metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, pid, "cpu", "system"}, float32(pidStats.CpuStats.SystemMode))
		metrics.EmitKey([]string{r.alloc.Job.Name, r.alloc.Name, r.alloc.ID, r.task.Name, pid, "cpu", "user"}, float32(pidStats.CpuStats.UserMode))
	}
}
