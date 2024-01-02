// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func Test_buildNomadBridgeNetConfig(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		name string
		b    *bridgeNetworkConfigurator
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
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			ci.Parallel(t)
			bCfg := buildNomadBridgeNetConfig(*tc.b)
			// Validate that the JSON created is rational
			must.True(t, json.Valid(bCfg))
		})
	}
}
