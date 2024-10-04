// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/restarts"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	cinterfaces "github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/pluginmanager/csimanager"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	cstate "github.com/hashicorp/nomad/client/state"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/client/widmgr"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pluginutils/hclspecutils"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/users/dynamic"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

const (
	// defaultMaxEvents is the default max capacity for task events on the
	// task state. Overrideable for testing.
	defaultMaxEvents = 10

	// killBackoffBaseline is the baseline time for exponential backoff while
	// killing a task.
	killBackoffBaseline = 5 * time.Second

	// killBackoffLimit is the limit of the exponential backoff for killing
	// the task.
	killBackoffLimit = 2 * time.Minute

	// killFailureLimit is how many times we will attempt to kill a task before
	// giving up and potentially leaking resources.
	killFailureLimit = 5

	// triggerUpdateChCap is the capacity for the triggerUpdateCh used for
	// triggering updates. It should be exactly 1 as even if multiple
	// updates have come in since the last one was handled, we only need to
	// handle the last one.
	triggerUpdateChCap = 1

	// restartChCap is the capacity for the restartCh used for triggering task
	// restarts. It should be exactly 1 as even if multiple restarts have come
	// we only need to handle the last one.
	restartChCap = 1
)

type TaskRunner struct {
	// allocID, taskName, taskLeader, and taskResources are immutable so these fields may
	// be accessed without locks
	allocID       string
	taskName      string
	taskLeader    bool
	taskResources *structs.AllocatedTaskResources

	alloc     *structs.Allocation
	allocLock sync.Mutex

	clientConfig *config.Config

	// stateUpdater is used to emit updated task state
	stateUpdater interfaces.TaskStateHandler

	// state captures the state of the task for updating the allocation
	// Must acquire stateLock to access.
	state *structs.TaskState

	// localState captures the node-local state of the task for when the
	// Nomad agent restarts.
	// Must acquire stateLock to access.
	localState *state.LocalState

	// stateLock must be acquired when accessing state or localState.
	stateLock sync.RWMutex

	// stateDB is for persisting localState and taskState
	stateDB cstate.StateDB

	// restartCh is used to signal that the task should restart.
	restartCh chan struct{}

	// shutdownCtx is used to exit the TaskRunner *without* affecting task state.
	shutdownCtx context.Context

	// shutdownCtxCancel causes the TaskRunner to exit immediately without
	// affecting task state. Useful for testing or graceful agent shutdown.
	shutdownCtxCancel context.CancelFunc

	// killCtx is the task runner's context representing the tasks's lifecycle.
	// The context is canceled when the task is killed.
	killCtx context.Context

	// killCtxCancel is called when killing a task.
	killCtxCancel context.CancelFunc

	// killErr is populated when killing a task. Access should be done use the
	// getter/setter
	killErr     error
	killErrLock sync.Mutex

	// shutdownDelayCtx is a context from the alloc runner which will
	// tell us to exit early from shutdown_delay
	shutdownDelayCtx      context.Context
	shutdownDelayCancelFn context.CancelFunc

	// Logger is the logger for the task runner.
	logger log.Logger

	// triggerUpdateCh is ticked whenever update hooks need to be run and
	// must be created with cap=1 to signal a pending update and prevent
	// callers from deadlocking if the receiver has exited.
	triggerUpdateCh chan struct{}

	// waitCh is closed when the task runner has transitioned to a terminal
	// state
	waitCh chan struct{}

	// driver is the driver for the task.
	driver drivers.DriverPlugin

	// driverCapabilities is the set capabilities the driver supports
	driverCapabilities *drivers.Capabilities

	// taskSchema is the hcl spec for the task driver configuration
	taskSchema hcldec.Spec

	// handleLock guards access to handle and handleResult
	handleLock sync.Mutex

	// handle to the running driver
	handle *DriverHandle

	// task is the task being run
	task     *structs.Task
	taskLock sync.RWMutex

	// taskDir is the directory structure for this task.
	taskDir *allocdir.TaskDir

	// envBuilder is used to build the task's environment
	envBuilder *taskenv.Builder

	// restartTracker is used to decide if the task should be restarted.
	restartTracker *restarts.RestartTracker

	// runnerHooks are task runner lifecycle hooks that should be run on state
	// transistions.
	runnerHooks []interfaces.TaskHook

	// hookResources captures the resources provided by hooks
	hookResources *hookResources

	// allocHookResources captures the resources provided by the allocrunner hooks
	allocHookResources *cstructs.AllocHookResources

	// consulClient is the client used by the consul service hook for
	// registering services and checks
	consulServiceClient serviceregistration.Handler

	// consulProxiesClientFunc gets a client used by the envoy version hook for
	// asking consul what version of envoy nomad should inject into the connect
	// sidecar or gateway task.
	consulProxiesClientFunc consul.SupportedProxiesAPIFunc

	// sidsClient is the client used by the service identity hook for managing
	// service identity tokens
	siClient consul.ServiceIdentityAPI

	// vaultClientFunc is the function to get a client to use to derive and
	// renew Vault tokens
	vaultClientFunc vaultclient.VaultClientFunc

	// vaultToken is the current Vault token. It should be accessed with the
	// getter.
	vaultToken     string
	vaultTokenLock sync.Mutex

	// nomadToken is the current Nomad workload identity token. It
	// should be accessed with the getter.
	nomadToken     string
	nomadTokenLock sync.Mutex

	// baseLabels are used when emitting tagged metrics. All task runner metrics
	// will have these tags, and optionally more.
	baseLabels []metrics.Label

	// logmonHookConfig is used to get the paths to the stdout and stderr fifos
	// to be passed to the driver for task logging
	logmonHookConfig *logmonHookConfig

	// resourceUsage is written via UpdateStats and read via
	// LatestResourceUsage. May be nil at all times.
	resourceUsage     *cstructs.TaskResourceUsage
	resourceUsageLock sync.Mutex

	// deviceStatsReporter is used to lookup resource usage for alloc devices
	deviceStatsReporter cinterfaces.DeviceStatsReporter

	// csiManager is used to manage the mounting of CSI volumes into tasks
	csiManager csimanager.Manager

	// devicemanager is used to mount devices as well as lookup device
	// statistics
	devicemanager devicemanager.Manager

	// driverManager is used to dispense driver plugins and register event
	// handlers
	driverManager drivermanager.Manager

	// dynamicRegistry is where dynamic plugins should be registered.
	dynamicRegistry dynamicplugins.Registry

	// maxEvents is the capacity of the TaskEvents on the TaskState.
	// Defaults to defaultMaxEvents but overrideable for testing.
	maxEvents int

	// serversContactedCh is passed to TaskRunners so they can detect when
	// GetClientAllocs has been called in case of a failed restore.
	serversContactedCh <-chan struct{}

	// startConditionMetCh signals the TaskRunner when it should start the task
	startConditionMetCh <-chan struct{}

	// waitOnServers defaults to false but will be set true if a restore
	// fails and the Run method should wait until serversContactedCh is
	// closed.
	waitOnServers bool

	networkIsolationLock sync.Mutex
	networkIsolationSpec *drivers.NetworkIsolationSpec

	// allocNetworkStatus is provided from the allocrunner and allows us to
	// include this information as env vars for the task. When manipulating
	// this the allocNetworkStatusLock should be used.
	allocNetworkStatusLock sync.Mutex
	allocNetworkStatus     *structs.AllocNetworkStatus

	// serviceRegWrapper is the handler wrapper that is used by service hooks
	// to perform service and check registration and deregistration.
	serviceRegWrapper *wrapper.HandlerWrapper

	// getter is an interface for retrieving artifacts.
	getter cinterfaces.ArtifactGetter

	// wranglers manage unix/windows processes leveraging operating
	// system features like cgroups
	wranglers cinterfaces.ProcessWranglers

	// widmgr manages workload identities
	widmgr widmgr.IdentityManager

	// users manages the pool of dynamic workload users
	users dynamic.Pool

	// pauser controls whether the task should be run or stopped based on a
	// schedule. (Enterprise)
	pauser *pauseGate
}

