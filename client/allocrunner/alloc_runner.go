package allocrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner"
	"github.com/hashicorp/nomad/client/allocwatcher"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/devicemanager"
	cinterfaces "github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// allocRunner is used to run all the tasks in a given allocation
type allocRunner struct {
	// id is the ID of the allocation. Can be accessed without a lock
	id string

	// Logger is the logger for the alloc runner.
	logger log.Logger

	clientConfig *config.Config

	// stateUpdater is used to emit updated alloc state
	stateUpdater cinterfaces.AllocStateHandler

	// taskStateUpdatedCh is ticked whenever task state as changed. Must
	// have len==1 to allow nonblocking notification of state updates while
	// the goroutine is already processing a previous update.
	taskStateUpdatedCh chan struct{}

	// taskStateUpdateHandlerCh is closed when the task state handling
	// goroutine exits. It is unsafe to destroy the local allocation state
	// before this goroutine exits.
	taskStateUpdateHandlerCh chan struct{}

	// allocUpdatedCh is a channel that is used to stream allocation updates into
	// the allocUpdate handler. Must have len==1 to allow nonblocking notification
	// of new allocation updates while the goroutine is processing a previous
	// update.
	allocUpdatedCh chan *structs.Allocation

	// consulClient is the client used by the consul service hook for
	// registering services and checks
	consulClient consul.ConsulServiceAPI

	// vaultClient is the used to manage Vault tokens
	vaultClient vaultclient.VaultClient

	// waitCh is closed when the Run() loop has exited
	waitCh chan struct{}

	// destroyed is true when the Run() loop has exited, postrun hooks have
	// run, and alloc runner has been destroyed. Must acquire destroyedLock
	// to access.
	destroyed bool

	// destroyCh is closed when the Run() loop has exited, postrun hooks have
	// run, and alloc runner has been destroyed.
	destroyCh chan struct{}

	// shutdown is true when the Run() loop has exited, and shutdown hooks have
	// run. Must acquire destroyedLock to access.
	shutdown bool

	// shutdownCh is closed when the Run() loop has exited, and shutdown hooks
	// have run.
	shutdownCh chan struct{}

	// runnersLaunched is true if TaskRunners were Run. Must acquire
	// destroyedLock to access.
	runnersLaunched bool

	// destroyLaunched is true if Destroy has been called. Must acquire
	// destroyedLock to access.
	destroyLaunched bool

	// shutdownLaunched is true if Shutdown has been called. Must acquire
	// destroyedLock to access.
	shutdownLaunched bool

	// destroyedLock guards destroyed, runnersLaunched, destroyLaunched,
	// shutdownLaunched, and serializes Shutdown/Destroy calls.
	destroyedLock sync.Mutex

	// Alloc captures the allocation being run.
	alloc     *structs.Allocation
	allocLock sync.RWMutex

	// state is the alloc runner's state
	state     *state.State
	stateLock sync.RWMutex

	stateDB cstate.StateDB

	// allocDir is used to build the allocations directory structure.
	allocDir *allocdir.AllocDir

	// runnerHooks are alloc runner lifecycle hooks that should be run on state
	// transistions.
	runnerHooks []interfaces.RunnerHook

	// tasks are the set of task runners
	tasks map[string]*taskrunner.TaskRunner

	// deviceStatsReporter is used to lookup resource usage for alloc devices
	deviceStatsReporter cinterfaces.DeviceStatsReporter

	// allocBroadcaster sends client allocation updates to all listeners
	allocBroadcaster *cstructs.AllocBroadcaster

	// prevAllocWatcher allows waiting for any previous or preempted allocations
	// to exit
	prevAllocWatcher allocwatcher.PrevAllocWatcher

	// prevAllocMigrator allows the migration of a previous allocations alloc dir.
	prevAllocMigrator allocwatcher.PrevAllocMigrator

	// devicemanager is used to mount devices as well as lookup device
	// statistics
	devicemanager devicemanager.Manager

	// driverManager is responsible for dispensing driver plugins and registering
	// event handlers
	driverManager drivermanager.Manager
}

