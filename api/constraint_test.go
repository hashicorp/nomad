package api

import (
	"reflect"
	"testing"
)

func TestCompose_Constraints(t *testing.T) {
	c := HardConstraint("kernel.name", "=", "darwin")
	expect := &Constraint{
		Hard:    true,
		LTarget: "kernel.name",
		RTarget: "darwin",
		Operand: "=",
		Weight:  0,
	}
	if !reflect.DeepEqual(c, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, c)
	}

	c = SoftConstraint("memory.totalbytes", ">=", "250000000", 5)
	expect = &Constraint{
		Hard:    false,
		LTarget: "memory.totalbytes",
		RTarget: "250000000",
		Operand: ">=",
		Weight:  5,
	}
	if !reflect.DeepEqual(c, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, c)
	}
}
