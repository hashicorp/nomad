package client

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"

	cstructs "github.com/hashicorp/nomad/client/structs"
)

const (
	// taskReceivedSyncLimit is how long the client will wait before sending
	// that a task was received to the server. The client does not immediately
	// send that the task was received to the server because another transition
	// to running or failed is likely to occur immediately after and a single
	// update will transfer all past state information. If not other transition
	// has occurred up to this limit, we will send to the server.
	taskReceivedSyncLimit = 30 * time.Second
)

var (
	// The following are the key paths written to the state database
	allocRunnerStateAllocKey     = []byte("alloc")
	allocRunnerStateImmutableKey = []byte("immutable")
	allocRunnerStateMutableKey   = []byte("mutable")
	allocRunnerStateAllocDirKey  = []byte("alloc-dir")
)

// AllocStateUpdater is used to update the status of an allocation
type AllocStateUpdater func(alloc *structs.Allocation)

type AllocStatsReporter interface {
	LatestAllocStats(taskFilter string) (*cstructs.AllocResourceUsage, error)
}

// AllocRunner is used to wrap an allocation and provide the execution context.
type AllocRunner struct {
	config  *config.Config
	updater AllocStateUpdater
	logger  *log.Logger

	// allocID is the ID of this runner's allocation. Since it does not
	// change for the lifetime of the AllocRunner it is safe to read
	// without acquiring a lock (unlike alloc).
	allocID string

	alloc                  *structs.Allocation
	allocClientStatus      string // Explicit status of allocation. Set when there are failures
	allocClientDescription string
	allocHealth            *bool // Whether the allocation is healthy
	allocBroadcast         *cstructs.AllocBroadcaster
	allocLock              sync.Mutex

	dirtyCh chan struct{}

	allocDir     *allocdir.AllocDir
	allocDirLock sync.Mutex

	tasks      map[string]*TaskRunner
	taskStates map[string]*structs.TaskState
	restored   map[string]struct{}
	taskLock   sync.RWMutex

	taskStatusLock sync.RWMutex

	updateCh chan *structs.Allocation

	vaultClient  vaultclient.VaultClient
	consulClient ConsulServiceAPI

	otherAllocDir *allocdir.AllocDir

	ctx    context.Context
	exitFn context.CancelFunc
	waitCh chan struct{}

	// State related fields
	// stateDB is used to store the alloc runners state
	stateDB        *bolt.DB
	allocStateLock sync.Mutex

	// persistedEval is the last persisted evaluation ID. Since evaluation
	// IDs change on every allocation update we only need to persist the
	// allocation when its eval ID != the last persisted eval ID.
	persistedEvalLock sync.Mutex
	persistedEval     string

	// immutablePersisted and allocDirPersisted are used to track whether the
	// immutable data and the alloc dir have been persisted. Once persisted we
	// can lower write volume by not re-writing these values
	immutablePersisted bool
	allocDirPersisted  bool
}

// COMPAT: Remove in 0.7.0
// allocRunnerState is used to snapshot the state of the alloc runner
type allocRunnerState struct {
	Version                string
	Alloc                  *structs.Allocation
	AllocDir               *allocdir.AllocDir
	AllocClientStatus      string
	AllocClientDescription string

	// COMPAT: Remove in 0.7.0: removing will break upgrading directly from
	//         0.5.2, so don't remove in the 0.6 series.
	// Context is deprecated and only used to migrate from older releases.
	// It will be removed in the future.
	Context *struct {
		AllocID  string // unused; included for completeness
		AllocDir struct {
			AllocDir  string
			SharedDir string // unused; included for completeness
			TaskDirs  map[string]string
		}
	} `json:"Context,omitempty"`
}

// allocRunnerAllocState is state that only has to be written when the alloc
// changes.
type allocRunnerAllocState struct {
	Alloc *structs.Allocation
}

// allocRunnerImmutableState is state that only has to be written once.
type allocRunnerImmutableState struct {
	Version string
}

// allocRunnerMutableState is state that has to be written on each save as it
// changes over the life-cycle of the alloc_runner.
type allocRunnerMutableState struct {
	AllocClientStatus      string
	AllocClientDescription string
	TaskStates             map[string]*structs.TaskState
	DeploymentStatus       *structs.AllocDeploymentStatus
}

