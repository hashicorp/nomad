// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	golog "log"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
)

// DiscoverInterface is an interface for the Discover type in the go-discover
// library. Using an interface allows for ease of testing.
type DiscoverInterface interface {
	// Addrs discovers ip addresses of nodes that match the given filter
	// criteria.
	// The config string must have the format 'provider=xxx key=val key=val ...'
	// where the keys and values are provider specific. The values are URL
	// encoded.
	Addrs(string, *golog.Logger) ([]string, error)

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

	// logger is the retry joiners logger
	logger log.Logger
}

// Validate ensures that the configuration passes validity checks for the
// retry_join block. If the configuration is not valid, returns an error that
// will be displayed to the operator, otherwise nil.
func (r *retryJoiner) Validate(config *Config) error {

	// If retry_join is defined for the server, ensure that deprecated
	// fields and the server_join block are not both set
	if config.Server != nil && config.Server.ServerJoin != nil && len(config.Server.ServerJoin.RetryJoin) != 0 {
		if len(config.Server.RetryJoin) != 0 {
			return fmt.Errorf("server_join and retry_join cannot both be defined; prefer setting the server_join block")
		}
		if len(config.Server.StartJoin) != 0 {
			return fmt.Errorf("server_join and start_join cannot both be defined; prefer setting the server_join block")
		}
		if config.Server.RetryMaxAttempts != 0 {
			return fmt.Errorf("server_join and retry_max cannot both be defined; prefer setting the server_join block")
		}

		if config.Server.RetryInterval != 0 {
			return fmt.Errorf("server_join and retry_interval cannot both be defined; prefer setting the server_join block")
		}

		if len(config.Server.ServerJoin.StartJoin) != 0 {
			return fmt.Errorf("retry_join and start_join cannot both be defined")
		}
	}

	// if retry_join is defined for the client, ensure that start_join is not
	// set as this configuration is only defined for servers.
	if config.Client != nil && config.Client.ServerJoin != nil {
		if config.Client.ServerJoin.StartJoin != nil {
			return fmt.Errorf("start_join is not supported for Nomad clients")
		}
	}

	return nil
}

// retryJoin is used to handle retrying a join until it succeeds or all retries
// are exhausted.
func (r *retryJoiner) RetryJoin(serverJoin *ServerJoin) {
	if len(serverJoin.RetryJoin) == 0 {
		return
	}

	attempt := 0

	addrsToJoin := strings.Join(serverJoin.RetryJoin, " ")
	r.logger.Info("starting retry join", "servers", addrsToJoin)

	standardLogger := r.logger.StandardLogger(&log.StandardLoggerOptions{InferLevels: true})
	for {
		var addrs []string
		var n int
		var err error

		for _, addr := range serverJoin.RetryJoin {
			switch {
			case strings.HasPrefix(addr, "provider="):
				servers, err := r.discover.Addrs(addr, standardLogger)
				if err != nil {
					r.logger.Error("determining join addresses failed", "error", err)
				} else {
					addrs = append(addrs, servers...)
				}
			default:
				addrs = append(addrs, addr)
			}
		}

		if len(addrs) > 0 {
			if r.serverEnabled && r.serverJoin != nil {
				n, err = r.serverJoin(addrs)
				if err == nil {
					r.logger.Info("retry join completed", "initial_servers", n, "agent_mode", "server")
					return
				}
			}
			if r.clientEnabled && r.clientJoin != nil {
				n, err = r.clientJoin(addrs)
				if err == nil {
					r.logger.Info("retry join completed", "initial_servers", n, "agent_mode", "client")
					return
				}
			}
		}

		attempt++
		if serverJoin.RetryMaxAttempts > 0 && attempt > serverJoin.RetryMaxAttempts {
			r.logger.Error("max join retry exhausted, exiting")
			close(r.errCh)
			return
		}

		if err != nil {
			r.logger.Warn("join failed", "error", err, "retry", serverJoin.RetryInterval)
		}
		time.Sleep(serverJoin.RetryInterval)
	}
}