// NewAllocRunner returns a new allocation runner.
func NewAllocRunner(config *Config) (*allocRunner, error) {
	alloc := config.Alloc
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil, fmt.Errorf("failed to lookup task group %q", alloc.TaskGroup)
	}

	ar := &allocRunner{
		id:                       alloc.ID,
		alloc:                    alloc,
		clientConfig:             config.ClientConfig,
		consulClient:             config.Consul,
		vaultClient:              config.Vault,
		tasks:                    make(map[string]*taskrunner.TaskRunner, len(tg.Tasks)),
		waitCh:                   make(chan struct{}),
		destroyCh:                make(chan struct{}),
		shutdownCh:               make(chan struct{}),
		state:                    &state.State{},
		stateDB:                  config.StateDB,
		stateUpdater:             config.StateUpdater,
		taskStateUpdatedCh:       make(chan struct{}, 1),
		taskStateUpdateHandlerCh: make(chan struct{}),
		allocUpdatedCh:           make(chan *structs.Allocation, 1),
		deviceStatsReporter:      config.DeviceStatsReporter,
		prevAllocWatcher:         config.PrevAllocWatcher,
		prevAllocMigrator:        config.PrevAllocMigrator,
		devicemanager:            config.DeviceManager,
		driverManager:            config.DriverManager,
	}

	// Create the logger based on the allocation ID
	ar.logger = config.Logger.Named("alloc_runner").With("alloc_id", alloc.ID)

	// Create alloc broadcaster
	ar.allocBroadcaster = cstructs.NewAllocBroadcaster(ar.logger)

	// Create alloc dir
	ar.allocDir = allocdir.NewAllocDir(ar.logger, filepath.Join(config.ClientConfig.AllocDir, alloc.ID))

	// Initialize the runners hooks.
	ar.initRunnerHooks()

	// Create the TaskRunners
	if err := ar.initTaskRunners(tg.Tasks); err != nil {
		return nil, err
	}

	return ar, nil
}

// initTaskRunners creates task runners but does *not* run them.
func (ar *allocRunner) initTaskRunners(tasks []*structs.Task) error {
	for _, task := range tasks {
		config := &taskrunner.Config{
			Alloc:               ar.alloc,
			ClientConfig:        ar.clientConfig,
			Task:                task,
			TaskDir:             ar.allocDir.NewTaskDir(task.Name),
			Logger:              ar.logger,
			StateDB:             ar.stateDB,
			StateUpdater:        ar,
			Consul:              ar.consulClient,
			Vault:               ar.vaultClient,
			DeviceStatsReporter: ar.deviceStatsReporter,
			DeviceManager:       ar.devicemanager,
			DriverManager:       ar.driverManager,
		}

		// Create, but do not Run, the task runner
		tr, err := taskrunner.NewTaskRunner(config)
		if err != nil {
			return fmt.Errorf("failed creating runner for task %q: %v", task.Name, err)
		}

		ar.tasks[task.Name] = tr
	}
	return nil
}

func (ar *allocRunner) WaitCh() <-chan struct{} {
	return ar.waitCh
}

// Run the AllocRunner. Starts tasks if the alloc is non-terminal and closes
// WaitCh when it exits. Should be started in a goroutine.
func (ar *allocRunner) Run() {
	// Close the wait channel on return
	defer close(ar.waitCh)

	// Start the task state update handler
	go ar.handleTaskStateUpdates()

	// Start the alloc update handler
	go ar.handleAllocUpdates()

	// If an alloc should not be run, ensure any restored task handles are
	// destroyed and exit to wait for the AR to be GC'd by the client.
	if !ar.shouldRun() {
		ar.logger.Debug("not running terminal alloc")

		// Ensure all tasks are cleaned up
		ar.killTasks()
		return
	}

	// Mark task runners as being run for Shutdown
	ar.destroyedLock.Lock()
	ar.runnersLaunched = true
	ar.destroyedLock.Unlock()

	// If task update chan has been closed, that means we've been shutdown.
	select {
	case <-ar.taskStateUpdateHandlerCh:
		return
	default:
	}

	// Run the prestart hooks
	if err := ar.prerun(); err != nil {
		ar.logger.Error("prerun failed", "error", err)
		goto POST
	}

	// Run the runners (blocks until they exit)
	ar.runTasks()

POST:
	// Run the postrun hooks
	// XXX Equivalent to TR.Poststop hook
	if err := ar.postrun(); err != nil {
		ar.logger.Error("postrun failed", "error", err)
	}

}

