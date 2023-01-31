package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
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
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			tc := tc
			require.NotPanics(t, func() { _ = buildNomadBridgeNetConfig(*tc.b) })
		})
	}
}
