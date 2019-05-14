package cty

import (
	"math/big"
	"testing"
)

func TestSetHashBytes(t *testing.T) {
	tests := []struct {
		value Value
		want  string
	}{
		{
			UnknownVal(Number),
			"?",
		},
		{
			UnknownVal(String),
			"?",
		},
		{
			NullVal(Number),
			"~",
		},
		{
			NullVal(String),
			"~",
		},
		{
			DynamicVal,
			"?",
		},
		{
			NumberVal(big.NewFloat(12)),
			"12",
		},
		{
			StringVal(""),
			`""`,
		},
		{
			StringVal("pizza"),
			`"pizza"`,
		},
		{
			True,
			"T",
		},
		{
			False,
			"F",
		},
		{
			ListValEmpty(Bool),
			"[]",
		},
		{
			ListValEmpty(DynamicPseudoType),
			"[]",
		},
		{
			ListVal([]Value{True, False}),
			"[T;F;]",
		},
		{
			ListVal([]Value{UnknownVal(Bool)}),
			"[?;]",
		},
		{
			ListVal([]Value{ListValEmpty(Bool)}),
			"[[];]",
		},
		{
			MapValEmpty(Bool),
			"{}",
		},
		{
			MapVal(map[string]Value{"true": True, "false": False}),
			`{"false":F;"true":T;}`,
		},
		{
			MapVal(map[string]Value{"true": True, "unknown": UnknownVal(Bool), "dynamic": DynamicVal}),
			`{"dynamic":?;"true":T;"unknown":?;}`,
		},
		{
			SetValEmpty(Bool),
			"[]",
		},
		{
			SetVal([]Value{True, True, False}),
			"[F;T;]",
		},
		{
			SetVal([]Value{UnknownVal(Bool), UnknownVal(Bool)}),
			"[?;?;]", // unknowns are never equal, so we can have multiple of them
		},
		{
			EmptyObjectVal,
			"<>",
		},
		{
			ObjectVal(map[string]Value{
				"name": StringVal("ermintrude"),
				"age":  NumberVal(big.NewFloat(54)),
			}),
			`<54;"ermintrude";>`,
		},
		{
			EmptyTupleVal,
			"<>",
		},
		{
			TupleVal([]Value{
				StringVal("ermintrude"),
				NumberVal(big.NewFloat(54)),
			}),
			`<"ermintrude";54;>`,
		},
	}

	for _, test := range tests {
		t.Run(test.value.GoString(), func(t *testing.T) {
			got := string(makeSetHashBytes(test.value))
			if got != test.want {
				t.Errorf("wrong result\ngot:  %s\nwant: %s", got, test.want)
			}
		})
	}
}
