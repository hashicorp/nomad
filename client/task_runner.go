package client

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/client/vaultclient"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/nomad/client/driver/env"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

const (
	// killBackoffBaseline is the baseline time for exponential backoff while
	// killing a task.
	killBackoffBaseline = 5 * time.Second

	// killBackoffLimit is the limit of the exponential backoff for killing
	// the task.
	killBackoffLimit = 2 * time.Minute

	// killFailureLimit is how many times we will attempt to kill a task before
	// giving up and potentially leaking resources.
	killFailureLimit = 5

	// vaultBackoffBaseline is the baseline time for exponential backoff when
	// attempting to retrieve a Vault token
	vaultBackoffBaseline = 5 * time.Second

	// vaultBackoffLimit is the limit of the exponential backoff when attempting
	// to retrieve a Vault token
	vaultBackoffLimit = 3 * time.Minute

	// vaultTokenFile is the name of the file holding the Vault token inside the
	// task's secret directory
	vaultTokenFile = "vault_token"
)

// TaskRunner is used to wrap a task within an allocation and provide the execution context.
type TaskRunner struct {
	config         *config.Config
	updater        TaskStateUpdater
	logger         *log.Logger
	ctx            *driver.ExecContext
	alloc          *structs.Allocation
	restartTracker *RestartTracker

	// running marks whether the task is running
	running     bool
	runningLock sync.Mutex

	resourceUsage     *cstructs.TaskResourceUsage
	resourceUsageLock sync.RWMutex

	task    *structs.Task
	taskDir string

	// taskEnv is the environment variables of the task
	taskEnv     *env.TaskEnvironment
	taskEnvLock sync.Mutex

	// updateCh is used to receive updated versions of the allocation
	updateCh chan *structs.Allocation

	handle     driver.DriverHandle
	handleLock sync.Mutex

	// artifactsDownloaded tracks whether the tasks artifacts have been
	// downloaded
	artifactsDownloaded bool

	// vaultFuture is the means to wait for and get a Vault token
	vaultFuture *tokenFuture

	// recoveredVaultToken is the token that was recovered through a restore
	recoveredVaultToken string

	// vaultClient is used to retrieve and renew any needed Vault token
	vaultClient vaultclient.VaultClient

	// templateManager is used to manage any consul-templates this task may have
	templateManager *TaskTemplateManager

	// templatesRendered mark whether the templates have been rendered
	templatesRendered bool

	// startCh is used to trigger the start of the task
	startCh chan struct{}

	// unblockCh is used to unblock the starting of the task
	unblockCh   chan struct{}
	unblocked   bool
	unblockLock sync.Mutex

	// restartCh is used to restart a task
	restartCh chan *structs.TaskEvent

	// signalCh is used to send a signal to a task
	signalCh chan SignalEvent

	destroy      bool
	destroyCh    chan struct{}
	destroyLock  sync.Mutex
	destroyEvent *structs.TaskEvent

	// waitCh closing marks the run loop as having exited
	waitCh chan struct{}

	// serialize SaveState calls
	persistLock sync.Mutex
}

// taskRunnerState is used to snapshot the state of the task runner
type taskRunnerState struct {
	Version            string
	Task               *structs.Task
	HandleID           string
	ArtifactDownloaded bool
	TemplatesRendered  bool
}

// TaskStateUpdater is used to signal that tasks state has changed.
type TaskStateUpdater func(taskName, state string, event *structs.TaskEvent)

// SignalEvent is a tuple of the signal and the event generating it
type SignalEvent struct {
	// s is the signal to be sent
	s os.Signal

	// e is the task event generating the signal
	e *structs.TaskEvent

	// result should be used to send back the result of the signal
	result chan<- error
}

// NewTaskRunner is used to create a new task context
func NewTaskRunner(logger *log.Logger, config *config.Config,
	updater TaskStateUpdater, ctx *driver.ExecContext,
	alloc *structs.Allocation, task *structs.Task,
	vaultClient vaultclient.VaultClient) *TaskRunner {

	// Merge in the task resources
	task.Resources = alloc.TaskResources[task.Name]

	// Build the restart tracker.
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		logger.Printf("[ERR] client: alloc '%s' for missing task group '%s'", alloc.ID, alloc.TaskGroup)
		return nil
	}
	restartTracker := newRestartTracker(tg.RestartPolicy, alloc.Job.Type)

	// Get the task directory
	taskDir, ok := ctx.AllocDir.TaskDirs[task.Name]
	if !ok {
		logger.Printf("[ERR] client: task directory for alloc %q task %q couldn't be found", alloc.ID, task.Name)
		return nil
	}

	tc := &TaskRunner{
		config:         config,
		updater:        updater,
		logger:         logger,
		restartTracker: restartTracker,
		ctx:            ctx,
		alloc:          alloc,
		task:           task,
		taskDir:        taskDir,
		vaultClient:    vaultClient,
		vaultFuture:    NewTokenFuture().Set(""),
		updateCh:       make(chan *structs.Allocation, 64),
		destroyCh:      make(chan struct{}),
		waitCh:         make(chan struct{}),
		startCh:        make(chan struct{}, 1),
		unblockCh:      make(chan struct{}),
		restartCh:      make(chan *structs.TaskEvent),
		signalCh:       make(chan SignalEvent),
	}

	return tc
}