// NewAllocRunner is used to create a new allocation context
func NewAllocRunner(logger *log.Logger, config *config.Config, stateDB *bolt.DB, updater AllocStateUpdater,
	alloc *structs.Allocation, vaultClient vaultclient.VaultClient,
	consulClient ConsulServiceAPI) *AllocRunner {

	ar := &AllocRunner{
		config:         config,
		stateDB:        stateDB,
		updater:        updater,
		logger:         logger,
		alloc:          alloc,
		allocID:        alloc.ID,
		allocBroadcast: cstructs.NewAllocBroadcaster(8),
		dirtyCh:        make(chan struct{}, 1),
		allocDir:       allocdir.NewAllocDir(logger, filepath.Join(config.AllocDir, alloc.ID)),
		tasks:          make(map[string]*TaskRunner),
		taskStates:     copyTaskStates(alloc.TaskStates),
		restored:       make(map[string]struct{}),
		updateCh:       make(chan *structs.Allocation, 64),
		waitCh:         make(chan struct{}),
		vaultClient:    vaultClient,
		consulClient:   consulClient,
	}

	// TODO Should be passed a context
	ar.ctx, ar.exitFn = context.WithCancel(context.TODO())
	return ar
}

// pre060StateFilePath returns the path to our state file that would have been
// written pre v0.6.0
// COMPAT: Remove in 0.7.0
func (r *AllocRunner) pre060StateFilePath() string {
	r.allocLock.Lock()
	defer r.allocLock.Unlock()
	path := filepath.Join(r.config.StateDir, "alloc", r.allocID, "state.json")
	return path
}

