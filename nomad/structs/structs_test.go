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
