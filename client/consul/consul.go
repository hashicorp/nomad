// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/integrations"
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

type ConsulClient interface {
	// Start initiates the renewal loop of tokens and secrets
	Start()

	// Stop terminates the renewal loop for tokens and secrets
	Stop()

	// DeriveToken contacts the nomad server and fetches wrapped tokens for
	// a set of tasks. The wrapped tokens will be unwrapped using consul and
	// returned.
	DeriveToken(map[string]JWTLoginRequest) (map[string]string, error)

	// RenewToken renews a token with the given increment and adds it to
	// the min-heap for periodic renewal.
	RenewToken(string, int) (<-chan error, error)
}

type consulClient struct {
	// tokenDeriver is a function pointer passed in by the client to derive
	// tokens by making RPC calls to the nomad server.
	tokenDeriver TokenDeriverFunc

	// running indicates if the renewal loop is active or not
	running bool

	// client is the API client to interact with vault
	client *consulapi.Client

	// updateCh is the channel to notify heap modifications to the renewal
	// loop
	updateCh chan struct{}

	// stopCh is the channel to trigger termination of renewal loop
	stopCh chan struct{}

	// heap is the min-heap to keep track of tokens
	heap *integrations.ClientHeap

	// config is the configuration to connect to consul
	config *config.ConsulConfig

	lock   sync.RWMutex
	logger hclog.Logger
}

func NewConsulClient(config *config.ConsulConfig, logger hclog.Logger, tokenDeriver TokenDeriverFunc) (*consulClient, error) {
	if config == nil {
		return nil, fmt.Errorf("nil consul config")
	}

	logger = logger.Named("consul")

	c := &consulClient{
		config: config,
		stopCh: make(chan struct{}),
		// Update channel should be a buffered channel
		updateCh:     make(chan struct{}, 1),
		heap:         integrations.NewClientHeap(),
		logger:       logger,
		tokenDeriver: tokenDeriver,
	}

	// quit early if UseIdentity is unset of set to false
	if config.UseIdentity == nil {
		return c, nil
	}
	if !*config.UseIdentity {
		return c, nil
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

func (c *consulClient) isRunning() bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.running
}

func (c *consulClient) Start() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.config.UseIdentity == nil || !*c.config.UseIdentity || c.running {
		return
	}

	c.running = true

	go c.run()
}

func (c *consulClient) Stop() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.config.UseIdentity == nil || !*c.config.UseIdentity || c.running {
		return
	}

	c.running = false
	close(c.stopCh)
}

func (c *consulClient) Login(token, method, identity string) (string, error) {
	t, _, err := c.client.ACL().Login(&consulapi.ACLLoginParams{
		AuthMethod:  method,
		BearerToken: token,
	}, &consulapi.WriteOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to authenticate with consul for identity %s: %v", identity, err)
	}
	return t.SecretID, nil
}

// DeriveToken takes a JWT from request and returns a consul token
func (c *consulClient) DeriveToken(reqs map[string]JWTLoginRequest) (map[string]string, error) {
	c.lock.Lock()
	defer c.unlockAndUnset()

	tokens := make(map[string]string, len(reqs))
	var mErr *multierror.Error
	for k, req := range reqs {
		// login using the JWT and obtain a Consul ACL token for each workload
		t, err := c.Login(req.JWT, req.AuthMethodName, k)
		if err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("failed to authenticate with consul for identity %s: %v", k, err))
			continue
		}

		siToken, _, err := c.client.ACL().TokenCreate(
			&consulapi.ACLToken{
				ServiceIdentities: []*consulapi.ACLServiceIdentity{
					&consulapi.ACLServiceIdentity{ServiceName: c.config.ServiceIdentity.ServiceName},
				},
			}, &consulapi.WriteOptions{Token: t})

		tokens[k] = siToken.SecretID
	}

	if err := mErr.ErrorOrNil(); err != nil {
		return nil, err
	}

	return tokens, nil
}

// run is the renewal loop which performs the periodic renewals of tokens
func (c *consulClient) run() {
	if c.config.UseIdentity == nil || !*c.config.UseIdentity || c.running {
		return
	}

	var renewalCh <-chan time.Time
	for c.isRunning() {
		// Fetches the candidate for next renewal
		renewalReq, renewalTime := c.nextRenewal()
		if renewalTime.IsZero() {
			// If the heap is empty, don't do anything
			renewalCh = nil
		} else {
			now := time.Now()
			if renewalTime.After(now) {
				// Compute the duration after which the item
				// needs renewal and set the renewalCh to fire
				// at that time.
				renewalDuration := time.Until(renewalTime)
				renewalCh = time.After(renewalDuration)
			} else {
				// If the renewals of multiple items are too
				// close to each other and by the time the
				// entry is fetched from heap it might be past
				// the current time (by a small margin). In
				// which case, fire immediately.
				renewalCh = time.After(0)
			}
		}

		select {
		case <-renewalCh:
			if err := c.renew(renewalReq); err != nil {
				c.logger.Error("error renewing token", "error", err)
				metrics.IncrCounter([]string{"client", "consul", "renew_token_error"}, 1)
			}
		case <-c.updateCh:
			continue
		case <-c.stopCh:
			c.logger.Debug("stopped")
			return
		}
	}
}

