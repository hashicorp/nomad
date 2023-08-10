// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"
	"time"

	multierror "github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocHealthSetter is a shim to allow the alloc health watcher hook to set
// and clear the alloc health without full access to the alloc runner state
type allocHealthSetter struct {
	ar *allocRunner
}

// HasHealth returns true if a deployment status is already set.
func (a *allocHealthSetter) HasHealth() bool {
	a.ar.stateLock.Lock()
	defer a.ar.stateLock.Unlock()
	return a.ar.state.DeploymentStatus.HasHealth()
}

// ClearHealth allows the health watcher hook to clear the alloc's deployment
// health if the deployment id changes. It does not update the server as the
// status is only cleared when already receiving an update from the server.
//
// Only for use by health hook.
func (a *allocHealthSetter) ClearHealth() {
	a.ar.stateLock.Lock()
	a.ar.state.ClearDeploymentStatus()
	a.ar.persistDeploymentStatus(nil)
	a.ar.stateLock.Unlock()
}

// SetHealth allows the health watcher hook to set the alloc's
// deployment/migration health and emit task events.
//
// Only for use by health hook.
func (a *allocHealthSetter) SetHealth(healthy, isDeploy bool, trackerTaskEvents map[string]*structs.TaskEvent) {
	// Updating alloc deployment state is tricky because it may be nil, but
	// if it's not then we need to maintain the values of Canary and
	// ModifyIndex as they're only mutated by the server.
	a.ar.stateLock.Lock()
	a.ar.state.SetDeploymentStatus(time.Now(), healthy)
	a.ar.persistDeploymentStatus(a.ar.state.DeploymentStatus)
	terminalDesiredState := a.ar.Alloc().ServerTerminalStatus()
	a.ar.stateLock.Unlock()

	// If deployment is unhealthy emit task events explaining why
	if !healthy && isDeploy && !terminalDesiredState {
		for task, event := range trackerTaskEvents {
			if tr, ok := a.ar.tasks[task]; ok {
				// Append but don't emit event since the server
				// will be updated below
				tr.AppendEvent(event)
			}
		}
	}

	// Gather the state of the other tasks
	states := make(map[string]*structs.TaskState, len(a.ar.tasks))
	for name, tr := range a.ar.tasks {
		states[name] = tr.TaskState()
	}

	// Build the client allocation
	calloc := a.ar.clientAlloc(states)

	// Update the server
	a.ar.stateUpdater.AllocStateUpdated(calloc)

	// Broadcast client alloc to listeners
	a.ar.allocBroadcaster.Send(calloc)
}

// initRunnerHooks initializes the runners hooks.
func (ar *allocRunner) initRunnerHooks(config *clientconfig.Config) error {
	hookLogger := ar.logger.Named("runner_hook")

	// create health setting shim
	hs := &allocHealthSetter{ar}

	// create network isolation setting shim
	ns := &allocNetworkIsolationSetter{ar: ar}

	// build the network manager
	nm, err := newNetworkManager(ar.Alloc(), ar.driverManager)
	if err != nil {
		return fmt.Errorf("failed to configure network manager: %v", err)
	}

	// create network configurator
	nc, err := newNetworkConfigurator(hookLogger, ar.Alloc(), config)
	if err != nil {
		return fmt.Errorf("failed to initialize network configurator: %v", err)
	}

	// Create a new taskenv.Builder which is used by hooks that mutate them to
	// build new taskenv.TaskEnv.
	newEnvBuilder := func() *taskenv.Builder {
		return taskenv.NewBuilder(config.Node, ar.Alloc(), nil, config.Region).
			SetAllocDir(ar.allocDir.AllocDir)
	}

	// Create a taskenv.TaskEnv which is used for read only purposes by the
	// newNetworkHook.
	builtTaskEnv := newEnvBuilder().Build()

	// Create the alloc directory hook. This is run first to ensure the
	// directory path exists for other hooks.
	alloc := ar.Alloc()
	ar.runnerHooks = []interfaces.RunnerHook{
		newAllocDirHook(hookLogger, ar.allocDir),
		newUpstreamAllocsHook(hookLogger, ar.prevAllocWatcher),
		newDiskMigrationHook(hookLogger, ar.prevAllocMigrator, ar.allocDir),
		newAllocHealthWatcherHook(hookLogger, alloc, newEnvBuilder, hs, ar.Listener(), ar.consulClient, ar.checkStore),
		newNetworkHook(hookLogger, ns, alloc, nm, nc, ar, builtTaskEnv),
		newGroupServiceHook(groupServiceHookConfig{
			alloc:             alloc,
			providerNamespace: alloc.ServiceProviderNamespace(),
			serviceRegWrapper: ar.serviceRegWrapper,
			restarter:         ar,
			taskEnvBuilder:    newEnvBuilder(),
			networkStatus:     ar,
			logger:            hookLogger,
			shutdownDelayCtx:  ar.shutdownDelayCtx,
		}),
		newConsulGRPCSocketHook(hookLogger, alloc, ar.allocDir, config.ConsulConfig, config.Node.Attributes),
		newConsulHTTPSocketHook(hookLogger, alloc, ar.allocDir, config.ConsulConfig),
		newCSIHook(alloc, hookLogger, ar.csiManager, ar.rpcClient, ar, ar.hookResources, ar.clientConfig.Node.SecretID),
		newChecksHook(hookLogger, alloc, ar.checkStore, ar),
	}
	if config.ExtraAllocHooks != nil {
		ar.runnerHooks = append(ar.runnerHooks, config.ExtraAllocHooks...)
	}

	return nil
}