// RestoreState is used to restore the state of the alloc runner
func (r *AllocRunner) RestoreState() error {

	// COMPAT: Remove in 0.7.0
	// Check if the old snapshot is there
	oldPath := r.pre060StateFilePath()
	var snap allocRunnerState
	var upgrading bool
	if err := pre060RestoreState(oldPath, &snap); err == nil {
		// Restore fields
		r.logger.Printf("[INFO] client: restoring pre v0.6.0 alloc runner state for alloc %q", r.allocID)
		r.alloc = snap.Alloc
		r.allocDir = snap.AllocDir
		r.allocClientStatus = snap.AllocClientStatus
		r.allocClientDescription = snap.AllocClientDescription

		if r.alloc != nil {
			r.taskStates = snap.Alloc.TaskStates
		}

		// COMPAT: Remove in 0.7.0
		// #2132 Upgrade path: if snap.AllocDir is nil, try to convert old
		// Context struct to new AllocDir struct
		if snap.AllocDir == nil && snap.Context != nil {
			r.logger.Printf("[DEBUG] client: migrating state snapshot for alloc %q", r.allocID)
			r.allocDir = allocdir.NewAllocDir(r.logger, snap.Context.AllocDir.AllocDir)
			for taskName := range snap.Context.AllocDir.TaskDirs {
				r.allocDir.NewTaskDir(taskName)
			}
		}

		// Delete the old state
		os.RemoveAll(oldPath)
		upgrading = true
	} else if !os.IsNotExist(err) {
		// Something corrupt in the old state file
		return err
	} else {
		// We are doing a normal restore
		err := r.stateDB.View(func(tx *bolt.Tx) error {
			bkt, err := getAllocationBucket(tx, r.allocID)
			if err != nil {
				return fmt.Errorf("failed to get allocation bucket: %v", err)
			}

			// Get the state objects
			var mutable allocRunnerMutableState
			var immutable allocRunnerImmutableState
			var allocState allocRunnerAllocState
			var allocDir allocdir.AllocDir

			if err := getObject(bkt, allocRunnerStateAllocKey, &allocState); err != nil {
				return fmt.Errorf("failed to read alloc runner alloc state: %v", err)
			}
			if err := getObject(bkt, allocRunnerStateImmutableKey, &immutable); err != nil {
				return fmt.Errorf("failed to read alloc runner immutable state: %v", err)
			}
			if err := getObject(bkt, allocRunnerStateMutableKey, &mutable); err != nil {
				return fmt.Errorf("failed to read alloc runner mutable state: %v", err)
			}
			if err := getObject(bkt, allocRunnerStateAllocDirKey, &allocDir); err != nil {
				return fmt.Errorf("failed to read alloc runner alloc_dir state: %v", err)
			}

			// Populate the fields
			r.alloc = allocState.Alloc
			r.allocDir = &allocDir
			r.allocClientStatus = mutable.AllocClientStatus
			r.allocClientDescription = mutable.AllocClientDescription
			r.taskStates = mutable.TaskStates
			r.alloc.ClientStatus = getClientStatus(r.taskStates)
			r.alloc.DeploymentStatus = mutable.DeploymentStatus
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to read allocation state: %v", err)
		}
	}

	var snapshotErrors multierror.Error
	if r.alloc == nil {
		snapshotErrors.Errors = append(snapshotErrors.Errors, fmt.Errorf("alloc_runner snapshot includes a nil allocation"))
	}
	if r.allocDir == nil {
		snapshotErrors.Errors = append(snapshotErrors.Errors, fmt.Errorf("alloc_runner snapshot includes a nil alloc dir"))
	}
	if e := snapshotErrors.ErrorOrNil(); e != nil {
		return e
	}

	tg := r.alloc.Job.LookupTaskGroup(r.alloc.TaskGroup)
	if tg == nil {
		return fmt.Errorf("restored allocation doesn't contain task group %q", r.alloc.TaskGroup)
	}

	// Restore the task runners
	taskDestroyEvent := structs.NewTaskEvent(structs.TaskKilled)
	var mErr multierror.Error
	for _, task := range tg.Tasks {
		name := task.Name
		state := r.taskStates[name]

		// Nomad exited before task could start, nothing to restore.
		// AllocRunner.Run will start a new TaskRunner for this task
		if state == nil {
			continue
		}

		// Mark the task as restored.
		r.restored[name] = struct{}{}

		td, ok := r.allocDir.TaskDirs[name]
		if !ok {
			// Create the task dir metadata if it doesn't exist.
			// Since task dirs are created during r.Run() the
			// client may save state and exit before all task dirs
			// are created
			td = r.allocDir.NewTaskDir(name)
		}

		// Skip tasks in terminal states.
		if state.State == structs.TaskStateDead {
			continue
		}

		tr := NewTaskRunner(r.logger, r.config, r.stateDB, r.setTaskState, td, r.Alloc(), task, r.vaultClient, r.consulClient)
		r.tasks[name] = tr

		if restartReason, err := tr.RestoreState(); err != nil {
			r.logger.Printf("[ERR] client: failed to restore state for alloc %s task %q: %v", r.allocID, name, err)
			mErr.Errors = append(mErr.Errors, err)
		} else if !r.alloc.TerminalStatus() {
			// Only start if the alloc isn't in a terminal status.
			go tr.Run()

			if upgrading {
				if err := tr.SaveState(); err != nil {
					r.logger.Printf("[WARN] client: initial save state for alloc %s task %s failed: %v", r.allocID, name, err)
				}
			}

			// Restart task runner if RestoreState gave a reason
			if restartReason != "" {
				r.logger.Printf("[INFO] client: restarting alloc %s task %s: %v", r.allocID, name, restartReason)
				tr.Restart("upgrade", restartReason)
			}
		} else {
			tr.Destroy(taskDestroyEvent)
		}
	}

	return mErr.ErrorOrNil()
}

// SaveState is used to snapshot the state of the alloc runner
// if the fullSync is marked as false only the state of the Alloc Runner
// is snapshotted. If fullSync is marked as true, we snapshot
// all the Task Runners associated with the Alloc
func (r *AllocRunner) SaveState() error {
	if err := r.saveAllocRunnerState(); err != nil {
		return err
	}

	// Save state for each task
	runners := r.getTaskRunners()
	var mErr multierror.Error
	for _, tr := range runners {
		if err := tr.SaveState(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("failed to save state for alloc %s task %q: %v",
				r.allocID, tr.task.Name, err))
		}
	}
	return mErr.ErrorOrNil()
}