type Config struct {
	Alloc        *structs.Allocation
	ClientConfig *config.Config
	Task         *structs.Task
	TaskDir      *allocdir.TaskDir
	Logger       log.Logger

	// ConsulServices is used for managing Consul service registrations
	ConsulServices serviceregistration.Handler

	// ConsulProxiesFunc gets a client to use for looking up supported envoy versions
	// from Consul.
	ConsulProxiesFunc consul.SupportedProxiesAPIFunc

	// ConsulSI is the client to use for managing Consul SI tokens
	ConsulSI consul.ServiceIdentityAPI

	// DynamicRegistry is where dynamic plugins should be registered.
	DynamicRegistry dynamicplugins.Registry

	// VaultFunc is function to get the client to use to derive and renew Vault tokens
	VaultFunc vaultclient.VaultClientFunc

	// StateDB is used to store and restore state.
	StateDB cstate.StateDB

	// StateUpdater is used to emit updated task state
	StateUpdater interfaces.TaskStateHandler

	// deviceStatsReporter is used to lookup resource usage for alloc devices
	DeviceStatsReporter cinterfaces.DeviceStatsReporter

	// CSIManager is used to manage the mounting of CSI volumes into tasks
	CSIManager csimanager.Manager

	// DeviceManager is used to mount devices as well as lookup device
	// statistics
	DeviceManager devicemanager.Manager

	// DriverManager is used to dispense driver plugins and register event
	// handlers
	DriverManager drivermanager.Manager

	// ServersContactedCh is closed when the first GetClientAllocs call to
	// servers succeeds and allocs are synced.
	ServersContactedCh chan struct{}

	// StartConditionMetCh signals the TaskRunner when it should start the task
	StartConditionMetCh <-chan struct{}

	// ShutdownDelayCtx is a context from the alloc runner which will
	// tell us to exit early from shutdown_delay
	ShutdownDelayCtx context.Context

	// ShutdownDelayCancelFn should only be used in testing.
	ShutdownDelayCancelFn context.CancelFunc

	// ServiceRegWrapper is the handler wrapper that is used by service hooks
	// to perform service and check registration and deregistration.
	ServiceRegWrapper *wrapper.HandlerWrapper

	// Getter is an interface for retrieving artifacts.
	Getter cinterfaces.ArtifactGetter

	// Wranglers is an interface for managing OS processes.
	Wranglers cinterfaces.ProcessWranglers

	// AllocHookResources is how taskrunner hooks can get state written by
	// allocrunner hooks
	AllocHookResources *cstructs.AllocHookResources

	// WIDMgr manages workload identities
	WIDMgr widmgr.IdentityManager

	// Users manages a pool of dynamic workload users
	Users dynamic.Pool
}

func NewTaskRunner(config *Config) (*TaskRunner, error) {
	// Create a context for causing the runner to exit
	trCtx, trCancel := context.WithCancel(context.Background())

	// Create a context for killing the runner
	killCtx, killCancel := context.WithCancel(context.Background())

	// Initialize the environment builder
	envBuilder := taskenv.NewBuilder(
		config.ClientConfig.Node,
		config.Alloc,
		config.Task,
		config.ClientConfig.Region,
	)

	// Initialize state from alloc if it is set
	tstate := structs.NewTaskState()
	if ts := config.Alloc.TaskStates[config.Task.Name]; ts != nil {
		tstate = ts.Copy()
	}

	tr := &TaskRunner{
		alloc:                   config.Alloc,
		allocID:                 config.Alloc.ID,
		clientConfig:            config.ClientConfig,
		task:                    config.Task,
		taskDir:                 config.TaskDir,
		taskName:                config.Task.Name,
		taskLeader:              config.Task.Leader,
		envBuilder:              envBuilder,
		dynamicRegistry:         config.DynamicRegistry,
		consulServiceClient:     config.ConsulServices,
		consulProxiesClientFunc: config.ConsulProxiesFunc,
		siClient:                config.ConsulSI,
		vaultClientFunc:         config.VaultFunc,
		state:                   tstate,
		localState:              state.NewLocalState(),
		allocHookResources:      config.AllocHookResources,
		stateDB:                 config.StateDB,
		stateUpdater:            config.StateUpdater,
		deviceStatsReporter:     config.DeviceStatsReporter,
		killCtx:                 killCtx,
		killCtxCancel:           killCancel,
		shutdownCtx:             trCtx,
		shutdownCtxCancel:       trCancel,
		triggerUpdateCh:         make(chan struct{}, triggerUpdateChCap),
		restartCh:               make(chan struct{}, restartChCap),
		waitCh:                  make(chan struct{}),
		csiManager:              config.CSIManager,
		devicemanager:           config.DeviceManager,
		driverManager:           config.DriverManager,
		maxEvents:               defaultMaxEvents,
		serversContactedCh:      config.ServersContactedCh,
		startConditionMetCh:     config.StartConditionMetCh,
		shutdownDelayCtx:        config.ShutdownDelayCtx,
		shutdownDelayCancelFn:   config.ShutdownDelayCancelFn,
		serviceRegWrapper:       config.ServiceRegWrapper,
		getter:                  config.Getter,
		wranglers:               config.Wranglers,
		widmgr:                  config.WIDMgr,
		users:                   config.Users,
	}

	// Create the logger based on the allocation ID
	tr.logger = config.Logger.Named("task_runner").With("task", config.Task.Name)

	// Create the pauser
	tr.pauser = newPauseGate(tr)

	// Pull out the task's resources
	ares := tr.alloc.AllocatedResources
	if ares == nil {
		return nil, fmt.Errorf("no task resources found on allocation")
	}

	tres, ok := ares.Tasks[tr.taskName]
	if !ok {
		return nil, fmt.Errorf("no task resources found on allocation")
	}

	// we had to allocate the tmpfs with the memory to get correct scheduling
	// and tracking on the node, but now that we're creating the task driver
	// config we only care about the memory without the secrets.
	tres.Memory.MemoryMB -= int64(tr.task.Resources.SecretsMB)
	tr.taskResources = tres

	// Build the restart tracker.
	rp := config.Task.RestartPolicy
	if rp == nil {
		tg := tr.alloc.Job.LookupTaskGroup(tr.alloc.TaskGroup)
		if tg == nil {
			tr.logger.Error("alloc missing task group")
			return nil, fmt.Errorf("alloc missing task group")
		}
		rp = tg.RestartPolicy
	}
	tr.restartTracker = restarts.NewRestartTracker(rp, tr.alloc.Job.Type, config.Task.Lifecycle)

	// Get the driver
	if err := tr.initDriver(); err != nil {
		tr.logger.Error("failed to create driver", "error", err)
		return nil, err
	}

	// Use the client secret only as the initial value; the identity hook will
	// update this with a workload identity if one is available
	tr.setNomadToken(config.ClientConfig.Node.SecretID)

	// Initialize the runners hooks. Must come after initDriver so hooks
	// can use tr.driverCapabilities
	tr.initHooks()

	// Initialize base labels
	tr.initLabels()

	// Initialize initial task received event
	tr.appendEvent(structs.NewTaskEvent(structs.TaskReceived))

	return tr, nil
}

