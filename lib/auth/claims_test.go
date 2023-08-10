// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"testing"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestSelectorData(t *testing.T) {
	cases := []struct {
		Name        string
		Mapping     map[string]string
		ListMapping map[string]string
		Data        map[string]interface{}
		Expected    *structs.ACLAuthClaims
	}{
		{
			"no mappings",
			nil,
			nil,
			map[string]interface{}{"iss": "https://hashicorp.com"},
			&structs.ACLAuthClaims{
				Value: map[string]string{},
				List:  map[string][]string{},
			},
		},

		{
			"key",
			map[string]string{"iss": "issuer"},
			nil,
			map[string]interface{}{"iss": "https://hashicorp.com"},
			&structs.ACLAuthClaims{
				Value: map[string]string{"issuer": "https://hashicorp.com"},
				List:  map[string][]string{},
			},
		},

		{
			"key doesn't exist",
			map[string]string{"iss": "issuer"},
			nil,
			map[string]interface{}{"nope": "https://hashicorp.com"},
			&structs.ACLAuthClaims{
				Value: map[string]string{},
				List:  map[string][]string{},
			},
		},

		{
			"list",
			nil,
			map[string]string{"groups": "g"},
			map[string]interface{}{
				"groups": []interface{}{
					"A", 42, false,
				},
			},
			&structs.ACLAuthClaims{
				Value: map[string]string{},
				List: map[string][]string{
					"g": {"A", "42", "false"},
				},
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.Name, func(t *testing.T) {

			am := &structs.ACLAuthMethod{
				Config: &structs.ACLAuthMethodConfig{
					ClaimMappings:     tt.Mapping,
					ListClaimMappings: tt.ListMapping,
				},
			}

			// Get real selector data
			actual, err := SelectorData(am, tt.Data, nil)
			must.NoError(t, err)
			must.Eq(t, actual, tt.Expected)
		})
	}
}
