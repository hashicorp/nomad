package cpuset

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCPUSet_Size(t *testing.T) {
	set := New(0, 1, 2, 3)
	require.Equal(t, 4, set.Size())
	require.Equal(t, 0, New().Size())
}

func TestCPUSet_ToSlice(t *testing.T) {
	cases := []struct {
		desc string
		in   CPUSet
		out  []uint16
	}{
		{
			"empty cpuset",
			New(),
			[]uint16{},
		},
		{
			"in order",
			New(0, 1, 2, 3, 4, 5, 6, 7),
			[]uint16{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{
			"out of order",
			New(3, 1, 2, 0),
			[]uint16{0, 1, 2, 3},
		},
	}

	for _, c := range cases {
		require.Exactly(t, c.out, c.in.ToSlice(), c.desc)
	}
}

func TestCPUSet_Equals(t *testing.T) {
	cases := []struct {
		a           CPUSet
		b           CPUSet
		shouldEqual bool
	}{
		{New(), New(), true},
		{New(5), New(5), true},
		{New(1, 2, 3, 4, 5), New(1, 2, 3, 4, 5), true},

		{New(), New(5), false},
		{New(5), New(), false},
		{New(), New(1, 2, 3, 4, 5), false},
		{New(1, 2, 3, 4, 5), New(), false},
		{New(5), New(1, 2, 3, 4, 5), false},
		{New(1, 2, 3, 4, 5), New(5), false},
	}

	for _, c := range cases {
		require.Equal(t, c.shouldEqual, c.a.Equals(c.b))
	}
}

func TestCPUSet_Union(t *testing.T) {
	cases := []struct {
		a        CPUSet
		b        CPUSet
		expected CPUSet
	}{
		{New(), New(), New()},

		{New(), New(0), New(0)},
		{New(0), New(), New(0)},
		{New(0), New(0), New(0)},

		{New(), New(0, 1, 2, 3), New(0, 1, 2, 3)},
		{New(0, 1), New(0, 1, 2, 3), New(0, 1, 2, 3)},
		{New(2, 3), New(4, 5), New(2, 3, 4, 5)},
		{New(3, 4), New(0, 1, 2, 3), New(0, 1, 2, 3, 4)},
	}

	for _, c := range cases {
		require.Exactly(t, c.expected.ToSlice(), c.a.Union(c.b).ToSlice())
	}
}

func TestCPUSet_Difference(t *testing.T) {
	cases := []struct {
		a        CPUSet
		b        CPUSet
		expected CPUSet
	}{
		{New(), New(), New()},

		{New(), New(0), New()},
		{New(0), New(), New(0)},
		{New(0), New(0), New()},

		{New(0, 1), New(0, 1, 2, 3), New()},
		{New(2, 3), New(4, 5), New(2, 3)},
		{New(3, 4), New(0, 1, 2, 3), New(4)},
	}

	for _, c := range cases {
		require.Exactly(t, c.expected.ToSlice(), c.a.Difference(c.b).ToSlice())
	}
}

func TestCPUSet_IsSubsetOf(t *testing.T) {
	cases := []struct {
		a        CPUSet
		b        CPUSet
		isSubset bool
	}{
		{New(0), New(0), true},
		{New(), New(0), true},
		{New(0), New(), false},
		{New(1, 2), New(0, 1, 2, 3), true},
		{New(2, 1), New(0, 1, 2, 3), true},
		{New(3, 4), New(0, 1, 2, 3), false},
	}

	for _, c := range cases {
		require.Equal(t, c.isSubset, c.a.IsSubsetOf(c.b))
	}
}

func TestCPUSet_IsSupersetOf(t *testing.T) {
	cases := []struct {
		a          CPUSet
		b          CPUSet
		isSuperset bool
	}{
		{New(0), New(0), true},
		{New(0), New(), true},
		{New(), New(0), false},
		{New(0, 1, 2, 3), New(0), true},
		{New(0, 1, 2, 3), New(2, 3), true},
		{New(0, 1, 2, 3), New(2, 3, 4), false},
	}

	for _, c := range cases {
		require.Equal(t, c.isSuperset, c.a.IsSupersetOf(c.b))
	}
}

func TestCPUSet_ContainsAny(t *testing.T) {
	cases := []struct {
		a           CPUSet
		b           CPUSet
		containsAny bool
	}{
		{New(0), New(0), true},
		{New(0), New(), false},
		{New(), New(0), false},
		{New(0, 1, 2, 3), New(0), true},
		{New(0, 1, 2, 3), New(2, 3), true},
		{New(0, 1, 2, 3), New(2, 3, 4), true},
	}

	for _, c := range cases {
		require.Equal(t, c.containsAny, c.a.ContainsAny(c.b))
	}
}

func TestParse(t *testing.T) {
	cases := []struct {
		cpuset   string
		expected CPUSet
	}{
		{"", New()},
		{"\n", New()},
		{"1", New(1)},
		{"1\n", New(1)},
		{"0,1,2,3", New(0, 1, 2, 3)},
		{"0-3", New(0, 1, 2, 3)},
		{"0,2-3,5", New(0, 2, 3, 5)},
	}

	for _, c := range cases {
		result, err := Parse(c.cpuset)
		require.NoError(t, err)
		require.True(t, result.Equals(c.expected))
	}
}

func TestCPUSet_String(t *testing.T) {
	cases := []struct {
		cpuset   CPUSet
		expected string
	}{
		{New(), ""},
		{New(0, 1, 2, 3), "0-3"},
		{New(1, 3), "1,3"},
		{New(0, 2, 3, 5), "0,2-3,5"},
	}

	for _, c := range cases {
		require.Equal(t, c.expected, c.cpuset.String())
	}
}