func (tr *TaskRunner) initLabels() {
	alloc := tr.Alloc()
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
			Value: tr.allocID,
		},
		{
			Name:  "task",
			Value: tr.taskName,
		},
		{
			Name:  "namespace",
			Value: tr.alloc.Namespace,
		},
	}

	if tr.clientConfig.IncludeAllocMetadataInMetrics {
		combined := alloc.Job.CombinedTaskMeta(alloc.TaskGroup, tr.taskName)
		for meta, metaValue := range combined {
			if len(tr.clientConfig.AllowedMetadataKeysInMetrics) > 0 && !slices.Contains(tr.clientConfig.AllowedMetadataKeysInMetrics, meta) {
				continue
			}

			tr.baseLabels = append(tr.baseLabels, metrics.Label{
				Name:  strings.ReplaceAll(meta, "-", "_"),
				Value: metaValue,
			})
		}
	}

	if tr.alloc.Job.ParentID != "" {
		tr.baseLabels = append(tr.baseLabels, metrics.Label{
			Name:  "parent_id",
			Value: tr.alloc.Job.ParentID,
		})
		if strings.Contains(tr.alloc.Job.Name, "/dispatch-") {
			tr.baseLabels = append(tr.baseLabels, metrics.Label{
				Name:  "dispatch_id",
				Value: strings.Split(tr.alloc.Job.Name, "/dispatch-")[1],
			})
		}
		if strings.Contains(tr.alloc.Job.Name, "/periodic-") {
			tr.baseLabels = append(tr.baseLabels, metrics.Label{
				Name:  "periodic_id",
				Value: strings.Split(tr.alloc.Job.Name, "/periodic-")[1],
			})
		}
	}
}

// MarkFailedKill marks a task as failed and should be killed.
// It should be invoked when alloc runner prestart hooks fail.
// Afterwards, Run() will perform any necessary cleanup.
func (tr *TaskRunner) MarkFailedKill(reason string) {
	// Emit an event that fails the task and gives reasons for humans.
	event := structs.NewTaskEvent(structs.TaskSetupFailure).
		SetKillReason(structs.TaskRestoreFailed).
		SetDisplayMessage(reason).
		SetFailsTask()
	tr.EmitEvent(event)

	// Cancel kill context, so later when allocRunner runs tr.Run(),
	// we'll follow the usual kill path and do all the appropriate cleanup steps.
	tr.killCtxCancel()
}

// Run the TaskRunner. Starts the user's task or reattaches to a restored task.
// Run closes WaitCh when it exits. Should be started in a goroutine.
func (tr *TaskRunner) Run() {
	defer close(tr.waitCh)
	var result *drivers.ExitResult

	tr.stateLock.RLock()
	dead := tr.state.State == structs.TaskStateDead
	runComplete := tr.localState.RunComplete
	tr.stateLock.RUnlock()

	// If restoring a dead task, ensure the task is cleared and, if the local
	// state indicates that the previous Run() call is complete, execute all
	// post stop hooks and exit early, otherwise proceed until the
	// ALLOC_RESTART loop skipping MAIN since the task is dead.
	if dead {
		// do cleanup functions without emitting any additional events/work
		// to handle cases where we restored a dead task where client terminated
		// after task finished before completing post-run actions.
		tr.clearDriverHandle()
		tr.stateUpdater.TaskStateUpdated()
		if runComplete {
			if err := tr.stop(); err != nil {
				tr.logger.Error("stop failed on terminal task", "error", err)
			}
			return
		}
	}

	// Updates are handled asynchronously with the other hooks but each
	// triggered update - whether due to alloc updates or a new vault token
	// - should be handled serially.
	go tr.handleUpdates()

	// If restore failed wait until servers are contacted before running.
	// #1795
	if tr.waitOnServers {
		tr.logger.Info("task failed to restore; waiting to contact server before restarting")
		select {
		case <-tr.killCtx.Done():
			tr.logger.Info("task killed while waiting for server contact")
		case <-tr.shutdownCtx.Done():
			return
		case <-tr.serversContactedCh:
			tr.logger.Info("server contacted; unblocking waiting task")
		}
	}

	// Set the initial task state.
	tr.stateUpdater.TaskStateUpdated()

	// start with a stopped timer; actual restart delay computed later
	timer, stop := helper.NewStoppedTimer()
	defer stop()

MAIN:
	for !tr.shouldShutdown() {
		if dead {
			break
		}

		select {
		case <-tr.killCtx.Done():
			break MAIN
		case <-tr.shutdownCtx.Done():
			// TaskRunner was told to exit immediately
			return
		case <-tr.startConditionMetCh:
			tr.logger.Debug("lifecycle start condition has been met, proceeding")
			// yay proceed
		}

		// Run the prestart hooks
		if err := tr.prestart(); err != nil {
			tr.logger.Error("prestart failed", "error", err)
			tr.restartTracker.SetStartError(err)
			goto RESTART
		}

		// Unblocks when the task runner is allowed to continue. (Enterprise)
		if err := tr.pauser.Wait(); err != nil {
			tr.logger.Error("pause scheduled failed", "error", err)
			tr.restartTracker.SetStartError(err)
			break MAIN
		}

		// Check for a terminal allocation once more before proceeding as the
		// prestart hooks may have been skipped.
		if tr.shouldShutdown() {
			break MAIN
		}

		select {
		case <-tr.killCtx.Done():
			break MAIN
		case <-tr.shutdownCtx.Done():
			// TaskRunner was told to exit immediately
			return
		default:
		}

		// Run the task
		if err := tr.runDriver(); err != nil {
			tr.logger.Error("running driver failed", "error", err)
			tr.restartTracker.SetStartError(err)
			goto RESTART
		}

		// Run the poststart hooks
		if err := tr.poststart(); err != nil {
			tr.logger.Error("poststart failed", "error", err)
		}

		// Grab the result proxy and wait for task to exit
	WAIT:
		{
			handle := tr.getDriverHandle()
			result = nil

			// Do *not* use tr.killCtx here as it would cause
			// Wait() to unblock before the task exits when Kill()
			// is called.
			if resultCh, err := handle.WaitCh(context.Background()); err != nil {
				tr.logger.Error("wait task failed", "error", err)
			} else {
				select {
				case <-tr.killCtx.Done():
					// We can go through the normal should restart check since
					// the restart tracker knowns it is killed
					result = tr.handleKill(resultCh)
				case <-tr.shutdownCtx.Done():
					// TaskRunner was told to exit immediately
					return
				case result = <-resultCh:
				}

				// WaitCh returned a result
				if retryWait := tr.handleTaskExitResult(result); retryWait {
					goto WAIT
				}
			}
		}

		// Clear the handle
		tr.clearDriverHandle()

		// Store the wait result on the restart tracker
		tr.restartTracker.SetExitResult(result)

		if err := tr.exited(); err != nil {
			tr.logger.Error("exited hooks failed", "error", err)
		}

	RESTART:
		restart, restartDelay := tr.shouldRestart()
		if !restart {
			break MAIN
		}

		timer.Reset(restartDelay)

		// Actually restart by sleeping and also watching for destroy events
		select {
		case <-timer.C:
		case <-tr.killCtx.Done():
			tr.logger.Trace("task killed between restarts", "delay", restartDelay)
			break MAIN
		case <-tr.shutdownCtx.Done():
			// TaskRunner was told to exit immediately
			tr.logger.Trace("gracefully shutting down during restart delay")
			return
		}
	}

	// Ensure handle is cleaned up. Restore could have recovered a task
	// that should be terminal, so if the handle still exists we should
	// kill it here.
	if tr.getDriverHandle() != nil {
		if result = tr.handleKill(nil); result != nil {
			tr.emitExitResultEvent(result)
		}

		tr.clearDriverHandle()

		if err := tr.exited(); err != nil {
			tr.logger.Error("exited hooks failed while cleaning up terminal task", "error", err)
		}
	}

	// Mark the task as dead
	tr.UpdateState(structs.TaskStateDead, nil)

	// Wait here in case the allocation is restarted. Poststop tasks will never
	// run again so skip them to avoid blocking forever.
	if !tr.Task().IsPoststop() {
	ALLOC_RESTART:
		// Run in a loop to handle cases where restartCh is triggered but the
		// task runner doesn't need to restart.
		for {
			select {
			case <-tr.killCtx.Done():
				break ALLOC_RESTART
			case <-tr.shutdownCtx.Done():
				return
			case <-tr.restartCh:
				// Restart without delay since the task is not running anymore.
				restart, _ := tr.shouldRestart()
				if restart {
					// Set runner as not dead to allow the MAIN loop to run.
					dead = false
					goto MAIN
				}
			}
		}
	}

	tr.stateLock.Lock()
	tr.localState.RunComplete = true
	err := tr.stateDB.PutTaskRunnerLocalState(tr.allocID, tr.taskName, tr.localState)
	if err != nil {
		tr.logger.Warn("error persisting task state on run loop exit", "error", err)
	}
	tr.stateLock.Unlock()

	// Run the stop hooks
	if err := tr.stop(); err != nil {
		tr.logger.Error("stop failed", "error", err)
	}

	tr.logger.Debug("task run loop exiting")
}

