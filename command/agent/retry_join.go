package agent

import (
	"log"
	"time"
)

type DiscoverInterface interface {
	Addrs(string, *log.Logger) ([]string, error)

	Help() string

	Names() []string
}

type retryJoiner struct {
	attempt int

	join func([]string) (int, error)

	discover DiscoverInterface

	errCh chan struct{}

	logger *log.Logger
}

func (r *retryJoiner) RetryJoin(config *Config) {
	if len(config.Server.RetryJoin) == 0 || !config.Server.Enabled {
		return
	}

	r.logger.Printf("[INFO] agent: Joining cluster...")

	for {
		addrs := config.Server.RetryJoin

		n, err := r.join(addrs)
		if err == nil {
			r.logger.Printf("[INFO] agent: Join completed. Synced with %d initial agents", n)
			return
		}

		r.attempt++
		if config.Server.RetryMaxAttempts > 0 && r.attempt > config.Server.RetryMaxAttempts {
			r.logger.Printf("[ERR] agent: max join retry exhausted, exiting")
			close(r.errCh)
			return
		}

		r.logger.Printf("[WARN] agent: Join failed: %v, retrying in %v", err,
			config.Server.RetryInterval)
		time.Sleep(config.Server.retryInterval)
	}
}
