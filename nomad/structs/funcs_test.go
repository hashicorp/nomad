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

func TestAllocsFit(t *testing.T) {
	n := &Node{
		Resources: &Resources{
			CPU:      2.0,
			MemoryMB: 2048,
			DiskMB:   10000,
			IOPS:     100,
			Networks: []*NetworkResource{
				&NetworkResource{
					CIDR:  "10.0.0.0/8",
					MBits: 100,
				},
			},
		},
		Reserved: &Resources{
			CPU:      1.0,
			MemoryMB: 1024,
			DiskMB:   5000,
			IOPS:     50,
			Networks: []*NetworkResource{
				&NetworkResource{
					CIDR:          "10.0.0.0/8",
					MBits:         50,
					ReservedPorts: []int{80},
				},
			},
		},
	}

	a1 := &Allocation{
		Resources: &Resources{
			CPU:      1.0,
			MemoryMB: 1024,
			DiskMB:   5000,
			IOPS:     50,
			Networks: []*NetworkResource{
				&NetworkResource{
					CIDR:          "10.0.0.0/8",
					MBits:         50,
					ReservedPorts: []int{8000},
				},
			},
		},
	}

	// Should fit one allocation
	fit, err := AllocsFit(n, []*Allocation{a1})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("Bad")
	}

	// Should not fit second allocation
	fit, err = AllocsFit(n, []*Allocation{a1, a1})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("Bad")
	}
}
