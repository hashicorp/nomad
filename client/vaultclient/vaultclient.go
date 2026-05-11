// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package vaultclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/useragent"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vaultapi "github.com/hashicorp/vault/api"
)

// VaultClientFunc is the interface of a function that retreives the VaultClient
// by cluster name. This function is injected into the allocrunner/taskrunner
type VaultClientFunc func(string) (VaultClient, error)

// JWTLoginRequest is used to derive a Vault ACL token using a JWT login
// request.
type JWTLoginRequest struct {
	// JWT is the signed JWT to be used for the login request.
	JWT string

	// Role is Vault ACL role to use for the login request. If empty, the
	// Nomad client's create_from_role value is used, or the Vault cluster
	// default role.
	Role string

	// Namespace is the Vault namespace to use for the login request. If empty,
	// the Nomad client's Vault configuration namespace will be used.
	Namespace string
}

// VaultClient is the interface which nomad client uses to interact with vault and
// periodically renews the tokens and secrets.
type VaultClient interface {
	// DeriveTokenWithJWT returns a Vault ACL token using the JWT login
	// endpoint, along with whether or not the token is renewable and its lease
	// duration.
	DeriveTokenWithJWT(context.Context, JWTLoginRequest) (string, bool, int, error)

	// Renew returns a tokens renewed lease duration and expiration
	Renew(context.Context, string, int) (time.Duration, time.Time, error)
}

// Implementation of VaultClient interface to interact with vault and perform
// token and lease renewals periodically.
type vaultClient struct {

	// running indicates if the renewal loop is active or not
	running bool

	// client is the API client to interact with vault
	client *vaultapi.Client

	// updateCh is the channel to notify heap modifications to the renewal
	// loop
	updateCh chan struct{}

	// stopCh is the channel to trigger termination of renewal loop
	stopCh chan struct{}

	// config is the configuration to connect to vault
	config *config.VaultConfig

	// TODO: maybe use to serialize requests to slow them down
	lock sync.RWMutex

	logger hclog.Logger
}

// NewVaultClient returns a new vault client from the given config.
func NewVaultClient(config *config.VaultConfig, logger hclog.Logger) (*vaultClient, error) {
	if config == nil {
		return nil, fmt.Errorf("nil vault config")
	}

	logger = logger.Named("vault").With("name", config.Name)

	c := &vaultClient{
		config:   config,
		stopCh:   make(chan struct{}),
		updateCh: make(chan struct{}, 1), // Update channel should be buffered.
		logger:   logger,
	}

	if !config.IsEnabled() {
		return c, nil
	}

	// Get the Vault API configuration
	apiConf, err := config.ApiConfig()
	if err != nil {
		logger.Error("error creating vault API config", "error", err)
		return nil, err
	}

	// Create the Vault API client
	client, err := vaultapi.NewClient(apiConf)
	if err != nil {
		logger.Error("error creating vault client", "error", err)
		return nil, err
	}

	// Set our Nomad user agent
	useragent.SetHeaders(client)

	// SetHeaders above will replace all headers, make this call second
	if config.Namespace != "" {
		logger.Debug("configuring Vault namespace", "namespace", config.Namespace)
		client.SetNamespace(config.Namespace)
	}

	c.client = client

	return c, nil
}

// DeriveTokenWithJWT returns a Vault ACL token using the JWT login endpoint.
func (c *vaultClient) DeriveTokenWithJWT(ctx context.Context, req JWTLoginRequest) (string, bool, int, error) {
	cc, err := c.client.Clone()
	if err != nil {
		return "", false, 0, err
	}

	// Make sure the login request is not passing any token and that we're using
	// the expected namespace to login
	cc.SetToken("")
	if req.Namespace != "" {
		cc.SetNamespace(req.Namespace)
	}

	jwtLoginPath := fmt.Sprintf("auth/%s/login", c.config.JWTAuthBackendPath)
	s, err := cc.Logical().WriteWithContext(ctx, jwtLoginPath,
		map[string]any{
			"role": req.Role,
			"jwt":  req.JWT,
		},
	)
	if err != nil {
		return "", false, 0, fmt.Errorf("failed to login with JWT: %v", err)
	}
	if s == nil {
		return "", false, 0, errors.New("JWT login returned an empty secret")
	}
	if s.Auth == nil {
		return "", false, 0, errors.New("JWT login did not return a token")
	}

	for _, w := range s.Warnings {
		c.logger.Warn("JWT login warning", "warning", w)
	}

	return s.Auth.ClientToken, s.Auth.Renewable, s.Auth.LeaseDuration, nil
}

func (c *vaultClient) Renew(ctx context.Context, token string, lease int) (duration time.Duration, exp time.Time, err error) {
	cc, err := c.client.Clone()
	if err != nil {
		return 0, time.Time{}, err
	}
	cc.SetToken(token)

	res, err := cc.Auth().Token().RenewSelfWithContext(ctx, lease)
	if err != nil {
		return 0, time.Time{}, err
	}

	duration = time.Duration(res.Auth.LeaseDuration * int(time.Second))

	return duration, time.Now().Add(duration), nil
}