func (tr *TaskRunner) shouldShutdown() bool {
	alloc := tr.Alloc()
	if alloc.ClientTerminalStatus() {
		return true
	}

	if !tr.IsPoststopTask() && alloc.ServerTerminalStatus() {
		return true
	}

	return false
}

// handleTaskExitResult handles the results returned by the task exiting. If
// retryWait is true, the caller should attempt to wait on the task again since
// it has not actually finished running. This can happen if the driver plugin
// has exited.
func (tr *TaskRunner) handleTaskExitResult(result *drivers.ExitResult) (retryWait bool) {
	if result == nil {
		return false
	}

	if result.Err == bstructs.ErrPluginShutdown {
		dn := tr.Task().Driver
		tr.logger.Debug("driver plugin has shutdown; attempting to recover task", "driver", dn)

		// Initialize a new driver handle
		if err := tr.initDriver(); err != nil {
			tr.logger.Error("failed to initialize driver after it exited unexpectedly", "error", err, "driver", dn)
			return false
		}

		// Try to restore the handle
		tr.stateLock.RLock()
		h := tr.localState.TaskHandle
		net := tr.localState.DriverNetwork
		tr.stateLock.RUnlock()
		if !tr.restoreHandle(h, net) {
			tr.logger.Error("failed to restore handle on driver after it exited unexpectedly", "driver", dn)
			return false
		}

		tr.logger.Debug("task successfully recovered on driver", "driver", dn)
		return true
	}

	// Emit Terminated event
	tr.emitExitResultEvent(result)

	return false
}

// emitExitResultEvent emits a TaskTerminated event for an ExitResult.
func (tr *TaskRunner) emitExitResultEvent(result *drivers.ExitResult) {
	event := structs.NewTaskEvent(structs.TaskTerminated).
		SetExitCode(result.ExitCode).
		SetSignal(result.Signal).
		SetOOMKilled(result.OOMKilled).
		SetExitMessage(result.Err)

	tr.EmitEvent(event)

	if result.OOMKilled {
		metrics.IncrCounterWithLabels([]string{"client", "allocs", "oom_killed"}, 1, tr.baseLabels)
	}
}

// handleUpdates runs update hooks when triggerUpdateCh is ticked and exits
// when Run has returned. Should only be run in a goroutine from Run.
func (tr *TaskRunner) handleUpdates() {
	for {
		select {
		case <-tr.triggerUpdateCh:
		case <-tr.waitCh:
			return
		}

		// Non-terminal update; run hooks
		tr.updateHooks()
	}
}

// shouldRestart determines whether the task should be restarted and updates
// the task state unless the task is killed or terminated.
func (tr *TaskRunner) shouldRestart() (bool, time.Duration) {
	// Determine if we should restart
	state, when := tr.restartTracker.GetState()
	reason := tr.restartTracker.GetReason()
	switch state {
	case structs.TaskKilled:
		// Never restart an explicitly killed task. Kill method handles
		// updating the server.
		tr.EmitEvent(structs.NewTaskEvent(state))
		return false, 0
	case structs.TaskNotRestarting, structs.TaskTerminated:
		tr.logger.Info("not restarting task", "reason", reason)
		if state == structs.TaskNotRestarting {
			tr.UpdateState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskNotRestarting).SetRestartReason(reason).SetFailsTask())
		}
		return false, 0
	case structs.TaskRestarting:
		tr.logger.Info("restarting task", "reason", reason, "delay", when)
		tr.UpdateState(structs.TaskStatePending, structs.NewTaskEvent(structs.TaskRestarting).SetRestartDelay(when).SetRestartReason(reason))
		return true, when
	default:
		tr.logger.Error("restart tracker returned unknown state", "state", state)
		return true, when
	}
}

func (tr *TaskRunner) assignCgroup(taskConfig *drivers.TaskConfig) {
	reserveCores := len(tr.taskResources.Cpu.ReservedCores) > 0
	p := cgroupslib.LinuxResourcesPath(taskConfig.AllocID, taskConfig.Name, reserveCores)
	taskConfig.Resources.LinuxResources.CpusetCgroupPath = p
}