// MarkReceived marks the task as received.
func (r *TaskRunner) MarkReceived() {
	r.updater(r.task.Name, structs.TaskStatePending, structs.NewTaskEvent(structs.TaskReceived))
}

// WaitCh returns a channel to wait for termination
func (r *TaskRunner) WaitCh() <-chan struct{} {
	return r.waitCh
}

// stateFilePath returns the path to our state file
func (r *TaskRunner) stateFilePath() string {
	// Get the MD5 of the task name
	hashVal := md5.Sum([]byte(r.task.Name))
	hashHex := hex.EncodeToString(hashVal[:])
	dirName := fmt.Sprintf("task-%s", hashHex)

	// Generate the path
	path := filepath.Join(r.config.StateDir, "alloc", r.alloc.ID,
		dirName, "state.json")
	return path
}

// RestoreState is used to restore our state
func (r *TaskRunner) RestoreState() error {
	// Load the snapshot
	var snap taskRunnerState
	if err := restoreState(r.stateFilePath(), &snap); err != nil {
		return err
	}

	// Restore fields
	if snap.Task == nil {
		return fmt.Errorf("task runner snapshot include nil Task")
	} else {
		r.task = snap.Task
	}
	r.artifactsDownloaded = snap.ArtifactDownloaded
	r.templatesRendered = snap.TemplatesRendered

	if err := r.setTaskEnv(); err != nil {
		return fmt.Errorf("client: failed to create task environment for task %q in allocation %q: %v",
			r.task.Name, r.alloc.ID, err)
	}

	if r.task.Vault != nil {
		secretDir, err := r.ctx.AllocDir.GetSecretDir(r.task.Name)
		if err != nil {
			return fmt.Errorf("failed to determine task %s secret dir in alloc %q: %v", r.task.Name, r.alloc.ID, err)
		}

		// Read the token from the secret directory
		tokenPath := filepath.Join(secretDir, vaultTokenFile)
		data, err := ioutil.ReadFile(tokenPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to read token for task %q in alloc %q: %v", r.task.Name, r.alloc.ID, err)
			}

			// Token file doesn't exist
		} else {
			// Store the recovered token
			r.recoveredVaultToken = string(data)
		}
	}

	// Restore the driver
	if snap.HandleID != "" {
		driver, err := r.createDriver()
		if err != nil {
			return err
		}

		handle, err := driver.Open(r.ctx, snap.HandleID)

		// In the case it fails, we relaunch the task in the Run() method.
		if err != nil {
			r.logger.Printf("[ERR] client: failed to open handle to task '%s' for alloc '%s': %v",
				r.task.Name, r.alloc.ID, err)
			return nil
		}
		r.handleLock.Lock()
		r.handle = handle
		r.handleLock.Unlock()

		r.runningLock.Lock()
		r.running = true
		r.runningLock.Unlock()
	}
	return nil
}

// SaveState is used to snapshot our state
func (r *TaskRunner) SaveState() error {
	r.persistLock.Lock()
	defer r.persistLock.Unlock()

	snap := taskRunnerState{
		Task:               r.task,
		Version:            r.config.Version,
		ArtifactDownloaded: r.artifactsDownloaded,
		TemplatesRendered:  r.templatesRendered,
	}
	r.handleLock.Lock()
	if r.handle != nil {
		snap.HandleID = r.handle.ID()
	}
	r.handleLock.Unlock()
	return persistState(r.stateFilePath(), &snap)
}

// DestroyState is used to cleanup after ourselves
func (r *TaskRunner) DestroyState() error {
	return os.RemoveAll(r.stateFilePath())
}

// setState is used to update the state of the task runner
func (r *TaskRunner) setState(state string, event *structs.TaskEvent) {
	// Persist our state to disk.
	if err := r.SaveState(); err != nil {
		r.logger.Printf("[ERR] client: failed to save state of Task Runner for task %q: %v", r.task.Name, err)
	}

	// Indicate the task has been updated.
	r.updater(r.task.Name, state, event)
}

// setTaskEnv sets the task environment. It returns an error if it could not be
// created.
func (r *TaskRunner) setTaskEnv() error {
	r.taskEnvLock.Lock()
	defer r.taskEnvLock.Unlock()

	taskEnv, err := driver.GetTaskEnv(r.ctx.AllocDir, r.config.Node, r.task.Copy(), r.alloc, r.vaultFuture.Get())
	if err != nil {
		return err
	}
	r.taskEnv = taskEnv
	return nil
}

// getTaskEnv returns the task environment
func (r *TaskRunner) getTaskEnv() *env.TaskEnvironment {
	r.taskEnvLock.Lock()
	defer r.taskEnvLock.Unlock()
	return r.taskEnv
}

