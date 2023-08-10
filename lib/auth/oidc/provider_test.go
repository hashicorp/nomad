// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"testing"
	"time"

	"github.com/hashicorp/cap/oidc"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestProviderCache(t *testing.T) {

	// Instantiate a new cache.
	testCache := NewProviderCache()
	defer testCache.Shutdown()

	// Create our OIDC test provider.
	oidcTestProvider := oidc.StartTestProvider(t)
	oidcTestProvider.SetClientCreds("bob", "ssshhhh")
	_, _, tpAlg, _ := oidcTestProvider.SigningKeys()

	// Create a mocked auth-method; avoiding the mock as the hashicorp/cap lib
	// performs validation on certain fields.
	authMethod := structs.ACLAuthMethod{
		Name:          "test-oidc-auth-method",
		Type:          "OIDC",
		TokenLocality: "global",
		MaxTokenTTL:   100 * time.Hour,
		Default:       true,
		Config: &structs.ACLAuthMethodConfig{
			OIDCDiscoveryURL:    oidcTestProvider.Addr(),
			OIDCClientID:        "alice",
			OIDCClientSecret:    "ssshhhh",
			AllowedRedirectURIs: []string{"http://example.com"},
			DiscoveryCaPem:      []string{oidcTestProvider.CACert()},
			SigningAlgs:         []string{string(tpAlg)},
		},
	}
	authMethod.SetHash()

	// Perform a lookup against the cache. This should generate a new provider
	// for our auth-method.
	oidcProvider1, err := testCache.Get(&authMethod)
	must.NoError(t, err)
	must.NotNil(t, oidcProvider1)

	// Perform another lookup, checking that the returned pointer value is the
	// same.
	oidcProvider2, err := testCache.Get(&authMethod)
	must.NoError(t, err)
	must.EqOp(t, oidcProvider1, oidcProvider2)

	// Update an aspect on the auth-method config and then perform a lookup.
	// This should return a non-cached provider.
	authMethod.Config.AllowedRedirectURIs = []string{"http://example.com/foo/bar/baz/haz"}
	oidcProvider3, err := testCache.Get(&authMethod)
	must.NoError(t, err)
	must.NotEqOp(t, oidcProvider2, oidcProvider3)

	// Ensure the cache only contains a single entry to show we successfully
	// replaced the stale entry.
	testCache.mu.RLock()
	must.MapLen(t, 1, testCache.providers)
	testCache.mu.RUnlock()
}
