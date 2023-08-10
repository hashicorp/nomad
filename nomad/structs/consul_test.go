// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestConsul_Copy(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		result := (*Consul)(nil).Copy()
		require.Nil(t, result)
	})

	t.Run("set", func(t *testing.T) {
		result := (&Consul{
			Namespace: "one",
		}).Copy()
		require.Equal(t, &Consul{Namespace: "one"}, result)
	})
}

func TestConsul_Equals(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil and nil", func(t *testing.T) {
		result := (*Consul)(nil).Equal((*Consul)(nil))
		require.True(t, result)
	})

	t.Run("nil and set", func(t *testing.T) {
		result := (*Consul)(nil).Equal(&Consul{Namespace: "one"})
		require.False(t, result)
	})

	t.Run("same", func(t *testing.T) {
		result := (&Consul{Namespace: "one"}).Equal(&Consul{Namespace: "one"})
		require.True(t, result)
	})

	t.Run("different", func(t *testing.T) {
		result := (&Consul{Namespace: "one"}).Equal(&Consul{Namespace: "two"})
		require.False(t, result)
	})
}

func TestConsul_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("empty ns", func(t *testing.T) {
		result := (&Consul{Namespace: ""}).Validate()
		require.Nil(t, result)
	})

	t.Run("with ns", func(t *testing.T) {
		result := (&Consul{Namespace: "one"}).Validate()
		require.Nil(t, result)
	})
}
