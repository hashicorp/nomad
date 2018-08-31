package allocrunnerv2

import (
	"context"
	"fmt"
	"time"

	log "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
)

// initRunnerHooks intializes the runners hooks.
func (ar *allocRunner) initRunnerHooks() {
	hookLogger := ar.logger.Named("runner_hook")

	// Create the alloc directory hook. This is run first to ensure the
	// directoy path exists for other hooks.
	ar.runnerHooks = []interfaces.RunnerHook{
		newAllocDirHook(hookLogger, ar.allocDir),
		newDiskMigrationHook(hookLogger, ar.prevAllocWatcher, ar.allocDir),
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

// TODO
type allocHealthWatcherHook struct {
	runner   *allocRunner
	logger   log.Logger
	ctx      context.Context
	cancelFn context.CancelFunc
}

func newAllocHealthWatcherHook(runner *allocRunner, logger log.Logger) *allocHealthWatcherHook {
	ctx, cancelFn := context.WithCancel(context.Background())
	ad := &allocHealthWatcherHook{
		runner:   runner,
		ctx:      ctx,
		cancelFn: cancelFn,
	}

	ad.logger = logger.Named(ad.Name())
	return ad
}

func (h *allocHealthWatcherHook) Name() string {
	return "alloc_health_watcher"
}

func (h *allocHealthWatcherHook) Prerun() error {
	return nil
}

func (h *allocHealthWatcherHook) Update() error {
	// Cancel the old watcher and create a new one
	h.cancelFn()

	// TODO create the new one
	return nil
}

func (h *allocHealthWatcherHook) Destroy() error {
	h.cancelFn()
	return nil
}
