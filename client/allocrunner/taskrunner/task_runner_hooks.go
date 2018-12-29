package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// hookResources captures the resources for the task provided by hooks.
type hookResources struct {
	Devices []*drivers.DeviceConfig
	Mounts  []*drivers.MountConfig
	sync.RWMutex
}

func (h *hookResources) setDevices(d []*drivers.DeviceConfig) {
	h.Lock()
	h.Devices = d
	h.Unlock()
}

func (h *hookResources) getDevices() []*drivers.DeviceConfig {
	h.RLock()
	defer h.RUnlock()
	return h.Devices
}

func (h *hookResources) setMounts(m []*drivers.MountConfig) {
	h.Lock()
	h.Mounts = m
	h.Unlock()
}

func (h *hookResources) getMounts() []*drivers.MountConfig {
	h.RLock()
	defer h.RUnlock()
	return h.Mounts
}

// initHooks intializes the tasks hooks.
func (tr *TaskRunner) initHooks() {
	hookLogger := tr.logger.Named("task_hook")
	task := tr.Task()

	tr.logmonHookConfig = newLogMonHookConfig(task.Name, tr.taskDir.LogDir)

	// Add the hook resources
	tr.hookResources = &hookResources{}

	// Create the task directory hook. This is run first to ensure the
	// directory path exists for other hooks.
	tr.runnerHooks = []interfaces.TaskHook{
		newValidateHook(tr.clientConfig, hookLogger),
		newTaskDirHook(tr, hookLogger),
		newLogMonHook(tr.logmonHookConfig, hookLogger),
		newDispatchHook(tr.Alloc(), hookLogger),
		newArtifactHook(tr, hookLogger),
		newStatsHook(tr, tr.clientConfig.StatsCollectionInterval, hookLogger),
		newDeviceHook(tr.devicemanager, hookLogger),
	}

	// If Vault is enabled, add the hook
	if task.Vault != nil {
		tr.runnerHooks = append(tr.runnerHooks, newVaultHook(&vaultHookConfig{
			vaultStanza: task.Vault,
			client:      tr.vaultClient,
			events:      tr,
			lifecycle:   tr,
			updater:     tr,
			logger:      hookLogger,
			alloc:       tr.Alloc(),
			task:        tr.taskName,
		}))
	}

	// If there are templates is enabled, add the hook
	if len(task.Templates) != 0 {
		tr.runnerHooks = append(tr.runnerHooks, newTemplateHook(&templateHookConfig{
			logger:       hookLogger,
			lifecycle:    tr,
			events:       tr,
			templates:    task.Templates,
			clientConfig: tr.clientConfig,
			envBuilder:   tr.envBuilder,
		}))
	}

	// If there are any services, add the hook
	if len(task.Services) != 0 {
		tr.runnerHooks = append(tr.runnerHooks, newServiceHook(serviceHookConfig{
			alloc:     tr.Alloc(),
			task:      tr.Task(),
			consul:    tr.consulClient,
			restarter: tr,
			logger:    hookLogger,
		}))
	}
}

// prestart is used to run the runners prestart hooks.
func (tr *TaskRunner) prestart() error {
	// Determine if the allocation is terminaland we should avoid running
	// prestart hooks.
	alloc := tr.Alloc()
	if alloc.TerminalStatus() {
		tr.logger.Trace("skipping prestart hooks since allocation is terminal")
		return nil
	}

	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running prestart hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished prestart hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range tr.runnerHooks {
		pre, ok := hook.(interfaces.TaskPrestartHook)
		if !ok {
			continue
		}

		name := pre.Name()

		// Build the request
		req := interfaces.TaskPrestartRequest{
			Task:          tr.Task(),
			TaskDir:       tr.taskDir,
			TaskEnv:       tr.envBuilder.Build(),
			TaskResources: tr.taskResources,
		}

		var origHookState *state.HookState
		tr.stateLock.RLock()
		if tr.localState.Hooks != nil {
			origHookState = tr.localState.Hooks[name]
		}
		tr.stateLock.RUnlock()

		if origHookState != nil {
			if origHookState.PrestartDone {
				tr.logger.Trace("skipping done prestart hook", "name", pre.Name())
				// Always set env vars from hooks
				tr.envBuilder.SetHookEnv(name, origHookState.Env)
				continue
			}

			// Give the hook it's old data
			req.HookData = origHookState.Data
		}

		req.VaultToken = tr.getVaultToken()

		// Time the prestart hook
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running prestart hook", "name", name, "start", start)
		}

		// Run the prestart hook
		var resp interfaces.TaskPrestartResponse
		if err := pre.Prestart(tr.killCtx, &req, &resp); err != nil {
			return structs.WrapRecoverable(fmt.Sprintf("prestart hook %q failed: %v", name, err), err)
		}

		// Store the hook state
		{
			hookState := &state.HookState{
				Data:         resp.HookData,
				PrestartDone: resp.Done,
				Env:          resp.Env,
			}

			// Store and persist local state if the hook state has changed
			if !hookState.Equal(origHookState) {
				tr.stateLock.Lock()
				tr.localState.Hooks[name] = hookState
				tr.stateLock.Unlock()

				if err := tr.persistLocalState(); err != nil {
					return err
				}
			}
		}

		// Store the environment variables returned by the hook
		tr.envBuilder.SetHookEnv(name, resp.Env)

		// Store the resources
		if len(resp.Devices) != 0 {
			tr.hookResources.setDevices(resp.Devices)
		}
		if len(resp.Mounts) != 0 {
			tr.hookResources.setMounts(resp.Mounts)
		}

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished prestart hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

// poststart is used to run the runners poststart hooks.
func (tr *TaskRunner) poststart() error {
	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running poststart hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished poststart hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	handle := tr.getDriverHandle()
	net := handle.Network()

	var merr multierror.Error
	for _, hook := range tr.runnerHooks {
		post, ok := hook.(interfaces.TaskPoststartHook)
		if !ok {
			continue
		}

		name := post.Name()
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running poststart hook", "name", name, "start", start)
		}

		req := interfaces.TaskPoststartRequest{
			DriverExec:    handle,
			DriverNetwork: net,
			DriverStats:   handle,
			TaskEnv:       tr.envBuilder.Build(),
		}
		var resp interfaces.TaskPoststartResponse
		if err := post.Poststart(tr.killCtx, &req, &resp); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("poststart hook %q failed: %v", name, err))
		}

		// No need to persist as PoststartResponse is currently empty

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished poststart hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return merr.ErrorOrNil()
}

