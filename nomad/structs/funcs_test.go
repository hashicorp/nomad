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

func TestPortsOvercommitted(t *testing.T) {
	r := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				ReservedPorts: []int{22, 80},
			},
			&NetworkResource{
				ReservedPorts: []int{22, 80},
			},
		},
	}
	if PortsOvercommited(r) {
		t.Fatalf("bad")
	}

	// Overcommit 22
	r.Networks[1].ReservedPorts[1] = 22
	if !PortsOvercommited(r) {
		t.Fatalf("bad")
	}
}
