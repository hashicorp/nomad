package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/restarts"
	"github.com/hashicorp/nomad/client/allocrunnerv2/config"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	arstate "github.com/hashicorp/nomad/client/allocrunnerv2/state"
	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/env"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

type allocRunner interface {
	State() *arstate.State // XXX Do we need it
	Alloc() *structs.Allocation
	ID() string
	Config() *config.Config
	GetAllocDir() *allocdir.AllocDir
	StateUpdated(*state.State) error
}

type TaskRunner struct {
	// config is the task runners configuration.
	config *Config

	// state captures the state of the task runner
	state *state.State

	// ctx is the alloc runners context. It is cancelled when all related
	// activity for this allocation should be terminated.
	ctx context.Context

	// ctxCancel is used to cancel the alloc runners cancel
	ctxCancel context.CancelFunc

	// Logger is the logger for the task runner.
	logger log.Logger

	// allocRunner is the parent allocRunner
	allocRunner allocRunner

	// waitCh is closed when the task runner has transitioned to a terminal
	// state
	waitCh chan struct{}

	// driver is the driver for the task.
	driver driver.Driver

	// handle is the handle to the currently running driver
	handle     driver.DriverHandle
	handleLock sync.Mutex

	// task is the task beign run
	task     *structs.Task
	taskLock sync.RWMutex

	// taskDir is the directory structure for this task.
	taskDir *allocdir.TaskDir

	// envBuilder is used to build the task's environment
	envBuilder *env.Builder

	// restartTracker is used to decide if the task should be restarted.
	restartTracker *restarts.RestartTracker

	// runnerHooks are task runner lifecycle hooks that should be run on state
	// transistions.
	runnerHooks []interfaces.TaskHook

	// baseLabels are used when emitting tagged metrics. All task runner metrics
	// will have these tags, and optionally more.
	baseLabels []metrics.Label
}

type Config struct {
	Parent allocRunner
	Task   *structs.Task
	Logger log.Logger

	// State is optionally restored task state
	State *state.State
}

func NewTaskRunner(config *Config) (*TaskRunner, error) {
	// Create a context for the runner
	trCtx, trCancel := context.WithCancel(context.Background())

	// Initialize the environment builder
	envBuilder := env.NewBuilder(
		config.Parent.Config().ClientConfig.Node,
		config.Parent.Alloc(),
		config.Task,
		config.Parent.Config().ClientConfig.Region)

	tr := &TaskRunner{
		config:      config,
		allocRunner: config.Parent,
		state:       config.State,
		ctx:         trCtx,
		ctxCancel:   trCancel,
		task:        config.Task,
		taskDir:     config.Parent.GetAllocDir().NewTaskDir(config.Task.Name),
		envBuilder:  envBuilder,
		waitCh:      make(chan struct{}),
	}

	// Create the logger based on the allocation ID
	tr.logger = config.Logger.Named("task_runner").With("task", config.Task.Name)

	// Build the restart tracker.
	alloc := tr.allocRunner.Alloc()
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		tr.logger.Error("alloc missing task group")
		return nil, fmt.Errorf("alloc missing task group")
	}
	tr.restartTracker = restarts.NewRestartTracker(tg.RestartPolicy, alloc.Job.Type)

	// Initialize the task state
	tr.initState()

	// Get the driver
	if err := tr.initDriver(); err != nil {
		tr.logger.Error("failed to create driver", "error", err)
		return nil, err
	}

	// Initialize the runners hooks.
	tr.initHooks()

	// Initialize base labels
	tr.initLabels()

	return tr, nil
}

func (tr *TaskRunner) initState() {
	if tr.state == nil {
		tr.state = &state.State{
			Task: &structs.TaskState{
				State: structs.TaskStatePending,
			},
			Hooks: make(map[string]*state.HookState),
		}
	}
}

func (tr *TaskRunner) initLabels() {
	alloc := tr.allocRunner.Alloc()
	tr.baseLabels = []metrics.Label{
		{
			Name:  "job",
			Value: alloc.Job.Name,
		},
		{
			Name:  "task_group",
			Value: alloc.TaskGroup,
		},
		{
			Name:  "alloc_id",
			Value: alloc.ID,
		},
		{
			Name:  "task",
			Value: tr.Name(),
		},
	}
}

