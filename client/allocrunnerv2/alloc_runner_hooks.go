package allocrunnerv2

import (
	"context"
	"fmt"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
)

// initRunnerHooks intializes the runners hooks.
func (ar *allocRunner) initRunnerHooks() {
	hookLogger := ar.logger.Named("runner_hook")
	ar.runnerHooks = make([]interfaces.RunnerHook, 0, 3)

	// Create the alloc directory hook. This is run first to ensure the
	// directoy path exists for other hooks.
	ar.runnerHooks = append(ar.runnerHooks, newAllocDirHook(ar, hookLogger))
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
			return fmt.Errorf("hook %q failed: %v", name, err)
		}

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

/*
What state is needed to transfer:
*/

/*
AR Hooks:
Alloc Dir Build:
Needs to know the folder to create

Alloc Migrate
Needs access to RPC

Alloc Health Watcher:
Requires: Access to consul to watch health, access to every task event, task status change
*/

type allocDirHook struct {
	runner *allocRunner
	logger log.Logger
}

func newAllocDirHook(runner *allocRunner, logger log.Logger) *allocDirHook {
	ad := &allocDirHook{
		runner: runner,
	}
	ad.logger = logger.Named(ad.Name())
	return ad
}

func (h *allocDirHook) Name() string {
	return "alloc_dir"
}

func (h *allocDirHook) Prerun() error {
	return h.runner.allocDir.Build()
}

func (h *allocDirHook) Destroy() error {
	return h.runner.allocDir.Destroy()
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
