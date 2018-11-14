package taskrunner

import (
	"context"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/driver"
	cstructs "github.com/hashicorp/nomad/client/structs"
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

	// stopCh is closed by Exited or Canceled
	stopCh chan struct{}

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
	if h.stopCh != nil {
		h.logger.Debug("poststart called twice without exiting between")
		close(h.stopCh)
	}

	h.stopCh = make(chan struct{})
	go h.collectResourceUsageStats(req.DriverStats, h.stopCh)

	return nil
}

func (h *statsHook) Exited(context.Context, *interfaces.TaskExitedRequest, *interfaces.TaskExitedResponse) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stopCh == nil {
		// No stats running
		return nil
	}

	// Close chan to stop stats collection
	close(h.stopCh)

	// Clear chan so we don't double close for any reason
	h.stopCh = nil

	return nil
}

// collectResourceUsageStats starts collecting resource usage stats of a Task.
// Collection ends when the passed channel is closed
func (h *statsHook) collectResourceUsageStats(handle interfaces.DriverStats, stopCh <-chan struct{}) {
	// start collecting the stats right away and then start collecting every
	// collection interval
	next := time.NewTimer(0)
	defer next.Stop()
	for {
		select {
		case <-next.C:
			// Reset the timer
			next.Reset(h.interval)

			// Collect stats from driver
			ru, err := handle.Stats()
			if err != nil {
				// Check if the driver doesn't implement stats
				if err.Error() == driver.DriverStatsNotImplemented.Error() {
					h.logger.Debug("driver does not support stats")
					return
				}

				//XXX This is a net/rpc specific error
				// We do not log when the plugin is shutdown as this is simply a
				// race between the stopCollection channel being closed and calling
				// Stats on the handle.
				if !strings.Contains(err.Error(), "connection is shut down") {
					h.logger.Debug("error fetching stats of task", "error", err)
				}

				continue
			}

			// Update stats on TaskRunner and emit them
			h.updater.UpdateStats(ru)
		case <-stopCh:
			return
		}
	}
}

func (h *statsHook) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stopCh == nil {
		return
	}

	select {
	case <-h.stopCh:
		// Already closed
	default:
		close(h.stopCh)
	}
}
