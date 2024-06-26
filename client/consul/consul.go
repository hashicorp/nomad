// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"fmt"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// TokenDeriverFunc takes an allocation and a set of tasks and derives a service
// identity token for each. Requests go through nomad server and the local
// Consul agent.
type TokenDeriverFunc func(context.Context, *structs.Allocation, []string) (map[string]string, error)

// ServiceIdentityAPI is the interface the Nomad Client uses to request Consul
// Service Identity tokens through Nomad Server. (Deprecated: will be removed in 1.9.0)
//
// ACL requirements
// - acl:write (used by Server only)
type ServiceIdentityAPI interface {
	// DeriveSITokens contacts the nomad server and requests consul service
	// identity tokens be generated for tasks in the allocation.
	DeriveSITokens(ctx context.Context, alloc *structs.Allocation, tasks []string) (map[string]string, error)
}

// SupportedProxiesAPI is the interface the Nomad Client uses to request from
// Consul the set of supported proxied to use for Consul Connect.
//
// No ACL requirements
type SupportedProxiesAPI interface {
	Proxies() (map[string][]string, error)
}

// SupportedProxiesAPIFunc returns an interface that the Nomad client uses for
// requesting the set of supported proxies from Consul.
type SupportedProxiesAPIFunc func(string) SupportedProxiesAPI

// JWTLoginRequest is an object representing a login request with JWT
type JWTLoginRequest struct {
	JWT            string
	AuthMethodName string
	Meta           map[string]string
}

// Client is the interface that the nomad client uses to interact with
// Consul tokens
type Client interface {
	// DeriveTokenWithJWT logs into Consul using JWT and retrieves a Consul ACL
	// token.
	DeriveTokenWithJWT(JWTLoginRequest) (*consulapi.ACLToken, error)

	RevokeTokens([]*consulapi.ACLToken) error

	// TokenPreflightCheck verifies that a token has been replicated before we
	// try to use it for registering services or bootstrapping Envoy
	TokenPreflightCheck(context.Context, *consulapi.ACLToken) error
}

type consulClient struct {
	// client is the API client to interact with consul
	client *consulapi.Client

	// partition is the Consul partition for the local agent
	partition string

	// config is the configuration to connect to consul
	config *config.ConsulConfig

	logger hclog.Logger

	// preflightCheckTimeout/BaseInterval control how long the client will wait
	// for Consul ACLs tokens to be fully replicated before giving up on the
	// allocation; these are configurable via node metadata
	preflightCheckTimeout      time.Duration
	preflightCheckBaseInterval time.Duration
}

// ConsulClientFunc creates a new Consul client for the specific Consul config
type ConsulClientFunc func(config *config.ConsulConfig, logger hclog.Logger) (Client, error)

// NodeGetter breaks a circular dependency between client/config.Config and this
// package
type NodeGetter interface {
	GetNode() *structs.Node
}

// NewConsulClientFactory returns a ConsulClientFunc that closes over the
// partition
func NewConsulClientFactory(nodeGetter NodeGetter) ConsulClientFunc {

	return func(config *config.ConsulConfig, logger hclog.Logger) (Client, error) {
		if config == nil {
			return nil, fmt.Errorf("nil consul config")
		}

		logger = logger.Named("consul").With("name", config.Name)

		node := nodeGetter.GetNode()
		partition := node.Attributes["consul.partition"]
		preflightCheckTimeout := durationFromMeta(
			node, "consul.token_preflight_check.timeout", time.Second*10)
		preflightCheckBaseInterval := durationFromMeta(
			node, "consul.token_preflight_check.base", time.Millisecond*500)

		c := &consulClient{
			config:                     config,
			logger:                     logger,
			partition:                  partition,
			preflightCheckTimeout:      preflightCheckTimeout,
			preflightCheckBaseInterval: preflightCheckBaseInterval,
		}

		// Get the Consul API configuration
		apiConf, err := config.ApiConfig()
		if err != nil {
			logger.Error("error creating default Consul API config", "error", err)
			return nil, err
		}

		// Create the API client
		client, err := consulapi.NewClient(apiConf)
		if err != nil {
			logger.Error("error creating Consul client", "error", err)
			return nil, err
		}

		useragent.SetHeaders(client)
		c.client = client

		return c, nil

	}
}

func durationFromMeta(node *structs.Node, key string, defaultDur time.Duration) time.Duration {
	val := node.Meta[key]
	if key == "" {
		return defaultDur
	}
	d, err := time.ParseDuration(val)
	if err != nil || d == 0 {
		return defaultDur
	}
	return d
}

// DeriveTokenWithJWT takes a JWT from request and returns a consul token.
func (c *consulClient) DeriveTokenWithJWT(req JWTLoginRequest) (*consulapi.ACLToken, error) {
	t, _, err := c.client.ACL().Login(&consulapi.ACLLoginParams{
		AuthMethod:  req.AuthMethodName,
		BearerToken: req.JWT,
		Meta:        req.Meta,
	}, &consulapi.WriteOptions{
		Partition: c.partition,
	})

	return t, err
}

func (c *consulClient) RevokeTokens(tokens []*consulapi.ACLToken) error {
	var mErr *multierror.Error
	for _, token := range tokens {
		_, err := c.client.ACL().Logout(&consulapi.WriteOptions{
			Namespace: token.Namespace,
			Partition: token.Partition,
			Token:     token.SecretID,
		})
		mErr = multierror.Append(mErr, err)
	}

	return mErr.ErrorOrNil()
}

// TokenPreflightCheck verifies that a token has been replicated before we
// try to use it for registering services or bootstrapping Envoy
func (c *consulClient) TokenPreflightCheck(pctx context.Context, t *consulapi.ACLToken) error {
	timer, timerStop := helper.NewStoppedTimer()
	defer timerStop()

	var retry uint64
	var err error
	ctx, cancel := context.WithTimeout(pctx, c.preflightCheckTimeout)
	defer cancel()

	for {
		_, _, err = c.client.ACL().TokenReadSelf(&consulapi.QueryOptions{
			Namespace:  t.Namespace,
			Partition:  c.partition,
			AllowStale: true,
			Token:      t.SecretID,
		})
		if err == nil {
			return nil
		}

		retry++
		backoff := helper.Backoff(
			c.preflightCheckBaseInterval, c.preflightCheckBaseInterval*2, retry)
		c.logger.Trace("Consul token not ready", "error", err, "backoff", backoff)
		timer.Reset(backoff)
		select {
		case <-ctx.Done():
			return err
		case <-timer.C:
			continue
		}
	}
}