// createDriver makes a driver for the task
func (r *TaskRunner) createDriver() (driver.Driver, error) {
	env := r.getTaskEnv()
	if env == nil {
		return nil, fmt.Errorf("task environment not made for task %q in allocation %q", r.task.Name, r.alloc.ID)
	}

	driverCtx := driver.NewDriverContext(r.task.Name, r.config, r.config.Node, r.logger, env)
	driver, err := driver.NewDriver(r.task.Driver, driverCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver '%s' for alloc %s: %v",
			r.task.Driver, r.alloc.ID, err)
	}
	return driver, err
}

// Run is a long running routine used to manage the task
func (r *TaskRunner) Run() {
	defer close(r.waitCh)
	r.logger.Printf("[DEBUG] client: starting task context for '%s' (alloc '%s')",
		r.task.Name, r.alloc.ID)

	if err := r.validateTask(); err != nil {
		r.setState(
			structs.TaskStateDead,
			structs.NewTaskEvent(structs.TaskFailedValidation).SetValidationError(err))
		return
	}

	// If there is no Vault policy leave the static future created in
	// NewTaskRunner
	if r.task.Vault != nil {
		// Start the go-routine to get a Vault token
		r.vaultFuture.Clear()
		go r.vaultManager(r.recoveredVaultToken)
	}

	// Start the run loop
	r.run()

	// Do any cleanup necessary
	r.postrun()

	return
}

// validateTask validates the fields of the task and returns an error if the
// task is invalid.
func (r *TaskRunner) validateTask() error {
	var mErr multierror.Error

	// Validate the user.
	unallowedUsers := r.config.ReadStringListToMapDefault("user.blacklist", config.DefaultUserBlacklist)
	checkDrivers := r.config.ReadStringListToMapDefault("user.checked_drivers", config.DefaultUserCheckedDrivers)
	if _, driverMatch := checkDrivers[r.task.Driver]; driverMatch {
		if _, unallowed := unallowedUsers[r.task.User]; unallowed {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("running as user %q is disallowed", r.task.User))
		}
	}

	// Validate the artifacts
	for i, artifact := range r.task.Artifacts {
		// Verify the artifact doesn't escape the task directory.
		if err := artifact.Validate(); err != nil {
			// If this error occurs there is potentially a server bug or
			// mallicious, server spoofing.
			r.logger.Printf("[ERR] client: allocation %q, task %v, artifact %#v (%v) fails validation: %v",
				r.alloc.ID, r.task.Name, artifact, i, err)
			mErr.Errors = append(mErr.Errors, fmt.Errorf("artifact (%d) failed validation: %v", i, err))
		}
	}

	if len(mErr.Errors) == 1 {
		return mErr.Errors[0]
	}
	return mErr.ErrorOrNil()
}

// tokenFuture stores the Vault token and allows consumers to block till a valid
// token exists
type tokenFuture struct {
	waiting []chan struct{}
	token   string
	set     bool
	m       sync.Mutex
}

// NewTokenFuture returns a new token future without any token set
func NewTokenFuture() *tokenFuture {
	return &tokenFuture{}
}

// Wait returns a channel that can be waited on. When this channel unblocks, a
// valid token will be available via the Get method
func (f *tokenFuture) Wait() <-chan struct{} {
	f.m.Lock()
	defer f.m.Unlock()

	c := make(chan struct{})
	if f.set {
		close(c)
		return c
	}

	f.waiting = append(f.waiting, c)
	return c
}

// Set sets the token value and unblocks any caller of Wait
func (f *tokenFuture) Set(token string) *tokenFuture {
	f.m.Lock()
	defer f.m.Unlock()

	f.set = true
	f.token = token
	for _, w := range f.waiting {
		close(w)
	}
	f.waiting = nil
	return f
}

// Clear clears the set vault token.
func (f *tokenFuture) Clear() *tokenFuture {
	f.m.Lock()
	defer f.m.Unlock()

	f.token = ""
	f.set = false
	return f
}

// Get returns the set Vault token
func (f *tokenFuture) Get() string {
	f.m.Lock()
	defer f.m.Unlock()
	return f.token
}