// shouldRun returns true if the alloc is in a state that the alloc runner
// should run it.
func (ar *allocRunner) shouldRun() bool {
	// Do not run allocs that are terminal
	if ar.Alloc().TerminalStatus() {
		ar.logger.Trace("alloc terminal; not running",
			"desired_status", ar.Alloc().DesiredStatus,
			"client_status", ar.Alloc().ClientStatus,
		)
		return false
	}

	// It's possible that the alloc local state was marked terminal before
	// the server copy of the alloc (checked above) was marked as terminal,
	// so check the local state as well.
	switch clientStatus := ar.AllocState().ClientStatus; clientStatus {
	case structs.AllocClientStatusComplete, structs.AllocClientStatusFailed, structs.AllocClientStatusLost:
		ar.logger.Trace("alloc terminal; updating server and not running", "status", clientStatus)
		return false
	}

	return true
}

// runTasks is used to run the task runners and block until they exit.
func (ar *allocRunner) runTasks() {
	for _, task := range ar.tasks {
		go task.Run()
	}

	for _, task := range ar.tasks {
		<-task.WaitCh()
	}
}

// Alloc returns the current allocation being run by this runner as sent by the
// server. This view of the allocation does not have updated task states.
func (ar *allocRunner) Alloc() *structs.Allocation {
	ar.allocLock.RLock()
	defer ar.allocLock.RUnlock()
	return ar.alloc
}

func (ar *allocRunner) setAlloc(updated *structs.Allocation) {
	ar.allocLock.Lock()
	ar.alloc = updated
	ar.allocLock.Unlock()
}

// GetAllocDir returns the alloc dir which is safe for concurrent use.
func (ar *allocRunner) GetAllocDir() *allocdir.AllocDir {
	return ar.allocDir
}

// Restore state from database. Must be called after NewAllocRunner but before
// Run.
func (ar *allocRunner) Restore() error {
	// Retrieve deployment status to avoid reseting it across agent
	// restarts. Once a deployment status is set Nomad no longer monitors
	// alloc health, so we must persist deployment state across restarts.
	ds, err := ar.stateDB.GetDeploymentStatus(ar.id)
	if err != nil {
		return err
	}

	ar.stateLock.Lock()
	ar.state.DeploymentStatus = ds
	ar.stateLock.Unlock()

	// Restore task runners
	for _, tr := range ar.tasks {
		if err := tr.Restore(); err != nil {
			return err
		}
	}

	return nil
}

// persistDeploymentStatus stores AllocDeploymentStatus.
func (ar *allocRunner) persistDeploymentStatus(ds *structs.AllocDeploymentStatus) {
	if err := ar.stateDB.PutDeploymentStatus(ar.id, ds); err != nil {
		// While any persistence errors are very bad, the worst case
		// scenario for failing to persist deployment status is that if
		// the agent is restarted it will monitor the deployment status
		// again. This could cause a deployment's status to change when
		// that shouldn't happen. However, allowing that seems better
		// than failing the entire allocation.
		ar.logger.Error("error storing deployment status", "error", err)
	}
}

// TaskStateUpdated is called by TaskRunner when a task's state has been
// updated. It does not process the update synchronously but instead notifies a
// goroutine the state has change. Since processing the state change may cause
// the task to be killed (thus change its state again) it cannot be done
// synchronously as it would cause a deadlock due to reentrancy.
//
// The goroutine is used to compute changes to the alloc's ClientStatus and to
// update the server with the new state.
func (ar *allocRunner) TaskStateUpdated() {
	select {
	case ar.taskStateUpdatedCh <- struct{}{}:
	default:
		// already pending updates
	}
}

