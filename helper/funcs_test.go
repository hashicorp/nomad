package helper

import (
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

func TestMapStringStringSliceValueSet(t *testing.T) {
	m := map[string][]string{
		"foo": []string{"1", "2"},
		"bar": []string{"3"},
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
		"x": []string{"a", "b", "c"},
		"y": []string{"1", "2", "3"},
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
		{"A~!@#$%^&*()_+-={}[]|\\;:'\"<,>.?/Z", "A_______________________________Z"},
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
