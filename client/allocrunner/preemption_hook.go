package allocrunner

import (
	"context"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocwatcher"
)

// preemptionWatchingHook waits for a PrevAllocWatcher to exit before allowing
// an allocation to be executed
type preemptionWatchingHook struct {
	allocWatcher allocwatcher.PrevAllocWatcher
	logger       log.Logger
}

func newPreemptionHook(logger log.Logger, allocWatcher allocwatcher.PrevAllocWatcher) *preemptionWatchingHook {
	h := &preemptionWatchingHook{
		allocWatcher: allocWatcher,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (h *preemptionWatchingHook) Name() string {
	return "await_preemptions"
}

func (h *preemptionWatchingHook) Prerun(ctx context.Context) error {
	// Wait for a previous alloc - if any - to terminate
	return h.allocWatcher.Wait(ctx)
}