func (r *AllocRunner) saveAllocRunnerState() error {
	r.allocStateLock.Lock()
	defer r.allocStateLock.Unlock()

	if r.ctx.Err() == context.Canceled {
		return nil
	}

	// Grab all the relevant data
	alloc := r.Alloc()

	r.allocLock.Lock()
	allocClientStatus := r.allocClientStatus
	allocClientDescription := r.allocClientDescription
	r.allocLock.Unlock()

	r.allocDirLock.Lock()
	allocDir := r.allocDir.Copy()
	r.allocDirLock.Unlock()

	// Start the transaction.
	return r.stateDB.Batch(func(tx *bolt.Tx) error {

		// Grab the allocation bucket
		allocBkt, err := getAllocationBucket(tx, r.allocID)
		if err != nil {
			return fmt.Errorf("failed to retrieve allocation bucket: %v", err)
		}

		// Write the allocation if the eval has changed
		r.persistedEvalLock.Lock()
		lastPersisted := r.persistedEval
		r.persistedEvalLock.Unlock()
		if alloc.EvalID != lastPersisted {
			allocState := &allocRunnerAllocState{
				Alloc: alloc,
			}

			if err := putObject(allocBkt, allocRunnerStateAllocKey, &allocState); err != nil {
				return fmt.Errorf("failed to write alloc_runner alloc state: %v", err)
			}

			tx.OnCommit(func() {
				r.persistedEvalLock.Lock()
				r.persistedEval = alloc.EvalID
				r.persistedEvalLock.Unlock()
			})
		}

		// Write immutable data iff it hasn't been written yet
		if !r.immutablePersisted {
			immutable := &allocRunnerImmutableState{
				Version: r.config.Version,
			}

			if err := putObject(allocBkt, allocRunnerStateImmutableKey, &immutable); err != nil {
				return fmt.Errorf("failed to write alloc_runner immutable state: %v", err)
			}

			tx.OnCommit(func() {
				r.immutablePersisted = true
			})
		}

		// Write the alloc dir data if it hasn't been written before and it exists.
		if !r.allocDirPersisted && allocDir != nil {
			if err := putObject(allocBkt, allocRunnerStateAllocDirKey, allocDir); err != nil {
				return fmt.Errorf("failed to write alloc_runner allocDir state: %v", err)
			}

			tx.OnCommit(func() {
				r.allocDirPersisted = true
			})
		}

		// Write the mutable state every time
		mutable := &allocRunnerMutableState{
			AllocClientStatus:      allocClientStatus,
			AllocClientDescription: allocClientDescription,
			TaskStates:             alloc.TaskStates,
			DeploymentStatus:       alloc.DeploymentStatus,
		}

		if err := putObject(allocBkt, allocRunnerStateMutableKey, &mutable); err != nil {
			return fmt.Errorf("failed to write alloc_runner mutable state: %v", err)
		}

		return nil
	})
}

// DestroyState is used to cleanup after ourselves
func (r *AllocRunner) DestroyState() error {
	r.allocStateLock.Lock()
	defer r.allocStateLock.Unlock()

	return r.stateDB.Update(func(tx *bolt.Tx) error {
		if err := deleteAllocationBucket(tx, r.allocID); err != nil {
			return fmt.Errorf("failed to delete allocation bucket: %v", err)
		}
		return nil
	})
}

// DestroyContext is used to destroy the context
func (r *AllocRunner) DestroyContext() error {
	return r.allocDir.Destroy()
}

// GetAllocDir returns the alloc dir for the alloc runner
func (r *AllocRunner) GetAllocDir() *allocdir.AllocDir {
	return r.allocDir
}

// copyTaskStates returns a copy of the passed task states.
func copyTaskStates(states map[string]*structs.TaskState) map[string]*structs.TaskState {
	copy := make(map[string]*structs.TaskState, len(states))
	for task, state := range states {
		copy[task] = state.Copy()
	}
	return copy
}