// vaultManager should be called in a go-routine and manages the derivation,
// renewal and handling of errors with the Vault token. The optional parameter
// allows setting the initial Vault token. This is useful when the Vault token
// is recovered off disk.
func (r *TaskRunner) vaultManager(token string) {
	// updatedToken lets us store state between loops. If true, a new token
	// has been retrieved and we need to apply the Vault change mode
	var updatedToken bool

OUTER:
	for {
		// Check if we should exit
		select {
		case <-r.waitCh:
			return
		default:
		}

		// Clear the token
		r.vaultFuture.Clear()

		// Check if there already is a token which can be the case for
		// restoring the TaskRunner
		if token == "" {
			// Get a token
			var ok bool
			token, ok = r.deriveVaultToken()
			if !ok {
				// We are shutting down
				return
			}

			// Write the token to disk
			if err := r.writeToken(token); err != nil {
				e := fmt.Errorf("failed to write Vault token to disk")
				r.logger.Printf("[ERR] client: %v for task %v on alloc %q: %v", e, r.task.Name, r.alloc.ID, err)
				r.Kill("vault", e.Error())
				return
			}
		}

		// Start the renewal process
		renewCh, err := r.vaultClient.RenewToken(token, 30)

		// An error returned means the token is not being renewed
		if err != nil {
			r.logger.Printf("[ERR] client: failed to start renewal of Vault token for task %v on alloc %q: %v", r.task.Name, r.alloc.ID, err)
			token = ""
			goto OUTER
		}

		// The Vault token is valid now, so set it
		r.vaultFuture.Set(token)

		if updatedToken {
			switch r.task.Vault.ChangeMode {
			case structs.VaultChangeModeSignal:
				s, err := signals.Parse(r.task.Vault.ChangeSignal)
				if err != nil {
					e := fmt.Errorf("failed to parse signal: %v", err)
					r.logger.Printf("[ERR] client: %v", err)
					r.Kill("vault", e.Error())
					return
				}

				if err := r.Signal("vault", "new Vault token acquired", s); err != nil {
					r.logger.Printf("[ERR] client: failed to send signal to task %v for alloc %q: %v", r.task.Name, r.alloc.ID, err)
					r.Kill("vault", fmt.Sprintf("failed to send signal to task: %v", err))
					return
				}
			case structs.VaultChangeModeRestart:
				r.Restart("vault", "new Vault token acquired")
			case structs.VaultChangeModeNoop:
				fallthrough
			default:
				r.logger.Printf("[ERR] client: Invalid Vault change mode: %q", r.task.Vault.ChangeMode)
			}

			// We have handled it
			updatedToken = false

			// Call the handler
			r.updatedTokenHandler()
		}

		// Start watching for renewal errors
		select {
		case err := <-renewCh:
			// Clear the token
			token = ""
			r.logger.Printf("[ERR] client: failed to renew Vault token for task %v on alloc %q: %v", r.task.Name, r.alloc.ID, err)

			// Check if we have to do anything
			if r.task.Vault.ChangeMode != structs.VaultChangeModeNoop {
				updatedToken = true
			}
		case <-r.waitCh:
			return
		}
	}
}

// deriveVaultToken derives the Vault token using exponential backoffs. It
// returns the Vault token and whether the token is valid. If it is not valid we
// are shutting down
func (r *TaskRunner) deriveVaultToken() (string, bool) {
	attempts := 0
	for {
		tokens, err := r.vaultClient.DeriveToken(r.alloc, []string{r.task.Name})
		if err == nil {
			return tokens[r.task.Name], true
		}

		// Handle the retry case
		backoff := (1 << (2 * uint64(attempts))) * vaultBackoffBaseline
		if backoff > vaultBackoffLimit {
			backoff = vaultBackoffLimit
		}
		r.logger.Printf("[ERR] client: failed to derive Vault token for task %v on alloc %q: %v; retrying in %v", r.task.Name, r.alloc.ID, err, backoff)

		attempts++

		// Wait till retrying
		select {
		case <-r.waitCh:
			return "", false
		case <-time.After(backoff):
		}
	}
}

// writeToken writes the given token to disk
func (r *TaskRunner) writeToken(token string) error {
	// Write the token to disk
	secretDir, err := r.ctx.AllocDir.GetSecretDir(r.task.Name)
	if err != nil {
		return fmt.Errorf("failed to determine task %s secret dir in alloc %q: %v", r.task.Name, r.alloc.ID, err)
	}

	// Write the token to the file system
	tokenPath := filepath.Join(secretDir, vaultTokenFile)
	if err := ioutil.WriteFile(tokenPath, []byte(token), 0777); err != nil {
		return fmt.Errorf("failed to save Vault tokens to secret dir for task %q in alloc %q: %v", r.task.Name, r.alloc.ID, err)
	}

	return nil
}

// updatedTokenHandler is called when a new Vault token is retrieved. Things
// that rely on the token should be updated here.
func (r *TaskRunner) updatedTokenHandler() {

	// Update the tasks environment
	if err := r.setTaskEnv(); err != nil {
		r.setState(
			structs.TaskStateDead,
			structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(err))
		return
	}

	if r.templateManager != nil {
		r.templateManager.Stop()

		// Create a new templateManager
		var err error
		r.templateManager, err = NewTaskTemplateManager(r, r.task.Templates, r.templatesRendered,
			r.config, r.vaultFuture.Get(), r.taskDir, r.getTaskEnv())
		if err != nil {
			err := fmt.Errorf("failed to build task's template manager: %v", err)
			r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskSetupFailure).SetSetupError(err))
			r.logger.Printf("[ERR] client: alloc %q, task %q %v", r.alloc.ID, r.task.Name, err)
			return
		}
	}
}

