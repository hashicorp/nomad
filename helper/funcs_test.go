package helper

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSliceStringIsSubset(t *testing.T) {
	l := []string{"a", "b", "c"}
	s := []string{"d"}

	sub, offending := SliceStringIsSubset(l, l[:1])
	if !sub || len(offending) != 0 {
		t.Fatalf("bad %v %v", sub, offending)
	}

	sub, offending = SliceStringIsSubset(l, s)
	if sub || len(offending) == 0 || offending[0] != "d" {
		t.Fatalf("bad %v %v", sub, offending)
	}
}

func TestSliceStringContains(t *testing.T) {
	list := []string{"a", "b", "c"}
	require.True(t, SliceStringContains(list, "a"))
	require.True(t, SliceStringContains(list, "b"))
	require.True(t, SliceStringContains(list, "c"))
	require.False(t, SliceStringContains(list, "d"))
}

func TestCompareTimePtrs(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		a := (*time.Duration)(nil)
		b := (*time.Duration)(nil)
		require.True(t, CompareTimePtrs(a, b))
		c := TimeToPtr(3 * time.Second)
		require.False(t, CompareTimePtrs(a, c))
		require.False(t, CompareTimePtrs(c, a))
	})

	t.Run("not nil", func(t *testing.T) {
		a := TimeToPtr(1 * time.Second)
		b := TimeToPtr(1 * time.Second)
		c := TimeToPtr(2 * time.Second)
		require.True(t, CompareTimePtrs(a, b))
		require.False(t, CompareTimePtrs(a, c))
	})
}

func TestCompareSliceSetString(t *testing.T) {
	cases := []struct {
		A      []string
		B      []string
		Result bool
	}{
		{
			A:      []string{},
			B:      []string{},
			Result: true,
		},
		{
			A:      []string{},
			B:      []string{"a"},
			Result: false,
		},
		{
			A:      []string{"a"},
			B:      []string{"a"},
			Result: true,
		},
		{
			A:      []string{"a"},
			B:      []string{"b"},
			Result: false,
		},
		{
			A:      []string{"a", "b"},
			B:      []string{"b"},
			Result: false,
		},
		{
			A:      []string{"a", "b"},
			B:      []string{"a"},
			Result: false,
		},
		{
			A:      []string{"a", "b"},
			B:      []string{"a", "b"},
			Result: true,
		},
		{
			A:      []string{"a", "b"},
			B:      []string{"b", "a"},
			Result: true,
		},
	}

	for i, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("case-%da", i), func(t *testing.T) {
			if res := CompareSliceSetString(tc.A, tc.B); res != tc.Result {
				t.Fatalf("expected %t but CompareSliceSetString(%v, %v) -> %t",
					tc.Result, tc.A, tc.B, res,
				)
			}
		})

		// Function is commutative so compare B and A
		t.Run(fmt.Sprintf("case-%db", i), func(t *testing.T) {
			if res := CompareSliceSetString(tc.B, tc.A); res != tc.Result {
				t.Fatalf("expected %t but CompareSliceSetString(%v, %v) -> %t",
					tc.Result, tc.B, tc.A, res,
				)
			}
		})
	}
}

func TestMapStringStringSliceValueSet(t *testing.T) {
	m := map[string][]string{
		"foo": {"1", "2"},
		"bar": {"3"},
		"baz": nil,
	}

	act := MapStringStringSliceValueSet(m)
	exp := []string{"1", "2", "3"}
	sort.Strings(act)
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Bad; got %v; want %v", act, exp)
	}
}

func TestCopyMapStringSliceString(t *testing.T) {
	m := map[string][]string{
		"x": {"a", "b", "c"},
		"y": {"1", "2", "3"},
		"z": nil,
	}

	c := CopyMapStringSliceString(m)
	if !reflect.DeepEqual(c, m) {
		t.Fatalf("%#v != %#v", m, c)
	}

	c["x"][1] = "---"
	if reflect.DeepEqual(c, m) {
		t.Fatalf("Shared slices: %#v == %#v", m["x"], c["x"])
	}
}

