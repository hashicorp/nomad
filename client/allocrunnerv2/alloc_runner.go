package allocrunnerv2

import (
	"fmt"
	"path/filepath"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/client/allocrunnerv2/state"
	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocRunner is used to run all the tasks in a given allocation
type allocRunner struct {
	// Logger is the logger for the alloc runner.
	logger log.Logger

	clientConfig *config.Config

	// waitCh is closed when the alloc runner has transitioned to a terminal
	// state
	waitCh chan struct{}

	// Alloc captures the allocation being run.
	alloc     *structs.Allocation
	allocLock sync.RWMutex

	// state captures the state of the alloc runner
	state *state.State

	// allocDir is used to build the allocations directory structure.
	allocDir *allocdir.AllocDir

	// runnerHooks are alloc runner lifecycle hooks that should be run on state
	// transistions.
	runnerHooks []interfaces.RunnerHook

	// tasks are the set of task runners
	tasks map[string]*taskrunner.TaskRunner

	// updateCh receives allocation updates via the Update method
	updateCh chan *structs.Allocation
}

// NewAllocRunner returns a new allocation runner.
func NewAllocRunner(config *Config) *allocRunner {
	ar := &allocRunner{
		alloc:        config.Alloc,
		clientConfig: config.ClientConfig,
		tasks:        make(map[string]*taskrunner.TaskRunner),
		waitCh:       make(chan struct{}),
		updateCh:     make(chan *structs.Allocation),
	}

	// Create alloc dir
	//XXX update AllocDir to hc log
	ar.allocDir = allocdir.NewAllocDir(nil, filepath.Join(config.ClientConfig.AllocDir, config.Alloc.ID))

	// Create the logger based on the allocation ID
	ar.logger = config.Logger.With("alloc_id", config.Alloc.ID)

	// Initialize the runners hooks.
	ar.initRunnerHooks()

	return ar
}

func (ar *allocRunner) WaitCh() <-chan struct{} {
	return ar.waitCh
}

// XXX How does alloc Restart work
// Run is the main go-routine that executes all the tasks.
func (ar *allocRunner) Run() {
	// Close the wait channel
	defer close(ar.waitCh)

	var err error
	var taskWaitCh <-chan struct{}

	// Run the prerun hooks
	// XXX Equivalent to TR.Prerun hook
	if err := ar.prerun(); err != nil {
		ar.logger.Error("prerun failed", "error", err)
		goto POST
	}

	// Run the runners
	taskWaitCh, err = ar.runImpl()
	if err != nil {
		ar.logger.Error("starting tasks failed", "error", err)
	}

	for {
		select {
		case <-taskWaitCh:
			// TaskRunners have all exited
		case updated := <-ar.updateCh:
			// Updated alloc received
			//XXX Update hooks
			//XXX Update ar.alloc
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
func (ar *allocRunner) runImpl() (<-chan struct{}, error) {
	// Grab the task group
	alloc := ar.Alloc()
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		// XXX Fail and exit
		ar.logger.Error("failed to lookup task group", "task_group", alloc.TaskGroup)
		return nil, fmt.Errorf("failed to lookup task group %q", alloc.TaskGroup)
	}

	for _, task := range tg.Tasks {
		if err := ar.runTask(alloc, task); err != nil {
			return nil, err
		}
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

	return waitCh, nil
}

// runTask is used to run a task.
func (ar *allocRunner) runTask(alloc *structs.Allocation, task *structs.Task) error {
	// Create the runner
	config := &taskrunner.Config{
		Alloc:        alloc,
		ClientConfig: ar.clientConfig,
		Task:         task,
		TaskDir:      ar.allocDir.NewTaskDir(task.Name),
		Logger:       ar.logger,
	}
	tr, err := taskrunner.NewTaskRunner(config)
	if err != nil {
		return err
	}

	// Start the runner
	go tr.Run()

	// Store the runner
	ar.tasks[task.Name] = tr
	return nil
}

// Alloc returns the current allocation being run by this runner.
//XXX how do we handle mutate the state saving stuff
func (ar *allocRunner) Alloc() *structs.Allocation {
	ar.allocLock.RLock()
	defer ar.allocLock.RUnlock()
	return ar.alloc
}

// SaveState does all the state related stuff. Who knows. FIXME
//XXX
func (ar *allocRunner) SaveState() error {
	return nil
}

// Update the running allocation with a new version received from the server.
//
// This method is safe for calling concurrently with Run() and does not modify
// the passed in allocation.
func (ar *allocRunner) Update(update *structs.Allocation) {
	ar.updateCh <- update
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
