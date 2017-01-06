package client

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/vaultclient"
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

	alloc                  *structs.Allocation
	allocClientStatus      string // Explicit status of allocation. Set when there are failures
	allocClientDescription string
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

	vaultClient vaultclient.VaultClient

	otherAllocDir *allocdir.AllocDir

	destroy     bool
	destroyCh   chan struct{}
	destroyLock sync.Mutex
	waitCh      chan struct{}

	// serialize saveAllocRunnerState calls
	persistLock sync.Mutex
}

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

// NewAllocRunner is used to create a new allocation context
func NewAllocRunner(logger *log.Logger, config *config.Config, updater AllocStateUpdater,
	alloc *structs.Allocation, vaultClient vaultclient.VaultClient) *AllocRunner {
	ar := &AllocRunner{
		config:      config,
		updater:     updater,
		logger:      logger,
		alloc:       alloc,
		dirtyCh:     make(chan struct{}, 1),
		tasks:       make(map[string]*TaskRunner),
		taskStates:  copyTaskStates(alloc.TaskStates),
		restored:    make(map[string]struct{}),
		updateCh:    make(chan *structs.Allocation, 64),
		destroyCh:   make(chan struct{}),
		waitCh:      make(chan struct{}),
		vaultClient: vaultClient,
	}
	return ar
}

// stateFilePath returns the path to our state file
func (r *AllocRunner) stateFilePath() string {
	r.allocLock.Lock()
	defer r.allocLock.Unlock()
	path := filepath.Join(r.config.StateDir, "alloc", r.alloc.ID, "state.json")
	return path
}

// RestoreState is used to restore the state of the alloc runner
func (r *AllocRunner) RestoreState() error {
	// Load the snapshot
	var snap allocRunnerState
	if err := restoreState(r.stateFilePath(), &snap); err != nil {
		return err
	}

	// #2132 Upgrade path: if snap.AllocDir is nil, try to convert old
	// Context struct to new AllocDir struct
	if snap.AllocDir == nil && snap.Context != nil {
		r.logger.Printf("[DEBUG] client: migrating state snapshot for alloc %q", r.alloc.ID)
		snap.AllocDir = allocdir.NewAllocDir(r.logger, snap.Context.AllocDir.AllocDir)
		for taskName := range snap.Context.AllocDir.TaskDirs {
			snap.AllocDir.NewTaskDir(taskName)
		}
	}

	// Restore fields
	r.alloc = snap.Alloc
	r.allocDir = snap.AllocDir
	r.allocClientStatus = snap.AllocClientStatus
	r.allocClientDescription = snap.AllocClientDescription

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

	r.taskStates = snap.Alloc.TaskStates

	// Restore the task runners
	var mErr multierror.Error
	for name, state := range r.taskStates {
		// Mark the task as restored.
		r.restored[name] = struct{}{}

		td, ok := r.allocDir.TaskDirs[name]
		if !ok {
			err := fmt.Errorf("failed to find task dir metadata for alloc %q task %q",
				r.alloc.ID, name)
			r.logger.Printf("[ERR] client: %v", err)
			return err
		}

		task := &structs.Task{Name: name}
		tr := NewTaskRunner(r.logger, r.config, r.setTaskState, td, r.Alloc(), task, r.vaultClient)
		r.tasks[name] = tr

		// Skip tasks in terminal states.
		if state.State == structs.TaskStateDead {
			continue
		}

		if err := tr.RestoreState(); err != nil {
			r.logger.Printf("[ERR] client: failed to restore state for alloc %s task '%s': %v", r.alloc.ID, name, err)
			mErr.Errors = append(mErr.Errors, err)
		} else if !r.alloc.TerminalStatus() {
			// Only start if the alloc isn't in a terminal status.
			go tr.Run()
		}
	}

	return mErr.ErrorOrNil()
}

// GetAllocDir returns the alloc dir for the alloc runner
func (r *AllocRunner) GetAllocDir() *allocdir.AllocDir {
	return r.allocDir
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
		if err := r.saveTaskRunnerState(tr); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
	}
	return mErr.ErrorOrNil()
}