// prestart handles life-cycle tasks that occur before the task has started.
func (r *TaskRunner) prestart(resultCh chan bool) {

	if r.task.Vault != nil {
		// Wait for the token
		r.logger.Printf("[DEBUG] client: waiting for Vault token for task %v in alloc %q", r.task.Name, r.alloc.ID)
		tokenCh := r.vaultFuture.Wait()
		select {
		case <-tokenCh:
		case <-r.waitCh:
			resultCh <- false
			return
		}
		r.logger.Printf("[DEBUG] client: retrieved Vault token for task %v in alloc %q", r.task.Name, r.alloc.ID)
	}

	if err := r.setTaskEnv(); err != nil {
		r.setState(
			structs.TaskStateDead,
			structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(err))
		resultCh <- false
		return
	}

	// Build the template manager
	if r.templateManager == nil {
		var err error
		r.templateManager, err = NewTaskTemplateManager(r, r.task.Templates, r.templatesRendered,
			r.config, r.vaultFuture.Get(), r.taskDir, r.getTaskEnv())
		if err != nil {
			err := fmt.Errorf("failed to build task's template manager: %v", err)
			r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskSetupFailure).SetSetupError(err))
			r.logger.Printf("[ERR] client: alloc %q, task %q %v", r.alloc.ID, r.task.Name, err)
			resultCh <- false
			return
		}
	}

	for {
		// Download the task's artifacts
		if !r.artifactsDownloaded && len(r.task.Artifacts) > 0 {
			r.setState(structs.TaskStatePending, structs.NewTaskEvent(structs.TaskDownloadingArtifacts))
			for _, artifact := range r.task.Artifacts {
				if err := getter.GetArtifact(r.getTaskEnv(), artifact, r.taskDir); err != nil {
					r.setState(structs.TaskStatePending,
						structs.NewTaskEvent(structs.TaskArtifactDownloadFailed).SetDownloadError(err))
					r.restartTracker.SetStartError(dstructs.NewRecoverableError(err, true))
					goto RESTART
				}
			}

			r.artifactsDownloaded = true
		}

		// We don't have to wait for any template
		if len(r.task.Templates) == 0 || r.templatesRendered {
			// Send the start signal
			select {
			case r.startCh <- struct{}{}:
			default:
			}

			resultCh <- true
			return
		}

		// Block for consul-template
		// TODO Hooks should register themselves as blocking and then we can
		// perioidcally enumerate what we are still blocked on
		select {
		case <-r.unblockCh:
			r.templatesRendered = true

			// Send the start signal
			select {
			case r.startCh <- struct{}{}:
			default:
			}

			resultCh <- true
			return
		case <-r.waitCh:
			// The run loop has exited so exit too
			resultCh <- false
			return
		}

	RESTART:
		restart := r.shouldRestart()
		if !restart {
			resultCh <- false
			return
		}
	}
}

// postrun is used to do any cleanup that is necessary after exiting the runloop
func (r *TaskRunner) postrun() {
	// Stop the template manager
	if r.templateManager != nil {
		r.templateManager.Stop()
	}
}

