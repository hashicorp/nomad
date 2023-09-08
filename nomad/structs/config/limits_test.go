// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/stretchr/testify/require"
)

// TestLimits_Defaults asserts the default limits are valid.
func TestLimits_Defaults(t *testing.T) {
	ci.Parallel(t)

	l := DefaultLimits()
	d, err := time.ParseDuration(l.HTTPSHandshakeTimeout)
	require.NoError(t, err)
	require.True(t, d > 0)

	d, err = time.ParseDuration(l.RPCHandshakeTimeout)
	require.NoError(t, err)
	require.True(t, d > 0)
}

// TestLimits_Copy asserts Limits structs are deep copied.
func TestLimits_Copy(t *testing.T) {
	ci.Parallel(t)

	o := DefaultLimits()
	c := o.Copy()

	// Assert changes to copy are not propagated to the original
	c.HTTPSHandshakeTimeout = "1s"
	c.HTTPMaxConnsPerClient = pointer.Of(50)
	c.RPCHandshakeTimeout = "1s"
	c.RPCMaxConnsPerClient = pointer.Of(50)

	require.NotEqual(t, c.HTTPSHandshakeTimeout, o.HTTPSHandshakeTimeout)

	// Pointers should be different
	require.True(t, c.HTTPMaxConnsPerClient != o.HTTPMaxConnsPerClient)

	require.NotEqual(t, c.HTTPMaxConnsPerClient, o.HTTPMaxConnsPerClient)
	require.NotEqual(t, c.RPCHandshakeTimeout, o.RPCHandshakeTimeout)

	// Pointers should be different
	require.True(t, c.RPCMaxConnsPerClient != o.RPCMaxConnsPerClient)

	require.NotEqual(t, c.RPCMaxConnsPerClient, o.RPCMaxConnsPerClient)
}

// TestLimits_Merge asserts non-zero fields from the method argument take
// precedence over the existing limits.
func TestLimits_Merge(t *testing.T) {
	ci.Parallel(t)

	l := Limits{}
	o := DefaultLimits()
	m := l.Merge(o)

	// Operands should not change
	require.Equal(t, Limits{}, l)
	require.Equal(t, DefaultLimits(), o)

	// m == o
	require.Equal(t, m, DefaultLimits())

	o.HTTPSHandshakeTimeout = "10s"
	m2 := m.Merge(o)

	// Operands should not change
	require.Equal(t, m, DefaultLimits())

	// Use short struct initialization style so it fails to compile if
	// fields are added
	expected := Limits{"10s", pointer.Of(100), "5s", pointer.Of(100)}
	require.Equal(t, expected, m2)

	// Mergin in 0 values should not change anything
	m3 := m2.Merge(Limits{})
	require.Equal(t, m2, m3)
}
