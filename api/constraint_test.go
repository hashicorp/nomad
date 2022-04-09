package api

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
)

func TestCompose_Constraints(t *testing.T) {
	testutil.Parallel(t)
	c := NewConstraint("kernel.name", "=", "darwin")
	expect := &Constraint{
		LTarget: "kernel.name",
		RTarget: "darwin",
		Operand: "=",
	}
	if !reflect.DeepEqual(c, expect) {
		t.Fatalf("expect: %#v, got: %#v", expect, c)
	}
}
