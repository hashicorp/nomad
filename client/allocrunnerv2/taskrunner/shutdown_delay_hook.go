package taskrunner

import (
	"context"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
)

// shutdownDelayHook delays shutting down a task between deregistering it from
// Consul and actually killing it.
type shutdownDelayHook struct {
	delay  time.Duration
	logger log.Logger
}

func newShutdownDelayHook(delay time.Duration, logger log.Logger) *shutdownDelayHook {
	h := &shutdownDelayHook{
		delay: delay,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*shutdownDelayHook) Name() string {
	return "shutdown-delay"
}

func (h *shutdownDelayHook) Kill(ctx context.Context, req *interfaces.TaskKillRequest, resp *interfaces.TaskKillResponse) error {
	select {
	case <-ctx.Done():
	case <-time.After(h.delay):
	}
	return nil
}
