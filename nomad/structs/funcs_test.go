package structs

import "testing"

func TestRemoveAllocs(t *testing.T) {
	l := []*Allocation{
		&Allocation{ID: "foo"},
		&Allocation{ID: "bar"},
		&Allocation{ID: "baz"},
		&Allocation{ID: "zip"},
	}

	out := RemoveAllocs(l, []string{"bar", "zip"})
	if len(out) != 2 {
		t.Fatalf("bad: %#v", out)
	}
	if out[0].ID != "foo" && out[1].ID != "baz" {
		t.Fatalf("bad: %#v", out)
	}
}
