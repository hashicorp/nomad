package taskrunner

import (
	"context"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
)

// StatsUpdater is the interface required by the StatsHook to update stats.
// Satisfied by TaskRunner.
type StatsUpdater interface {
	UpdateStats(*cstructs.TaskResourceUsage)
}

// statsHook manages the task stats collection goroutine.
type statsHook struct {
	updater  StatsUpdater
	interval time.Duration

	// cancel is called by Exited
	cancel context.CancelFunc

	mu sync.Mutex

	logger hclog.Logger
}

func newStatsHook(su StatsUpdater, interval time.Duration, logger hclog.Logger) *statsHook {
	h := &statsHook{
		updater:  su,
		interval: interval,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*statsHook) Name() string {
	return "stats_hook"
}

func (h *statsHook) Poststart(ctx context.Context, req *interfaces.TaskPoststartRequest, _ *interfaces.TaskPoststartResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// This shouldn't happen, but better safe than risk leaking a goroutine
	if h.cancel != nil {
		h.logger.Debug("poststart called twice without exiting between")
		h.cancel()
	}

	// Using a new context here because the existing context is for the scope of
	// the Poststart request. If that context was used, stats collection would
	// stop when the task was killed. It makes for more readable code and better
	// follows the taskrunner hook model to create a new context that can be
	// canceled on the Exited hook.
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go h.collectResourceUsageStats(ctx, req.DriverStats)

	return nil
}

func (h *statsHook) Exited(context.Context, *interfaces.TaskExitedRequest, *interfaces.TaskExitedResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cancel == nil {
		// No stats running
		return nil
	}

	// Call cancel to stop stats collection
	h.cancel()

	// Clear cancel func so we don't double call for any reason
	h.cancel = nil

	return nil
}

// collectResourceUsageStats starts collecting resource usage stats of a Task.
// Collection ends when the passed channel is closed
func (h *statsHook) collectResourceUsageStats(ctx context.Context, handle interfaces.DriverStats) {

	ch, err := handle.Stats(ctx, h.interval)
	if err != nil {
		// Check if the driver doesn't implement stats
		if err.Error() == cstructs.DriverStatsNotImplemented.Error() {
			h.logger.Debug("driver does not support stats")
			return
		}
		h.logger.Error("failed to start stats collection for task", "error", err)
	}

	var backoff time.Duration
	var retry int
	limit := time.Second * 5
	for {
		time.Sleep(backoff)
		select {
		case ru, ok := <-ch:
			// Channel is closed
			if !ok {
				var re *structs.RecoverableError
				ch, err = handle.Stats(ctx, h.interval)
				if err == nil {
					goto RETRY
				}

				// We do not log when the plugin is shutdown since this is
				// likely because the driver plugin has unexpectedly exited,
				// in which case sleeping and trying again or returning based
				// on the stop channel is the correct behavior
				if err != bstructs.ErrPluginShutdown {
					h.logger.Debug("error fetching stats of task", "error", err)
					goto RETRY
				}
				// check if the error is terminal otherwise it's likely a
				// transport error and we should retry
				re, ok = err.(*structs.RecoverableError)
				if ok && re.IsUnrecoverable() {
					return
				}
				h.logger.Warn("stats collection for task failed", "error", err)
			RETRY:
				// Calculate the new backoff
				backoff = (1 << (2 * uint64(retry))) * time.Second
				if backoff > limit {
					backoff = limit
				}
				// Increment retry counter
				retry++

				continue
			}

			// Update stats on TaskRunner and emit them
			h.updater.UpdateStats(ru)

		case <-ctx.Done():
			return
		}
	}
}

func (h *statsHook) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cancel == nil {
		return
	}

	h.cancel()
}