func (c *consulClient) nextRenewal() (*integrations.RenewalRequest, time.Time) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.heap.Length() == 0 {
		return nil, time.Time{}
	}

	// Fetches the root element in the min-heap
	nextEntry := c.heap.Peek()
	if nextEntry == nil {
		return nil, time.Time{}
	}

	return nextEntry.Req, nextEntry.Next
}

// renew is a common method to handle renewal of tokens. If renewal is
// successful, min-heap is updated based on the duration after which it needs
// renewal again. The next renewal time is randomly selected to avoid spikes in
// the number of APIs periodically.
func (c *consulClient) renew(req *integrations.RenewalRequest) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if req == nil {
		return fmt.Errorf("nil renewal request")
	}
	if req.ErrCh == nil {
		return fmt.Errorf("renewal request error channel nil")
	}

	if !c.running {
		close(req.ErrCh)
		return fmt.Errorf("vault client is not running")
	}
	if req.ID == "" {
		close(req.ErrCh)
		return fmt.Errorf("missing id in renewal request")
	}
	if req.Increment < 1 {
		close(req.ErrCh)
		return fmt.Errorf("increment cannot be less than 1")
	}

	var renewalErr error
	leaseDuration := req.Increment
	if req.IsToken {
		// Set the token in the API client to the one that needs renewal
		c.client.SetToken(req.ID)

		// Renew the token
		renewResp, err := c.client.Auth().Token().RenewSelf(req.Increment)
		if err != nil {
			renewalErr = fmt.Errorf("failed to renew the vault token: %v", err)
		} else if renewResp == nil || renewResp.Auth == nil {
			renewalErr = fmt.Errorf("failed to renew the vault token")
		} else {
			// Don't set this if renewal fails
			leaseDuration = renewResp.Auth.LeaseDuration
		}

		// Reset the token in the API client before returning
		c.client.SetToken("")
	} else {
		// Renew the secret
		renewResp, err := c.client.Sys().Renew(req.ID, req.Increment)
		if err != nil {
			renewalErr = fmt.Errorf("failed to renew vault secret: %v", err)
		} else if renewResp == nil {
			renewalErr = fmt.Errorf("failed to renew vault secret")
		} else {
			// Don't set this if renewal fails
			leaseDuration = renewResp.LeaseDuration
		}
	}

	// Determine the next renewal time
	renewalDuration := integrations.RenewalTime(rand.Intn, leaseDuration)
	next := time.Now().Add(renewalDuration)

	fatal := false
	if renewalErr != nil &&
		(strings.Contains(renewalErr.Error(), "lease not found or lease is not renewable") ||
			strings.Contains(renewalErr.Error(), "lease is not renewable") ||
			strings.Contains(renewalErr.Error(), "token not found") ||
			strings.Contains(renewalErr.Error(), "permission denied")) {
		fatal = true
	} else if renewalErr != nil {
		c.logger.Debug("renewal error details", "req.increment", req.Increment, "lease_duration", leaseDuration, "renewal_duration", renewalDuration)
		c.logger.Error("error during renewal of lease or token failed due to a non-fatal error; retrying",
			"error", renewalErr, "period", next)
	}

	if c.isTracked(req.ID) {
		if fatal {
			// If encountered with an error where in a lease or a
			// token is not valid at all with vault, and if that
			// item is tracked by the renewal loop, stop renewing
			// it by removing the corresponding heap entry.
			if err := c.heap.Remove(req.ID); err != nil {
				return fmt.Errorf("failed to remove heap entry: %v", err)
			}

			// Report the fatal error to the client
			req.ErrCh <- renewalErr
			close(req.ErrCh)

			return renewalErr
		}

		// If the identifier is already tracked, this indicates a
		// subsequest renewal. In this case, update the existing
		// element in the heap with the new renewal time.
		if err := c.heap.Update(req, next); err != nil {
			return fmt.Errorf("failed to update heap entry. err: %v", err)
		}

		// There is no need to signal an update to the renewal loop
		// here because this case is hit from the renewal loop itself.
	} else {
		if fatal {
			// If encountered with an error where in a lease or a
			// token is not valid at all with vault, and if that
			// item is not tracked by renewal loop, don't add it.

			// Report the fatal error to the client
			req.ErrCh <- renewalErr
			close(req.ErrCh)

			return renewalErr
		}

		// If the identifier is not already tracked, this is a first
		// renewal request. In this case, add an entry into the heap
		// with the next renewal time.
		if err := c.heap.Push(req, next); err != nil {
			return fmt.Errorf("failed to push an entry to heap.  err: %v", err)
		}

		// Signal an update for the renewal loop to trigger a fresh
		// computation for the next best candidate for renewal.
		if c.running {
			select {
			case c.updateCh <- struct{}{}:
			default:
			}
		}
	}

	return nil
}

func (c *consulClient) unlockAndUnset() {
	c.client.SetToken("")
	c.lock.Unlock()
}