// WaitCh is closed when TaskRunner.Run exits.
func (tr *TaskRunner) WaitCh() <-chan struct{} {
	return tr.waitCh
}

// Update the running allocation with a new version received from the server.
//
// This method is safe for calling concurrently with Run() and does not modify
// the passed in allocation.
func (tr *TaskRunner) Update(update *structs.Allocation) {
	// XXX
}

func (tr *TaskRunner) Stop() {
	// XXX
	tr.ctxCancel()
}

func (tr *TaskRunner) Run() {
	defer close(tr.waitCh)

	var err error
	var restart bool
	var restartWait time.Duration
	var waitRes *dstructs.WaitResult
	for {
		// Run the prerun hooks
		if err = tr.prerun(); err != nil {
			tr.logger.Error("prerun failed", "error", err)
			tr.restartTracker.SetStartError(err)
			goto RESTART
		}

		// Run and wait
		waitRes, err = tr.runDriver()
		if err != nil {
			tr.logger.Error("running driver failed", "error", err)
			tr.restartTracker.SetStartError(err)
			goto RESTART
		}
		tr.restartTracker.SetWaitResult(waitRes)

		// Run the postrun hooks
		if err = tr.postrun(); err != nil {
			tr.logger.Error("postrun failed", "error", err)
		}

		// Check if the context is closed already and go straight to destroy
		if err := tr.ctx.Err(); err != nil {
			goto DESTROY
		}

	RESTART:
		// Actually restart by sleeping and also watching for destroy events
		restart, restartWait = tr.shouldRestart()
		if restart {
			select {
			case <-time.After(restartWait):
				continue
			case <-tr.ctx.Done():
			}
		}

	DESTROY:
		// Run the destroy hooks
		if err = tr.destroy(); err != nil {
			tr.logger.Error("postrun failed", "error", err)
		}

		tr.logger.Debug("task run loop exiting")
		return
	}
}

func (tr *TaskRunner) shouldRestart() (bool, time.Duration) {
	// Determine if we should restart
	state, when := tr.restartTracker.GetState()
	reason := tr.restartTracker.GetReason()
	switch state {
	case structs.TaskNotRestarting, structs.TaskTerminated:
		tr.logger.Info("not restarting task", "reason", reason)
		if state == structs.TaskNotRestarting {
			tr.SetState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskNotRestarting).SetRestartReason(reason).SetFailsTask())
		}
		return false, 0
	case structs.TaskRestarting:
		tr.logger.Info("restarting task", "reason", reason, "delay", when)
		tr.SetState(structs.TaskStatePending, structs.NewTaskEvent(structs.TaskRestarting).SetRestartDelay(when).SetRestartReason(reason))
		return true, 0
	default:
		tr.logger.Error("restart tracker returned unknown state", "state", state)
		return true, when
	}
}

// runDriver runs the driver and waits for it to exit
func (tr *TaskRunner) runDriver() (*dstructs.WaitResult, error) {
	// Run prestart
	ctx := driver.NewExecContext(tr.taskDir, tr.envBuilder.Build())
	_, err := tr.driver.Prestart(ctx, tr.task)
	if err != nil {
		tr.logger.Error("driver pre-start failed", "error", err)
		return nil, err
	}

	// Create a new context for Start since the environment may have been updated.
	ctx = driver.NewExecContext(tr.taskDir, tr.envBuilder.Build())

	// Start the job
	sresp, err := tr.driver.Start(ctx, tr.task)
	if err != nil {
		tr.logger.Warn("driver start failed", "error", err)
		return nil, err
	}

	// Wait on the handle
	tr.handleLock.Lock()
	handle := sresp.Handle
	tr.handle = handle
	tr.handleLock.Unlock()

	// Emit an event that we started
	tr.SetState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))

	// Wait for the task to exit
	waitRes := <-handle.WaitCh()
	return waitRes, nil
}

