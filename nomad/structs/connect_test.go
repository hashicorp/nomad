// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestTaskKind_IsAnyConnectGateway(t *testing.T) {
	ci.Parallel(t)

	t.Run("gateways", func(t *testing.T) {
		require.True(t, NewTaskKind(ConnectIngressPrefix, "foo").IsAnyConnectGateway())
		require.True(t, NewTaskKind(ConnectTerminatingPrefix, "foo").IsAnyConnectGateway())
		require.True(t, NewTaskKind(ConnectMeshPrefix, "foo").IsAnyConnectGateway())
	})

	t.Run("not gateways", func(t *testing.T) {
		require.False(t, NewTaskKind(ConnectProxyPrefix, "foo").IsAnyConnectGateway())
		require.False(t, NewTaskKind(ConnectNativePrefix, "foo").IsAnyConnectGateway())
		require.False(t, NewTaskKind("", "foo").IsAnyConnectGateway())
	})
}

func TestConnectTransparentProxy_Validate(t *testing.T) {
	testCases := []struct {
		name      string
		tp        *ConsulTransparentProxy
		expectErr string
	}{
		{
			name: "empty is valid",
			tp:   &ConsulTransparentProxy{},
		},
		{
			name:      "invalid CIDR",
			tp:        &ConsulTransparentProxy{ExcludeOutboundCIDRs: []string{"192.168.1.1"}},
			expectErr: `could not parse transparent proxy excluded outbound CIDR as network prefix: netip.ParsePrefix("192.168.1.1"): no '/'`,
		},
		{
			name:      "invalid UID",
			tp:        &ConsulTransparentProxy{UID: "foo"},
			expectErr: `transparent proxy block has invalid UID field: invalid user ID "foo": invalid syntax`,
		},
		{
			name:      "invalid ExcludeUIDs",
			tp:        &ConsulTransparentProxy{ExcludeUIDs: []string{"500000"}},
			expectErr: `transparent proxy block has invalid ExcludeUIDs field: invalid user ID "500000": value out of range`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tp.Validate()
			if tc.expectErr != "" {
				must.EqError(t, err, tc.expectErr)
			} else {
				must.NoError(t, err)
			}
		})
	}

}

func TestConnectTransparentProxy_Equal(t *testing.T) {
	tp1 := &ConsulTransparentProxy{
		UID:                  "101",
		OutboundPort:         1001,
		ExcludeInboundPorts:  []string{"9000", "443"},
		ExcludeOutboundPorts: []uint16{443, 80},
		ExcludeOutboundCIDRs: []string{"10.0.0.0/8", "192.168.1.1"},
		ExcludeUIDs:          []string{"1001", "10"},
		NoDNS:                true,
	}
	tp2 := &ConsulTransparentProxy{
		UID:                  "101",
		OutboundPort:         1001,
		ExcludeInboundPorts:  []string{"443", "9000"},
		ExcludeOutboundPorts: []uint16{80, 443},
		ExcludeOutboundCIDRs: []string{"192.168.1.1", "10.0.0.0/8"},
		ExcludeUIDs:          []string{"10", "1001"},
		NoDNS:                true,
	}
	must.Equal(t, tp1, tp2)
}
