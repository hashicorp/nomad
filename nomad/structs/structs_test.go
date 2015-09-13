package structs

import (
	"reflect"
	"testing"
)

func TestResource_NetIndexByCIDR(t *testing.T) {
	r := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{CIDR: "10.0.0.0/8"},
			&NetworkResource{CIDR: "127.0.0.0/24"},
		},
	}
	if idx := r.NetIndexByCIDR("10.0.0.0/8"); idx != 0 {
		t.Fatalf("Bad: %d", idx)
	}
	if idx := r.NetIndexByCIDR("127.0.0.0/24"); idx != 1 {
		t.Fatalf("Bad: %d", idx)
	}
	if idx := r.NetIndexByCIDR("10.0.0.0/16"); idx != -1 {
		t.Fatalf("Bad: %d", idx)
	}
}

func TestResource_NetIndexByIP(t *testing.T) {
	r := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{CIDR: "10.0.0.0/8"},
			&NetworkResource{CIDR: "127.0.0.0/24"},
		},
	}
	if idx := r.NetIndexByIP("10.1.2.3"); idx != 0 {
		t.Fatalf("Bad: %d", idx)
	}
	if idx := r.NetIndexByIP("127.0.0.1"); idx != 1 {
		t.Fatalf("Bad: %d", idx)
	}
	if idx := r.NetIndexByIP("11.2.3.4"); idx != -1 {
		t.Fatalf("Bad: %d", idx)
	}
}

func TestResource_Superset(t *testing.T) {
	r1 := &Resources{
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
	}
	r2 := &Resources{
		CPU:      1.0,
		MemoryMB: 1024,
		DiskMB:   5000,
		IOPS:     50,
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:  "10.0.0.0/8",
				MBits: 50,
			},
		},
	}

	if !r1.Superset(r1) {
		t.Fatalf("bad")
	}
	if !r1.Superset(r2) {
		t.Fatalf("bad")
	}
	if r2.Superset(r1) {
		t.Fatalf("bad")
	}
	if !r2.Superset(r2) {
		t.Fatalf("bad")
	}
}

func TestResource_Superset_IPCIDR(t *testing.T) {
	r1 := &Resources{
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
	}
	r2 := &Resources{
		CPU:      1.0,
		MemoryMB: 1024,
		DiskMB:   5000,
		IOPS:     50,
		Networks: []*NetworkResource{
			&NetworkResource{
				IP:    "10.0.0.5",
				MBits: 50,
			},
			&NetworkResource{
				IP:    "10.0.0.6",
				MBits: 50,
			},
		},
	}

	if !r1.Superset(r2) {
		t.Fatalf("bad")
	}

	// Use more network
	r2.Networks = append(r2.Networks, &NetworkResource{
		IP:    "10.0.0.7",
		MBits: 50,
	})

	if r1.Superset(r2) {
		t.Fatalf("bad")
	}
}

func TestResource_Add(t *testing.T) {
	r1 := &Resources{
		CPU:      2.0,
		MemoryMB: 2048,
		DiskMB:   10000,
		IOPS:     100,
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:          "10.0.0.0/8",
				MBits:         100,
				ReservedPorts: []int{22},
			},
		},
	}
	r2 := &Resources{
		CPU:      1.0,
		MemoryMB: 1024,
		DiskMB:   5000,
		IOPS:     50,
		Networks: []*NetworkResource{
			&NetworkResource{
				IP:            "10.0.0.1",
				MBits:         50,
				ReservedPorts: []int{80},
			},
		},
	}

	err := r1.Add(r2)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	expect := &Resources{
		CPU:      3.0,
		MemoryMB: 3072,
		DiskMB:   15000,
		IOPS:     150,
		Networks: []*NetworkResource{
			&NetworkResource{
				CIDR:          "10.0.0.0/8",
				MBits:         150,
				ReservedPorts: []int{22, 80},
			},
		},
	}

	if !reflect.DeepEqual(expect.Networks, r1.Networks) {
		t.Fatalf("bad: %#v %#v", expect, r1)
	}
}

func TestResource_Add_Network(t *testing.T) {
	r1 := &Resources{}
	r2 := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        50,
				DynamicPorts: 2,
			},
		},
	}
	r3 := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        25,
				DynamicPorts: 1,
			},
		},
	}

	err := r1.Add(r2)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	err = r1.Add(r3)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}

	expect := &Resources{
		Networks: []*NetworkResource{
			&NetworkResource{
				MBits:        75,
				DynamicPorts: 3,
			},
		},
	}

	if !reflect.DeepEqual(expect.Networks, r1.Networks) {
		t.Fatalf("bad: %#v %#v", expect, r1)
	}
}

func TestEncodeDecode(t *testing.T) {
	type FooRequest struct {
		Foo string
		Bar int
		Baz bool
	}
	arg := &FooRequest{
		Foo: "test",
		Bar: 42,
		Baz: true,
	}
	buf, err := Encode(1, arg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out FooRequest
	err = Decode(buf[1:], &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(arg, &out) {
		t.Fatalf("bad: %#v %#v", arg, out)
	}
}