// run is the main run loop that handles starting the application, destroying
// it, restarts and signals.
func (r *TaskRunner) run() {
	// Predeclare things so we can jump to the RESTART
	var stopCollection chan struct{}
	var handleWaitCh chan *dstructs.WaitResult

	for {
		// Do the prestart activities
		prestartResultCh := make(chan bool, 1)
		go r.prestart(prestartResultCh)

	WAIT:
		for {
			select {
			case success := <-prestartResultCh:
				if !success {
					return
				}
			case <-r.startCh:
				// Start the task if not yet started or it is being forced. This logic
				// is necessary because in the case of a restore the handle already
				// exists.
				r.handleLock.Lock()
				handleEmpty := r.handle == nil
				r.handleLock.Unlock()

				if handleEmpty {
					startErr := r.startTask()
					r.restartTracker.SetStartError(startErr)
					if startErr != nil {
						r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskDriverFailure).SetDriverError(startErr))
						goto RESTART
					}

					// Mark the task as started
					r.setState(structs.TaskStateRunning, structs.NewTaskEvent(structs.TaskStarted))
					r.runningLock.Lock()
					r.running = true
					r.runningLock.Unlock()
				}

				if stopCollection == nil {
					stopCollection = make(chan struct{})
					go r.collectResourceUsageStats(stopCollection)
				}

				handleWaitCh = r.handle.WaitCh()

			case waitRes := <-handleWaitCh:
				if waitRes == nil {
					panic("nil wait")
				}

				r.runningLock.Lock()
				r.running = false
				r.runningLock.Unlock()

				// Stop collection of the task's resource usage
				close(stopCollection)

				// Log whether the task was successful or not.
				r.restartTracker.SetWaitResult(waitRes)
				r.setState(structs.TaskStateDead, r.waitErrorToEvent(waitRes))
				if !waitRes.Successful() {
					r.logger.Printf("[INFO] client: task %q for alloc %q failed: %v", r.task.Name, r.alloc.ID, waitRes)
				} else {
					r.logger.Printf("[INFO] client: task %q for alloc %q completed successfully", r.task.Name, r.alloc.ID)
				}

				break WAIT
			case update := <-r.updateCh:
				if err := r.handleUpdate(update); err != nil {
					r.logger.Printf("[ERR] client: update to task %q failed: %v", r.task.Name, err)
				}

			case se := <-r.signalCh:
				r.logger.Printf("[DEBUG] client: task being signalled with %v: %s", se.s, se.e.TaskSignalReason)
				r.setState(structs.TaskStateRunning, se.e)

				res := r.handle.Signal(se.s)
				se.result <- res

			case event := <-r.restartCh:
				r.logger.Printf("[DEBUG] client: task being restarted: %s", event.RestartReason)
				r.setState(structs.TaskStateRunning, event)
				r.killTask(event.RestartReason)

				close(stopCollection)

				// Since the restart isn't from a failure, restart immediately
				// and don't count against the restart policy
				r.restartTracker.SetRestartTriggered()
				break WAIT

			case <-r.destroyCh:
				r.runningLock.Lock()
				running := r.running
				r.runningLock.Unlock()
				if !running {
					r.setState(structs.TaskStateDead, r.destroyEvent)
					return
				}

				// Store the task event that provides context on the task
				// destroy. The Killed event is set from the alloc_runner and
				// doesn't add detail
				reason := ""
				if r.destroyEvent.Type != structs.TaskKilled {
					if r.destroyEvent.Type == structs.TaskKilling {
						reason = r.destroyEvent.KillReason
					} else {
						r.setState(structs.TaskStateRunning, r.destroyEvent)
					}
				}

				r.killTask(reason)
				close(stopCollection)
				return
			}
		}

	RESTART:
		restart := r.shouldRestart()
		if !restart {
			return
		}

		// Clear the handle so a new driver will be created.
		r.handleLock.Lock()
		r.handle = nil
		handleWaitCh = nil
		stopCollection = nil
		r.handleLock.Unlock()
	}
}

// shouldRestart returns if the task should restart. If the return value is
// true, the task's restart policy has already been considered and any wait time
// between restarts has been applied.
func (r *TaskRunner) shouldRestart() bool {
	state, when := r.restartTracker.GetState()
	reason := r.restartTracker.GetReason()
	switch state {
	case structs.TaskNotRestarting, structs.TaskTerminated:
		r.logger.Printf("[INFO] client: Not restarting task: %v for alloc: %v ", r.task.Name, r.alloc.ID)
		if state == structs.TaskNotRestarting {
			r.setState(structs.TaskStateDead,
				structs.NewTaskEvent(structs.TaskNotRestarting).
					SetRestartReason(reason))
		}
		return false
	case structs.TaskRestarting:
		r.logger.Printf("[INFO] client: Restarting task %q for alloc %q in %v", r.task.Name, r.alloc.ID, when)
		r.setState(structs.TaskStatePending,
			structs.NewTaskEvent(structs.TaskRestarting).
				SetRestartDelay(when).
				SetRestartReason(reason))
	default:
		r.logger.Printf("[ERR] client: restart tracker returned unknown state: %q", state)
		return false
	}

	// Sleep but watch for destroy events.
	select {
	case <-time.After(when):
	case <-r.destroyCh:
	}

	// Destroyed while we were waiting to restart, so abort.
	r.destroyLock.Lock()
	destroyed := r.destroy
	r.destroyLock.Unlock()
	if destroyed {
		r.logger.Printf("[DEBUG] client: Not restarting task: %v because it has been destroyed due to: %s", r.task.Name, r.destroyEvent.Message)
		r.setState(structs.TaskStateDead, r.destroyEvent)
		return false
	}

	return true
}

// killTask kills the running task, storing the reason in the Killing TaskEvent.
func (r *TaskRunner) killTask(reason string) {
	r.runningLock.Lock()
	running := r.running
	r.runningLock.Unlock()
	if !running {
		return
	}

	// Mark that we received the kill event
	timeout := driver.GetKillTimeout(r.task.KillTimeout, r.config.MaxKillTimeout)
	r.setState(structs.TaskStateRunning,
		structs.NewTaskEvent(structs.TaskKilling).SetKillTimeout(timeout).SetKillReason(reason))

	// Kill the task using an exponential backoff in-case of failures.
	destroySuccess, err := r.handleDestroy()
	if !destroySuccess {
		// We couldn't successfully destroy the resource created.
		r.logger.Printf("[ERR] client: failed to kill task %q. Resources may have been leaked: %v", r.task.Name, err)
	}

	r.runningLock.Lock()
	r.running = false
	r.runningLock.Unlock()

	// Store that the task has been destroyed and any associated error.
	r.setState(structs.TaskStateDead, structs.NewTaskEvent(structs.TaskKilled).SetKillError(err))
}