// handleTaskStateUpdates must be run in goroutine as it monitors
// taskStateUpdatedCh for task state update notifications and processes task
// states.
//
// Processing task state updates must be done in a goroutine as it may have to
// kill tasks which causes further task state updates.
func (ar *allocRunner) handleTaskStateUpdates() {
	defer close(ar.taskStateUpdateHandlerCh)

	for done := false; !done; {
		select {
		case <-ar.taskStateUpdatedCh:
		case <-ar.waitCh:
			// Run has exited, sync once more to ensure final
			// states are collected.
			done = true
		}

		ar.logger.Trace("handling task state update", "done", done)

		// Set with the appropriate event if task runners should be
		// killed.
		var killEvent *structs.TaskEvent

		// If task runners should be killed, this is set to the task
		// name whose fault it is.
		killTask := ""

		// True if task runners should be killed because a leader
		// failed (informational).
		leaderFailed := false

		// Task state has been updated; gather the state of the other tasks
		trNum := len(ar.tasks)
		liveRunners := make([]*taskrunner.TaskRunner, 0, trNum)
		states := make(map[string]*structs.TaskState, trNum)

		for name, tr := range ar.tasks {
			state := tr.TaskState()
			states[name] = state

			// Capture live task runners in case we need to kill them
			if state.State != structs.TaskStateDead {
				liveRunners = append(liveRunners, tr)
				continue
			}

			// Task is dead, determine if other tasks should be killed
			if state.Failed {
				// Only set failed event if no event has been
				// set yet to give dead leaders priority.
				if killEvent == nil {
					killTask = name
					killEvent = structs.NewTaskEvent(structs.TaskSiblingFailed).
						SetFailedSibling(name)
				}
			} else if tr.IsLeader() {
				killEvent = structs.NewTaskEvent(structs.TaskLeaderDead)
				leaderFailed = true
				killTask = name
			}
		}

		// If there's a kill event set and live runners, kill them
		if killEvent != nil && len(liveRunners) > 0 {

			// Log kill reason
			if leaderFailed {
				ar.logger.Debug("leader task dead, destroying all tasks", "leader_task", killTask)
			} else {
				ar.logger.Debug("task failure, destroying all tasks", "failed_task", killTask)
			}

			// Emit kill event for live runners
			for _, tr := range liveRunners {
				tr.EmitEvent(killEvent)
			}

			// Kill 'em all
			states = ar.killTasks()

			// Wait for TaskRunners to exit before continuing to
			// prevent looping before TaskRunners have transitioned
			// to Dead.
			for _, tr := range liveRunners {
				select {
				case <-tr.WaitCh():
				case <-ar.waitCh:
				}
			}
		}

		// Get the client allocation
		calloc := ar.clientAlloc(states)

		// Update the server
		ar.stateUpdater.AllocStateUpdated(calloc)

		// Broadcast client alloc to listeners
		ar.allocBroadcaster.Send(calloc)
	}
}

// killTasks kills all task runners, leader (if there is one) first. Errors are
// logged except taskrunner.ErrTaskNotRunning which is ignored. Task states
// after Kill has been called are returned.
func (ar *allocRunner) killTasks() map[string]*structs.TaskState {
	var mu sync.Mutex
	states := make(map[string]*structs.TaskState, len(ar.tasks))

	// Kill leader first, synchronously
	for name, tr := range ar.tasks {
		if !tr.IsLeader() {
			continue
		}

		err := tr.Kill(context.TODO(), structs.NewTaskEvent(structs.TaskKilling))
		if err != nil && err != taskrunner.ErrTaskNotRunning {
			ar.logger.Warn("error stopping leader task", "error", err, "task_name", name)
		}

		state := tr.TaskState()
		states[name] = state
		break
	}

	// Kill the rest concurrently
	wg := sync.WaitGroup{}
	for name, tr := range ar.tasks {
		if tr.IsLeader() {
			continue
		}

		wg.Add(1)
		go func(name string, tr *taskrunner.TaskRunner) {
			defer wg.Done()
			err := tr.Kill(context.TODO(), structs.NewTaskEvent(structs.TaskKilling))
			if err != nil && err != taskrunner.ErrTaskNotRunning {
				ar.logger.Warn("error stopping task", "error", err, "task_name", name)
			}

			state := tr.TaskState()
			mu.Lock()
			states[name] = state
			mu.Unlock()
		}(name, tr)
	}
	wg.Wait()

	return states
}

