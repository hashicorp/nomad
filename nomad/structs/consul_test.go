package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsul_Copy(t *testing.T) {
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
	t.Run("nil and nil", func(t *testing.T) {
		result := (*Consul)(nil).Equals((*Consul)(nil))
		require.True(t, result)
	})

	t.Run("nil and set", func(t *testing.T) {
		result := (*Consul)(nil).Equals(&Consul{Namespace: "one"})
		require.False(t, result)
	})

	t.Run("same", func(t *testing.T) {
		result := (&Consul{Namespace: "one"}).Equals(&Consul{Namespace: "one"})
		require.True(t, result)
	})

	t.Run("different", func(t *testing.T) {
		result := (&Consul{Namespace: "one"}).Equals(&Consul{Namespace: "two"})
		require.False(t, result)
	})
}

func TestConsul_Validate(t *testing.T) {
	t.Run("empty ns", func(t *testing.T) {
		result := (&Consul{Namespace: ""}).Validate()
		require.Nil(t, result)
	})

	t.Run("with ns", func(t *testing.T) {
		result := (&Consul{Namespace: "one"}).Validate()
		require.Nil(t, result)
	})
}
