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
	// serverJoin adds the specified servers to the serf cluster
	serverJoin func([]string) (int, error)

	// serverEnabled indicates whether the nomad agent will run in server mode
	serverEnabled bool

	// clientJoin adds the specified servers to the serf cluster
	clientJoin func([]string) (int, error)

	// clientEnabled indicates whether the nomad agent will run in client mode
	clientEnabled bool

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
func (r *retryJoiner) RetryJoin(serverJoin *ServerJoin) {
	if len(serverJoin.RetryJoin) == 0 {
		return
	}

	attempt := 0

	addrsToJoin := strings.Join(serverJoin.RetryJoin, " ")
	r.logger.Printf("[INFO] agent: Joining cluster... %s", addrsToJoin)

	for {
		var addrs []string
		var err error

		for _, addr := range serverJoin.RetryJoin {
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
			if r.serverEnabled && r.serverJoin != nil {
				n, err := r.serverJoin(addrs)
				if err == nil {
					r.logger.Printf("[INFO] agent: Join completed. Server synced with %d initial servers", n)
					return
				}
			}
			if r.clientEnabled && r.clientJoin != nil {
				n, err := r.clientJoin(addrs)
				if err == nil {
					r.logger.Printf("[INFO] agent: Join completed. Client synced with %d initial servers", n)
					return
				}
			}
		}

		attempt++
		if serverJoin.RetryMaxAttempts > 0 && attempt > serverJoin.RetryMaxAttempts {
			r.logger.Printf("[ERR] agent: max join retry exhausted, exiting")
			close(r.errCh)
			return
		}

		if err != nil {
			r.logger.Printf("[WARN] agent: Join failed: %v, retrying in %v", err,
				serverJoin.RetryInterval)
		}
		time.Sleep(serverJoin.retryInterval)
	}
}
