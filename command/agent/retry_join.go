// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	golog "log"

	hclog "github.com/hashicorp/go-hclog"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-netaddrs"
)

// cfgPrefixes are autoDisocver's allowed cfg prefixes
var cfgPrefixes = regexp.MustCompile("^(exec|provider|dns)=(.+)$")

// AutoDiscoverInterface is an interface for autoDiscover to ease testing
type AutoDiscoverInterface interface {
	Addrs(cfg string, logger hclog.Logger) ([]string, error)
}

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

// NetaddrsInterface is an interface for go-netaddrs to ease testing
type NetaddrsInterface interface {
	IPAddrs(ctx context.Context, cfg string, l netaddrs.Logger) ([]net.IPAddr, error)
}

type netAddrs struct{}

func (n *netAddrs) IPAddrs(ctx context.Context, cfg string, l netaddrs.Logger) ([]net.IPAddr, error) {
	return netaddrs.IPAddrs(ctx, cfg, l)
}

// autoDiscover uses go-netaddrs and go-discover to discover IP addresses when
// auto-joining clusters
//
// autoDiscover implements AutoDiscoverInterface
type autoDiscover struct {
	netAddrs   NetaddrsInterface
	goDiscover DiscoverInterface
}

// Addrs looks up and returns IP addresses specified by cfg.
//
// If cfg is a IPv4 or IPv6 address, return the stringified IP address.
//
// If cfg has a dns= prefix, the configuration is treated as a DNS lookup on the
// name following the prefix.
//
// If cfg has an exec= prefix, IP addresses are looked up by executing the command
// after exec=. The command may include optional arguments. Command arguments
// must be space separated (spaces in argument values can not be escaped).
// The command may output IPv4 or IPv6 addresses, and IPv6 addresses can
// optionally include a zone index.
//
// The executable must follow these rules:
//
//	on success - exit 0 and print whitespace delimited IP addresses to stdout.
//	on failure - exits with a non-zero code, and should print an error message
//	             of up to 1024 bytes to stderr.
//
// If cfg has a provider= prefix, IP addresses are looked up using the go-discover
// provider specified in cfg.
func (d autoDiscover) Addrs(cfg string, logger hclog.Logger) (addrs []string, err error) {
	ip := net.ParseIP(cfg)
	if ip != nil {
		return []string{cfg}, nil
	}

	if !cfgPrefixes.MatchString(cfg) {
		return nil, fmt.Errorf("auto-discover configurations must begin with: exec=, provider=, or dns=; got %s", cfg)
	}

	var ipAddrs []net.IPAddr
	prefixMatches := cfgPrefixes.FindStringSubmatch(cfg)
	cfgPrefix := prefixMatches[1]
	cfgValue := prefixMatches[2]
	switch cfgPrefix {
	case "exec":
		ipAddrs, err = d.netAddrs.IPAddrs(context.Background(), cfg, logger)

		for _, addr := range ipAddrs {
			addrs = append(addrs, addr.String())
		}
	case "dns":
		ipAddrs, err = d.netAddrs.IPAddrs(context.Background(),
			cfgValue,
			logger)
		for _, addr := range ipAddrs {
			addrs = append(addrs, addr.String())
		}
	case "provider":
		addrs, err = d.goDiscover.Addrs(cfg, logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}))
	default:
		return nil, fmt.Errorf("auto-discover configurations must begin with: exec=, provider=, or dns=; got '%s'",
			cfgPrefix)
	}

	return
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

	// autoDiscover is either an agent.autoDiscover, or a mock used for testing
	autoDiscover AutoDiscoverInterface

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

	for {
		var addrs []string
		var n int
		var err error

		for _, addr := range serverJoin.RetryJoin {
			servers, err := r.autoDiscover.Addrs(addr, r.logger)
			if err != nil {
				r.logger.Warn("discovering join addresses failed", "join_config", addr, "error", err)
				addrs = append(addrs, addr)
				continue
			}

			addrs = append(addrs, servers...)
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
