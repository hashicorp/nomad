package helper

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
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