func TestCopyMapSliceInterface(t *testing.T) {
	m := map[string]interface{}{
		"foo": "bar",
		"baz": 2,
	}

	c := CopyMapStringInterface(m)
	require.True(t, reflect.DeepEqual(m, c))

	m["foo"] = "zzz"
	require.False(t, reflect.DeepEqual(m, c))
}

func TestClearEnvVar(t *testing.T) {
	type testCase struct {
		input    string
		expected string
	}
	cases := []testCase{
		{"asdf", "asdf"},
		{"ASDF", "ASDF"},
		{"0sdf", "_sdf"},
		{"asd0", "asd0"},
		{"_asd", "_asd"},
		{"-asd", "_asd"},
		{"asd.fgh", "asd.fgh"},
		{"A~!@#$%^&*()_+-={}[]|\\;:'\"<,>?/Z", "A______________________________Z"},
		{"A\U0001f4a9Z", "A____Z"},
	}
	for _, c := range cases {
		if output := CleanEnvVar(c.input, '_'); output != c.expected {
			t.Errorf("CleanEnvVar(%q, '_') -> %q != %q", c.input, output, c.expected)
		}
	}
}

func BenchmarkCleanEnvVar(b *testing.B) {
	in := "NOMAD_ADDR_redis-cache"
	replacement := byte('_')
	b.SetBytes(int64(len(in)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CleanEnvVar(in, replacement)
	}
}

func TestCheckNamespaceScope(t *testing.T) {
	cases := []struct {
		desc      string
		provided  string
		requested []string
		offending []string
	}{
		{
			desc:      "root ns requesting namespace",
			provided:  "",
			requested: []string{"engineering"},
		},
		{
			desc:      "matching parent ns with child",
			provided:  "engineering",
			requested: []string{"engineering", "engineering/sub-team"},
		},
		{
			desc:      "mismatch ns",
			provided:  "engineering",
			requested: []string{"finance", "engineering/sub-team", "eng"},
			offending: []string{"finance", "eng"},
		},
		{
			desc:      "mismatch child",
			provided:  "engineering/sub-team",
			requested: []string{"engineering/new-team", "engineering/sub-team", "engineering/sub-team/child"},
			offending: []string{"engineering/new-team"},
		},
		{
			desc:      "matching prefix",
			provided:  "engineering",
			requested: []string{"engineering/new-team", "engineering/new-team/sub-team"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			offending := CheckNamespaceScope(tc.provided, tc.requested)
			require.Equal(t, offending, tc.offending)
		})
	}
}

func TestGetPathInSandbox(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		dir         string
		expected    string
		expectedErr string
	}{
		{
			name:     "ok absolute path inside sandbox",
			path:     "/alloc/safe",
			dir:      "/alloc",
			expected: "/alloc/safe",
		},
		{
			name:     "ok relative path inside sandbox",
			path:     "./safe",
			dir:      "/alloc",
			expected: "/alloc/safe",
		},
		{
			name:     "ok relative path traversal constrained to sandbox",
			path:     "../../alloc/safe",
			dir:      "/alloc",
			expected: "/alloc/safe",
		},
		{
			name:     "ok absolute path traversal constrained to sandbox",
			path:     "/../alloc/safe",
			dir:      "/alloc",
			expected: "/alloc/safe",
		},
		{
			name:        "fail absolute path outside sandbox",
			path:        "/unsafe",
			dir:         "/alloc",
			expected:    "/unsafe",
			expectedErr: "\"/unsafe\" escapes sandbox directory",
		},
		{
			name:        "fail relative path traverses outside sandbox",
			path:        "../../../unsafe",
			dir:         "/alloc",
			expected:    "/unsafe",
			expectedErr: "\"/unsafe\" escapes sandbox directory",
		},
		{
			name:        "fail absolute path tries to transverse outside sandbox",
			path:        "/alloc/../unsafe",
			dir:         "/alloc",
			expected:    "/unsafe",
			expectedErr: "\"/unsafe\" escapes sandbox directory",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseMsg := fmt.Sprintf("path: %v\ndir: %v", tc.path, tc.dir)
			escapes, err := GetPathInSandbox(tc.dir, tc.path)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr, caseMsg)
			} else {
				require.NoError(t, err, caseMsg)
			}
			require.Equal(t, tc.expected, escapes, caseMsg)
		})
	}
}