// Alloc returns the associated allocation
func (r *AllocRunner) Alloc() *structs.Allocation {
	r.allocLock.Lock()

	// Don't do a deep copy of the job
	alloc := r.alloc.CopySkipJob()

	// The status has explicitly been set.
	if r.allocClientStatus != "" || r.allocClientDescription != "" {
		alloc.ClientStatus = r.allocClientStatus
		alloc.ClientDescription = r.allocClientDescription

		// Copy over the task states so we don't lose them
		r.taskStatusLock.RLock()
		alloc.TaskStates = copyTaskStates(r.taskStates)
		r.taskStatusLock.RUnlock()

		r.allocLock.Unlock()
		return alloc
	}

	// The health has been set
	if r.allocHealth != nil {
		if alloc.DeploymentStatus == nil {
			alloc.DeploymentStatus = &structs.AllocDeploymentStatus{}
		}
		alloc.DeploymentStatus.Healthy = helper.BoolToPtr(*r.allocHealth)
	}
	r.allocLock.Unlock()

	// Scan the task states to determine the status of the alloc
	r.taskStatusLock.RLock()
	alloc.TaskStates = copyTaskStates(r.taskStates)
	alloc.ClientStatus = getClientStatus(r.taskStates)
	r.taskStatusLock.RUnlock()

	// If the client status is failed and we are part of a deployment, mark the
	// alloc as unhealthy. This guards against the watcher not be started.
	r.allocLock.Lock()
	if alloc.ClientStatus == structs.AllocClientStatusFailed &&
		alloc.DeploymentID != "" && !alloc.DeploymentStatus.IsUnhealthy() {
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: helper.BoolToPtr(false),
		}
	}
	r.allocLock.Unlock()

	return alloc
}

// getClientStatus takes in the task states for a given allocation and computes
// the client status
func getClientStatus(taskStates map[string]*structs.TaskState) string {
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
		return structs.AllocClientStatusFailed
	} else if running {
		return structs.AllocClientStatusRunning
	} else if pending {
		return structs.AllocClientStatusPending
	} else if dead {
		return structs.AllocClientStatusComplete
	}

	return ""
}

// dirtySyncState is used to watch for state being marked dirty to sync
func (r *AllocRunner) dirtySyncState() {
	for {
		select {
		case <-r.dirtyCh:
			if err := r.syncStatus(); err != nil {
				// Only WARN instead of ERR because we continue on
				r.logger.Printf("[WARN] client: error persisting alloc %q state: %v",
					r.allocID, err)
			}
		case <-r.ctx.Done():
			return
		}
	}
}

// syncStatus is used to run and sync the status when it changes
func (r *AllocRunner) syncStatus() error {
	// Get a copy of our alloc, update status server side and sync to disk
	alloc := r.Alloc()
	r.updater(alloc)
	r.sendBroadcast(alloc)
	return r.saveAllocRunnerState()
}

