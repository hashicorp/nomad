package allocrunnerv2

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/boltdb/bolt"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/client/allocrunnerv2/state"
	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner"
	"github.com/hashicorp/nomad/client/config"
	clientstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocRunner is used to run all the tasks in a given allocation
type allocRunner struct {
	// Logger is the logger for the alloc runner.
	logger log.Logger

	clientConfig *config.Config

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
	state *state.State

	stateDB *bolt.DB

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
func NewAllocRunner(config *Config) (*allocRunner, error) {
	alloc := config.Alloc
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil, fmt.Errorf("failed to lookup task group %q", alloc.TaskGroup)
	}

	ar := &allocRunner{
		alloc:        alloc,
		clientConfig: config.ClientConfig,
		vaultClient:  config.Vault,
		tasks:        make(map[string]*taskrunner.TaskRunner, len(tg.Tasks)),
		waitCh:       make(chan struct{}),
		updateCh:     make(chan *structs.Allocation),
		stateDB:      config.StateDB,
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
//XXX how do we handle mutate the state saving stuff
func (ar *allocRunner) Alloc() *structs.Allocation {
	ar.allocLock.RLock()
	defer ar.allocLock.RUnlock()
	return ar.alloc
}

// SaveState does all the state related stuff. Who knows. FIXME
//XXX do we need to do periodic syncing? if Saving is only called *before* Run
//    *and* within Run -- *and* Updates are applid within Run -- we may be able to
//    skip quite a bit of locking? maybe?
func (ar *allocRunner) SaveState() error {
	return ar.stateDB.Update(func(tx *bolt.Tx) error {
		//XXX Track EvalID to only write alloc on change?
		// Write the allocation
		return clientstate.PutAllocation(tx, ar.Alloc())
	})
}

// Restore state from database. Must be called after NewAllocRunner but before
// Run.
func (ar *allocRunner) Restore() error {
	return ar.stateDB.View(func(tx *bolt.Tx) error {
		// Restore task runners
		for _, tr := range ar.tasks {
			if err := tr.Restore(tx); err != nil {
				return err
			}
		}
		return nil
	})
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
