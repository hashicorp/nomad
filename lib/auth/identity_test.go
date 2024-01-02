// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"testing"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
)

func Test_NewIdentity(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                   string
		inputAuthMethodConfig  *structs.ACLAuthMethodConfig
		inputAuthClaims        *structs.ACLAuthClaims
		expectedOutputIdentity *Identity
	}{
		{
			name: "identity with claims",
			inputAuthMethodConfig: &structs.ACLAuthMethodConfig{
				ClaimMappings:     map[string]string{"http://nomad.internal/username": "username"},
				ListClaimMappings: map[string]string{"http://nomad.internal/roles": "roles"},
			},
			inputAuthClaims: &structs.ACLAuthClaims{
				Value: map[string]string{"username": "jrasell"},
				List:  map[string][]string{"roles": {"engineering"}},
			},
			expectedOutputIdentity: &Identity{
				Claims: &structs.ACLAuthClaims{
					Value: map[string]string{"username": "jrasell"},
					List:  map[string][]string{"roles": {"engineering"}},
				},
				ClaimMappings: map[string]string{"value.username": "jrasell"},
			},
		},
		{
			name: "identity without claims",
			inputAuthMethodConfig: &structs.ACLAuthMethodConfig{
				ClaimMappings:     map[string]string{"http://nomad.internal/username": "username"},
				ListClaimMappings: map[string]string{"http://nomad.internal/roles": "roles"},
			},
			inputAuthClaims: &structs.ACLAuthClaims{
				Value: map[string]string{"username": ""},
				List:  map[string][]string{"roles": {""}},
			},
			expectedOutputIdentity: &Identity{
				Claims: &structs.ACLAuthClaims{
					Value: map[string]string{"username": ""},
					List:  map[string][]string{"roles": {""}},
				},
				ClaimMappings: map[string]string{"value.username": ""},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := NewIdentity(tc.inputAuthMethodConfig, tc.inputAuthClaims)
			must.Eq(t, tc.expectedOutputIdentity, actualOutput)
		})
	}
}