func (r *AllocRunner) saveAllocRunnerState() error {
	r.persistLock.Lock()
	defer r.persistLock.Unlock()

	// Create the snapshot.
	alloc := r.Alloc()

	r.allocLock.Lock()
	allocClientStatus := r.allocClientStatus
	allocClientDescription := r.allocClientDescription
	r.allocLock.Unlock()

	r.allocDirLock.Lock()
	allocDir := r.allocDir
	r.allocDirLock.Unlock()

	snap := allocRunnerState{
		Version:                r.config.Version,
		Alloc:                  alloc,
		AllocDir:               allocDir,
		AllocClientStatus:      allocClientStatus,
		AllocClientDescription: allocClientDescription,
	}
	return persistState(r.stateFilePath(), &snap)
}

func (r *AllocRunner) saveTaskRunnerState(tr *TaskRunner) error {
	if err := tr.SaveState(); err != nil {
		return fmt.Errorf("failed to save state for alloc %s task '%s': %v",
			r.alloc.ID, tr.task.Name, err)
	}
	return nil
}

// DestroyState is used to cleanup after ourselves
func (r *AllocRunner) DestroyState() error {
	return os.RemoveAll(filepath.Dir(r.stateFilePath()))
}

// DestroyContext is used to destroy the context
func (r *AllocRunner) DestroyContext() error {
	return r.allocDir.Destroy()
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
	alloc := r.alloc.Copy()

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
	r.allocLock.Unlock()

	// Scan the task states to determine the status of the alloc
	var pending, running, dead, failed bool
	r.taskStatusLock.RLock()
	alloc.TaskStates = copyTaskStates(r.taskStates)
	for _, state := range r.taskStates {
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
	r.taskStatusLock.RUnlock()

	// Determine the alloc status
	if failed {
		alloc.ClientStatus = structs.AllocClientStatusFailed
	} else if running {
		alloc.ClientStatus = structs.AllocClientStatusRunning
	} else if pending {
		alloc.ClientStatus = structs.AllocClientStatusPending
	} else if dead {
		alloc.ClientStatus = structs.AllocClientStatusComplete
	}

	return alloc
}

// dirtySyncState is used to watch for state being marked dirty to sync
func (r *AllocRunner) dirtySyncState() {
	for {
		select {
		case <-r.dirtyCh:
			r.syncStatus()
		case <-r.destroyCh:
			return
		}
	}
}

// syncStatus is used to run and sync the status when it changes
func (r *AllocRunner) syncStatus() error {
	// Get a copy of our alloc, update status server side and sync to disk
	alloc := r.Alloc()
	r.updater(alloc)
	return r.saveAllocRunnerState()
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
		r.appendTaskEvent(taskState, event)
	}

	if state == "" {
		return
	}

	taskState.State = state
	if state == structs.TaskStateDead {
		// If the task failed, we should kill all the other tasks in the task group.
		if taskState.Failed {
			var destroyingTasks []string
			for task, tr := range r.tasks {
				if task != taskName {
					destroyingTasks = append(destroyingTasks, task)
					tr.Destroy(structs.NewTaskEvent(structs.TaskSiblingFailed).SetFailedSibling(taskName))
				}
			}
			if len(destroyingTasks) > 0 {
				r.logger.Printf("[DEBUG] client: task %q failed, destroying other tasks in task group: %v", taskName, destroyingTasks)
			}
		}
	}

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
	alloc := r.alloc
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		r.logger.Printf("[ERR] client: alloc '%s' for missing task group '%s'", alloc.ID, alloc.TaskGroup)
		r.setStatus(structs.AllocClientStatusFailed, fmt.Sprintf("missing task group '%s'", alloc.TaskGroup))
		return
	}

	// Create the execution context
	r.allocDirLock.Lock()
	if r.allocDir == nil {
		// Build allocation directory
		r.allocDir = allocdir.NewAllocDir(r.logger, filepath.Join(r.config.AllocDir, r.alloc.ID))
		if err := r.allocDir.Build(); err != nil {
			r.logger.Printf("[WARN] client: failed to build task directories: %v", err)
			r.setStatus(structs.AllocClientStatusFailed, fmt.Sprintf("failed to build task dirs for '%s'", alloc.TaskGroup))
			r.allocDirLock.Unlock()
			return
		}

		if r.otherAllocDir != nil {
			if err := r.allocDir.Move(r.otherAllocDir, tg.Tasks); err != nil {
				r.logger.Printf("[ERROR] client: failed to move alloc dir into alloc %q: %v", r.alloc.ID, err)
			}
			if err := r.otherAllocDir.Destroy(); err != nil {
				r.logger.Printf("[ERROR] client: error destroying allocdir %v", r.otherAllocDir.AllocDir, err)
			}
		}
	}
	r.allocDirLock.Unlock()

	// Check if the allocation is in a terminal status. In this case, we don't
	// start any of the task runners and directly wait for the destroy signal to
	// clean up the allocation.
	if alloc.TerminalStatus() {
		r.logger.Printf("[DEBUG] client: alloc %q in terminal status, waiting for destroy", r.alloc.ID)
		r.handleDestroy()
		r.logger.Printf("[DEBUG] client: terminating runner for alloc '%s'", r.alloc.ID)
		return
	}

	// Start the task runners
	r.logger.Printf("[DEBUG] client: starting task runners for alloc '%s'", r.alloc.ID)
	r.taskLock.Lock()
	for _, task := range tg.Tasks {
		if _, ok := r.restored[task.Name]; ok {
			continue
		}

		r.allocDirLock.Lock()
		taskdir := r.allocDir.NewTaskDir(task.Name)
		r.allocDirLock.Unlock()

		tr := NewTaskRunner(r.logger, r.config, r.setTaskState, taskdir, r.Alloc(), task.Copy(), r.vaultClient)
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
			r.alloc = update
			r.allocLock.Unlock()

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
		case <-r.destroyCh:
			taskDestroyEvent = structs.NewTaskEvent(structs.TaskKilled)
			break OUTER
		}
	}

	// Kill the task runners
	r.destroyTaskRunners(taskDestroyEvent)

	// Block until we should destroy the state of the alloc
	r.handleDestroy()
	r.logger.Printf("[DEBUG] client: terminating runner for alloc '%s'", r.alloc.ID)
}