// exited is used to run the exited hooks before a task is stopped.
func (tr *TaskRunner) exited() error {
	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running exited hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished exited hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	var merr multierror.Error
	for _, hook := range tr.runnerHooks {
		post, ok := hook.(interfaces.TaskExitedHook)
		if !ok {
			continue
		}

		name := post.Name()
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running exited hook", "name", name, "start", start)
		}

		req := interfaces.TaskExitedRequest{}
		var resp interfaces.TaskExitedResponse
		if err := post.Exited(tr.killCtx, &req, &resp); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("exited hook %q failed: %v", name, err))
		}

		// No need to persist as TaskExitedResponse is currently empty

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished exited hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return merr.ErrorOrNil()

}

// stop is used to run the stop hooks.
func (tr *TaskRunner) stop() error {
	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running stop hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished stop hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	var merr multierror.Error
	for _, hook := range tr.runnerHooks {
		post, ok := hook.(interfaces.TaskStopHook)
		if !ok {
			continue
		}

		name := post.Name()
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running stop hook", "name", name, "start", start)
		}

		req := interfaces.TaskStopRequest{}
		var resp interfaces.TaskStopResponse
		if err := post.Stop(tr.killCtx, &req, &resp); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("stop hook %q failed: %v", name, err))
		}

		// No need to persist as TaskStopResponse is currently empty

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished stop hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return merr.ErrorOrNil()
}

// update is used to run the runners update hooks. Should only be called from
// Run(). To trigger an update, update state on the TaskRunner and call
// triggerUpdateHooks.
func (tr *TaskRunner) updateHooks() {
	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running update hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished update hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	// Prepare state needed by Update hooks
	alloc := tr.Alloc()

	// Execute Update hooks
	for _, hook := range tr.runnerHooks {
		upd, ok := hook.(interfaces.TaskUpdateHook)
		if !ok {
			continue
		}

		name := upd.Name()

		// Build the request
		req := interfaces.TaskUpdateRequest{
			VaultToken: tr.getVaultToken(),
			Alloc:      alloc,
			TaskEnv:    tr.envBuilder.Build(),
		}

		// Time the update hook
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running update hook", "name", name, "start", start)
		}

		// Run the update hook
		var resp interfaces.TaskUpdateResponse
		if err := upd.Update(tr.killCtx, &req, &resp); err != nil {
			tr.logger.Error("update hook failed", "name", name, "error", err)
		}

		// No need to persist as TaskUpdateResponse is currently empty

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished update hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}
}

// killing is used to run the runners kill hooks.
func (tr *TaskRunner) killing() {
	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running kill hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished kill hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range tr.runnerHooks {
		killHook, ok := hook.(interfaces.TaskKillHook)
		if !ok {
			continue
		}

		name := killHook.Name()

		// Time the update hook
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running kill hook", "name", name, "start", start)
		}

		// Run the kill hook
		req := interfaces.TaskKillRequest{}
		var resp interfaces.TaskKillResponse
		if err := killHook.Killing(context.Background(), &req, &resp); err != nil {
			tr.logger.Error("kill hook failed", "name", name, "error", err)
		}

		// No need to persist as TaskKillResponse is currently empty

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished kill hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}
}

// shutdownHooks is called when the TaskRunner is gracefully shutdown but the
// task is not being stopped or garbage collected.
func (tr *TaskRunner) shutdownHooks() {
	for _, hook := range tr.runnerHooks {
		sh, ok := hook.(interfaces.ShutdownHook)
		if !ok {
			continue
		}

		name := sh.Name()

		// Time the update hook
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running shutdown hook", "name", name, "start", start)
		}

		sh.Shutdown()

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished shutdown hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}
}
