// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ipaddr

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_IsAny(t *testing.T) {
	testCases := []struct {
		inputIP        string
		expectedOutput bool
		name           string
	}{
		{
			inputIP:        "0.0.0.0",
			expectedOutput: true,
			name:           "string ipv4 any IP",
		},
		{
			inputIP:        "::",
			expectedOutput: true,
			name:           "string ipv6 any IP",
		},
		{
			inputIP:        net.IPv4zero.String(),
			expectedOutput: true,
			name:           "net.IP ipv4 any",
		},
		{
			inputIP:        net.IPv6zero.String(),
			expectedOutput: true,
			name:           "net.IP ipv6 any",
		},
		{
			inputIP:        "10.10.10.10",
			expectedOutput: false,
			name:           "internal ipv4 address",
		},
		{
			inputIP:        "8.8.8.8",
			expectedOutput: false,
			name:           "public ipv4 address",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedOutput, IsAny(tc.inputIP))
		})
	}
}
