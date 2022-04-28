package capabilities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSet_Empty(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		result := New(nil).Empty()
		require.True(t, result)
	})

	t.Run("empty", func(t *testing.T) {
		result := New([]string{}).Empty()
		require.True(t, result)
	})

	t.Run("full", func(t *testing.T) {
		result := New([]string{"chown", "sys_time"}).Empty()
		require.False(t, result)
	})
}

func TestSet_New(t *testing.T) {
	t.Parallel()

	t.Run("duplicates", func(t *testing.T) {
		result := New([]string{"chown", "sys_time", "chown"})
		require.Equal(t, "chown, sys_time", result.String())
	})

	t.Run("empty string", func(t *testing.T) {
		result := New([]string{""})
		require.True(t, result.Empty())
	})

	t.Run("all", func(t *testing.T) {
		result := New([]string{"all"})
		exp := len(Supported().Slice(false))
		require.Len(t, result.Slice(false), exp)
	})
}

func TestSet_Slice(t *testing.T) {
	t.Parallel()

	exp := []string{"chown", "net_raw", "sys_time"}

	t.Run("lower case", func(t *testing.T) {
		s := New([]string{"net_raw", "chown", "sys_time"})
		require.Equal(t, exp, s.Slice(false))
	})

	t.Run("upper case", func(t *testing.T) {
		s := New([]string{"NET_RAW", "CHOWN", "SYS_TIME"})
		require.Equal(t, exp, s.Slice(false))
	})

	t.Run("prefix", func(t *testing.T) {
		s := New([]string{"CAP_net_raw", "sys_TIME", "cap_chown"})
		require.Equal(t, exp, s.Slice(false))
	})
}

func TestSet_String(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		result := New(nil).String()
		require.Equal(t, "", result)
	})

	t.Run("full", func(t *testing.T) {
		exp := "chown, net_raw, sys_time"
		in := []string{"net_raw", "CAP_CHOWN", "cap_sys_time"}
		result := New(in).String()
		require.Equal(t, exp, result)
	})
}

func TestSet_Add(t *testing.T) {
	t.Parallel()

	t.Run("add one", func(t *testing.T) {
		s := New([]string{"chown", "net_raw"})
		require.Equal(t, "chown, net_raw", s.String())

		s.Add("CAP_SYS_TIME")
		require.Equal(t, "chown, net_raw, sys_time", s.String())

		s.Add("AF_NET")
		require.Equal(t, "af_net, chown, net_raw, sys_time", s.String())
	})

	t.Run("add empty string", func(t *testing.T) {
		s := New([]string{"chown"})
		s.Add("")
		require.Equal(t, "chown", s.String())
	})

	t.Run("add all", func(t *testing.T) {
		s := New([]string{"chown", "net_raw"})
		require.Equal(t, "chown, net_raw", s.String())

		exp := len(Supported().Slice(false))
		s.Add("all")
		require.Len(t, s.Slice(false), exp)
	})

}

func TestSet_Remove(t *testing.T) {
	t.Parallel()

	t.Run("remove one", func(t *testing.T) {
		s := New([]string{"af_net", "chown", "net_raw", "seteuid", "sys_time"})
		s.Remove([]string{"CAP_NET_RAW"})
		require.Equal(t, "af_net, chown, seteuid, sys_time", s.String())
	})

	t.Run("remove couple", func(t *testing.T) {
		s := New([]string{"af_net", "chown", "net_raw", "seteuid", "sys_time"})
		s.Remove([]string{"CAP_NET_RAW", "af_net"})
		require.Equal(t, "chown, seteuid, sys_time", s.String())
	})

	t.Run("remove all", func(t *testing.T) {
		s := New([]string{"af_net", "chown", "net_raw", "seteuid", "sys_time"})
		s.Remove([]string{"all"})
		require.True(t, s.Empty())
		require.Equal(t, "", s.String())
	})
}

func TestSet_Difference(t *testing.T) {
	t.Parallel()

	t.Run("a is empty", func(t *testing.T) {
		a := New(nil)
		b := New([]string{"chown", "af_net"})
		result := a.Difference(b)
		require.Equal(t, "af_net, chown", result.String())
	})

	t.Run("b is empty", func(t *testing.T) {
		a := New([]string{"chown", "af_net"})
		b := New(nil)
		result := a.Difference(b)
		require.True(t, result.Empty())
	})

	t.Run("a diff b", func(t *testing.T) {
		a := New([]string{"A", "b", "C", "d", "e", "f"})
		b := New([]string{"B", "x", "Y", "a"})
		result := a.Difference(b)
		require.Equal(t, "x, y", result.String())
	})
}

func TestSet_Intersect(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		a := New(nil)
		b := New([]string{"a", "b"})

		result := a.Intersect(b)
		require.True(t, result.Empty())

		result2 := b.Intersect(a)
		require.True(t, result2.Empty())
	})

	t.Run("intersect", func(t *testing.T) {
		a := New([]string{"A", "b", "C", "d", "e", "f", "G"})
		b := New([]string{"Z", "B", "E", "f", "y"})

		result := a.Intersect(b)
		require.Equal(t, "b, e, f", result.String())

		result2 := b.Intersect(a)
		require.Equal(t, "b, e, f", result2.String())
	})
}

func TestSet_Union(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		a := New(nil)
		b := New([]string{"a", "b"})

		result := a.Union(b)
		require.Equal(t, "a, b", result.String())

		result2 := b.Union(a)
		require.Equal(t, "a, b", result2.String())
	})

	t.Run("union", func(t *testing.T) {
		a := New([]string{"A", "b", "C", "d", "e", "f", "G"})
		b := New([]string{"Z", "B", "E", "f", "y"})

		result := a.Union(b)
		require.Equal(t, "a, b, c, d, e, f, g, y, z", result.String())

		result2 := b.Union(a)
		require.Equal(t, "a, b, c, d, e, f, g, y, z", result2.String())
	})
}
