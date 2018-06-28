package allocrunnerv2

import (
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunnerv2/config"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/client/allocrunnerv2/state"
	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocRunner is used to run all the tasks in a given allocation
type allocRunner struct {
	// Logger is the logger for the alloc runner.
	logger log.Logger

	// Config is the configuration for the alloc runner.
	config *config.Config

	// waitCh is closed when the alloc runner has transitioned to a terminal
	// state
	waitCh chan struct{}

	// Alloc captures the allocation being run.
	alloc     *structs.Allocation
	allocLock sync.Mutex

	// state captures the state of the alloc runner
	state *state.State

	// allocDir is used to build the allocations directory structure.
	allocDir *allocdir.AllocDir

	// runnerHooks are alloc runner lifecycle hooks that should be run on state
	// transistions.
	runnerHooks []interfaces.RunnerHook

	// tasks are the set of task runners
	tasks map[string]*taskrunner.TaskRunner

	// updateCh receives allocation updates
	updateCh chan *structs.Allocation
}

// NewAllocRunner returns a new allocation runner.
func NewAllocRunner(config *config.Config) (*allocRunner, error) {
	ar := &allocRunner{
		config:   config,
		alloc:    config.Allocation,
		waitCh:   make(chan struct{}),
		updateCh: make(chan *structs.Allocation),
	}

	// Create the logger based on the allocation ID
	ar.logger = config.Logger.With("alloc_id", ar.ID())

	// Initialize the runners hooks.
	ar.initRunnerHooks()

	return ar, nil
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
	tg := ar.alloc.Job.LookupTaskGroup(ar.alloc.TaskGroup)
	if tg == nil {
		// XXX Fail and exit
		ar.logger.Error("failed to lookup task group", "task_group", ar.alloc.TaskGroup)
		return nil, fmt.Errorf("failed to lookup task group %q", ar.alloc.TaskGroup)
	}

	for _, task := range tg.Tasks {
		if err := ar.runTask(task); err != nil {
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
func (ar *allocRunner) runTask(task *structs.Task) error {
	// Create the runner
	config := &taskrunner.Config{
		Parent: &allocRunnerShim{ar},
		Task:   task,
		Logger: ar.logger,
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

// Update the running allocation with a new version received from the server.
//
// This method is safe for calling concurrently with Run() and does not modify
// the passed in allocation.
func (ar *allocRunner) Update(update *structs.Allocation) {
	ar.updateCh <- update
}