// startTask creates the driver and starts the task.
func (r *TaskRunner) startTask() error {
	// Create a driver
	driver, err := r.createDriver()
	if err != nil {
		return fmt.Errorf("failed to create driver of task '%s' for alloc '%s': %v",
			r.task.Name, r.alloc.ID, err)
	}

	// Start the job
	handle, err := driver.Start(r.ctx, r.task)
	if err != nil {
		return fmt.Errorf("failed to start task '%s' for alloc '%s': %v",
			r.task.Name, r.alloc.ID, err)
	}

	r.handleLock.Lock()
	r.handle = handle
	r.handleLock.Unlock()
	return nil
}

// collectResourceUsageStats starts collecting resource usage stats of a Task.
// Collection ends when the passed channel is closed
func (r *TaskRunner) collectResourceUsageStats(stopCollection <-chan struct{}) {
	// start collecting the stats right away and then start collecting every
	// collection interval
	next := time.NewTimer(0)
	defer next.Stop()
	for {
		select {
		case <-next.C:
			next.Reset(r.config.StatsCollectionInterval)
			if r.handle == nil {
				continue
			}
			ru, err := r.handle.Stats()

			if err != nil {
				// We do not log when the plugin is shutdown as this is simply a
				// race between the stopCollection channel being closed and calling
				// Stats on the handle.
				if !strings.Contains(err.Error(), "connection is shut down") {
					r.logger.Printf("[WARN] client: error fetching stats of task %v: %v", r.task.Name, err)
				}
				continue
			}

			r.resourceUsageLock.Lock()
			r.resourceUsage = ru
			r.resourceUsageLock.Unlock()
			if ru != nil {
				r.emitStats(ru)
			}
		case <-stopCollection:
			return
		}
	}
}

// LatestResourceUsage returns the last resource utilization datapoint collected
func (r *TaskRunner) LatestResourceUsage() *cstructs.TaskResourceUsage {
	r.resourceUsageLock.RLock()
	defer r.resourceUsageLock.RUnlock()
	r.runningLock.Lock()
	defer r.runningLock.Unlock()

	// If the task is not running there can be no latest resource
	if !r.running {
		return nil
	}

	return r.resourceUsage
}

// handleUpdate takes an updated allocation and updates internal state to
// reflect the new config for the task.
func (r *TaskRunner) handleUpdate(update *structs.Allocation) error {
	// Extract the task group from the alloc.
	tg := update.Job.LookupTaskGroup(update.TaskGroup)
	if tg == nil {
		return fmt.Errorf("alloc '%s' missing task group '%s'", update.ID, update.TaskGroup)
	}

	// Extract the task.
	var updatedTask *structs.Task
	for _, t := range tg.Tasks {
		if t.Name == r.task.Name {
			updatedTask = t
		}
	}
	if updatedTask == nil {
		return fmt.Errorf("task group %q doesn't contain task %q", tg.Name, r.task.Name)
	}

	// Merge in the task resources
	updatedTask.Resources = update.TaskResources[updatedTask.Name]

	// Update will update resources and store the new kill timeout.
	var mErr multierror.Error
	r.handleLock.Lock()
	if r.handle != nil {
		if err := r.handle.Update(updatedTask); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("updating task resources failed: %v", err))
		}
	}
	r.handleLock.Unlock()

	// Update the restart policy.
	if r.restartTracker != nil {
		r.restartTracker.SetPolicy(tg.RestartPolicy)
	}

	// Store the updated alloc.
	r.alloc = update
	r.task = updatedTask
	return mErr.ErrorOrNil()
}

// handleDestroy kills the task handle. In the case that killing fails,
// handleDestroy will retry with an exponential backoff and will give up at a
// given limit. It returns whether the task was destroyed and the error
// associated with the last kill attempt.
func (r *TaskRunner) handleDestroy() (destroyed bool, err error) {
	// Cap the number of times we attempt to kill the task.
	for i := 0; i < killFailureLimit; i++ {
		if err = r.handle.Kill(); err != nil {
			// Calculate the new backoff
			backoff := (1 << (2 * uint64(i))) * killBackoffBaseline
			if backoff > killBackoffLimit {
				backoff = killBackoffLimit
			}

			r.logger.Printf("[ERR] client: failed to kill task '%s' for alloc %q. Retrying in %v: %v",
				r.task.Name, r.alloc.ID, backoff, err)
			time.Sleep(time.Duration(backoff))
		} else {
			// Kill was successful
			return true, nil
		}
	}
	return
}

// Restart will restart the task
func (r *TaskRunner) Restart(source, reason string) {

	reasonStr := fmt.Sprintf("%s: %s", source, reason)
	event := structs.NewTaskEvent(structs.TaskRestartSignal).SetRestartReason(reasonStr)

	r.logger.Printf("[DEBUG] client: restarting task %v for alloc %q: %v",
		r.task.Name, r.alloc.ID, reasonStr)

	r.runningLock.Lock()
	running := r.running
	r.runningLock.Unlock()

	// Drop the restart event
	if !running {
		r.logger.Printf("[DEBUG] client: skipping restart since task isn't running")
		return
	}

	select {
	case r.restartCh <- event:
	case <-r.waitCh:
	}
}

