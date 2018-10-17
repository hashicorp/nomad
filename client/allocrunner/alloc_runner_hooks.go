package allocrunner

import (
	"context"
	"fmt"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// allocHealthSetter is a shim to allow the alloc health watcher hook to set
// and clear the alloc health without full access to the alloc runner state
type allocHealthSetter struct {
	ar *allocRunner
}

// ClearHealth allows the health watcher hook to clear the alloc's deployment
// health if the deployment id changes. It does not update the server as the
// status is only cleared when already receiving an update from the server.
//
// Only for use by health hook.
func (a *allocHealthSetter) ClearHealth() {
	a.ar.stateLock.Lock()
	a.ar.state.ClearDeploymentStatus()
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
	a.ar.stateLock.Unlock()

	// If deployment is unhealthy emit task events explaining why
	if !healthy && isDeploy {
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

// initRunnerHooks intializes the runners hooks.
func (ar *allocRunner) initRunnerHooks() {
	hookLogger := ar.logger.Named("runner_hook")

	// create health setting shim
	hs := &allocHealthSetter{ar}

	// Create the alloc directory hook. This is run first to ensure the
	// directory path exists for other hooks.
	ar.runnerHooks = []interfaces.RunnerHook{
		newAllocDirHook(hookLogger, ar.allocDir),
		newDiskMigrationHook(hookLogger, ar.prevAllocWatcher, ar.allocDir),
		newAllocHealthWatcherHook(hookLogger, ar.Alloc(), hs, ar.Listener(), ar.consulClient),
	}
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

		//TODO Check hook state

		name := pre.Name()
		var start time.Time
		if ar.logger.IsTrace() {
			start = time.Now()
			ar.logger.Trace("running pre-run hook", "name", name, "start", start)
		}

		if err := pre.Prerun(context.TODO()); err != nil {
			return fmt.Errorf("pre-run hook %q failed: %v", name, err)
		}

		//TODO Persist hook state locally

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished pre-run hooks", "name", name, "end", end, "duration", end.Sub(start))
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
			ar.logger.Trace("running pre-run hook", "name", name, "start", start)
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

	return nil
}