// prerun is used to run the runners prerun hooks.
func (ar *allocRunner) prerun() error {
	if ar.logger.IsTrace() {
		start := time.Now()
		ar.logger.Trace("running pre-run hooks", "start", start)
		defer func() {
			end := time.Now()
			ar.logger.Trace("finished pre-run hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range ar.runnerHooks {
		pre, ok := hook.(interfaces.RunnerPrerunHook)
		if !ok {
			continue
		}

		name := pre.Name()
		var start time.Time
		if ar.logger.IsTrace() {
			start = time.Now()
			ar.logger.Trace("running pre-run hook", "name", name, "start", start)
		}

		if err := pre.Prerun(); err != nil {
			return fmt.Errorf("pre-run hook %q failed: %v", name, err)
		}

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished pre-run hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

// update runs the alloc runner update hooks. Update hooks are run
// asynchronously with all other alloc runner operations.
func (ar *allocRunner) update(update *structs.Allocation) error {
	if ar.logger.IsTrace() {
		start := time.Now()
		ar.logger.Trace("running update hooks", "start", start)
		defer func() {
			end := time.Now()
			ar.logger.Trace("finished update hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	req := &interfaces.RunnerUpdateRequest{
		Alloc: update,
	}

	var merr multierror.Error
	for _, hook := range ar.runnerHooks {
		h, ok := hook.(interfaces.RunnerUpdateHook)
		if !ok {
			continue
		}

		name := h.Name()
		var start time.Time
		if ar.logger.IsTrace() {
			start = time.Now()
			ar.logger.Trace("running update hook", "name", name, "start", start)
		}

		if err := h.Update(req); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("update hook %q failed: %v", name, err))
		}

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished update hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return merr.ErrorOrNil()
}

// postrun is used to run the runners postrun hooks.
func (ar *allocRunner) postrun() error {
	if ar.logger.IsTrace() {
		start := time.Now()
		ar.logger.Trace("running post-run hooks", "start", start)
		defer func() {
			end := time.Now()
			ar.logger.Trace("finished post-run hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	for _, hook := range ar.runnerHooks {
		post, ok := hook.(interfaces.RunnerPostrunHook)
		if !ok {
			continue
		}

		name := post.Name()
		var start time.Time
		if ar.logger.IsTrace() {
			start = time.Now()
			ar.logger.Trace("running post-run hook", "name", name, "start", start)
		}

		if err := post.Postrun(); err != nil {
			return fmt.Errorf("hook %q failed: %v", name, err)
		}

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished post-run hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
}

// destroy is used to run the runners destroy hooks. All hooks are run and
// errors are returned as a multierror.
func (ar *allocRunner) destroy() error {
	if ar.logger.IsTrace() {
		start := time.Now()
		ar.logger.Trace("running destroy hooks", "start", start)
		defer func() {
			end := time.Now()
			ar.logger.Trace("finished destroy hooks", "end", end, "duration", end.Sub(start))
		}()
	}

	var merr multierror.Error
	for _, hook := range ar.runnerHooks {
		h, ok := hook.(interfaces.RunnerDestroyHook)
		if !ok {
			continue
		}

		name := h.Name()
		var start time.Time
		if ar.logger.IsTrace() {
			start = time.Now()
			ar.logger.Trace("running destroy hook", "name", name, "start", start)
		}

		if err := h.Destroy(); err != nil {
			merr.Errors = append(merr.Errors, fmt.Errorf("destroy hook %q failed: %v", name, err))
		}

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished destroy hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return merr.ErrorOrNil()
}

func (ar *allocRunner) preKillHooks() {
	for _, hook := range ar.runnerHooks {
		pre, ok := hook.(interfaces.RunnerPreKillHook)

		if !ok {
			continue
		}

		name := pre.Name()
		var start time.Time
		if ar.logger.IsTrace() {
			start = time.Now()
			ar.logger.Trace("running alloc pre shutdown hook", "name", name, "start", start)
		}

		pre.PreKill()

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished alloc pre shutdown hook", "name", name, "end", end, "duration", end.Sub(start))
		}
	}
}

// shutdownHooks calls graceful shutdown hooks for when the agent is exiting.
func (ar *allocRunner) shutdownHooks() {
	for _, hook := range ar.runnerHooks {
		sh, ok := hook.(interfaces.ShutdownHook)
		if !ok {
			continue
		}

		name := sh.Name()
		var start time.Time
		if ar.logger.IsTrace() {
			start = time.Now()
			ar.logger.Trace("running shutdown hook", "name", name, "start", start)
		}

		sh.Shutdown()

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished shutdown hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}
}

func (ar *allocRunner) taskRestartHooks() {
	for _, hook := range ar.runnerHooks {
		re, ok := hook.(interfaces.RunnerTaskRestartHook)
		if !ok {
			continue
		}

		name := re.Name()
		var start time.Time
		if ar.logger.IsTrace() {
			start = time.Now()
			ar.logger.Trace("running alloc task restart hook",
				"name", name, "start", start)
		}

		re.PreTaskRestart()

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished alloc task restart hook",
				"name", name, "end", end, "duration", end.Sub(start))
		}
	}
}