// initDriver creates the driver for the task
func (tr *TaskRunner) initDriver() error {
	// Create a task-specific event emitter callback to expose minimal
	// state to drivers
	eventEmitter := func(m string, args ...interface{}) {
		msg := fmt.Sprintf(m, args...)
		tr.logger.Debug("driver event", "event", msg)
		tr.SetState("", structs.NewTaskEvent(structs.TaskDriverMessage).SetDriverMessage(msg))
	}

	alloc := tr.allocRunner.Alloc()
	driverCtx := driver.NewDriverContext(
		alloc.Job.Name,
		alloc.TaskGroup,
		tr.Name(),
		tr.allocRunner.ID(),
		tr.allocRunner.Config().ClientConfig,      // XXX Why does it need this
		tr.allocRunner.Config().ClientConfig.Node, // XXX THIS I NEED TO FIX
		tr.logger.StandardLogger(nil),             // XXX Should pass this through
		eventEmitter)

	driver, err := driver.NewDriver(tr.task.Driver, driverCtx)
	if err != nil {
		return err
	}

	tr.driver = driver
	return nil
}

// SetState sets the task runners state.
func (tr *TaskRunner) SetState(state string, event *structs.TaskEvent) {
	// Ensure the event is populated with human readable strings
	event.PopulateEventDisplayMessage()

	// Lock our state
	tr.state.Lock()
	defer tr.state.Unlock()
	task := tr.state.Task

	// Update the state of the task
	if state != "" {
		task.State = state
	}

	// Handle the event
	if event == nil {
		if event.FailsTask {
			task.Failed = true
		}

		if event.Type == structs.TaskRestarting {
			if !tr.allocRunner.Config().ClientConfig.DisableTaggedMetrics {
				metrics.IncrCounterWithLabels([]string{"client", "allocs", "restart"}, 1, tr.baseLabels)
			}
			//if r.config.BackwardsCompatibleMetrics {
			//metrics.IncrCounter([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, taskName, "restart"}, 1)
			//}
			task.Restarts++
			task.LastRestart = time.Unix(0, event.Time)
		}
		appendTaskEvent(task, event)
	}

	// Handle the state transistion.
	switch state {
	case structs.TaskStateRunning:
		// Capture the start time if it is just starting
		if task.State != structs.TaskStateRunning {
			task.StartedAt = time.Now().UTC()
			if !tr.allocRunner.Config().ClientConfig.DisableTaggedMetrics {
				metrics.IncrCounterWithLabels([]string{"client", "allocs", "running"}, 1, tr.baseLabels)
			}
			//if r.config.BackwardsCompatibleMetrics {
			//metrics.IncrCounter([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, taskName, "running"}, 1)
			//}
		}
	case structs.TaskStateDead:
		// Capture the finished time if not already set
		if task.FinishedAt.IsZero() {
			task.FinishedAt = time.Now().UTC()
		}

		// Emitting metrics to indicate task complete and failures
		if task.Failed {
			if !tr.allocRunner.Config().ClientConfig.DisableTaggedMetrics {
				metrics.IncrCounterWithLabels([]string{"client", "allocs", "failed"}, 1, tr.baseLabels)
			}
			//if r.config.BackwardsCompatibleMetrics {
			//metrics.IncrCounter([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, taskName, "failed"}, 1)
			//}
		} else {
			if !tr.allocRunner.Config().ClientConfig.DisableTaggedMetrics {
				metrics.IncrCounterWithLabels([]string{"client", "allocs", "complete"}, 1, tr.baseLabels)
			}
			//if r.config.BackwardsCompatibleMetrics {
			//metrics.IncrCounter([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, taskName, "complete"}, 1)
			//}
		}
	}

	// Create a copy and notify the alloc runner of the transition
	if err := tr.allocRunner.StateUpdated(tr.state.Copy()); err != nil {
		tr.logger.Error("failed to save state", "error", err)
	}
}

// appendTaskEvent updates the task status by appending the new event.
func appendTaskEvent(state *structs.TaskState, event *structs.TaskEvent) {
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
