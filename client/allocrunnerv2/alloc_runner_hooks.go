package allocrunnerv2

import (
	"context"
	"fmt"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// initRunnerHooks intializes the runners hooks.
func (ar *allocRunner) initRunnerHooks() {
	hookLogger := ar.logger.Named("runner_hook")

	// Create the alloc directory hook. This is run first to ensure the
	// directoy path exists for other hooks.
	ar.runnerHooks = []interfaces.RunnerHook{
		newAllocDirHook(hookLogger, ar.allocDir),
		newDiskMigrationHook(hookLogger, ar.prevAllocWatcher, ar.allocDir),
		newAllocHealthWatcherHook(hookLogger, ar, ar.consulClient),
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
			return fmt.Errorf("hook %q failed: %v", name, err)
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
			return fmt.Errorf("hook %q failed: %v", name, err)
		}

		if ar.logger.IsTrace() {
			end := time.Now()
			ar.logger.Trace("finished update hooks", "name", name, "end", end, "duration", end.Sub(start))
		}
	}

	return nil
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