// runDriver runs the driver and waits for it to exit
// runDriver emits an appropriate task event on success/failure
func (tr *TaskRunner) runDriver() error {
	taskConfig := tr.buildTaskConfig()
	tr.assignCgroup(taskConfig)

	// Build hcl context variables
	vars, errs, err := tr.envBuilder.Build().AllValues()
	if err != nil {
		return fmt.Errorf("error building environment variables: %v", err)
	}

	// Handle per-key errors
	if len(errs) > 0 {
		keys := make([]string, 0, len(errs))
		for k, err := range errs {
			keys = append(keys, k)

			if tr.logger.IsTrace() {
				// Verbosely log every diagnostic for debugging
				tr.logger.Trace("error building environment variables", "key", k, "error", err)
			}
		}

		tr.logger.Warn("some environment variables not available for rendering", "keys", strings.Join(keys, ", "))
	}

	val, diag, diagErrs := hclutils.ParseHclInterface(tr.task.Config, tr.taskSchema, vars)
	if diag.HasErrors() {
		parseErr := multierror.Append(errors.New("failed to parse config: "), diagErrs...)
		tr.EmitEvent(structs.NewTaskEvent(structs.TaskFailedValidation).SetValidationError(parseErr))
		return parseErr
	}

	if err := taskConfig.EncodeDriverConfig(val); err != nil {
		encodeErr := fmt.Errorf("failed to encode driver config: %v", err)
		tr.EmitEvent(structs.NewTaskEvent(structs.TaskFailedValidation).SetValidationError(encodeErr))
		return encodeErr
	}

	// If there's already a task handle (eg from a Restore) there's nothing
	// to do except update state.
	if tr.getDriverHandle() != nil {
		// Ensure running state is persisted but do *not* append a new
		// task event as restoring is a client event and not relevant
		// to a task's lifecycle.
		if err := tr.updateStateImpl(structs.TaskStateRunning); err != nil {
			//TODO return error and destroy task to avoid an orphaned task?
			tr.logger.Warn("error persisting task state", "error", err)
		}
		return nil
	}

	// Start the job if there's no existing handle (or if RecoverTask failed)
	handle, net, err := tr.driver.StartTask(taskConfig)
	if err != nil {
		// The plugin has died, try relaunching it
		if err == bstructs.ErrPluginShutdown {
			tr.logger.Info("failed to start task because plugin shutdown unexpectedly; attempting to recover")
			if err := tr.initDriver(); err != nil {
				taskErr := fmt.Errorf("failed to initialize driver after it exited unexpectedly: %v", err)
				tr.EmitEvent(structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(taskErr))
				return taskErr
			}

			handle, net, err = tr.driver.StartTask(taskConfig)
			if err != nil {
				taskErr := fmt.Errorf("failed to start task after driver exited unexpectedly: %v", err)
				tr.EmitEvent(structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(taskErr))
				return taskErr
			}
		} else {
			// Do *NOT* wrap the error here without maintaining whether or not is Recoverable.
			// You must emit a task event failure to be considered Recoverable
			tr.EmitEvent(structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(err))
			return err
		}
	}

	tr.stateLock.Lock()
	tr.localState.TaskHandle = handle
	tr.localState.DriverNetwork = net
	if err := tr.stateDB.PutTaskRunnerLocalState(tr.allocID, tr.taskName, tr.localState); err != nil {
		//TODO Nomad will be unable to restore this task; try to kill
		//     it now and fail? In general we prefer to leave running
		//     tasks running even if the agent encounters an error.
		tr.logger.Warn("error persisting local task state; may be unable to restore after a Nomad restart",
			"error", err, "task_id", handle.Config.ID)
	}
	tr.stateLock.Unlock()

	tr.setDriverHandle(NewDriverHandle(tr.driver, taskConfig.ID, tr.Task(), tr.clientConfig.MaxKillTimeout, net))

	// Emit an event that we started
	tr.UpdateState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))
	return nil
}

// initDriver retrives the DriverPlugin from the plugin loader for this task
func (tr *TaskRunner) initDriver() error {
	driver, err := tr.driverManager.Dispense(tr.Task().Driver)
	if err != nil {
		return err
	}
	tr.driver = driver

	schema, err := tr.driver.TaskConfigSchema()
	if err != nil {
		return err
	}
	spec, diag := hclspecutils.Convert(schema)
	if diag.HasErrors() {
		return multierror.Append(errors.New("failed to convert task schema"), diag.Errs()...)
	}
	tr.taskSchema = spec

	caps, err := tr.driver.Capabilities()
	if err != nil {
		return err
	}
	tr.driverCapabilities = caps

	return nil
}

// handleKill is used to handle the a request to kill a task. It will return
// the handle exit result if one is available and store any error in the task
// runner killErr value.
func (tr *TaskRunner) handleKill(resultCh <-chan *drivers.ExitResult) *drivers.ExitResult {
	// Run the pre killing hooks
	tr.preKill()

	// Wait for task ShutdownDelay after running prekill hooks
	// This allows for things like service de-registration to run
	// before waiting to kill task
	if delay := tr.Task().ShutdownDelay; delay != 0 {
		var ev *structs.TaskEvent
		if tr.alloc.DesiredTransition.ShouldIgnoreShutdownDelay() {
			tr.logger.Debug("skipping shutdown_delay", "shutdown_delay", delay)
			ev = structs.NewTaskEvent(structs.TaskSkippingShutdownDelay).
				SetDisplayMessage(fmt.Sprintf("Skipping shutdown_delay of %s before killing the task.", delay))
		} else {
			tr.logger.Debug("waiting before killing task", "shutdown_delay", delay)
			ev = structs.NewTaskEvent(structs.TaskWaitingShuttingDownDelay).
				SetDisplayMessage(fmt.Sprintf("Waiting for shutdown_delay of %s before killing the task.", delay))
		}
		tr.UpdateState(structs.TaskStatePending, ev)

		select {
		case result := <-resultCh:
			return result
		case <-tr.shutdownDelayCtx.Done():
			break
		case <-time.After(delay):
		}
	}

	// Tell the restart tracker that the task has been killed so it doesn't
	// attempt to restart it.
	tr.restartTracker.SetKilled()

	// Check it is running
	select {
	case result := <-resultCh:
		return result
	default:
	}

	handle := tr.getDriverHandle()
	if handle == nil {
		return nil
	}

	// Kill the task using an exponential backoff in-case of failures.
	result, killErr := tr.killTask(handle, resultCh)
	if killErr != nil {
		// We couldn't successfully destroy the resource created.
		tr.logger.Error("failed to kill task. Resources may have been leaked", "error", killErr)
		tr.setKillErr(killErr)
	}

	if result != nil {
		return result
	}

	// Block until task has exited.
	if resultCh == nil {
		var err error
		resultCh, err = handle.WaitCh(tr.shutdownCtx)

		// The error should be nil or TaskNotFound, if it's something else then a
		// failure in the driver or transport layer occurred
		if err != nil {
			if err == drivers.ErrTaskNotFound {
				return nil
			}
			tr.logger.Error("failed to wait on task. Resources may have been leaked", "error", err)
			tr.setKillErr(killErr)
			return nil
		}
	}

	select {
	case result := <-resultCh:
		return result
	case <-tr.shutdownCtx.Done():
		return nil
	}
}

