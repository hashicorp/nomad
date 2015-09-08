package api

import (
	"testing"
)

func TestAllocs_List(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Allocs()

	// Querying when no allocs exist returns nothing
	allocs, err := a.List()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}
}
