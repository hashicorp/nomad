package allocrunnerv2

import (
	"context"
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
	// ctx is the alloc runners context. It is cancelled when all related
	// activity for this allocation should be terminated.
	ctx context.Context

	// ctxCancel is used to cancel the alloc runners cancel
	ctxCancel context.CancelFunc

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
	tasks    map[string]*taskrunner.TaskRunner
	taskLock sync.Mutex
}

// NewAllocRunner returns a new allocation runner.
func NewAllocRunner(ctx context.Context, config *config.Config) (*allocRunner, error) {
	// Create a context for the runner
	arCtx, arCancel := context.WithCancel(ctx)

	ar := &allocRunner{
		ctx:       arCtx,
		ctxCancel: arCancel,
		config:    config,
		alloc:     config.Allocation,
		waitCh:    make(chan struct{}),
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

	// Run the prerun hooks
	// XXX Equivalent to TR.Prerun hook
	if err := ar.prerun(); err != nil {
		ar.logger.Error("prerun failed", "error", err)
		goto POST
	}

	// Run the runners
	if err := ar.runImpl(); err != nil {
		ar.logger.Error("starting tasks failed", "error", err)
	}

POST:
	// Run the postrun hooks
	// XXX Equivalent to TR.Poststop hook
	if err := ar.postrun(); err != nil {
		ar.logger.Error("postrun failed", "error", err)
	}
}

// runImpl is used to run the runners.
func (ar *allocRunner) runImpl() error {
	// Grab the task group
	tg := ar.alloc.Job.LookupTaskGroup(ar.alloc.TaskGroup)
	if tg == nil {
		// XXX Fail and exit
		ar.logger.Error("failed to lookup task group", "task_group", ar.alloc.TaskGroup)
		return fmt.Errorf("failed to lookup task group %q", ar.alloc.TaskGroup)
	}

	for _, task := range tg.Tasks {
		if err := ar.runTask(task); err != nil {
			return err
		}
	}

	// Block until all tasks are done.
	var wg sync.WaitGroup
	ar.taskLock.Lock()
	for _, task := range ar.tasks {
		wg.Add(1)
		go func() {
			<-task.WaitCh()
			wg.Done()
		}()
	}
	ar.taskLock.Unlock()

	wg.Wait()
	return nil
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
	ar.taskLock.Lock()
	ar.tasks[task.Name] = tr
	ar.taskLock.Unlock()
	return nil
}