// killTask kills the task handle. In the case that killing fails,
// killTask will retry with an exponential backoff and will give up at a
// given limit. Returns an error if the task could not be killed.
func (tr *TaskRunner) killTask(handle *DriverHandle, resultCh <-chan *drivers.ExitResult) (*drivers.ExitResult, error) {
	// Cap the number of times we attempt to kill the task.
	var err error
	for i := 0; i < killFailureLimit; i++ {
		if err = handle.Kill(); err != nil {
			if err == drivers.ErrTaskNotFound {
				tr.logger.Warn("couldn't find task to kill", "task_id", handle.ID())
				return nil, nil
			}
			// Calculate the new backoff
			backoff := (1 << (2 * uint64(i))) * killBackoffBaseline
			if backoff > killBackoffLimit {
				backoff = killBackoffLimit
			}

			tr.logger.Error("failed to kill task", "backoff", backoff, "error", err)
			select {
			case result := <-resultCh:
				return result, nil
			case <-time.After(backoff):
			}
		} else {
			// Kill was successful
			return nil, nil
		}
	}
	return nil, err
}

// persistLocalState persists local state to disk synchronously.
func (tr *TaskRunner) persistLocalState() error {
	tr.stateLock.RLock()
	defer tr.stateLock.RUnlock()

	return tr.stateDB.PutTaskRunnerLocalState(tr.allocID, tr.taskName, tr.localState)
}

// buildTaskConfig builds a drivers.TaskConfig with an unique ID for the task.
// The ID is unique for every invocation, it is built from the alloc ID, task
// name and 8 random characters.
func (tr *TaskRunner) buildTaskConfig() *drivers.TaskConfig {
	task := tr.Task()
	alloc := tr.Alloc()
	invocationid := uuid.Short()
	taskResources := tr.taskResources
	ports := tr.Alloc().AllocatedResources.Shared.Ports
	env := tr.envBuilder.Build()
	tr.networkIsolationLock.Lock()
	defer tr.networkIsolationLock.Unlock()

	var dns *drivers.DNSConfig

	// set DNS from any CNI plugins
	netStatus := tr.allocHookResources.GetAllocNetworkStatus()
	if netStatus != nil && netStatus.DNS != nil {
		dns = &drivers.DNSConfig{
			Servers:  netStatus.DNS.Servers,
			Searches: netStatus.DNS.Searches,
			Options:  netStatus.DNS.Options,
		}
	}

	// override DNS if set by job submitter
	if alloc.AllocatedResources != nil && len(alloc.AllocatedResources.Shared.Networks) > 0 {
		allocDNS := alloc.AllocatedResources.Shared.Networks[0].DNS
		if allocDNS != nil {
			interpolatedNetworks := taskenv.InterpolateNetworks(env, alloc.AllocatedResources.Shared.Networks)
			dns = &drivers.DNSConfig{
				Servers:  interpolatedNetworks[0].DNS.Servers,
				Searches: interpolatedNetworks[0].DNS.Searches,
				Options:  interpolatedNetworks[0].DNS.Options,
			}
		}
	}

	memoryLimit := taskResources.Memory.MemoryMB
	if max := taskResources.Memory.MemoryMaxMB; max > memoryLimit {
		memoryLimit = max
	}

	cpusetCpus := make([]string, len(taskResources.Cpu.ReservedCores))
	for i, v := range taskResources.Cpu.ReservedCores {
		cpusetCpus[i] = fmt.Sprintf("%d", v)
	}

	return &drivers.TaskConfig{
		ID:            fmt.Sprintf("%s/%s/%s", alloc.ID, task.Name, invocationid),
		Name:          task.Name,
		JobName:       alloc.Job.Name,
		JobID:         alloc.Job.ID,
		TaskGroupName: alloc.TaskGroup,
		Namespace:     alloc.Namespace,
		NodeName:      alloc.NodeName,
		NodeID:        alloc.NodeID,
		ParentJobID:   alloc.Job.ParentID,
		Resources: &drivers.Resources{
			NomadResources: taskResources,
			LinuxResources: &drivers.LinuxResources{
				MemoryLimitBytes: memoryLimit * 1024 * 1024,
				CPUShares:        taskResources.Cpu.CpuShares,
				CpusetCpus:       strings.Join(cpusetCpus, ","),
				PercentTicks:     float64(taskResources.Cpu.CpuShares) / float64(tr.clientConfig.Node.NodeResources.Processors.Topology.UsableCompute()),
			},
			Ports: &ports,
		},
		Devices:          tr.hookResources.getDevices(),
		Mounts:           tr.hookResources.getMounts(),
		Env:              env.Map(),
		DeviceEnv:        env.DeviceEnv(),
		User:             task.User,
		AllocDir:         tr.taskDir.AllocDir,
		StdoutPath:       tr.logmonHookConfig.stdoutFifo,
		StderrPath:       tr.logmonHookConfig.stderrFifo,
		AllocID:          tr.allocID,
		NetworkIsolation: tr.networkIsolationSpec,
		DNS:              dns,
	}
}

// Restore task runner state. Called by AllocRunner.Restore after NewTaskRunner
// but before Run so no locks need to be acquired.
func (tr *TaskRunner) Restore() error {
	ls, ts, err := tr.stateDB.GetTaskRunnerState(tr.allocID, tr.taskName)
	if err != nil {
		return err
	}

	if ls != nil {
		ls.Canonicalize()
		tr.localState = ls
	}

	if ts != nil {
		ts.Canonicalize()
		tr.state = ts
	}

	// If a TaskHandle was persisted, ensure it is valid or destroy it.
	if taskHandle := tr.localState.TaskHandle; taskHandle != nil {
		//TODO if RecoverTask returned the DriverNetwork we wouldn't
		//     have to persist it at all!
		restored := tr.restoreHandle(taskHandle, tr.localState.DriverNetwork)

		// If the handle could not be restored, the alloc is
		// non-terminal, and the task isn't a system job: wait until
		// servers have been contacted before running. #1795
		if restored {
			return nil
		}

		alloc := tr.Alloc()
		if tr.state.State == structs.TaskStateDead || alloc.TerminalStatus() || alloc.Job.Type == structs.JobTypeSystem {
			return nil
		}

		tr.logger.Trace("failed to reattach to task; will not run until server is contacted")
		tr.waitOnServers = true

		ev := structs.NewTaskEvent(structs.TaskRestoreFailed).
			SetDisplayMessage("failed to restore task; will not run until server is contacted")
		tr.UpdateState(structs.TaskStatePending, ev)
	}

	return nil
}

// restoreHandle ensures a TaskHandle is valid by calling Driver.RecoverTask
// and sets the driver handle. If the TaskHandle is not valid, DestroyTask is
// called.
func (tr *TaskRunner) restoreHandle(taskHandle *drivers.TaskHandle, net *drivers.DriverNetwork) (success bool) {
	// Ensure handle is well-formed
	if taskHandle.Config == nil {
		return true
	}

	if err := tr.driver.RecoverTask(taskHandle); err != nil {
		if tr.TaskState().State != structs.TaskStateRunning {
			// RecoverTask should fail if the Task wasn't running
			return true
		}

		tr.logger.Error("error recovering task; cleaning up",
			"error", err, "task_id", taskHandle.Config.ID)

		// Try to cleanup any existing task state in the plugin before restarting
		if err := tr.driver.DestroyTask(taskHandle.Config.ID, true); err != nil {
			// Ignore ErrTaskNotFound errors as ideally
			// this task has already been stopped and
			// therefore doesn't exist.
			if err != drivers.ErrTaskNotFound {
				tr.logger.Warn("error destroying unrecoverable task",
					"error", err, "task_id", taskHandle.Config.ID)
			}

		}

		return false
	}

	// Update driver handle on task runner
	tr.setDriverHandle(NewDriverHandle(tr.driver, taskHandle.Config.ID, tr.Task(), tr.clientConfig.MaxKillTimeout, net))
	return true
}

