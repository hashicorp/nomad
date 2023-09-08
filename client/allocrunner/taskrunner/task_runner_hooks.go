// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/LK4D4/joincontext"
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

// initHooks initializes the tasks hooks.
func (tr *TaskRunner) initHooks() {
	hookLogger := tr.logger.Named("task_hook")
	task := tr.Task()

	tr.logmonHookConfig = newLogMonHookConfig(task.Name, task.LogConfig, tr.taskDir.LogDir)

	// Add the hook resources
	tr.hookResources = &hookResources{}

	// Create the task directory hook. This is run first to ensure the
	// directory path exists for other hooks.
	alloc := tr.Alloc()
	tr.runnerHooks = []interfaces.TaskHook{
		newValidateHook(tr.clientConfig, hookLogger),
		newTaskDirHook(tr, hookLogger),
		newIdentityHook(tr, hookLogger),
		newLogMonHook(tr, hookLogger),
		newDispatchHook(alloc, hookLogger),
		newVolumeHook(tr, hookLogger),
		newArtifactHook(tr, tr.getter, hookLogger),
		newStatsHook(tr, tr.clientConfig.StatsCollectionInterval, hookLogger),
		newDeviceHook(tr.devicemanager, hookLogger),
		newAPIHook(tr.shutdownCtx, tr.clientConfig.APIListenerRegistrar, hookLogger),
		newWranglerHook(tr.wranglers, task.Name, alloc.ID, hookLogger),
	}

	// If the task has a CSI block, add the hook.
	if task.CSIPluginConfig != nil {
		tr.runnerHooks = append(tr.runnerHooks, newCSIPluginSupervisorHook(
			&csiPluginSupervisorHookConfig{
				clientStateDirPath: tr.clientConfig.StateDir,
				events:             tr,
				runner:             tr,
				lifecycle:          tr,
				capabilities:       tr.driverCapabilities,
				logger:             hookLogger,
			}))
	}

	// If Vault is enabled, add the hook
	if task.Vault != nil {
		tr.runnerHooks = append(tr.runnerHooks, newVaultHook(&vaultHookConfig{
			vaultBlock: task.Vault,
			client:     tr.vaultClient,
			events:     tr,
			lifecycle:  tr,
			updater:    tr,
			logger:     hookLogger,
			alloc:      tr.Alloc(),
			task:       tr.taskName,
		}))
	}

	// Get the consul namespace for the TG of the allocation.
	consulNamespace := tr.alloc.ConsulNamespace()

	// Identify the service registration provider, which can differ from the
	// Consul namespace depending on which provider is used.
	serviceProviderNamespace := tr.alloc.ServiceProviderNamespace()

	// If there are templates is enabled, add the hook
	if len(task.Templates) != 0 {
		tr.runnerHooks = append(tr.runnerHooks, newTemplateHook(&templateHookConfig{
			logger:              hookLogger,
			lifecycle:           tr,
			events:              tr,
			templates:           task.Templates,
			clientConfig:        tr.clientConfig,
			envBuilder:          tr.envBuilder,
			consulNamespace:     consulNamespace,
			nomadNamespace:      tr.alloc.Job.Namespace,
			renderOnTaskRestart: task.RestartPolicy.RenderTemplates,
		}))
	}

	// Always add the service hook. A task with no services on initial registration
	// may be updated to include services, which must be handled with this hook.
	tr.runnerHooks = append(tr.runnerHooks, newServiceHook(serviceHookConfig{
		alloc:             tr.Alloc(),
		task:              tr.Task(),
		providerNamespace: serviceProviderNamespace,
		serviceRegWrapper: tr.serviceRegWrapper,
		restarter:         tr,
		logger:            hookLogger,
	}))

	// If this is a Connect sidecar proxy (or a Connect Native) service,
	// add the sidsHook for requesting a Service Identity token (if ACLs).
	if task.UsesConnect() {
		// Enable the Service Identity hook only if the Nomad client is configured
		// with a consul token, indicating that Consul ACLs are enabled
		if tr.clientConfig.ConsulConfig.Token != "" {
			tr.runnerHooks = append(tr.runnerHooks, newSIDSHook(sidsHookConfig{
				alloc:      tr.Alloc(),
				task:       tr.Task(),
				sidsClient: tr.siClient,
				lifecycle:  tr,
				logger:     hookLogger,
			}))
		}

		if task.UsesConnectSidecar() {
			tr.runnerHooks = append(tr.runnerHooks,
				newEnvoyVersionHook(newEnvoyVersionHookConfig(alloc, tr.consulProxiesClient, hookLogger)),
				newEnvoyBootstrapHook(newEnvoyBootstrapHookConfig(alloc, tr.clientConfig.ConsulConfig, consulNamespace, hookLogger)),
			)
		} else if task.Kind.IsConnectNative() {
			tr.runnerHooks = append(tr.runnerHooks, newConnectNativeHook(
				newConnectNativeHookConfig(alloc, tr.clientConfig.ConsulConfig, hookLogger),
			))
		}
	}

	// Always add the script checks hook. A task with no script check hook on
	// initial registration may be updated to include script checks, which must
	// be handled with this hook.
	tr.runnerHooks = append(tr.runnerHooks, newScriptCheckHook(scriptCheckHookConfig{
		alloc:  tr.Alloc(),
		task:   tr.Task(),
		consul: tr.consulServiceClient,
		logger: hookLogger,
	}))

	// If this task driver has remote capabilities, add the remote task
	// hook.
	if tr.driverCapabilities.RemoteTasks {
		tr.runnerHooks = append(tr.runnerHooks, newRemoteTaskHook(tr, hookLogger))
	}
}