// Signal will send a signal to the task
func (r *TaskRunner) Signal(source, reason string, s os.Signal) error {

	reasonStr := fmt.Sprintf("%s: %s", source, reason)
	event := structs.NewTaskEvent(structs.TaskSignaling).SetTaskSignal(s).SetTaskSignalReason(reasonStr)

	r.logger.Printf("[DEBUG] client: sending signal %v to task %v for alloc %q", s, r.task.Name, r.alloc.ID)

	r.runningLock.Lock()
	running := r.running
	r.runningLock.Unlock()

	// Drop the restart event
	if !running {
		r.logger.Printf("[DEBUG] client: skipping signal since task isn't running")
		return nil
	}

	resCh := make(chan error)
	se := SignalEvent{
		s:      s,
		e:      event,
		result: resCh,
	}
	select {
	case r.signalCh <- se:
	case <-r.waitCh:
	}

	return <-resCh
}

// Kill will kill a task and store the error, no longer restarting the task
// TODO need to be able to fail the task
func (r *TaskRunner) Kill(source, reason string) {
	reasonStr := fmt.Sprintf("%s: %s", source, reason)
	event := structs.NewTaskEvent(structs.TaskKilling).SetKillReason(reasonStr)

	r.logger.Printf("[DEBUG] client: killing task %v for alloc %q: %v", r.task.Name, r.alloc.ID, reasonStr)
	r.Destroy(event)
}

// UnblockStart unblocks the starting of the task. It currently assumes only
// consul-template will unblock
func (r *TaskRunner) UnblockStart(source string) {
	r.unblockLock.Lock()
	defer r.unblockLock.Unlock()
	if r.unblocked {
		return
	}

	r.logger.Printf("[DEBUG] client: unblocking task %v for alloc %q: %v", r.task.Name, r.alloc.ID, source)
	r.unblocked = true
	close(r.unblockCh)
}

// Helper function for converting a WaitResult into a TaskTerminated event.
func (r *TaskRunner) waitErrorToEvent(res *dstructs.WaitResult) *structs.TaskEvent {
	return structs.NewTaskEvent(structs.TaskTerminated).
		SetExitCode(res.ExitCode).
		SetSignal(res.Signal).
		SetExitMessage(res.Err)
}

// Update is used to update the task of the context
func (r *TaskRunner) Update(update *structs.Allocation) {
	select {
	case r.updateCh <- update:
	default:
		r.logger.Printf("[ERR] client: dropping task update '%s' (alloc '%s')",
			r.task.Name, r.alloc.ID)
	}
}

// Destroy is used to indicate that the task context should be destroyed. The
// event parameter provides a context for the destroy.
func (r *TaskRunner) Destroy(event *structs.TaskEvent) {
	r.destroyLock.Lock()
	defer r.destroyLock.Unlock()

	if r.destroy {
		return
	}
	r.destroy = true
	r.destroyEvent = event
	close(r.destroyCh)
}

// emitStats emits resource usage stats of tasks to remote metrics collector
// sinks
func (r *TaskRunner) emitStats(ru *cstructs.TaskResourceUsage) {
	if ru.ResourceUsage.MemoryStats != nil && r.config.PublishAllocationMetrics {
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "memory", "rss"}, float32(ru.ResourceUsage.MemoryStats.RSS))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "memory", "cache"}, float32(ru.ResourceUsage.MemoryStats.Cache))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "memory", "swap"}, float32(ru.ResourceUsage.MemoryStats.Swap))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "memory", "max_usage"}, float32(ru.ResourceUsage.MemoryStats.MaxUsage))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "memory", "kernel_usage"}, float32(ru.ResourceUsage.MemoryStats.KernelUsage))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "memory", "kernel_max_usage"}, float32(ru.ResourceUsage.MemoryStats.KernelMaxUsage))
	}

	if ru.ResourceUsage.CpuStats != nil && r.config.PublishAllocationMetrics {
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "cpu", "total_percent"}, float32(ru.ResourceUsage.CpuStats.Percent))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "cpu", "system"}, float32(ru.ResourceUsage.CpuStats.SystemMode))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "cpu", "user"}, float32(ru.ResourceUsage.CpuStats.UserMode))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "cpu", "throttled_time"}, float32(ru.ResourceUsage.CpuStats.ThrottledTime))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "cpu", "throttled_periods"}, float32(ru.ResourceUsage.CpuStats.ThrottledPeriods))
		metrics.SetGauge([]string{"client", "allocs", r.alloc.Job.Name, r.alloc.TaskGroup, r.alloc.ID, r.task.Name, "cpu", "total_ticks"}, float32(ru.ResourceUsage.CpuStats.TotalTicks))
	}
}
