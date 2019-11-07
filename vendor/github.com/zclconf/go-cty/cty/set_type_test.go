package cty

import (
	"testing"
)

func TestSetOperations(t *testing.T) {
	// This test is for the mechanisms that allow a calling application to
	// implement set operations using the underlying set.Set type. This is
	// not expected to be a common case but is useful, for example, for
	// implementing the set-related functions in function/stdlib .

	s1 := SetVal([]Value{
		StringVal("a"),
		StringVal("b"),
		StringVal("c"),
	})
	s2 := SetVal([]Value{
		StringVal("c"),
		StringVal("d"),
		StringVal("e"),
	})

	s1r := s1.AsValueSet()
	s2r := s2.AsValueSet()
	s3r := s1r.Union(s2r)

	s3 := SetValFromValueSet(s3r)

	if got, want := s3.LengthInt(), 5; got != want {
		t.Errorf("wrong length %d; want %d", got, want)
	}

	for _, wantStr := range []string{"a", "b", "c", "d", "e"} {
		if got, want := s3.HasElement(StringVal(wantStr)), True; got != want {
			t.Errorf("missing element %q", wantStr)
		}
	}

}