// clientAlloc takes in the task states and returns an Allocation populated
// with Client specific fields
func (ar *allocRunner) clientAlloc(taskStates map[string]*structs.TaskState) *structs.Allocation {
	ar.stateLock.Lock()
	defer ar.stateLock.Unlock()

	// store task states for AllocState to expose
	ar.state.TaskStates = taskStates

	a := &structs.Allocation{
		ID:         ar.id,
		TaskStates: taskStates,
	}

	if d := ar.state.DeploymentStatus; d != nil {
		a.DeploymentStatus = d.Copy()
	}

	// Compute the ClientStatus
	if ar.state.ClientStatus != "" {
		// The client status is being forced
		a.ClientStatus, a.ClientDescription = ar.state.ClientStatus, ar.state.ClientDescription
	} else {
		a.ClientStatus, a.ClientDescription = getClientStatus(taskStates)
	}

	// If the allocation is terminal, make sure all required fields are properly
	// set.
	if a.ClientTerminalStatus() {
		alloc := ar.Alloc()

		// If we are part of a deployment and the task has failed, mark the
		// alloc as unhealthy. This guards against the watcher not be started.
		if a.ClientStatus == structs.AllocClientStatusFailed &&
			alloc.DeploymentID != "" && !a.DeploymentStatus.IsUnhealthy() {
			a.DeploymentStatus = &structs.AllocDeploymentStatus{
				Healthy: helper.BoolToPtr(false),
			}
		}

		// Make sure we have marked the finished at for every task. This is used
		// to calculate the reschedule time for failed allocations.
		now := time.Now()
		for _, task := range alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks {
			ts, ok := a.TaskStates[task.Name]
			if !ok {
				ts = &structs.TaskState{}
				a.TaskStates[task.Name] = ts
			}
			if ts.FinishedAt.IsZero() {
				ts.FinishedAt = now
			}
		}
	}

	return a
}

// getClientStatus takes in the task states for a given allocation and computes
// the client status and description
func getClientStatus(taskStates map[string]*structs.TaskState) (status, description string) {
	var pending, running, dead, failed bool
	for _, state := range taskStates {
		switch state.State {
		case structs.TaskStateRunning:
			running = true
		case structs.TaskStatePending:
			pending = true
		case structs.TaskStateDead:
			if state.Failed {
				failed = true
			} else {
				dead = true
			}
		}
	}

	// Determine the alloc status
	if failed {
		return structs.AllocClientStatusFailed, "Failed tasks"
	} else if running {
		return structs.AllocClientStatusRunning, "Tasks are running"
	} else if pending {
		return structs.AllocClientStatusPending, "No tasks have started"
	} else if dead {
		return structs.AllocClientStatusComplete, "All tasks have completed"
	}

	return "", ""
}

// SetClientStatus is a helper for forcing a specific client
// status on the alloc runner. This is used during restore errors
// when the task state can't be restored.
func (ar *allocRunner) SetClientStatus(clientStatus string) {
	ar.stateLock.Lock()
	defer ar.stateLock.Unlock()
	ar.state.ClientStatus = clientStatus
}

// AllocState returns a copy of allocation state including a snapshot of task
// states.
func (ar *allocRunner) AllocState() *state.State {
	ar.stateLock.RLock()
	state := ar.state.Copy()
	ar.stateLock.RUnlock()

	// If TaskStateUpdated has not been called yet, ar.state.TaskStates
	// won't be set as it is not the canonical source of TaskStates.
	if len(state.TaskStates) == 0 {
		ar.state.TaskStates = make(map[string]*structs.TaskState, len(ar.tasks))
		for k, tr := range ar.tasks {
			state.TaskStates[k] = tr.TaskState()
		}
	}

	// Generate alloc to get other state fields
	alloc := ar.clientAlloc(state.TaskStates)
	state.ClientStatus = alloc.ClientStatus
	state.ClientDescription = alloc.ClientDescription
	state.DeploymentStatus = alloc.DeploymentStatus

	return state
}

