package allocrunner

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_buildNomadBridgeNetConfig(t *testing.T) {
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
			name: "bad_name",
			b: &bridgeNetworkConfigurator{
				bridgeName: `bad","foo":"bar`,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			var out []byte
			require.NotPanics(t, func() { out = buildNomadBridgeNetConfig(*tc.b) })
			if tc.name == "bad_name" {
				fmt.Println(string(out))
			}
		})
	}
}
