// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// TokenDeriverFunc takes an allocation and a set of tasks and derives a
// service identity token for each. Requests go through nomad server.
type TokenDeriverFunc func(*structs.Allocation, []string) (map[string]string, error)

// ServiceIdentityAPI is the interface the Nomad Client uses to request Consul
// Service Identity tokens through Nomad Server.
//
// ACL requirements
// - acl:write (used by Server only)
type ServiceIdentityAPI interface {
	// DeriveSITokens contacts the nomad server and requests consul service
	// identity tokens be generated for tasks in the allocation.
	DeriveSITokens(alloc *structs.Allocation, tasks []string) (map[string]string, error)
}

// SupportedProxiesAPI is the interface the Nomad Client uses to request from
// Consul the set of supported proxied to use for Consul Connect.
//
// No ACL requirements
type SupportedProxiesAPI interface {
	Proxies() (map[string][]string, error)
}

// JWTLoginRequest is an object representing a login request with JWT
type JWTLoginRequest struct {
	JWT            string
	Role           string
	AuthMethodName string
}

// ConsulClient is the interface that the nomad client uses to interact with
// Consul.
type ConsulClient interface {
	// DeriveSITokenWithJWT logs into Consul using JWT and retrieves a Consul
	// SI ACL token.
	DeriveSITokenWithJWT(map[string]JWTLoginRequest) (map[string]string, error)
}

type consulClient struct {
	// client is the API client to interact with consul
	client *consulapi.Client

	// config is the configuration to connect to consul
	config *config.ConsulConfig

	logger hclog.Logger
}

// NewConsulClient creates a new Consul client
func NewConsulClient(config *config.ConsulConfig, logger hclog.Logger) (*consulClient, error) {
	if config == nil {
		return nil, fmt.Errorf("nil consul config")
	}

	logger = logger.Named("consul")

	// if UseIdentity is unset of set to false, return an empty client
	if config.UseIdentity == nil || !*config.UseIdentity {
		return nil, nil
	}

	c := &consulClient{
		config: config,
		logger: logger,
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

// DeriveSITokenWithJWT takes a JWT from request and returns a consul token for
// each workload in the request
func (c *consulClient) DeriveSITokenWithJWT(reqs map[string]JWTLoginRequest) (map[string]string, error) {
	tokens := make(map[string]string, len(reqs))
	var mErr *multierror.Error

	for k, req := range reqs {
		// login using the JWT and obtain a Consul ACL token for each workload
		t, _, err := c.client.ACL().Login(&consulapi.ACLLoginParams{
			AuthMethod:  req.AuthMethodName,
			BearerToken: req.JWT,
		}, &consulapi.WriteOptions{})
		if err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf(
				"failed to authenticate with consul for identity %s: %v", k, err,
			))
			continue
		}

		tokens[k] = t.SecretID
	}

	if err := mErr.ErrorOrNil(); err != nil {
		return tokens, err
	}

	return tokens, nil
}