// sendBroadcast broadcasts an alloc update.
func (r *AllocRunner) sendBroadcast(alloc *structs.Allocation) {
	// Try to send the alloc up to three times with a delay to allow recovery.
	sent := false
	for i := 0; i < 3; i++ {
		if sent = r.allocBroadcast.Send(alloc); sent {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !sent {
		r.logger.Printf("[WARN] client: failed to broadcast update to allocation %q", r.allocID)
	}
}

// setStatus is used to update the allocation status
func (r *AllocRunner) setStatus(status, desc string) {
	r.allocLock.Lock()
	r.allocClientStatus = status
	r.allocClientDescription = desc
	r.allocLock.Unlock()
	select {
	case r.dirtyCh <- struct{}{}:
	default:
	}
}

// setTaskState is used to set the status of a task. If state is empty then the
// event is appended but not synced with the server. The event may be omitted
func (r *AllocRunner) setTaskState(taskName, state string, event *structs.TaskEvent) {
	r.taskStatusLock.Lock()
	defer r.taskStatusLock.Unlock()
	taskState, ok := r.taskStates[taskName]
	if !ok {
		taskState = &structs.TaskState{}
		r.taskStates[taskName] = taskState
	}

	// Set the tasks state.
	if event != nil {
		if event.FailsTask {
			taskState.Failed = true
		}
		if event.Type == structs.TaskRestarting {
			taskState.Restarts++
			taskState.LastRestart = time.Unix(0, event.Time)
		}
		r.appendTaskEvent(taskState, event)
	}

	if state == "" {
		return
	}

	switch state {
	case structs.TaskStateRunning:
		// Capture the start time if it is just starting
		if taskState.State != structs.TaskStateRunning {
			taskState.StartedAt = time.Now().UTC()
		}
	case structs.TaskStateDead:
		// Capture the finished time. If it has never started there is no finish
		// time
		if !taskState.StartedAt.IsZero() {
			taskState.FinishedAt = time.Now().UTC()
		}

		// Find all tasks that are not the one that is dead and check if the one
		// that is dead is a leader
		var otherTaskRunners []*TaskRunner
		var otherTaskNames []string
		leader := false
		for task, tr := range r.tasks {
			if task != taskName {
				otherTaskRunners = append(otherTaskRunners, tr)
				otherTaskNames = append(otherTaskNames, task)
			} else if tr.task.Leader {
				leader = true
			}
		}

		// If the task failed, we should kill all the other tasks in the task group.
		if taskState.Failed {
			for _, tr := range otherTaskRunners {
				tr.Destroy(structs.NewTaskEvent(structs.TaskSiblingFailed).SetFailedSibling(taskName))
			}
			if len(otherTaskRunners) > 0 {
				r.logger.Printf("[DEBUG] client: task %q failed, destroying other tasks in task group: %v", taskName, otherTaskNames)
			}
		} else if leader {
			// If the task was a leader task we should kill all the other tasks.
			for _, tr := range otherTaskRunners {
				tr.Destroy(structs.NewTaskEvent(structs.TaskLeaderDead))
			}
			if len(otherTaskRunners) > 0 {
				r.logger.Printf("[DEBUG] client: leader task %q is dead, destroying other tasks in task group: %v", taskName, otherTaskNames)
			}
		}
	}

	// Store the new state
	taskState.State = state

	select {
	case r.dirtyCh <- struct{}{}:
	default:
	}
}

// appendTaskEvent updates the task status by appending the new event.
func (r *AllocRunner) appendTaskEvent(state *structs.TaskState, event *structs.TaskEvent) {
	capacity := 10
	if state.Events == nil {
		state.Events = make([]*structs.TaskEvent, 0, capacity)
	}

	// If we hit capacity, then shift it.
	if len(state.Events) == capacity {
		old := state.Events
		state.Events = make([]*structs.TaskEvent, 0, capacity)
		state.Events = append(state.Events, old[1:]...)
	}

	state.Events = append(state.Events, event)
}

// Run is a long running goroutine used to manage an allocation
func (r *AllocRunner) Run() {
	defer close(r.waitCh)
	go r.dirtySyncState()

	// Find the task group to run in the allocation
	alloc := r.Alloc()
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		r.logger.Printf("[ERR] client: alloc %q for missing task group %q", r.allocID, alloc.TaskGroup)
		r.setStatus(structs.AllocClientStatusFailed, fmt.Sprintf("missing task group '%s'", alloc.TaskGroup))
		return
	}

	// Create the execution context
	r.allocDirLock.Lock()
	// Build allocation directory (idempotent)
	if err := r.allocDir.Build(); err != nil {
		r.logger.Printf("[ERR] client: failed to build task directories: %v", err)
		r.setStatus(structs.AllocClientStatusFailed, fmt.Sprintf("failed to build task dirs for '%s'", alloc.TaskGroup))
		r.allocDirLock.Unlock()
		return
	}

	if r.otherAllocDir != nil {
		if err := r.allocDir.Move(r.otherAllocDir, tg.Tasks); err != nil {
			r.logger.Printf("[ERR] client: failed to move alloc dir into alloc %q: %v", r.allocID, err)
		}
		if err := r.otherAllocDir.Destroy(); err != nil {
			r.logger.Printf("[ERR] client: error destroying allocdir %v: %v", r.otherAllocDir.AllocDir, err)
		}
	}
	r.allocDirLock.Unlock()

	// Check if the allocation is in a terminal status. In this case, we don't
	// start any of the task runners and directly wait for the destroy signal to
	// clean up the allocation.
	if alloc.TerminalStatus() {
		r.logger.Printf("[DEBUG] client: alloc %q in terminal status, waiting for destroy", r.allocID)
		r.handleDestroy()
		r.logger.Printf("[DEBUG] client: terminating runner for alloc '%s'", r.allocID)
		return
	}

	// Start the watcher
	wCtx, watcherCancel := context.WithCancel(r.ctx)
	go r.watchHealth(wCtx)

	// Start the task runners
	r.logger.Printf("[DEBUG] client: starting task runners for alloc '%s'", r.allocID)
	r.taskLock.Lock()
	for _, task := range tg.Tasks {
		if _, ok := r.restored[task.Name]; ok {
			continue
		}

		r.allocDirLock.Lock()
		taskdir := r.allocDir.NewTaskDir(task.Name)
		r.allocDirLock.Unlock()

		tr := NewTaskRunner(r.logger, r.config, r.stateDB, r.setTaskState, taskdir, r.Alloc(), task.Copy(), r.vaultClient, r.consulClient)
		r.tasks[task.Name] = tr
		tr.MarkReceived()

		go tr.Run()
	}
	r.taskLock.Unlock()

	// taskDestroyEvent contains an event that caused the destroyment of a task
	// in the allocation.
	var taskDestroyEvent *structs.TaskEvent

OUTER:
	// Wait for updates
	for {
		select {
		case update := <-r.updateCh:
			// Store the updated allocation.
			r.allocLock.Lock()

			// If the deployment ids have changed clear the health
			if r.alloc.DeploymentID != update.DeploymentID {
				r.allocHealth = nil
			}

			r.alloc = update
			r.allocLock.Unlock()

			// Create a new watcher
			watcherCancel()
			wCtx, watcherCancel = context.WithCancel(r.ctx)
			go r.watchHealth(wCtx)

			// Check if we're in a terminal status
			if update.TerminalStatus() {
				taskDestroyEvent = structs.NewTaskEvent(structs.TaskKilled)
				break OUTER
			}

			// Update the task groups
			runners := r.getTaskRunners()
			for _, tr := range runners {
				tr.Update(update)
			}

			if err := r.syncStatus(); err != nil {
				r.logger.Printf("[WARN] client: failed to sync alloc %q status upon receiving alloc update: %v",
					r.allocID, err)
			}
		case <-r.ctx.Done():
			taskDestroyEvent = structs.NewTaskEvent(structs.TaskKilled)
			break OUTER
		}
	}

	// Kill the task runners
	r.destroyTaskRunners(taskDestroyEvent)

	// Block until we should destroy the state of the alloc
	r.handleDestroy()

	// Free up the context. It has likely exited already
	watcherCancel()

	r.logger.Printf("[DEBUG] client: terminating runner for alloc '%s'", r.allocID)
}

// SetPreviousAllocDir sets the previous allocation directory of the current
// allocation
func (r *AllocRunner) SetPreviousAllocDir(allocDir *allocdir.AllocDir) {
	r.otherAllocDir = allocDir
}

// destroyTaskRunners destroys the task runners, waits for them to terminate and
// then saves state.
func (r *AllocRunner) destroyTaskRunners(destroyEvent *structs.TaskEvent) {
	// First destroy the leader if one exists
	tg := r.alloc.Job.LookupTaskGroup(r.alloc.TaskGroup)
	leader := ""
	for _, task := range tg.Tasks {
		if task.Leader {
			leader = task.Name
			break
		}
	}
	if leader != "" {
		r.taskLock.RLock()
		tr := r.tasks[leader]
		r.taskLock.RUnlock()

		r.logger.Printf("[DEBUG] client: alloc %q destroying leader task %q of task group %q first",
			r.allocID, leader, r.alloc.TaskGroup)
		tr.Destroy(destroyEvent)
		<-tr.WaitCh()
	}

	// Then destroy non-leader tasks concurrently
	r.taskLock.RLock()
	for name, tr := range r.tasks {
		if name != leader {
			tr.Destroy(destroyEvent)
		}
	}
	r.taskLock.RUnlock()

	// Wait for termination of the task runners
	for _, tr := range r.getTaskRunners() {
		<-tr.WaitCh()
	}
}

// handleDestroy blocks till the AllocRunner should be destroyed and does the
// necessary cleanup.
func (r *AllocRunner) handleDestroy() {
	// Final state sync. We do this to ensure that the server has the correct
	// state as we wait for a destroy.
	alloc := r.Alloc()

	//TODO(schmichael) updater can cause a GC which can block on this alloc
	// runner shutting down. Since handleDestroy can be called by Run() we
	// can't block shutdown here as it would cause a deadlock.
	go r.updater(alloc)

	// Broadcast and persist state synchronously
	r.sendBroadcast(alloc)
	if err := r.saveAllocRunnerState(); err != nil {
		r.logger.Printf("[WARN] client: alloc %q unable to persist state but should be GC'd soon anyway:%v",
			r.allocID, err)
	}

	for {
		select {
		case <-r.ctx.Done():
			if err := r.DestroyContext(); err != nil {
				r.logger.Printf("[ERR] client: failed to destroy context for alloc '%s': %v",
					r.allocID, err)
			}
			if err := r.DestroyState(); err != nil {
				r.logger.Printf("[ERR] client: failed to destroy state for alloc '%s': %v",
					r.allocID, err)
			}

			return
		case <-r.updateCh:
			r.logger.Printf("[DEBUG] client: dropping update to terminal alloc '%s'", r.allocID)
		}
	}
}

// Update is used to update the allocation of the context
func (r *AllocRunner) Update(update *structs.Allocation) {
	select {
	case r.updateCh <- update:
	default:
		r.logger.Printf("[ERR] client: dropping update to alloc '%s'", update.ID)
	}
}

// StatsReporter returns an interface to query resource usage statistics of an
// allocation
func (r *AllocRunner) StatsReporter() AllocStatsReporter {
	return r
}

// getTaskRunners is a helper that returns a copy of the task runners list using
// the taskLock.
func (r *AllocRunner) getTaskRunners() []*TaskRunner {
	// Get the task runners
	r.taskLock.RLock()
	defer r.taskLock.RUnlock()
	runners := make([]*TaskRunner, 0, len(r.tasks))
	for _, tr := range r.tasks {
		runners = append(runners, tr)
	}
	return runners
}

// LatestAllocStats returns the latest allocation stats. If the optional taskFilter is set
// the allocation stats will only include the given task.
func (r *AllocRunner) LatestAllocStats(taskFilter string) (*cstructs.AllocResourceUsage, error) {
	astat := &cstructs.AllocResourceUsage{
		Tasks: make(map[string]*cstructs.TaskResourceUsage),
	}

	var flat []*cstructs.TaskResourceUsage
	if taskFilter != "" {
		r.taskLock.RLock()
		tr, ok := r.tasks[taskFilter]
		r.taskLock.RUnlock()
		if !ok {
			return nil, fmt.Errorf("allocation %q has no task %q", r.allocID, taskFilter)
		}
		l := tr.LatestResourceUsage()
		if l != nil {
			astat.Tasks[taskFilter] = l
			flat = []*cstructs.TaskResourceUsage{l}
			astat.Timestamp = l.Timestamp
		}
	} else {
		// Get the task runners
		runners := r.getTaskRunners()
		for _, tr := range runners {
			l := tr.LatestResourceUsage()
			if l != nil {
				astat.Tasks[tr.task.Name] = l
				flat = append(flat, l)
				if l.Timestamp > astat.Timestamp {
					astat.Timestamp = l.Timestamp
				}
			}
		}
	}

	astat.ResourceUsage = sumTaskResourceUsage(flat)
	return astat, nil
}

// sumTaskResourceUsage takes a set of task resources and sums their resources
func sumTaskResourceUsage(usages []*cstructs.TaskResourceUsage) *cstructs.ResourceUsage {
	summed := &cstructs.ResourceUsage{
		MemoryStats: &cstructs.MemoryStats{},
		CpuStats:    &cstructs.CpuStats{},
	}
	for _, usage := range usages {
		summed.Add(usage.ResourceUsage)
	}
	return summed
}

// shouldUpdate takes the AllocModifyIndex of an allocation sent from the server and
// checks if the current running allocation is behind and should be updated.
func (r *AllocRunner) shouldUpdate(serverIndex uint64) bool {
	r.allocLock.Lock()
	defer r.allocLock.Unlock()
	return r.alloc.AllocModifyIndex < serverIndex
}

// Destroy is used to indicate that the allocation context should be destroyed
func (r *AllocRunner) Destroy() {
	// Lock when closing the context as that gives the save state code
	// serialization.
	r.allocStateLock.Lock()
	defer r.allocStateLock.Unlock()

	r.exitFn()
	r.allocBroadcast.Close()
}

// WaitCh returns a channel to wait for termination
func (r *AllocRunner) WaitCh() <-chan struct{} {
	return r.waitCh
}
