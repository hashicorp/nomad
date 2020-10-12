package allocrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/config"
	cinterfaces "github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
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

	// waitCh is closed when the Run loop has exited
	waitCh chan struct{}

	// destroyed is true when the Run loop has exited, postrun hooks have
	// run, and alloc runner has been destroyed. Must acquire destroyedLock
	// to access.
	destroyed bool

	// destroyCh is closed when the Run loop has exited, postrun hooks have
	// run, and alloc runner has been destroyed.
	destroyCh chan struct{}

	// shutdown is true when the Run loop has exited, and shutdown hooks have
	// run. Must acquire destroyedLock to access.
	shutdown bool

	// shutdownCh is closed when the Run loop has exited, and shutdown hooks
	// have run.
	shutdownCh chan struct{}

	// destroyLaunched is true if Destroy has been called. Must acquire
	// destroyedLock to access.
	destroyLaunched bool

	// shutdownLaunched is true if Shutdown has been called. Must acquire
	// destroyedLock to access.
	shutdownLaunched bool

	// destroyedLock guards destroyed, destroyLaunched, shutdownLaunched,
	// and serializes Shutdown/Destroy calls.
	destroyedLock sync.Mutex

	// Alloc captures the allocation being run.
	alloc     *structs.Allocation
	allocLock sync.RWMutex

	// state is the alloc runner's state
	state     *state.State
	stateLock sync.RWMutex

	stateDB cstate.StateDB

	// deviceStatsReporter is used to lookup resource usage for alloc devices
	deviceStatsReporter cinterfaces.DeviceStatsReporter

	// allocBroadcaster sends client allocation updates to all listeners
	allocBroadcaster *cstructs.AllocBroadcaster

	// rpcClient is the RPC Client that should be used by the allocrunner and its
	// hooks to communicate with Nomad Servers.
	rpcClient RPCer
}

// RPCer is the interface needed by hooks to make RPC calls.
type RPCer interface {
	RPC(method string, args interface{}, reply interface{}) error
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
		rpcClient:                config.RPCClient,
	}

	// Create the logger based on the allocation ID
	ar.logger = config.Logger.Named("alloc_runner").With("alloc_id", alloc.ID)

	// Create alloc broadcaster
	ar.allocBroadcaster = cstructs.NewAllocBroadcaster(ar.logger)

	return ar, nil
}

func (ar *allocRunner) WaitCh() <-chan struct{} {
	return ar.waitCh
}

// Run the AllocRunner. Starts tasks if the alloc is non-terminal and closes
// WaitCh when it exits. Should be started in a goroutine.
func (ar *allocRunner) Run() {
	// Close the wait channel on return
	defer close(ar.waitCh)

	for {

		ar.allocLock.Lock()
		if ar.alloc.ServerTerminalStatus() {
			// Wait to exit until Destroy or Shutdown are called
			ar.allocLock.Unlock()
			goto DESTROY
		}

		// If we're not dead, then we're alive!
		ar.alloc.ClientStatus = structs.AllocClientStatusRunning
		ar.allocLock.Unlock()

		// Update server
		ar.sendUpdatedAlloc()

		select {
		case newAlloc := <-ar.allocUpdatedCh:
			// New alloc, set and loop
			ar.alloc = newAlloc
		case <-ar.destroyCh:
			ar.logger.Warn("Run exiting: destroyed while running")
			goto DESTROY
		case <-ar.shutdownCh:
			ar.logger.Info("Run exiting: shutdown while running")
			goto DESTROY

		}
	}

DESTROY:
	if !ar.alloc.ClientTerminalStatus() {
		alloc := ar.Alloc()
		alloc.ClientStatus = structs.AllocClientStatusComplete
		alloc.ClientDescription = "told to stop by scheduler"
		ar.setAlloc(alloc)
	}

	select {
	case <-ar.destroyCh:
		ar.logger.Info("Run exiting: destroyed")
		return
	case <-ar.shutdownCh:
		ar.logger.Info("Run exiting: shutdown")
		return
	}
}

// Alloc returns the current allocation being run by this runner as sent by the
// server. This view of the allocation does not have updated task states.
func (ar *allocRunner) Alloc() *structs.Allocation {
	ar.allocLock.RLock()
	defer ar.allocLock.RUnlock()
	return ar.alloc
}

func (ar *allocRunner) AllocState() *state.State {
	ar.allocLock.RLock()
	defer ar.allocLock.RUnlock()
	return &state.State{
		ClientStatus:      ar.alloc.ClientStatus,
		ClientDescription: ar.alloc.ClientDescription,
		DeploymentStatus:  ar.alloc.DeploymentStatus,
		TaskStates:        ar.alloc.TaskStates,
	}
}

func (ar *allocRunner) setAlloc(updated *structs.Allocation) {
	ar.allocLock.Lock()
	ar.alloc = updated
	ar.allocLock.Unlock()
}

// Restore state from database. Must be called after NewAllocRunner but before
// Run.
func (ar *allocRunner) Restore() error {
	return nil
}

