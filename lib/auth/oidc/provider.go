// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/cap/oidc"
	"github.com/hashicorp/nomad/nomad/structs"
)

// providerConfig returns the OIDC provider configuration for an OIDC
// auth-method.
func providerConfig(authMethod *structs.ACLAuthMethod) (*oidc.Config, error) {
	var algs []oidc.Alg
	if len(authMethod.Config.SigningAlgs) > 0 {
		for _, alg := range authMethod.Config.SigningAlgs {
			algs = append(algs, oidc.Alg(alg))
		}
	} else {
		algs = []oidc.Alg{oidc.RS256}
	}

	return oidc.NewConfig(
		authMethod.Config.OIDCDiscoveryURL,
		authMethod.Config.OIDCClientID,
		oidc.ClientSecret(authMethod.Config.OIDCClientSecret),
		algs,
		authMethod.Config.AllowedRedirectURIs,
		oidc.WithAudiences(authMethod.Config.BoundAudiences...),
		oidc.WithProviderCA(strings.Join(authMethod.Config.DiscoveryCaPem, "\n")),
	)
}

// ProviderCache is a cache for OIDC providers. OIDC providers are something
// you don't want to recreate per-request since they make HTTP requests
// when they're constructed.
//
// The ProviderCache purges a provider under two scenarios: (1) the
// provider config is updated, and it is different and (2) after a set
// amount of time (see cacheExpiry for value) in case the remote provider
// configuration changed.
type ProviderCache struct {
	providers map[string]*oidc.Provider
	mu        sync.RWMutex

	// cancel is used to trigger cancellation of any routines when the cache
	// has been informed its parent process is exiting.
	cancel context.CancelFunc
}

// NewProviderCache should be used to initialize a provider cache. This
// will start up background resources to manage the cache.
func NewProviderCache() *ProviderCache {

	// Create a context, so a server that is shutting down can correctly
	// shut down the cache loop and OIDC provider background processes.
	ctx, cancel := context.WithCancel(context.Background())

	result := &ProviderCache{
		providers: map[string]*oidc.Provider{},
		cancel:    cancel,
	}

	// Start the cleanup timer
	go result.runCleanupLoop(ctx)

	return result
}

// Get returns the OIDC provider for the given auth method configuration.
// This will initialize the provider if it isn't already in the cache or
// if the configuration changed.
func (c *ProviderCache) Get(authMethod *structs.ACLAuthMethod) (*oidc.Provider, error) {

	// No matter what we'll use the config of the arg method since we'll
	// use it to compare to existing (if exists) or initialize a new provider.
	oidcCfg, err := providerConfig(authMethod)
	if err != nil {
		return nil, err
	}

	// Get any current provider for the named auth-method.
	var (
		current *oidc.Provider
		ok      bool
	)

	c.mu.RLock()
	current, ok = c.providers[authMethod.Name]
	c.mu.RUnlock()

	// If we have a current value, we want to compare hashes to detect changes.
	if ok {
		currentHash, err := current.ConfigHash()
		if err != nil {
			return nil, err
		}

		newHash, err := oidcCfg.Hash()
		if err != nil {
			return nil, err
		}

		// If the hashes match, this is can be classed as a cache hit.
		if currentHash == newHash {
			return current, nil
		}
	}

	// If we made it here, the provider isn't in the cache OR the config
	// changed. We therefore, need to initialize a new provider.
	newProvider, err := oidc.NewProvider(oidcCfg)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// If we have an old provider, clean up resources.
	if current != nil {
		current.Done()
	}

	c.providers[authMethod.Name] = newProvider

	return newProvider, nil
}

// Delete force deletes a single auth method from the cache by name.
func (c *ProviderCache) Delete(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, ok := c.providers[name]
	if ok {
		p.Done()
		delete(c.providers, name)
	}
}

// Shutdown stops any long-lived cache process and informs each OIDC provider
// that they are done. This should be called whenever the Nomad server is
// shutting down.
func (c *ProviderCache) Shutdown() {
	c.cancel()
	c.clear()
}

// runCleanupLoop runs an infinite loop that clears the cache every cacheExpiry
// duration. This ensures that we force refresh our provider info periodically
// in case anything changes.
func (c *ProviderCache) runCleanupLoop(ctx context.Context) {

	ticker := time.NewTicker(cacheExpiry)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		// We could be more clever and do a per-entry expiry but Nomad won't
		// have more than one ot two auth methods configured, therefore it's
		// not worth the added complexity.
		case <-ticker.C:
			c.clear()
		}
	}
}

// clear is called to delete all the providers in the cache.
func (c *ProviderCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.providers {
		p.Done()
	}
	c.providers = map[string]*oidc.Provider{}
}

// cacheExpiry is the duration after which the provider cache is reset.
const cacheExpiry = 6 * time.Hour
