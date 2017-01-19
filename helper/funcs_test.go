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
