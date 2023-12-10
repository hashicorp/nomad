// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

var (
	ossFeatures = Features{
		Enterprise: false,
		Namespaces: false,
	}
)

func TestSelf_SKU(t *testing.T) {
	ci.Parallel(t)

	t.Run("oss", func(t *testing.T) {
		s, ok := SKU(Self{
			"Config": {"Version": "v1.9.5"},
		})
		require.True(t, ok)
		require.Equal(t, "oss", s)
	})

	t.Run("oss dev", func(t *testing.T) {
		s, ok := SKU(Self{
			"Config": {"Version": "v1.9.5-dev"},
		})
		require.True(t, ok)
		require.Equal(t, "oss", s)
	})

	t.Run("ent", func(t *testing.T) {
		s, ok := SKU(Self{
			"Config": {"Version": "v1.9.5+ent"},
		})
		require.True(t, ok)
		require.Equal(t, "ent", s)
	})

	t.Run("ent dev", func(t *testing.T) {
		s, ok := SKU(Self{
			"Config": {"Version": "v1.9.5+ent-dev"},
		})
		require.True(t, ok)
		require.Equal(t, "ent", s)
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := SKU(Self{
			"Config": {},
		})
		require.False(t, ok)
	})

	t.Run("malformed", func(t *testing.T) {
		_, ok := SKU(Self{
			"Config": {"Version": "***"},
		})
		require.False(t, ok)
	})
}

func TestSelf_Namespaces(t *testing.T) {
	ci.Parallel(t)

	t.Run("supports namespaces", func(t *testing.T) {
		enabled := Namespaces(Self{
			"Stats": {"license": map[string]interface{}{"features": "Automated Backups, Automated Upgrades, Enhanced Read Scalability, Network Segments, Redundancy Zone, Advanced Network Federation, Namespaces, SSO, Audit Logging"}},
		})
		require.True(t, enabled)
	})

	t.Run("no namespaces", func(t *testing.T) {
		enabled := Namespaces(Self{
			"Stats": {"license": map[string]interface{}{"features": "Automated Backups, Automated Upgrades, Enhanced Read Scalability, Network Segments, Redundancy Zone, Advanced Network Federation, SSO, Audit Logging"}},
		})
		require.False(t, enabled)
	})

	t.Run("stats missing", func(t *testing.T) {
		enabled := Namespaces(Self{})
		require.False(t, enabled)
	})

	t.Run("license missing", func(t *testing.T) {
		enabled := Namespaces(Self{"Stats": {}})
		require.False(t, enabled)
	})

	t.Run("features missing", func(t *testing.T) {
		enabled := Namespaces(Self{"Stats": {"license": map[string]interface{}{}}})
		require.False(t, enabled)
	})
}
