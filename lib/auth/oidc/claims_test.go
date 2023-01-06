package oidc

import (
	"encoding/json"
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
		Expected    map[string]interface{}
	}{
		{
			"no mappings",
			nil,
			nil,
			map[string]interface{}{"iss": "https://hashicorp.com"},
			map[string]interface{}{
				"value": map[string]string{},
				"list":  map[string][]string{},
			},
		},

		{
			"key",
			map[string]string{"iss": "issuer"},
			nil,
			map[string]interface{}{"iss": "https://hashicorp.com"},
			map[string]interface{}{
				"value": map[string]string{
					"issuer": "https://hashicorp.com",
				},
				"list": map[string][]string{},
			},
		},

		{
			"key doesn't exist",
			map[string]string{"iss": "issuer"},
			nil,
			map[string]interface{}{"nope": "https://hashicorp.com"},
			map[string]interface{}{
				"value": map[string]string{},
				"list":  map[string][]string{},
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
			map[string]interface{}{
				"value": map[string]string{},
				"list": map[string][]string{
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

			// Marshal our test data
			jsonRaw, err := json.Marshal(tt.Data)
			must.NoError(t, err)

			// Get real selector data
			actual, err := SelectorData(am, jsonRaw, nil)
			must.NoError(t, err)
			must.Eq(t, actual, tt.Expected)
		})
	}
}