// Update asyncronously updates the running allocation with a new version
// received from the server.
// When processing a new update, we will first attempt to drain stale updates
// from the queue, before appending the new one.
func (ar *allocRunner) Update(update *structs.Allocation) {
	select {
	// Drain queued update from the channel if possible, and check the modify
	// index
	case oldUpdate := <-ar.allocUpdatedCh:
		// If the old update is newer than the replacement, then skip the new one
		// and return. This case shouldn't happen, but may in the case of a bug
		// elsewhere inside the system.
		if oldUpdate.AllocModifyIndex > update.AllocModifyIndex {
			ar.logger.Debug("Discarding allocation update due to newer alloc revision in queue",
				"old_modify_index", oldUpdate.AllocModifyIndex,
				"new_modify_index", update.AllocModifyIndex)
			ar.allocUpdatedCh <- oldUpdate
			return
		} else {
			ar.logger.Debug("Discarding allocation update",
				"skipped_modify_index", oldUpdate.AllocModifyIndex,
				"new_modify_index", update.AllocModifyIndex)
		}
	case <-ar.waitCh:
		ar.logger.Trace("AllocRunner has terminated, skipping alloc update",
			"modify_index", update.AllocModifyIndex)
		return
	default:
	}

	// Queue the new update
	ar.allocUpdatedCh <- update
}

func (ar *allocRunner) handleAllocUpdates() {
	for {
		select {
		case update := <-ar.allocUpdatedCh:
			ar.handleAllocUpdate(update)
		case <-ar.waitCh:
			return
		}
	}
}

// This method sends the updated alloc to Run for serially processing updates.
// If there is already a pending update it will be discarded and replaced by
// the latest update.
func (ar *allocRunner) handleAllocUpdate(update *structs.Allocation) {
	// Detect Stop updates
	stopping := !ar.Alloc().TerminalStatus() && update.TerminalStatus()

	// Update ar.alloc
	ar.setAlloc(update)

	// Run update hooks if not stopping or dead
	if !update.TerminalStatus() {
		if err := ar.update(update); err != nil {
			ar.logger.Error("error running update hooks", "error", err)
		}

	}

	// Update task runners
	for _, tr := range ar.tasks {
		tr.Update(update)
	}

	// If alloc is being terminated, kill all tasks, leader first
	if stopping {
		ar.killTasks()
	}

}

func (ar *allocRunner) Listener() *cstructs.AllocListener {
	return ar.allocBroadcaster.Listen()
}

func (ar *allocRunner) destroyImpl() {
	// Stop any running tasks and persist states in case the client is
	// shutdown before Destroy finishes.
	states := ar.killTasks()
	calloc := ar.clientAlloc(states)
	ar.stateUpdater.AllocStateUpdated(calloc)

	// Wait for tasks to exit and postrun hooks to finish
	<-ar.waitCh

	// Run destroy hooks
	if err := ar.destroy(); err != nil {
		ar.logger.Warn("error running destroy hooks", "error", err)
	}

	// Wait for task state update handler to exit before removing local
	// state if Run() ran at all.
	<-ar.taskStateUpdateHandlerCh

	// Cleanup state db
	if err := ar.stateDB.DeleteAllocationBucket(ar.id); err != nil {
		ar.logger.Warn("failed to delete allocation state", "error", err)
	}

	// Mark alloc as destroyed
	ar.destroyedLock.Lock()

	if !ar.shutdown {
		ar.shutdown = true
		close(ar.shutdownCh)
	}

	ar.destroyed = true
	close(ar.destroyCh)

	ar.destroyedLock.Unlock()
}

// Destroy the alloc runner by stopping it if it is still running and cleaning
// up all of its resources.
//
// This method is safe for calling concurrently with Run() and will cause it to
// exit (thus closing WaitCh).
// When the destroy action is completed, it will close DestroyCh().
func (ar *allocRunner) Destroy() {
	ar.destroyedLock.Lock()
	defer ar.destroyedLock.Unlock()

	if ar.destroyed {
		// Only destroy once
		return
	}

	if ar.destroyLaunched {
		// Only dispatch a destroy once
		return
	}

	ar.destroyLaunched = true

	// Synchronize calls to shutdown/destroy
	if ar.shutdownLaunched {
		go func() {
			ar.logger.Debug("Waiting for shutdown before destroying runner")
			<-ar.shutdownCh
			ar.destroyImpl()
		}()

		return
	}

	go ar.destroyImpl()
}

// IsDestroyed returns true if the alloc runner has been destroyed (stopped and
// garbage collected).
//
// This method is safe for calling concurrently with Run(). Callers must
// receive on WaitCh() to block until alloc runner has stopped and been
// destroyed.
func (ar *allocRunner) IsDestroyed() bool {
	ar.destroyedLock.Lock()
	defer ar.destroyedLock.Unlock()
	return ar.destroyed
}

