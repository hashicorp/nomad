// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func Test_buildNomadBridgeNetConfig(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		name          string
		withConsulCNI bool
		b             *bridgeNetworkConfigurator
	}{
		{
			name: "empty",
			b:    &bridgeNetworkConfigurator{},
		},

		{
			name: "hairpin",
			b: &bridgeNetworkConfigurator{
				bridgeName:  defaultNomadBridgeName,
				allocSubnet: defaultNomadAllocSubnet,
				hairpinMode: true,
			},
		},
		{
			name: "bad_input",
			b: &bridgeNetworkConfigurator{
				bridgeName:  `bad"`,
				allocSubnet: defaultNomadAllocSubnet,
				hairpinMode: true,
			},
		},
		{
			name:          "consul-cni",
			withConsulCNI: true,
			b: &bridgeNetworkConfigurator{
				bridgeName:  defaultNomadBridgeName,
				allocSubnet: defaultNomadAllocSubnet,
				hairpinMode: true,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bCfg, err := buildNomadBridgeNetConfig(*tc.b, tc.withConsulCNI)
			must.NoError(t, err)

			// Validate that the JSON created is rational
			must.True(t, json.Valid(bCfg))

			// and that it matches golden expectations
			goldenFile := filepath.Join("test_fixtures", tc.name+".conflist.json")
			expect, err := os.ReadFile(goldenFile)
			must.NoError(t, err)
			must.Eq(t, string(expect), string(bCfg)+"\n")
		})
	}
}