// sendUpdatedAlloc sends the alloc to the server.
func (ar *allocRunner) sendUpdatedAlloc() {
	alloc := ar.Alloc()
	if alloc.DeploymentStatus == nil {
		healthy := true
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy:   &healthy,
			Timestamp: time.Now(),
		}
	}

	tg := alloc.Job.LookupTaskGroup(ar.alloc.TaskGroup)
	if tg != nil {
		// Set task states
		taskState := &structs.TaskState{
			State:     structs.TaskStateRunning,
			StartedAt: time.Now(),
		}
		if ar.alloc.TerminalStatus() {
			taskState.State = structs.TaskStateDead
			taskState.FinishedAt = time.Now()
		}

		if alloc.TaskStates == nil {
			alloc.TaskStates = make(map[string]*structs.TaskState)
		}

		for _, task := range tg.Tasks {
			if s, ok := alloc.TaskStates[task.Name]; ok {
				if s.State == taskState.State {
					// Noop, skip
					continue
				}
			}
			alloc.TaskStates[task.Name] = taskState.Copy()
		}
	}

	ar.setAlloc(alloc)
	ar.stateUpdater.AllocStateUpdated(alloc)
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

func (ar *allocRunner) Listener() *cstructs.AllocListener {
	return ar.allocBroadcaster.Listen()
}

func (ar *allocRunner) destroyImpl() {
	ar.destroyedLock.Lock()
	defer ar.destroyedLock.Unlock()

	ar.destroyed = true
	close(ar.destroyCh)

	// Wait for tasks to exit and postrun hooks to finish
	<-ar.waitCh

	// Cleanup state db; while holding the lock to avoid
	// a race periodic PersistState that may resurrect the alloc
	if err := ar.stateDB.DeleteAllocationBucket(ar.id); err != nil {
		ar.logger.Warn("failed to delete allocation state", "error", err)
	}

	if !ar.shutdown {
		ar.shutdown = true
		close(ar.shutdownCh)
	}
}

func (ar *allocRunner) PersistState() error {
	ar.destroyedLock.Lock()
	defer ar.destroyedLock.Unlock()

	if ar.destroyed {
		err := ar.stateDB.DeleteAllocationBucket(ar.id)
		if err != nil {
			ar.logger.Warn("failed to delete allocation bucket", "error", err)
		}
		return nil
	}

	// TODO: consider persisting deployment state along with task status.
	// While we study why only the alloc is persisted, I opted to maintain current
	// behavior and not risk adding yet more IO calls unnecessarily.
	return ar.stateDB.PutAllocation(ar.Alloc())
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
	return false
}

// isShuttingDown returns true if the alloc runner is in a shutdown state
// due to a call to Shutdown() or Destroy()
func (ar *allocRunner) isShuttingDown() bool {
	ar.destroyedLock.Lock()
	defer ar.destroyedLock.Unlock()
	return ar.shutdownLaunched
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
	ar.shutdown = true
	close(ar.shutdownCh)

	// Wait for Run to exit
	<-ar.waitCh
}

// IsMigrating returns true if the alloc runner is migrating data from its
// previous allocation.
//
// This method is safe for calling concurrently with Run().
func (ar *allocRunner) IsMigrating() bool {
	return false
}

func (ar *allocRunner) StatsReporter() interfaces.AllocStatsReporter {
	return ar
}

// LatestAllocStats returns the latest stats for an allocation. If taskFilter
// is set, only stats for that task -- if it exists -- are returned.
func (ar *allocRunner) LatestAllocStats(taskFilter string) (*cstructs.AllocResourceUsage, error) {
	astat := &cstructs.AllocResourceUsage{
		Tasks: make(map[string]*cstructs.TaskResourceUsage),
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: &cstructs.MemoryStats{},
			CpuStats:    &cstructs.CpuStats{},
			DeviceStats: []*device.DeviceGroupStats{},
		},
	}

	return astat, nil
}

func (*allocRunner) RestartTask(taskName string, taskEvent *structs.TaskEvent) error { return nil }
func (*allocRunner) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	return nil
}
func (*allocRunner) RestartAll(taskEvent *structs.TaskEvent) error { return nil }
func (ar *allocRunner) Signal(taskName, signal string) error       { return nil }

func (ar *allocRunner) GetTaskExecHandler(taskName string) drivermanager.TaskExecHandler {
	return nil
}

func (ar *allocRunner) GetTaskDriverCapabilities(taskName string) (*drivers.Capabilities, error) {
	return &drivers.Capabilities{
		SendSignals:         false,
		Exec:                false,
		FSIsolation:         drivers.FSIsolationNone,
		NetIsolationModes:   []drivers.NetIsolationMode{drivers.NetIsolationModeNone},
		MustInitiateNetwork: false,
		MountConfigs:        drivers.MountConfigSupportNone,
	}, nil
}

func (*allocRunner) GetAllocDir() *allocdir.AllocDir                                { return nil }
func (*allocRunner) GetTaskEventHandler(taskName string) drivermanager.EventHandler { return nil }