// IsWaiting returns true if the alloc runner is waiting for its previous
// allocation to terminate.
//
// This method is safe for calling concurrently with Run().
func (ar *allocRunner) IsWaiting() bool {
	return ar.prevAllocWatcher.IsWaiting()
}

// DestroyCh is a channel that is closed when an allocrunner is closed due to
// an explicit call to Destroy().
func (ar *allocRunner) DestroyCh() <-chan struct{} {
	return ar.destroyCh
}

// ShutdownCh is a channel that is closed when an allocrunner is closed due to
// either an explicit call to Shutdown(), or Destroy().
func (ar *allocRunner) ShutdownCh() <-chan struct{} {
	return ar.shutdownCh
}

// Shutdown AllocRunner gracefully. Asynchronously shuts down all TaskRunners.
// Tasks are unaffected and may be restored.
// When the destroy action is completed, it will close ShutdownCh().
func (ar *allocRunner) Shutdown() {
	ar.destroyedLock.Lock()
	defer ar.destroyedLock.Unlock()

	// Destroy is a superset of Shutdown so there's nothing to do if this
	// has already been destroyed.
	if ar.destroyed {
		return
	}

	// Destroy is a superset of Shutdown so if it's been marked for destruction,
	// don't try and shutdown in parallel. If shutdown has been launched, don't
	// try again.
	if ar.destroyLaunched || ar.shutdownLaunched {
		return
	}

	ar.shutdownLaunched = true

	go func() {
		ar.logger.Trace("shutting down")

		// Shutdown tasks gracefully if they were run
		if ar.runnersLaunched {
			wg := sync.WaitGroup{}
			for _, tr := range ar.tasks {
				wg.Add(1)
				go func(tr *taskrunner.TaskRunner) {
					tr.Shutdown()
					wg.Done()
				}(tr)
			}
			wg.Wait()
		}

		// Wait for Run to exit
		<-ar.waitCh

		// Run shutdown hooks
		ar.shutdownHooks()

		// Wait for updater to finish its final run
		<-ar.taskStateUpdateHandlerCh

		ar.destroyedLock.Lock()
		ar.shutdown = true
		close(ar.shutdownCh)
		ar.destroyedLock.Unlock()
	}()
}

// IsMigrating returns true if the alloc runner is migrating data from its
// previous allocation.
//
// This method is safe for calling concurrently with Run().
func (ar *allocRunner) IsMigrating() bool {
	return ar.prevAllocMigrator.IsMigrating()
}

func (ar *allocRunner) StatsReporter() interfaces.AllocStatsReporter {
	return ar
}

// LatestAllocStats returns the latest stats for an allocation. If taskFilter
// is set, only stats for that task -- if it exists -- are returned.
func (ar *allocRunner) LatestAllocStats(taskFilter string) (*cstructs.AllocResourceUsage, error) {
	astat := &cstructs.AllocResourceUsage{
		Tasks: make(map[string]*cstructs.TaskResourceUsage, len(ar.tasks)),
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: &cstructs.MemoryStats{},
			CpuStats:    &cstructs.CpuStats{},
			DeviceStats: []*device.DeviceGroupStats{},
		},
	}

	for name, tr := range ar.tasks {
		if taskFilter != "" && taskFilter != name {
			// Getting stats for a particular task and its not this one!
			continue
		}

		if usage := tr.LatestResourceUsage(); usage != nil {
			astat.Tasks[name] = usage
			astat.ResourceUsage.Add(usage.ResourceUsage)
			if usage.Timestamp > astat.Timestamp {
				astat.Timestamp = usage.Timestamp
			}
		}
	}

	return astat, nil
}

func (ar *allocRunner) GetTaskEventHandler(taskName string) drivermanager.EventHandler {
	if tr, ok := ar.tasks[taskName]; ok {
		return func(ev *drivers.TaskEvent) {
			tr.EmitEvent(&structs.TaskEvent{
				Type:          structs.TaskDriverMessage,
				Time:          ev.Timestamp.UnixNano(),
				Details:       ev.Annotations,
				DriverMessage: ev.Message,
			})
		}
	}
	return nil
}
