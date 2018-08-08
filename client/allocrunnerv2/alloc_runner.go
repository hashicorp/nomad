package allocrunnerv2

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/client/allocrunnerv2/state"
	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/consul"
	cinterfaces "github.com/hashicorp/nomad/client/interfaces"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// updateChCap is the capacity of AllocRunner's updateCh. It must be 1
	// as we only want to process the latest update, so if there's already
	// a pending update it will be removed from the chan before adding the
	// newer update.
	updateChCap = 1
)

// allocRunner is used to run all the tasks in a given allocation
type allocRunner struct {
	// id is the ID of the allocation. Can be accessed without a lock
	id string

	// Logger is the logger for the alloc runner.
	logger log.Logger

	clientConfig *config.Config

	// stateUpdater is used to emit updated task state
	stateUpdater cinterfaces.AllocStateHandler

	// consulClient is the client used by the consul service hook for
	// registering services and checks
	consulClient consul.ConsulServiceAPI

	// vaultClient is the used to manage Vault tokens
	vaultClient vaultclient.VaultClient

	// waitCh is closed when the alloc runner has transitioned to a terminal
	// state
	waitCh chan struct{}

	// Alloc captures the allocation being run.
	alloc     *structs.Allocation
	allocLock sync.RWMutex

	//XXX implement for local state
	// state captures the state of the alloc runner
	state     *state.State
	stateLock sync.RWMutex

	stateDB cstate.StateDB

	// allocDir is used to build the allocations directory structure.
	allocDir *allocdir.AllocDir

	// runnerHooks are alloc runner lifecycle hooks that should be run on state
	// transistions.
	runnerHooks []interfaces.RunnerHook

	// tasks are the set of task runners
	tasks     map[string]*taskrunner.TaskRunner
	tasksLock sync.RWMutex

	// updateCh receives allocation updates via the Update method. Must
	// have buffer size 1 in order to support dropping pending updates when
	// a newer allocation is received.
	updateCh chan *structs.Allocation
}

// NewAllocRunner returns a new allocation runner.
func NewAllocRunner(config *Config) (*allocRunner, error) {
	alloc := config.Alloc
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil, fmt.Errorf("failed to lookup task group %q", alloc.TaskGroup)
	}

	ar := &allocRunner{
		id:           alloc.ID,
		alloc:        alloc,
		clientConfig: config.ClientConfig,
		consulClient: config.Consul,
		vaultClient:  config.Vault,
		tasks:        make(map[string]*taskrunner.TaskRunner, len(tg.Tasks)),
		waitCh:       make(chan struct{}),
		updateCh:     make(chan *structs.Allocation, updateChCap),
		state:        &state.State{},
		stateDB:      config.StateDB,
		stateUpdater: config.StateUpdater,
	}

	// Create alloc dir
	//XXX update AllocDir to hc log
	ar.allocDir = allocdir.NewAllocDir(nil, filepath.Join(config.ClientConfig.AllocDir, alloc.ID))

	// Create the logger based on the allocation ID
	ar.logger = config.Logger.With("alloc_id", alloc.ID)

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
			Alloc:        ar.alloc,
			ClientConfig: ar.clientConfig,
			Task:         task,
			TaskDir:      ar.allocDir.NewTaskDir(task.Name),
			Logger:       ar.logger,
			StateDB:      ar.stateDB,
			StateUpdater: ar,
			Consul:       ar.consulClient,
			VaultClient:  ar.vaultClient,
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

// XXX How does alloc Restart work
// Run is the main go-routine that executes all the tasks.
func (ar *allocRunner) Run() {
	// Close the wait channel
	defer close(ar.waitCh)

	var taskWaitCh <-chan struct{}

	// Run the prestart hooks
	// XXX Equivalent to TR.Prestart hook
	if err := ar.prerun(); err != nil {
		ar.logger.Error("prerun failed", "error", err)
		goto POST
	}

	// Run the runners
	taskWaitCh = ar.runImpl()

MAIN:
	for {
		select {
		case <-taskWaitCh:
			// TaskRunners have all exited
			break MAIN
		case updated := <-ar.updateCh:
			// Update ar.alloc
			ar.setAlloc(updated)

			//TODO Run AR Update hooks

			// Update task runners
			for _, tr := range ar.tasks {
				tr.Update(updated)
			}
		}
	}

POST:
	// Run the postrun hooks
	// XXX Equivalent to TR.Poststop hook
	if err := ar.postrun(); err != nil {
		ar.logger.Error("postrun failed", "error", err)
	}
}

// runImpl is used to run the runners.
func (ar *allocRunner) runImpl() <-chan struct{} {
	for _, task := range ar.tasks {
		go task.Run()
	}

	// Return a combined WaitCh that is closed when all task runners have
	// exited.
	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		for _, task := range ar.tasks {
			<-task.WaitCh()
		}
	}()

	return waitCh
}

// Alloc returns the current allocation being run by this runner.
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

// SaveState does all the state related stuff. Who knows. FIXME
//XXX do we need to do periodic syncing? if Saving is only called *before* Run
//    *and* within Run -- *and* Updates are applid within Run -- we may be able to
//    skip quite a bit of locking? maybe?
func (ar *allocRunner) SaveState() error {
	//XXX Do we move this to the client
	//XXX Track AllocModifyIndex to only write alloc on change?

	// Write the allocation
	return ar.stateDB.PutAllocation(ar.Alloc())
}

