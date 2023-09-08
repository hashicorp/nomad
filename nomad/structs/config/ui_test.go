// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestUIConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	fullConfig := &UIConfig{
		Enabled: true,
		Consul: &ConsulUIConfig{
			BaseUIURL: "http://consul.example.com:8500",
		},
		Vault: &VaultUIConfig{
			BaseUIURL: "http://vault.example.com:8200",
		},
		Label: &LabelUIConfig{
			Text:            "Example Cluster",
			BackgroundColor: "blue",
			TextColor:       "#fff",
		},
		ContentSecurityPolicy: DefaultCSPConfig(),
	}

	testCases := []struct {
		name   string
		left   *UIConfig
		right  *UIConfig
		expect *UIConfig
	}{
		{
			name:   "merge onto empty config",
			left:   &UIConfig{},
			right:  fullConfig,
			expect: fullConfig,
		},
		{
			name:   "merge in a nil config",
			left:   fullConfig,
			right:  nil,
			expect: fullConfig,
		},
		{
			name: "merge onto zero-values",
			left: &UIConfig{
				Enabled: false,
				Consul: &ConsulUIConfig{
					BaseUIURL: "http://consul-other.example.com:8500",
				},
			},
			right:  fullConfig,
			expect: fullConfig,
		},
		{
			name: "merge from zero-values",
			left: &UIConfig{
				Enabled: true,
				Consul: &ConsulUIConfig{
					BaseUIURL: "http://consul-other.example.com:8500",
				},
				ContentSecurityPolicy: DefaultCSPConfig(),
			},
			right: &UIConfig{},
			expect: &UIConfig{
				Enabled: false,
				Consul: &ConsulUIConfig{
					BaseUIURL: "http://consul-other.example.com:8500",
				},
				Vault:                 &VaultUIConfig{},
				Label:                 &LabelUIConfig{},
				ContentSecurityPolicy: DefaultCSPConfig(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			result := tc.left.Merge(tc.right)
			require.Equal(t, tc.expect, result)
		})
	}

}
