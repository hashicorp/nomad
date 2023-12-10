// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestNamespacesClient_List(t *testing.T) {
	ci.Parallel(t)

	t.Run("oss", func(t *testing.T) {
		c := NewNamespacesClient(NewMockNamespaces(nil), NewMockAgent(Features{
			Enterprise: false,
			Namespaces: false,
		}))
		list, err := c.List()
		require.NoError(t, err)
		require.Equal(t, []string{"default"}, list) // todo(shoenig): change in followup PR
	})

	t.Run("ent without namespaces", func(t *testing.T) {
		c := NewNamespacesClient(NewMockNamespaces(nil), NewMockAgent(Features{
			Enterprise: true,
			Namespaces: false,
		}))
		list, err := c.List()
		require.NoError(t, err)
		require.Equal(t, []string{"default"}, list) // todo(shoenig): change in followup PR
	})

	t.Run("ent with namespaces", func(t *testing.T) {
		c := NewNamespacesClient(NewMockNamespaces([]string{"banana", "apple", "cherry"}), NewMockAgent(Features{
			Enterprise: true,
			Namespaces: true,
		}))
		list, err := c.List()
		require.NoError(t, err)

		// remember default always exists... if enterprise and namespaces are enabled
		require.Equal(t, []string{"apple", "banana", "cherry", "default"}, list)
	})
}

func TestNewNamespacesClient_stale(t *testing.T) {
	ci.Parallel(t)

	t.Run("ok", func(t *testing.T) {
		now := time.Now()
		updated := now.Add(-59 * time.Second)
		result := stale(updated, now)
		require.False(t, result)
	})

	t.Run("stale", func(t *testing.T) {
		now := time.Now()
		updated := now.Add(-61 * time.Second)
		result := stale(updated, now)
		require.True(t, result)
	})
}

func TestNewNamespacesClient_allowable(t *testing.T) {
	ci.Parallel(t)

	try := func(ent, feature, enabled, exp bool, updated, now time.Time) {
		expired := now.After(updated.Add(namespaceEnabledCacheTTL))
		name := fmt.Sprintf("ent:%t_feature:%t_enabled:%t_exp:%t_expired:%t", ent, feature, enabled, exp, expired)
		t.Run(name, func(t *testing.T) {
			c := NewNamespacesClient(NewMockNamespaces([]string{"a", "b"}), NewMockAgent(Features{
				Enterprise: ent,
				Namespaces: feature,
			}))

			// put the client into the state we want
			c.enabled = enabled
			c.updated = updated

			result := c.allowable(now)
			require.Equal(t, exp, result)
			require.Equal(t, exp, c.enabled) // cached value should match result
		})
	}

	previous := time.Now()
	over := previous.Add(namespaceEnabledCacheTTL + 1)
	under := previous.Add(namespaceEnabledCacheTTL - 1)

	// oss, no refresh, no state change
	try(false, false, false, false, previous, under)

	// oss, refresh, no state change
	try(false, false, false, false, previous, over)

	// ent->oss, refresh, state change
	try(false, false, true, false, previous, over)

	// ent, disabled, no refresh, no state change
	try(true, false, false, false, previous, under)

	// ent, disabled, refresh, no state change
	try(true, false, false, false, previous, over)

	// ent, enabled, no refresh, no state change
	try(true, true, true, true, previous, under)

	// ent, enabled, refresh, no state change
	try(true, true, true, true, previous, over)

	// ent, disabled, refresh, state change (i.e. new license with namespaces)
	try(true, true, false, true, previous, over) // ???

	// ent, disabled, refresh, no state change yet (i.e. new license with namespaces, still cached without)
	try(true, true, false, false, previous, under)
}