// Restore state from database. Must be called after NewAllocRunner but before
// Run.
func (ar *allocRunner) Restore() error {
	// Restore task runners
	for _, tr := range ar.tasks {
		if err := tr.Restore(); err != nil {
			return err
		}
	}

	return nil
}

// TaskStateUpdated is called by TaskRunner when a task's state has been
// updated. This hook is used to compute changes to the alloc's ClientStatus
// and to update the server with the new state.
func (ar *allocRunner) TaskStateUpdated(taskName string, state *structs.TaskState) {
	// If a task is dead, we potentially want to kill other tasks in the group
	if state.State == structs.TaskStateDead {
		// Find all tasks that are not the one that is dead and check if the one
		// that is dead is a leader
		var otherTaskRunners []*taskrunner.TaskRunner
		var otherTaskNames []string
		leader := false
		for name, tr := range ar.tasks {
			if name != taskName {
				otherTaskRunners = append(otherTaskRunners, tr)
				otherTaskNames = append(otherTaskNames, name)
			} else if tr.Task().Leader {
				leader = true
			}
		}

		// If the task failed, we should kill all the other tasks in the task group.
		if state.Failed {
			if len(otherTaskRunners) > 0 {
				ar.logger.Debug("task failure, destroying all tasks", "failed_task", taskName, "destroying", otherTaskNames)
			}
			for _, tr := range otherTaskRunners {
				tr.Kill(context.Background(), structs.NewTaskEvent(structs.TaskSiblingFailed).SetFailedSibling(taskName))
			}
		} else if leader {
			if len(otherTaskRunners) > 0 {
				ar.logger.Debug("leader task dead, destroying all tasks", "leader_task", taskName, "destroying", otherTaskNames)
			}
			// If the task was a leader task we should kill all the other tasks.
			for _, tr := range otherTaskRunners {
				tr.Kill(context.Background(), structs.NewTaskEvent(structs.TaskLeaderDead))
			}
		}
	}

	// Gather the state of the other tasks
	states := make(map[string]*structs.TaskState, len(ar.tasks))
	for name, tr := range ar.tasks {
		if name == taskName {
			states[name] = state
		} else {
			states[name] = tr.TaskState()
		}
	}

	// Get the client allocation
	calloc := ar.clientAlloc(states)

	// Update the server
	if err := ar.stateUpdater.AllocStateUpdated(calloc); err != nil {
		ar.logger.Error("failed to update remote allocation state", "error", err)
	}
}

// clientAlloc takes in the task states and returns an Allocation populated
// with Client specific fields
func (ar *allocRunner) clientAlloc(taskStates map[string]*structs.TaskState) *structs.Allocation {
	ar.stateLock.RLock()
	defer ar.stateLock.RUnlock()

	a := &structs.Allocation{
		ID:         ar.id,
		TaskStates: taskStates,
	}

	s := ar.state
	if d := s.DeploymentStatus; d != nil {
		a.DeploymentStatus = d.Copy()
	}

	// Compute the ClientStatus
	if s.ClientStatus != "" {
		// The client status is being forced
		a.ClientStatus, a.ClientDescription = s.ClientStatus, s.ClientDescription
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

// Update the running allocation with a new version received from the server.
//
// This method sends the updated alloc to Run for serially processing updates.
// If there is already a pending update it will be discarded and replaced by
// the latest update.
func (ar *allocRunner) Update(update *structs.Allocation) {
	select {
	case ar.updateCh <- update:
		// Updated alloc sent
	case <-ar.updateCh:
		// There was a pending update; replace it with the new update.
		// This also prevents Update() from blocking if its called
		// concurrently with Run() exiting.
		ar.updateCh <- update
	}
}

// Destroy the alloc runner by stopping it if it is still running and cleaning
// up all of its resources.
//
// This method is safe for calling concurrently with Run(). Callers must
// receive on WaitCh() to block until alloc runner has stopped and been
// destroyed.
//XXX TODO
func (ar *allocRunner) Destroy() {
	//TODO

	for _, tr := range ar.tasks {
		tr.Kill(context.Background(), structs.NewTaskEvent(structs.TaskKilled))
	}
}

// IsDestroyed returns true if the alloc runner has been destroyed (stopped and
// garbage collected).
//
// This method is safe for calling concurrently with Run(). Callers must
// receive on WaitCh() to block until alloc runner has stopped and been
// destroyed.
//XXX TODO
func (ar *allocRunner) IsDestroyed() bool {
	return false
}

// IsWaiting returns true if the alloc runner is waiting for its previous
// allocation to terminate.
//
// This method is safe for calling concurrently with Run().
//XXX TODO
func (ar *allocRunner) IsWaiting() bool {
	return false
}

// IsMigrating returns true if the alloc runner is migrating data from its
// previous allocation.
//
// This method is safe for calling concurrently with Run().
//XXX TODO
func (ar *allocRunner) IsMigrating() bool {
	return false
}

// StatsReporter needs implementing
//XXX
func (ar *allocRunner) StatsReporter() allocrunner.AllocStatsReporter {
	return noopStatsReporter{}
}

//FIXME implement
type noopStatsReporter struct{}

func (noopStatsReporter) LatestAllocStats(taskFilter string) (*cstructs.AllocResourceUsage, error) {
	return nil, fmt.Errorf("not implemented")
}
