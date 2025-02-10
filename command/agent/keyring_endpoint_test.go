// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHTTP_Keyring_CRUD(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {

		respW := httptest.NewRecorder()

		// List (get bootstrap key)

		req, err := http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		must.NoError(t, err)
		obj, err := s.Server.KeyringRequest(respW, req)
		must.NoError(t, err)
		listResp := obj.([]*structs.RootKeyMeta)
		must.Len(t, 1, listResp)
		key0 := listResp[0].KeyID

		// Create a variable to test force key deletion
		state := s.server.State()
		encryptedVar := mock.VariableEncrypted()
		encryptedVar.KeyID = key0
		varSetResp := state.VarSet(0, &structs.VarApplyStateRequest{Var: encryptedVar})
		must.NoError(t, varSetResp.Error)

		// Rotate

		req, err = http.NewRequest(http.MethodPut, "/v1/operator/keyring/rotate", nil)
		must.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		must.NoError(t, err)
		must.NotEq(t, "", respW.HeaderMap.Get("X-Nomad-Index"))
		rotateResp := obj.(structs.KeyringRotateRootKeyResponse)
		must.NotNil(t, rotateResp.Key)
		must.True(t, rotateResp.Key.IsActive())
		key1 := rotateResp.Key.KeyID

		// Rotate with prepublish

		publishTime := time.Now().Add(24 * time.Hour).UnixNano()
		req, err = http.NewRequest(http.MethodPut,
			fmt.Sprintf("/v1/operator/keyring/rotate?publish_time=%d", publishTime), nil)
		must.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		must.NoError(t, err)
		must.NotEq(t, "", respW.HeaderMap.Get("X-Nomad-Index"))
		rotateResp = obj.(structs.KeyringRotateRootKeyResponse)
		must.NotNil(t, rotateResp.Key)
		must.True(t, rotateResp.Key.IsPrepublished())
		key2 := rotateResp.Key.KeyID

		// List

		req, err = http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		must.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		must.NoError(t, err)
		listResp = obj.([]*structs.RootKeyMeta)
		must.Len(t, 3, listResp)
		for _, key := range listResp {
			switch key.KeyID {
			case key0:
				must.True(t, key.IsInactive(), must.Sprint("initial key should be inactive"))
			case key1:
				must.True(t, key.IsActive(), must.Sprint("new key should be active"))
			case key2:
				must.True(t, key.IsPrepublished(),
					must.Sprint("prepublished key should not be active"))
			}
		}

		// Delete the original key and verify its gone

		req, err = http.NewRequest(http.MethodDelete, "/v1/operator/keyring/key/"+key0, nil)
		must.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		must.Error(t, err)
		must.EqError(t, err, "root key in use, cannot delete")

		req, err = http.NewRequest(http.MethodDelete, "/v1/operator/keyring/key/"+key0+"?force=true", nil)
		must.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		must.NoError(t, err)

		req, err = http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		must.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		must.NoError(t, err)
		listResp = obj.([]*structs.RootKeyMeta)
		must.Len(t, 2, listResp)
		for _, key := range listResp {
			switch key.KeyID {
			case key0:
				t.Fatalf("initial key should have been deleted")
			case key1:
				must.True(t, key.IsActive(), must.Sprint("new key should be active"))
			case key2:
				must.True(t, key.IsPrepublished(),
					must.Sprint("prepublished key should not be active"))
			}
		}
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