// SetPreviousAllocDir sets the previous allocation directory of the current
// allocation
func (r *AllocRunner) SetPreviousAllocDir(allocDir *allocdir.AllocDir) {
	r.otherAllocDir = allocDir
}

// destroyTaskRunners destroys the task runners, waits for them to terminate and
// then saves state.
func (r *AllocRunner) destroyTaskRunners(destroyEvent *structs.TaskEvent) {
	// Destroy each sub-task
	runners := r.getTaskRunners()
	for _, tr := range runners {
		tr.Destroy(destroyEvent)
	}

	// Wait for termination of the task runners
	for _, tr := range runners {
		<-tr.WaitCh()
	}

	// Final state sync
	r.syncStatus()
}

// handleDestroy blocks till the AllocRunner should be destroyed and does the
// necessary cleanup.
func (r *AllocRunner) handleDestroy() {
	select {
	case <-r.destroyCh:
		if err := r.DestroyContext(); err != nil {
			r.logger.Printf("[ERR] client: failed to destroy context for alloc '%s': %v",
				r.alloc.ID, err)
		}
		if err := r.DestroyState(); err != nil {
			r.logger.Printf("[ERR] client: failed to destroy state for alloc '%s': %v",
				r.alloc.ID, err)
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
			return nil, fmt.Errorf("allocation %q has no task %q", r.alloc.ID, taskFilter)
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
	r.destroyLock.Lock()
	defer r.destroyLock.Unlock()

	if r.destroy {
		return
	}
	r.destroy = true
	close(r.destroyCh)
}

// WaitCh returns a channel to wait for termination
func (r *AllocRunner) WaitCh() <-chan struct{} {
	return r.waitCh
}
