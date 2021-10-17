package helper

import (
	"fmt"
	"path/filepath"
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

func TestSliceStringHasPrefix(t *testing.T) {
	list := []string{"alpha", "bravo", "charlie", "definitely", "most definitely"}
	// At least one string in the slice above starts with the following test prefix strings
	require.True(t, SliceStringHasPrefix(list, "a"))
	require.True(t, SliceStringHasPrefix(list, "b"))
	require.True(t, SliceStringHasPrefix(list, "c"))
	require.True(t, SliceStringHasPrefix(list, "d"))
	require.True(t, SliceStringHasPrefix(list, "mos"))
	require.True(t, SliceStringHasPrefix(list, "def"))
	require.False(t, SliceStringHasPrefix(list, "delta"))

}

func TestStringHasPrefixInSlice(t *testing.T) {
	prefixes := []string{"a", "b", "c", "definitely", "most definitely"}
	// The following strings all start with at least one prefix in the slice above
	require.True(t, StringHasPrefixInSlice("alpha", prefixes))
	require.True(t, StringHasPrefixInSlice("bravo", prefixes))
	require.True(t, StringHasPrefixInSlice("charlie", prefixes))
	require.True(t, StringHasPrefixInSlice("definitely", prefixes))
	require.True(t, StringHasPrefixInSlice("most definitely", prefixes))

	require.False(t, StringHasPrefixInSlice("mos", prefixes))
	require.False(t, StringHasPrefixInSlice("def", prefixes))
	require.False(t, StringHasPrefixInSlice("delta", prefixes))

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

func TestCleanEnvVar(t *testing.T) {
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

type testCase struct {
	input    string
	expected string
}

func commonCleanFilenameCases() (cases []testCase) {
	// Common set of test cases for all 3 TestCleanFilenameX functions
	cases = []testCase{
		{"asdf", "asdf"},
		{"ASDF", "ASDF"},
		{"0sdf", "0sdf"},
		{"asd0", "asd0"},
		{"_asd", "_asd"},
		{"-asd", "-asd"},
		{"asd.fgh", "asd.fgh"},
		{"Linux/Forbidden", "Linux_Forbidden"},
		{"Windows<>:\"/\\|?*Forbidden", "Windows_________Forbidden"},
		{`Windows<>:"/\|?*Forbidden_StringLiteral`, "Windows_________Forbidden_StringLiteral"},
	}
	return cases
}

func TestCleanFilename(t *testing.T) {
	cases := append(
		[]testCase{
			{"A\U0001f4a9Z", "A💩Z"}, // CleanFilename allows unicode
			{"A💩Z", "A💩Z"},
			{"A~!@#$%^&*()_+-={}[]|\\;:'\"<,>?/Z", "A~!@#$%^&_()_+-={}[]__;_'__,___Z"},
		}, commonCleanFilenameCases()...)

	for i, c := range cases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			output := CleanFilename(c.input, "_")
			failMsg := fmt.Sprintf("CleanFilename(%q, '_') -> %q != %q", c.input, output, c.expected)
			require.Equal(t, c.expected, output, failMsg)
		})
	}
}

func TestCleanFilenameASCIIOnly(t *testing.T) {
	ASCIIOnlyCases := append(
		[]testCase{
			{"A\U0001f4a9Z", "A_Z"}, // CleanFilenameASCIIOnly does not allow unicode
			{"A💩Z", "A_Z"},
			{"A~!@#$%^&*()_+-={}[]|\\;:'\"<,>?/Z", "A~!@#$%^&_()_+-={}[]__;_'__,___Z"},
		}, commonCleanFilenameCases()...)

	for i, c := range ASCIIOnlyCases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			output := CleanFilenameASCIIOnly(c.input, "_")
			failMsg := fmt.Sprintf("CleanFilenameASCIIOnly(%q, '_') -> %q != %q", c.input, output, c.expected)
			require.Equal(t, c.expected, output, failMsg)
		})
	}
}

func TestCleanFilenameStrict(t *testing.T) {
	strictCases := append(
		[]testCase{
			{"A\U0001f4a9Z", "A💩Z"}, // CleanFilenameStrict allows unicode
			{"A💩Z", "A💩Z"},
			{"A~!@#$%^&*()_+-={}[]|\\;:'\"<,>?/Z", "A_!___%^______-_{}_____________Z"},
		}, commonCleanFilenameCases()...)

	for i, c := range strictCases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			output := CleanFilenameStrict(c.input, "_")
			failMsg := fmt.Sprintf("CleanFilenameStrict(%q, '_') -> %q != %q", c.input, output, c.expected)
			require.Equal(t, c.expected, output, failMsg)
		})
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

func TestPathEscapesSandbox(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		dir      string
		expected bool
	}{
		{
			// this is the ${NOMAD_SECRETS_DIR} case
			name:     "ok joined absolute path inside sandbox",
			path:     filepath.Join("/alloc", "/secrets"),
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "fail unjoined absolute path outside sandbox",
			path:     "/secrets",
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "ok joined relative path inside sandbox",
			path:     filepath.Join("/alloc", "./safe"),
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "fail unjoined relative path outside sandbox",
			path:     "./safe",
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "ok relative path traversal constrained to sandbox",
			path:     filepath.Join("/alloc", "../../alloc/safe"),
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "ok unjoined absolute path traversal constrained to sandbox",
			path:     filepath.Join("/alloc", "/../alloc/safe"),
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "ok unjoined absolute path traversal constrained to sandbox",
			path:     "/../alloc/safe",
			dir:      "/alloc",
			expected: false,
		},
		{
			name:     "fail joined relative path traverses outside sandbox",
			path:     filepath.Join("/alloc", "../../../unsafe"),
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "fail unjoined relative path traverses outside sandbox",
			path:     "../../../unsafe",
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "fail joined absolute path tries to transverse outside sandbox",
			path:     filepath.Join("/alloc", "/alloc/../../unsafe"),
			dir:      "/alloc",
			expected: true,
		},
		{
			name:     "fail unjoined absolute path tries to transverse outside sandbox",
			path:     "/alloc/../../unsafe",
			dir:      "/alloc",
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseMsg := fmt.Sprintf("path: %v\ndir: %v", tc.path, tc.dir)
			escapes := PathEscapesSandbox(tc.dir, tc.path)
			require.Equal(t, tc.expected, escapes, caseMsg)
		})
	}
}
