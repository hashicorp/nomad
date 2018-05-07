package agent

import (
	"log"
	"strings"
	"time"
)

// DiscoverInterface is an interface for the Discover type in the go-discover
// library. Using an interface allows for ease of testing.
type DiscoverInterface interface {
	// Addrs discovers ip addresses of nodes that match the given filter
	// criteria.
	// The config string must have the format 'provider=xxx key=val key=val ...'
	// where the keys and values are provider specific. The values are URL
	// encoded.
	Addrs(string, *log.Logger) ([]string, error)

	// Help describes the format of the configuration string for address
	// discovery and the various provider specific options.
	Help() string

	// Names returns the names of the configured providers.
	Names() []string
}

// retryJoiner is used to handle retrying a join until it succeeds or all of
// its tries are exhausted.
type retryJoiner struct {
	// join adds the specified servers to the serf cluster
	join func([]string) (int, error)

	// discover is of type Discover, where this is either the go-discover
	// implementation or a mock used for testing
	discover DiscoverInterface

	// errCh is used to communicate with the agent when the max retry attempt
	// limit has been reached
	errCh chan struct{}

	// logger is the agent logger.
	logger *log.Logger
}

// retryJoin is used to handle retrying a join until it succeeds or all retries
// are exhausted.
func (r *retryJoiner) RetryJoin(config *Config) {
	if len(config.Server.RetryJoin) == 0 || !config.Server.Enabled {
		return
	}

	attempt := 0

	addrsToJoin := strings.Join(config.Server.RetryJoin, " ")
	r.logger.Printf("[INFO] agent: Joining cluster... %s", addrsToJoin)

	for {
		var addrs []string
		var err error

		for _, addr := range config.Server.RetryJoin {
			switch {
			case strings.HasPrefix(addr, "provider="):
				servers, err := r.discover.Addrs(addr, r.logger)
				if err != nil {
					r.logger.Printf("[ERR] agent: Join error %s", err)
				} else {
					addrs = append(addrs, servers...)
				}
			default:
				addrs = append(addrs, addr)
			}
		}

		if len(addrs) > 0 {
			n, err := r.join(addrs)
			if err == nil {
				r.logger.Printf("[INFO] agent: Join completed. Synced with %d initial agents", n)
			}
		}

		attempt++
		if config.Server.RetryMaxAttempts > 0 && attempt > config.Server.RetryMaxAttempts {
			r.logger.Printf("[ERR] agent: max join retry exhausted, exiting")
			close(r.errCh)
			return
		}

		if err != nil {
			r.logger.Printf("[WARN] agent: Join failed: %v, retrying in %v", err,
				config.Server.RetryInterval)
		}
		time.Sleep(config.Server.retryInterval)
	}
}
