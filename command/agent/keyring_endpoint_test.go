// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHTTP_Keyring_CRUD(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {

		respW := httptest.NewRecorder()

		// List (get bootstrap key)

		req, err := http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		require.NoError(t, err)
		obj, err := s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		listResp := obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 1)
		oldKeyID := listResp[0].KeyID

		// Rotate

		req, err = http.NewRequest(http.MethodPut, "/v1/operator/keyring/rotate", nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))
		rotateResp := obj.(structs.KeyringRotateRootKeyResponse)
		require.NotNil(t, rotateResp.Key)
		require.True(t, rotateResp.Key.Active())
		newID1 := rotateResp.Key.KeyID

		// List

		req, err = http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		listResp = obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 2)
		for _, key := range listResp {
			if key.KeyID == newID1 {
				require.True(t, key.Active(), "new key should be active")
			} else {
				require.False(t, key.Active(), "initial key should be inactive")
			}
		}

		// Delete the old key and verify its gone

		req, err = http.NewRequest(http.MethodDelete, "/v1/operator/keyring/key/"+oldKeyID, nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)

		req, err = http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		listResp = obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 1)
		require.Equal(t, newID1, listResp[0].KeyID)
		require.True(t, listResp[0].Active())
		require.Len(t, listResp, 1)
	})
}

// TestHTTP_Keyring_JWKS asserts the JWKS endpoint is enabled by default and
// caches relative to the key rotation threshold.
func TestHTTP_Keyring_JWKS(t *testing.T) {
	ci.Parallel(t)

	threshold := 3 * 24 * time.Hour
	cb := func(c *Config) {
		c.Server.RootKeyRotationThreshold = threshold.String()
	}

	httpTest(t, cb, func(s *TestAgent) {
		respW := httptest.NewRecorder()

		req, err := http.NewRequest(http.MethodGet, structs.JWKSPath, nil)
		must.NoError(t, err)

		obj, err := s.Server.JWKSRequest(respW, req)
		must.NoError(t, err)

		jwks := obj.(*jose.JSONWebKeySet)
		must.SliceLen(t, 1, jwks.Keys)

		// Assert that caching headers are set to < the rotation threshold
		cacheHeaders := respW.Header().Values("Cache-Control")
		must.SliceLen(t, 1, cacheHeaders)
		must.StrHasPrefix(t, "max-age=", cacheHeaders[0])
		parts := strings.Split(cacheHeaders[0], "=")
		ttl, err := strconv.Atoi(parts[1])
		must.NoError(t, err)
		must.Less(t, int(threshold.Seconds()), ttl)
	})
}

// TestHTTP_Keyring_OIDCDisco_Disabled asserts that the OIDC Discovery endpoint
// is disabled by default.
func TestHTTP_Keyring_OIDCDisco_Disabled(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {
		respW := httptest.NewRecorder()

		req, err := http.NewRequest(http.MethodGet, structs.JWKSPath, nil)
		must.NoError(t, err)

		_, err = s.Server.OIDCDiscoveryRequest(respW, req)
		must.ErrorContains(t, err, "OIDC Discovery endpoint disabled")
		codedErr := err.(HTTPCodedError)
		must.Eq(t, http.StatusNotFound, codedErr.Code())
	})
}

// TestHTTP_Keyring_OIDCDisco_Enabled asserts that the OIDC Discovery endpoint
// is enabled when OIDCIssuer is set.
func TestHTTP_Keyring_OIDCDisco_Enabled(t *testing.T) {
	ci.Parallel(t)

	// Set OIDCIssuer to a valid looking (but fake) issuer
	const testIssuer = "https://oidc.test.nomadproject.io"

	cb := func(c *Config) {
		c.Server.OIDCIssuer = testIssuer
	}

	httpTest(t, cb, func(s *TestAgent) {
		respW := httptest.NewRecorder()

		req, err := http.NewRequest(http.MethodGet, structs.JWKSPath, nil)
		must.NoError(t, err)

		obj, err := s.Server.OIDCDiscoveryRequest(respW, req)
		must.NoError(t, err)

		oidcConf := obj.(*structs.OIDCDiscoveryConfig)
		must.Eq(t, testIssuer, oidcConf.Issuer)
		must.StrHasPrefix(t, testIssuer, oidcConf.JWKS)
	})
}