func (tr *TaskRunner) emitHookError(err error, hookName string) {
	var taskEvent *structs.TaskEvent
	if herr, ok := err.(*hookError); ok {
		taskEvent = herr.taskEvent
	} else {
		message := fmt.Sprintf("%s: %v", hookName, err)
		taskEvent = structs.NewTaskEvent(structs.TaskHookFailed).SetMessage(message)
	}

	tr.EmitEvent(taskEvent)
}

// prestart is used to run the runners prestart hooks.
func (tr *TaskRunner) prestart() error {
	// Determine if the allocation is terminal and we should avoid running
	// prestart hooks.
	if tr.shouldShutdown() {
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

	// use a join context to allow any blocking pre-start hooks
	// to be canceled by either killCtx or shutdownCtx
	joinedCtx, joinedCancel := joincontext.Join(tr.killCtx, tr.shutdownCtx)
	defer joinedCancel()

	alloc := tr.Alloc()

	for _, hook := range tr.runnerHooks {
		pre, ok := hook.(interfaces.TaskPrestartHook)
		if !ok {
			continue
		}

		name := pre.Name()

		// Build the request
		req := interfaces.TaskPrestartRequest{
			Alloc:         alloc,
			Task:          tr.Task(),
			TaskDir:       tr.taskDir,
			TaskEnv:       tr.envBuilder.Build(),
			TaskResources: tr.taskResources,
		}

		origHookState := tr.hookState(name)
		if origHookState != nil {
			if origHookState.PrestartDone {
				tr.logger.Trace("skipping done prestart hook", "name", pre.Name())

				// Always set env vars from hooks
				if name == HookNameDevices {
					tr.envBuilder.SetDeviceHookEnv(name, origHookState.Env)
				} else {
					tr.envBuilder.SetHookEnv(name, origHookState.Env)
				}

				continue
			}

			// Give the hook it's old data
			req.PreviousState = origHookState.Data
		}

		req.VaultToken = tr.getVaultToken()
		req.NomadToken = tr.getNomadToken()

		// Time the prestart hook
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running prestart hook", "name", name, "start", start)
		}

		// Run the prestart hook
		var resp interfaces.TaskPrestartResponse
		if err := pre.Prestart(joinedCtx, &req, &resp); err != nil {
			tr.emitHookError(err, name)
			return structs.WrapRecoverable(fmt.Sprintf("prestart hook %q failed: %v", name, err), err)
		}

		// Store the hook state
		{
			hookState := &state.HookState{
				Data:         resp.State,
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
		if name == HookNameDevices {
			tr.envBuilder.SetDeviceHookEnv(name, resp.Env)
		} else {
			tr.envBuilder.SetHookEnv(name, resp.Env)
		}

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

	// Pass the lazy handle to the hooks so even if the driver exits and we
	// launch a new one (external plugin), the handle will refresh.
	lazyHandle := NewLazyHandle(tr.shutdownCtx, tr.getDriverHandle, tr.logger)

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
			DriverExec:    lazyHandle,
			DriverNetwork: net,
			DriverStats:   lazyHandle,
			TaskEnv:       tr.envBuilder.Build(),
		}
		var resp interfaces.TaskPoststartResponse
		if err := post.Poststart(tr.killCtx, &req, &resp); err != nil {
			tr.emitHookError(err, name)
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
			tr.emitHookError(err, name)
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

		req := interfaces.TaskStopRequest{
			TaskDir: tr.taskDir,
		}

		origHookState := tr.hookState(name)
		if origHookState != nil {
			// Give the hook data provided by prestart
			req.ExistingState = origHookState.Data
		}

		var resp interfaces.TaskStopResponse
		if err := post.Stop(tr.killCtx, &req, &resp); err != nil {
			tr.emitHookError(err, name)
			merr.Errors = append(merr.Errors, fmt.Errorf("stop hook %q failed: %v", name, err))
		}

		// Stop hooks cannot alter state and must be idempotent, so
		// unlike prestart there's no state to persist here.

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
			NomadToken: tr.getNomadToken(),
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
			tr.emitHookError(err, name)
			tr.logger.Error("update hook failed", "name", name, "error", err)
		}

		// No need to persist as TaskUpdateResponse is currently empty

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished update hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}
}

// preKill is used to run the runners preKill hooks
// preKill hooks contain logic that must be executed before
// a task is killed or restarted
func (tr *TaskRunner) preKill() {
	if tr.logger.IsTrace() {
		start := time.Now()
		tr.logger.Trace("running pre kill hooks", "start", start)
		defer func() {
			end := time.Now()
			tr.logger.Trace("finished pre kill hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range tr.runnerHooks {
		killHook, ok := hook.(interfaces.TaskPreKillHook)
		if !ok {
			continue
		}

		name := killHook.Name()

		// Time the pre kill hook
		var start time.Time
		if tr.logger.IsTrace() {
			start = time.Now()
			tr.logger.Trace("running prekill hook", "name", name, "start", start)
		}

		// Run the pre kill hook
		req := interfaces.TaskPreKillRequest{}
		var resp interfaces.TaskPreKillResponse
		if err := killHook.PreKilling(context.Background(), &req, &resp); err != nil {
			tr.emitHookError(err, name)
			tr.logger.Error("prekill hook failed", "name", name, "error", err)
		}

		// No need to persist as TaskKillResponse is currently empty

		if tr.logger.IsTrace() {
			end := time.Now()
			tr.logger.Trace("finished prekill hook", "name", name, "end", end, "duration", end.Sub(start))
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