// UpdateState sets the task runners allocation state and triggers a server
// update.
func (tr *TaskRunner) UpdateState(state string, event *structs.TaskEvent) {
	tr.stateLock.Lock()
	defer tr.stateLock.Unlock()

	tr.logger.Trace("setting task state", "state", state)

	if event != nil {
		// Append the event
		tr.appendEvent(event)
	}

	// Update the state
	if err := tr.updateStateImpl(state); err != nil {
		// Only log the error as we persistence errors should not
		// affect task state.
		tr.logger.Error("error persisting task state", "error", err, "event", event, "state", state)
	}

	// Store task handle for remote tasks
	if tr.driverCapabilities != nil && tr.driverCapabilities.RemoteTasks {
		tr.logger.Trace("storing remote task handle state")
		tr.localState.TaskHandle.Store(tr.state)
	}

	// Notify the alloc runner of the transition
	tr.stateUpdater.TaskStateUpdated()
}

// updateStateImpl updates the in-memory task state and persists to disk.
func (tr *TaskRunner) updateStateImpl(state string) error {

	// Update the task state
	oldState := tr.state.State
	taskState := tr.state
	taskState.State = state

	// Handle the state transition.
	switch state {
	case structs.TaskStateRunning:
		// Capture the start time if it is just starting
		if oldState != structs.TaskStateRunning {
			taskState.StartedAt = time.Now().UTC()
			metrics.IncrCounterWithLabels([]string{"client", "allocs", "running"}, 1, tr.baseLabels)
		}
	case structs.TaskStateDead:
		// Capture the finished time if not already set
		if taskState.FinishedAt.IsZero() {
			taskState.FinishedAt = time.Now().UTC()
		}

		// Emitting metrics to indicate task complete and failures
		if taskState.Failed {
			metrics.IncrCounterWithLabels([]string{"client", "allocs", "failed"}, 1, tr.baseLabels)
		} else {
			metrics.IncrCounterWithLabels([]string{"client", "allocs", "complete"}, 1, tr.baseLabels)
		}
	}

	// Persist the state and event
	return tr.stateDB.PutTaskState(tr.allocID, tr.taskName, taskState)
}

// EmitEvent appends a new TaskEvent to this task's TaskState. The actual
// TaskState.State (pending, running, dead) is not changed. Use UpdateState to
// transition states.
// Events are persisted locally and sent to the server, but errors are simply
// logged. Use AppendEvent to simply add a new event.
func (tr *TaskRunner) EmitEvent(event *structs.TaskEvent) {
	tr.stateLock.Lock()
	defer tr.stateLock.Unlock()

	tr.appendEvent(event)

	if err := tr.stateDB.PutTaskState(tr.allocID, tr.taskName, tr.state); err != nil {
		// Only a warning because the next event/state-transition will
		// try to persist it again.
		tr.logger.Warn("error persisting event", "error", err, "event", event)
	}

	// Notify the alloc runner of the event
	tr.stateUpdater.TaskStateUpdated()
}

// AppendEvent appends a new TaskEvent to this task's TaskState. The actual
// TaskState.State (pending, running, dead) is not changed. Use UpdateState to
// transition states.
// Events are persisted locally and errors are simply logged. Use EmitEvent
// also update AllocRunner.
func (tr *TaskRunner) AppendEvent(event *structs.TaskEvent) {
	tr.stateLock.Lock()
	defer tr.stateLock.Unlock()

	tr.appendEvent(event)

	if err := tr.stateDB.PutTaskState(tr.allocID, tr.taskName, tr.state); err != nil {
		// Only a warning because the next event/state-transition will
		// try to persist it again.
		tr.logger.Warn("error persisting event", "error", err, "event", event)
	}
}

// appendEvent to task's event slice. Caller must acquire stateLock.
func (tr *TaskRunner) appendEvent(event *structs.TaskEvent) error {
	// Ensure the event is populated with human readable strings
	event.PopulateEventDisplayMessage()

	// Propagate failure from event to task state
	if event.FailsTask {
		tr.state.Failed = true
	}

	// XXX This seems like a super awkward spot for this? Why not shouldRestart?
	// Update restart metrics
	if event.Type == structs.TaskRestarting {
		metrics.IncrCounterWithLabels([]string{"client", "allocs", "restart"}, 1, tr.baseLabels)
		tr.state.Restarts++
		tr.state.LastRestart = time.Unix(0, event.Time)
	}

	tr.logger.Info("Task event", "type", event.Type, "msg", event.DisplayMessage, "failed", event.FailsTask)

	// Append event to slice
	appendTaskEvent(tr.state, event, tr.maxEvents)

	return nil
}

// WaitCh is closed when TaskRunner.Run exits.
func (tr *TaskRunner) WaitCh() <-chan struct{} {
	return tr.waitCh
}

// Update the running allocation with a new version received from the server.
// Calls Update hooks asynchronously with Run.
//
// This method is safe for calling concurrently with Run and does not modify
// the passed in allocation.
func (tr *TaskRunner) Update(update *structs.Allocation) {
	task := update.LookupTask(tr.taskName)
	if task == nil {
		// This should not happen and likely indicates a bug in the
		// server or client.
		tr.logger.Error("allocation update is missing task; killing",
			"group", update.TaskGroup)
		te := structs.NewTaskEvent(structs.TaskKilled).
			SetKillReason("update missing task").
			SetFailsTask()
		tr.Kill(context.Background(), te)
		return
	}

	// Update tr.alloc
	tr.setAlloc(update, task)

	// Trigger update hooks if not terminal
	if !update.TerminalStatus() {
		tr.triggerUpdateHooks()
	}
}

// SetNetworkIsolation is called by the PreRun allocation hook after configuring
// the network isolation for the allocation
func (tr *TaskRunner) SetNetworkIsolation(n *drivers.NetworkIsolationSpec) {
	tr.networkIsolationLock.Lock()
	tr.networkIsolationSpec = n
	tr.networkIsolationLock.Unlock()
}

// SetNetworkStatus is called from the allocrunner to propagate the
// network status of an allocation. This call occurs once the network hook has
// run and allows this information to be exported as env vars within the
// taskenv.
func (tr *TaskRunner) SetNetworkStatus(s *structs.AllocNetworkStatus) {
	tr.allocNetworkStatusLock.Lock()
	tr.allocNetworkStatus = s
	tr.allocNetworkStatusLock.Unlock()

	// Update the taskenv builder.
	tr.envBuilder = tr.envBuilder.SetNetworkStatus(s)
}

