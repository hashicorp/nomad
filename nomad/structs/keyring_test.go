// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestKEKProviderConfig_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                  string
		inputKeyringConfig    *KEKProviderConfig
		expectedErrorContains string
	}{
		{
			name:                  "nil",
			inputKeyringConfig:    nil,
			expectedErrorContains: "",
		},
		{
			name: "aead",
			inputKeyringConfig: &KEKProviderConfig{
				Provider: "aead",
			},
			expectedErrorContains: "",
		},
		{
			name: "awskms",
			inputKeyringConfig: &KEKProviderConfig{
				Provider: "awskms",
			},
			expectedErrorContains: "",
		},
		{
			name: "azurekeyvault",
			inputKeyringConfig: &KEKProviderConfig{
				Provider: "azurekeyvault",
			},
			expectedErrorContains: "",
		},
		{
			name: "gcpckms",
			inputKeyringConfig: &KEKProviderConfig{
				Provider: "gcpckms",
			},
			expectedErrorContains: "",
		},
		{
			name: "transit",
			inputKeyringConfig: &KEKProviderConfig{
				Provider: "transit",
			},
			expectedErrorContains: "",
		},
		{
			name: "unknown",
			inputKeyringConfig: &KEKProviderConfig{
				Provider: "unknown",
			},
			expectedErrorContains: "unknown keyring provider",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := tc.inputKeyringConfig.Validate()
			if tc.expectedErrorContains == "" {
				must.NoError(t, actualError)
			} else {
				must.ErrorContains(t, actualError, tc.expectedErrorContains)
			}
		})
	}
}

func TestKeyring_OIDCDiscoveryConfig(t *testing.T) {
	ci.Parallel(t)

	c, err := NewOIDCDiscoveryConfig("")
	must.Error(t, err)
	must.Nil(t, c)

	c, err = NewOIDCDiscoveryConfig(":/invalid")
	must.Error(t, err)
	must.Nil(t, c)

	const testIssuer = "https://oidc.test.nomadproject.io/"
	c, err = NewOIDCDiscoveryConfig(testIssuer)
	must.NoError(t, err)
	must.NotNil(t, c)
	must.Eq(t, testIssuer, c.Issuer)
	must.StrHasPrefix(t, testIssuer, c.JWKS)
	must.SliceNotEmpty(t, c.IDTokenAlgs)
	must.SliceNotEmpty(t, c.ResponseTypes)
	must.SliceNotEmpty(t, c.Subjects)
}
