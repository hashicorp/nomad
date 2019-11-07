package cty

import (
	"reflect"
	"testing"
)

func TestPathSet(t *testing.T) {
	helloWorld := Path{
		GetAttrStep{Name: "hello"},
		GetAttrStep{Name: "world"},
	}
	s := NewPathSet(helloWorld)

	if got, want := s.Has(helloWorld), true; got != want {
		t.Errorf("set does not have hello.world; should have it")
	}
	if got, want := s.Has(helloWorld[:1]), false; got != want {
		t.Errorf("set has hello; should not have it")
	}

	if got, want := s.List(), []Path{helloWorld}; !reflect.DeepEqual(got, want) {
		t.Errorf("wrong list result\ngot:  %#v\nwant: %#v", got, want)
	}

	fooBarBaz := Path{
		GetAttrStep{Name: "foo"},
		IndexStep{Key: StringVal("bar")},
		GetAttrStep{Name: "baz"},
	}
	s.AddAllSteps(fooBarBaz)
	if got, want := s.Has(helloWorld), true; got != want {
		t.Errorf("set does not have hello.world; should have it")
	}
	if got, want := s.Has(fooBarBaz), true; got != want {
		t.Errorf("set does not have foo['bar'].baz; should have it")
	}
	if got, want := s.Has(fooBarBaz[:2]), true; got != want {
		t.Errorf("set does not have foo['bar']; should have it")
	}
	if got, want := s.Has(fooBarBaz[:1]), true; got != want {
		t.Errorf("set does not have foo; should have it")
	}

	s.Remove(fooBarBaz[:2])
	if got, want := s.Has(fooBarBaz[:2]), false; got != want {
		t.Errorf("set has foo['bar']; should not have it")
	}
	if got, want := s.Has(fooBarBaz), true; got != want {
		t.Errorf("set does not have foo['bar'].baz; should have it")
	}
	if got, want := s.Has(fooBarBaz[:1]), true; got != want {
		t.Errorf("set does not have foo; should have it")
	}

	new := NewPathSet(s.List()...)
	if got, want := s.Equal(new), true; got != want {
		t.Errorf("new set does not equal original; want equal sets")
	}
	new.Remove(helloWorld)
	if got, want := s.Equal(new), false; got != want {
		t.Errorf("new set equals original; want non-equal sets")
	}
	new.Add(Path{
		GetAttrStep{Name: "goodbye"},
		GetAttrStep{Name: "world"},
	})
	if got, want := s.Equal(new), false; got != want {
		t.Errorf("new set equals original; want non-equal sets")
	}
}
