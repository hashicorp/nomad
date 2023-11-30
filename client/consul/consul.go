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
// Consul.
type Client interface {
	// DeriveTokenWithJWT logs into Consul using JWT and retrieves a Consul ACL
	// token.
	DeriveTokenWithJWT(JWTLoginRequest) (*consulapi.ACLToken, error)

	RevokeTokens([]*consulapi.ACLToken) error
}

type consulClient struct {
	// client is the API client to interact with consul
	client *consulapi.Client

	// config is the configuration to connect to consul
	config *config.ConsulConfig

	logger hclog.Logger
}

// NewConsulClient creates a new Consul client
func NewConsulClient(config *config.ConsulConfig, logger hclog.Logger) (Client, error) {
	if config == nil {
		return nil, fmt.Errorf("nil consul config")
	}

	logger = logger.Named("consul").With("name", config.Name)

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

// DeriveTokenWithJWT takes a JWT from request and returns a consul token.
func (c *consulClient) DeriveTokenWithJWT(req JWTLoginRequest) (*consulapi.ACLToken, error) {
	t, _, err := c.client.ACL().Login(&consulapi.ACLLoginParams{
		AuthMethod:  req.AuthMethodName,
		BearerToken: req.JWT,
		Meta:        req.Meta,
	}, nil)
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
