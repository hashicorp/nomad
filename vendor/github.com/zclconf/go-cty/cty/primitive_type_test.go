package cty

import (
	"fmt"
	"testing"
)

func TestTypeIsPrimitiveType(t *testing.T) {
	tests := []struct {
		Type Type
		Want bool
	}{
		{String, true},
		{Number, true},
		{Bool, true},
		{DynamicPseudoType, false},
		{List(String), false},

		// Make sure our primitive constants are correctly constructed
		{True.Type(), true},
		{False.Type(), true},
		{Zero.Type(), true},
		{PositiveInfinity.Type(), true},
		{NegativeInfinity.Type(), true},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d %#v", i, test.Type), func(t *testing.T) {
			got := test.Type.IsPrimitiveType()
			if got != test.Want {
				t.Errorf(
					"wrong result\ntype: %#v\ngot:  %#v\nwant: %#v",
					test.Type,
					test.Want, got,
				)
			}
		})
	}
}
