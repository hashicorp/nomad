package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsul_Canonicalize(t *testing.T) {
	t.Run("missing ns", func(t *testing.T) {
		c := new(Consul)
		c.Canonicalize()
		require.Empty(t, c.Namespace)
	})

	t.Run("complete", func(t *testing.T) {
		c := &Consul{Namespace: "foo"}
		c.Canonicalize()
		require.Equal(t, "foo", c.Namespace)
	})
}

func TestConsul_Copy(t *testing.T) {
	t.Run("complete", func(t *testing.T) {
		result := (&Consul{
			Namespace: "foo",
		}).Copy()
		require.Equal(t, &Consul{
			Namespace: "foo",
		}, result)
	})
}

func TestConsul_MergeNamespace(t *testing.T) {
	t.Run("already set", func(t *testing.T) {
		a := &Consul{Namespace: "foo"}
		ns := stringToPtr("bar")
		a.MergeNamespace(ns)
		require.Equal(t, "foo", a.Namespace)
		require.Equal(t, "bar", *ns)
	})

	t.Run("inherit", func(t *testing.T) {
		a := &Consul{Namespace: ""}
		ns := stringToPtr("bar")
		a.MergeNamespace(ns)
		require.Equal(t, "bar", a.Namespace)
		require.Equal(t, "bar", *ns)
	})

	t.Run("parent is nil", func(t *testing.T) {
		a := &Consul{Namespace: "foo"}
		ns := (*string)(nil)
		a.MergeNamespace(ns)
		require.Equal(t, "foo", a.Namespace)
		require.Nil(t, ns)
	})
}
