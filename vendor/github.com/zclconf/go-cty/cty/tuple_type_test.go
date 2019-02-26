package cty

import (
	"fmt"
	"testing"
)

func TestTupleTypeEquals(t *testing.T) {
	tests := []struct {
		LHS      Type // Must be typeTuple
		RHS      Type
		Expected bool
	}{
		{
			Tuple([]Type{}),
			Tuple([]Type{}),
			true,
		},
		{
			EmptyTuple,
			Tuple([]Type{}),
			true,
		},
		{
			Tuple([]Type{String}),
			Tuple([]Type{String}),
			true,
		},
		{
			Tuple([]Type{Tuple([]Type{String})}),
			Tuple([]Type{Tuple([]Type{String})}),
			true,
		},
		{
			Tuple([]Type{String}),
			EmptyTuple,
			false,
		},
		{
			Tuple([]Type{String}),
			Tuple([]Type{Number}),
			false,
		},
		{
			Tuple([]Type{String}),
			Tuple([]Type{String, Number}),
			false,
		},
		{
			Tuple([]Type{String}),
			Tuple([]Type{Tuple([]Type{String})}),
			false,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Equals(%#v)", test.LHS, test.RHS), func(t *testing.T) {
			got := test.LHS.Equals(test.RHS)
			if got != test.Expected {
				t.Errorf("Equals returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}