// triggerUpdate if there isn't already an update pending. Should be called
// instead of calling updateHooks directly to serialize runs of update hooks.
// TaskRunner state should be updated prior to triggering update hooks.
//
// Does not block.
func (tr *TaskRunner) triggerUpdateHooks() {
	select {
	case tr.triggerUpdateCh <- struct{}{}:
	default:
		// already an update hook pending
	}
}

// Shutdown TaskRunner gracefully without affecting the state of the task.
// Shutdown blocks until the main Run loop exits.
func (tr *TaskRunner) Shutdown() {
	tr.logger.Trace("shutting down")
	tr.shutdownCtxCancel()

	<-tr.WaitCh()

	// Run shutdown hooks to cleanup
	tr.shutdownHooks()

	// Persist once more
	tr.persistLocalState()
}

// LatestResourceUsage returns the last resource utilization datapoint
// collected. May return nil if the task is not running or no resource
// utilization has been collected yet.
func (tr *TaskRunner) LatestResourceUsage() *cstructs.TaskResourceUsage {
	tr.resourceUsageLock.Lock()
	ru := tr.resourceUsage
	tr.resourceUsageLock.Unlock()

	// Look up device statistics lazily when fetched, as currently we do not emit any stats for them yet
	if ru != nil && tr.deviceStatsReporter != nil {
		deviceResources := tr.taskResources.Devices
		ru.ResourceUsage.DeviceStats = tr.deviceStatsReporter.LatestDeviceResourceStats(deviceResources)
	}
	return ru
}

// UpdateStats updates and emits the latest stats from the driver.
func (tr *TaskRunner) UpdateStats(ru *cstructs.TaskResourceUsage) {
	tr.resourceUsageLock.Lock()
	tr.resourceUsage = ru
	tr.resourceUsageLock.Unlock()
	if ru != nil {
		tr.emitStats(ru)
	}
}

// TODO Remove Backwardscompat or use tr.Alloc()?
func (tr *TaskRunner) setGaugeForMemory(ru *cstructs.TaskResourceUsage) {
	alloc := tr.Alloc()
	var allocatedMem float32
	var allocatedMemMax float32
	if taskRes := alloc.AllocatedResources.Tasks[tr.taskName]; taskRes != nil {
		// Convert to bytes to match other memory metrics
		allocatedMem = float32(taskRes.Memory.MemoryMB) * 1024 * 1024
		allocatedMemMax = float32(taskRes.Memory.MemoryMaxMB) * 1024 * 1024
	}

	ms := ru.ResourceUsage.MemoryStats

	publishMetric := func(v uint64, reported, measured string) {
		if v != 0 || slices.Contains(ms.Measured, measured) {
			metrics.SetGaugeWithLabels([]string{"client", "allocs", "memory", reported},
				float32(v), tr.baseLabels)
		}
	}

	publishMetric(ms.RSS, "rss", "RSS")
	publishMetric(ms.Cache, "cache", "Cache")
	publishMetric(ms.Swap, "swap", "Swap")
	publishMetric(ms.MappedFile, "mapped_file", "Mapped File")
	publishMetric(ms.Usage, "usage", "Usage")
	publishMetric(ms.MaxUsage, "max_usage", "Max Usage")
	publishMetric(ms.KernelUsage, "kernel_usage", "Kernel Usage")
	publishMetric(ms.KernelMaxUsage, "kernel_max_usage", "Kernel Max Usage")
	if allocatedMem > 0 {
		metrics.SetGaugeWithLabels([]string{"client", "allocs", "memory", "allocated"},
			allocatedMem, tr.baseLabels)
	}
	if allocatedMemMax > 0 {
		metrics.SetGaugeWithLabels([]string{"client", "allocs", "memory", "max_allocated"},
			allocatedMemMax, tr.baseLabels)
	}
}

// TODO Remove Backwardscompat or use tr.Alloc()?
func (tr *TaskRunner) setGaugeForCPU(ru *cstructs.TaskResourceUsage) {
	alloc := tr.Alloc()
	var allocatedCPU float32
	if taskRes := alloc.AllocatedResources.Tasks[tr.taskName]; taskRes != nil {
		allocatedCPU = float32(taskRes.Cpu.CpuShares)
	}

	metrics.SetGaugeWithLabels([]string{"client", "allocs", "cpu", "total_percent"},
		float32(ru.ResourceUsage.CpuStats.Percent), tr.baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "allocs", "cpu", "system"},
		float32(ru.ResourceUsage.CpuStats.SystemMode), tr.baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "allocs", "cpu", "user"},
		float32(ru.ResourceUsage.CpuStats.UserMode), tr.baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "allocs", "cpu", "throttled_time"},
		float32(ru.ResourceUsage.CpuStats.ThrottledTime), tr.baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "allocs", "cpu", "throttled_periods"},
		float32(ru.ResourceUsage.CpuStats.ThrottledPeriods), tr.baseLabels)
	metrics.SetGaugeWithLabels([]string{"client", "allocs", "cpu", "total_ticks"},
		float32(ru.ResourceUsage.CpuStats.TotalTicks), tr.baseLabels)
	metrics.IncrCounterWithLabels([]string{"client", "allocs", "cpu", "total_ticks_count"},
		float32(ru.ResourceUsage.CpuStats.TotalTicks), tr.baseLabels)
	if allocatedCPU > 0 {
		metrics.SetGaugeWithLabels([]string{"client", "allocs", "cpu", "allocated"},
			allocatedCPU, tr.baseLabels)
	}
}

// emitStats emits resource usage stats of tasks to remote metrics collector
// sinks
func (tr *TaskRunner) emitStats(ru *cstructs.TaskResourceUsage) {
	if !tr.clientConfig.PublishAllocationMetrics {
		return
	}

	if ru.ResourceUsage.MemoryStats != nil {
		tr.setGaugeForMemory(ru)
	} else {
		tr.logger.Debug("Skipping memory stats for allocation", "reason", "MemoryStats is nil")
	}

	if ru.ResourceUsage.CpuStats != nil {
		tr.setGaugeForCPU(ru)
	} else {
		tr.logger.Debug("Skipping cpu stats for allocation", "reason", "CpuStats is nil")
	}
}

// appendTaskEvent updates the task status by appending the new event.
func appendTaskEvent(state *structs.TaskState, event *structs.TaskEvent, capacity int) {
	if state.Events == nil {
		state.Events = make([]*structs.TaskEvent, 1, capacity)
		state.Events[0] = event
		return
	}

	// If we hit capacity, then shift it.
	if len(state.Events) == capacity {
		old := state.Events
		state.Events = make([]*structs.TaskEvent, 0, capacity)
		state.Events = append(state.Events, old[1:]...)
	}

	state.Events = append(state.Events, event)
}

func (tr *TaskRunner) TaskExecHandler() drivermanager.TaskExecHandler {
	// Check it is running
	handle := tr.getDriverHandle()
	if handle == nil {
		return nil
	}
	return handle.ExecStreaming
}

func (tr *TaskRunner) DriverCapabilities() (*drivers.Capabilities, error) {
	return tr.driver.Capabilities()
}

// shutdownDelayCancel is used for testing only and cancels the
// shutdownDelayCtx
func (tr *TaskRunner) shutdownDelayCancel() {
	tr.shutdownDelayCancelFn()
}
