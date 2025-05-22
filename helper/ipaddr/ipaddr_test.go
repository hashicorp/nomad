// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ipaddr

import (
	"net"
	"testing"

	"github.com/shoenig/test/must"
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
			must.Eq(t, tc.expectedOutput, IsAny(tc.inputIP))
		})
	}
}

// TestNormalizeAddr ensures that strings that match either an IP address or URL
// and contain an IPv6 address conform to RFC-5942 §4
// See: https://rfc-editor.org/rfc/rfc5952.html
// Note: This was copied verbatim from Vault:
// https://github.com/hashicorp/vault/blob/58a49e6/internalshared/configutil/normalize_test.go
func TestNormalizeAddr(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		addr            string
		expected        string
		isErrorExpected bool
	}{
		"hostname": {
			addr:     "vaultproject.io",
			expected: "vaultproject.io",
		},
		"hostname port": {
			addr:     "vaultproject.io:8200",
			expected: "vaultproject.io:8200",
		},
		"hostname URL": {
			addr:     "https://vaultproject.io",
			expected: "https://vaultproject.io",
		},
		"hostname port URL": {
			addr:     "https://vaultproject.io:8200",
			expected: "https://vaultproject.io:8200",
		},
		"hostname destination address": {
			addr:     "user@vaultproject.io",
			expected: "user@vaultproject.io",
		},
		"hostname destination address URL": {
			addr:     "http://user@vaultproject.io",
			expected: "http://user@vaultproject.io",
		},
		"hostname destination address URL port": {
			addr:     "http://user@vaultproject.io:8200",
			expected: "http://user@vaultproject.io:8200",
		},
		"ipv4": {
			addr:     "10.10.1.10",
			expected: "10.10.1.10",
		},
		"ipv4 invalid bracketed": {
			addr:     "[10.10.1.10]",
			expected: "10.10.1.10",
		},
		"ipv4 IP:Port addr": {
			addr:     "10.10.1.10:8500",
			expected: "10.10.1.10:8500",
		},
		"ipv4 invalid IP:Port addr": {
			addr:     "[10.10.1.10]:8500",
			expected: "10.10.1.10:8500",
		},
		"ipv4 URL": {
			addr:     "https://10.10.1.10:8200",
			expected: "https://10.10.1.10:8200",
		},
		"ipv4 invalid URL": {
			addr:     "https://[10.10.1.10]:8200",
			expected: "https://10.10.1.10:8200",
		},
		"ipv4 destination address": {
			addr:     "username@10.10.1.10",
			expected: "username@10.10.1.10",
		},
		"ipv4 invalid destination address": {
			addr:     "username@10.10.1.10",
			expected: "username@10.10.1.10",
		},
		"ipv4 destination address port": {
			addr:     "username@10.10.1.10:8200",
			expected: "username@10.10.1.10:8200",
		},
		"ipv4 invalid destination address port": {
			addr:     "username@[10.10.1.10]:8200",
			expected: "username@10.10.1.10:8200",
		},
		"ipv4 destination address URL": {
			addr:     "https://username@10.10.1.10",
			expected: "https://username@10.10.1.10",
		},
		"ipv4 destination address URL port": {
			addr:     "https://username@10.10.1.10:8200",
			expected: "https://username@10.10.1.10:8200",
		},
		"ipv6 invalid address": {
			addr:     "[2001:0db8::0001]",
			expected: "2001:db8::1",
		},
		"ipv6 IP:Port RFC-5952 4.1 conformance leading zeroes": {
			addr:     "[2001:0db8::0001]:8500",
			expected: "[2001:db8::1]:8500",
		},
		"ipv6 RFC-5952 4.1 conformance leading zeroes": {
			addr:     "2001:0db8::0001",
			expected: "2001:db8::1",
		},
		"ipv6 URL RFC-5952 4.1 conformance leading zeroes": {
			addr:     "https://[2001:0db8::0001]:8200",
			expected: "https://[2001:db8::1]:8200",
		},
		"ipv6 bracketed destination address with port RFC-5952 4.1 conformance leading zeroes": {
			addr:     "username@[2001:0db8::0001]:8200",
			expected: "username@[2001:db8::1]:8200",
		},
		"ipv6 invalid ambiguous destination address with port": {
			addr: "username@2001:0db8::0001:8200",
			// Since the address and port are ambiguous the value appears to be
			// only an address and as such is normalized as an address only
			expected: "username@2001:db8::1:8200",
		},
		"ipv6 invalid leading zeroes ambiguous destination address with port": {
			addr: "username@2001:db8:0:1:1:1:1:1:8200",
			// Since the address and port are ambiguous the value is treated as
			// a string because it has too many colons to be a valid IPv6 address.
			expected: "username@2001:db8:0:1:1:1:1:1:8200",
		},
		"ipv6 destination address no port RFC-5952 4.1 conformance leading zeroes": {
			addr:     "username@2001:0db8::0001",
			expected: "username@2001:db8::1",
		},
		"ipv6 RFC-5952 4.2.2 conformance one 16-bit 0 field": {
			addr:     "2001:db8:0:1:1:1:1:1",
			expected: "2001:db8:0:1:1:1:1:1",
		},
		"ipv6 URL RFC-5952 4.2.2 conformance one 16-bit 0 field": {
			addr:     "https://[2001:db8:0:1:1:1:1:1]:8200",
			expected: "https://[2001:db8:0:1:1:1:1:1]:8200",
		},
		"ipv6 destination address with port RFC-5952 4.2.2 conformance one 16-bit 0 field": {
			addr:     "username@[2001:db8:0:1:1:1:1:1]:8200",
			expected: "username@[2001:db8:0:1:1:1:1:1]:8200",
		},
		"ipv6 destination address no port RFC-5952 4.2.2 conformance one 16-bit 0 field": {
			addr:     "username@2001:db8:0:1:1:1:1:1",
			expected: "username@2001:db8:0:1:1:1:1:1",
		},
		"ipv6 RFC-5952 4.2.3 conformance longest run of 0 bits shortened": {
			addr:     "2001:0:0:1:0:0:0:1",
			expected: "2001:0:0:1::1",
		},
		"ipv6 URL RFC-5952 4.2.3 conformance longest run of 0 bits shortened": {
			addr:     "https://[2001:0:0:1:0:0:0:1]:8200",
			expected: "https://[2001:0:0:1::1]:8200",
		},
		"ipv6 destination address with port RFC-5952 4.2.3 conformance longest run of 0 bits shortened": {
			addr:     "username@[2001:0:0:1:0:0:0:1]:8200",
			expected: "username@[2001:0:0:1::1]:8200",
		},
		"ipv6 destination address no port RFC-5952 4.2.3 conformance longest run of 0 bits shortened": {
			addr:     "username@2001:0:0:1:0:0:0:1",
			expected: "username@2001:0:0:1::1",
		},
		"ipv6 RFC-5952 4.2.3 conformance equal runs of 0 bits shortened": {
			addr:     "2001:db8:0:0:1:0:0:1",
			expected: "2001:db8::1:0:0:1",
		},
		"ipv6 URL no port RFC-5952 4.2.3 conformance equal runs of 0 bits shortened": {
			addr:     "https://[2001:db8:0:0:1:0:0:1]",
			expected: "https://[2001:db8::1:0:0:1]",
		},
		"ipv6 URL with port RFC-5952 4.2.3 conformance equal runs of 0 bits shortened": {
			addr:     "https://[2001:db8:0:0:1:0:0:1]:8200",
			expected: "https://[2001:db8::1:0:0:1]:8200",
		},

		"ipv6 destination address with port RFC-5952 4.2.3 conformance equal runs of 0 bits shortened": {
			addr:     "username@[2001:db8:0:0:1:0:0:1]:8200",
			expected: "username@[2001:db8::1:0:0:1]:8200",
		},
		"ipv6 destination address no port RFC-5952 4.2.3 conformance equal runs of 0 bits shortened": {
			addr:     "username@2001:db8:0:0:1:0:0:1",
			expected: "username@2001:db8::1:0:0:1",
		},
		"ipv6 RFC-5952 4.3 conformance downcase hex letters": {
			addr:     "2001:DB8:AC3:FE4::1",
			expected: "2001:db8:ac3:fe4::1",
		},
		"ipv6 URL RFC-5952 4.3 conformance downcase hex letters": {
			addr:     "https://[2001:DB8:AC3:FE4::1]:8200",
			expected: "https://[2001:db8:ac3:fe4::1]:8200",
		},
		"ipv6 destination address with port RFC-5952 4.3 conformance downcase hex letters": {
			addr:     "username@[2001:DB8:AC3:FE4::1]:8200",
			expected: "username@[2001:db8:ac3:fe4::1]:8200",
		},
		"ipv6 destination address no port RFC-5952 4.3 conformance downcase hex letters": {
			addr:     "username@2001:DB8:AC3:FE4::1",
			expected: "username@2001:db8:ac3:fe4::1",
		},
	}
	for name, tc := range tests {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			must.Eq(t, tc.expected, NormalizeAddr(tc.addr))
		})
	}
}
