// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/cap/oidc"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestValidate(t *testing.T) {
	iat := time.Now().Unix()
	nbf := time.Now().Unix()
	exp := time.Now().Add(time.Hour).Unix()

	claims := jwt.MapClaims{
		"foo":    "bar",
		"issuer": "test suite",
		"float":  3.14,
		"iat":    iat,
		"nbf":    nbf,
		"exp":    exp,
	}

	wantedClaims := map[string]any{
		"foo":    "bar",
		"issuer": "test suite",
		"float":  3.14,
		"iat":    float64(iat),
		"nbf":    float64(nbf),
		"exp":    float64(exp),
	}

	// appended to JWKS test server URL
	wellKnownJWKS := "/.well-known/jwks.json"

	// generate a key pair, so that we can use it for consistent signing and
	// set it as our test server key
	rsaKey, err := rsa.GenerateKey(rand.Reader, 4096)
	must.NoError(t, err)

	token, _, err := mock.SampleJWTokenWithKeys(claims, rsaKey)
	must.NoError(t, err)
	tokenWithNoClaims, pubKeyPem, err := mock.SampleJWTokenWithKeys(nil, rsaKey)
	must.NoError(t, err)

	// make an expired token...
	expired := time.Now().Add(-time.Hour).Unix()
	expiredClaims := jwt.MapClaims{"iat": iat, "nbf": nbf, "exp": expired}
	expiredToken, _, err := mock.SampleJWTokenWithKeys(expiredClaims, rsaKey)
	must.NoError(t, err)

	// ...and one with invalid issuer, too
	invalidIssuer := jwt.MapClaims{"iat": iat, "nbf": nbf, "exp": exp, "iss": "hashicorp vault"}
	invalidIssuerToken, _, err := mock.SampleJWTokenWithKeys(invalidIssuer, rsaKey)
	must.NoError(t, err)

	testServer := oidc.StartTestProvider(t)
	defer testServer.Stop()

	keyID := strconv.Itoa(int(time.Now().Unix()))
	testServer.SetSigningKeys(rsaKey, rsaKey.Public(), oidc.RS256, keyID)
	tokenSignedWithRemoteServerKeys, _, err := mock.SampleJWTokenWithKeys(claims, rsaKey)
	must.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		conf    *structs.ACLAuthMethodConfig
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid signature, local verification",
			token:   token,
			conf:    &structs.ACLAuthMethodConfig{JWTValidationPubKeys: []string{pubKeyPem}},
			want:    wantedClaims,
			wantErr: false,
		},
		{
			name:    "valid signature, local verification, no claims",
			token:   tokenWithNoClaims,
			conf:    &structs.ACLAuthMethodConfig{JWTValidationPubKeys: []string{pubKeyPem}},
			want:    nil,
			wantErr: true,
		},
		{
			name:  "valid signature, JWKS verification",
			token: tokenSignedWithRemoteServerKeys,
			conf: &structs.ACLAuthMethodConfig{
				JWKSURL:    testServer.Addr() + wellKnownJWKS,
				JWKSCACert: testServer.CACert(),
			},
			want:    wantedClaims,
			wantErr: false,
		},
		{
			name:  "valid signature, OIDC verification",
			token: tokenSignedWithRemoteServerKeys,
			conf: &structs.ACLAuthMethodConfig{
				OIDCDiscoveryURL: testServer.Addr(),
				DiscoveryCaPem:   []string{testServer.CACert()},
			},
			want:    wantedClaims,
			wantErr: false,
		},
		{
			name:    "expired token, local verification",
			token:   expiredToken,
			conf:    &structs.ACLAuthMethodConfig{JWTValidationPubKeys: []string{pubKeyPem}},
			want:    nil,
			wantErr: true,
		},
		{
			name:  "invalid issuer, local verification",
			token: invalidIssuerToken,
			conf: &structs.ACLAuthMethodConfig{
				JWTValidationPubKeys: []string{pubKeyPem},
				BoundIssuer:          []string{"test suite"},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Validate(context.Background(), tt.token, tt.conf)
			if !tt.wantErr {
				must.Nil(t, err, must.Sprint(err))
			}
			must.Eq(t, got, tt.want)
		})
	}
}
