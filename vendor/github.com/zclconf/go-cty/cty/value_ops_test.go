package cty

import (
	"fmt"
	"reflect"
	"testing"
)

func TestValueEquals(t *testing.T) {
	capsuleA := CapsuleVal(capsuleTestType1, &capsuleTestType1Native{"capsuleA"})
	capsuleB := CapsuleVal(capsuleTestType1, &capsuleTestType1Native{"capsuleB"})
	capsuleC := CapsuleVal(capsuleTestType2, &capsuleTestType2Native{"capsuleC"})

	tests := []struct {
		LHS      Value
		RHS      Value
		Expected Value
	}{
		// Booleans
		{
			BoolVal(true),
			BoolVal(true),
			BoolVal(true),
		},
		{
			BoolVal(false),
			BoolVal(false),
			BoolVal(true),
		},
		{
			BoolVal(true),
			BoolVal(false),
			BoolVal(false),
		},

		// Numbers
		{
			NumberIntVal(1),
			NumberIntVal(2),
			BoolVal(false),
		},
		{
			NumberIntVal(2),
			NumberIntVal(2),
			BoolVal(true),
		},

		// Strings
		{
			StringVal(""),
			StringVal(""),
			BoolVal(true),
		},
		{
			StringVal("hello"),
			StringVal("hello"),
			BoolVal(true),
		},
		{
			StringVal("hello"),
			StringVal("world"),
			BoolVal(false),
		},
		{
			StringVal("0"),
			StringVal(""),
			BoolVal(false),
		},
		{
			StringVal("años"),
			StringVal("años"),
			BoolVal(true),
		},
		{
			// Combining marks are normalized by StringVal
			StringVal("años"),  // (precomposed tilde-n)
			StringVal("años"), // (combining tilde followed by bare n)
			BoolVal(true),
		},
		{
			// tilde-n does not normalize with bare n
			StringVal("años"),
			StringVal("anos"),
			BoolVal(false),
		},

		// Objects
		{
			ObjectVal(map[string]Value{}),
			ObjectVal(map[string]Value{}),
			BoolVal(true),
		},
		{
			ObjectVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			ObjectVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			BoolVal(true),
		},
		{
			ObjectVal(map[string]Value{
				"h\u00e9llo": NumberIntVal(1), // precombined é
			}),
			ObjectVal(map[string]Value{
				"he\u0301llo": NumberIntVal(1), // e with combining acute accent
			}),
			BoolVal(true),
		},
		{
			ObjectVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			ObjectVal(map[string]Value{}),
			BoolVal(false),
		},
		{
			ObjectVal(map[string]Value{
				"num":  NumberIntVal(1),
				"flag": BoolVal(true),
			}),
			ObjectVal(map[string]Value{
				"num":  NumberIntVal(1),
				"flag": BoolVal(true),
			}),
			BoolVal(true),
		},
		{
			ObjectVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			ObjectVal(map[string]Value{
				"num": NumberIntVal(2),
			}),
			BoolVal(false),
		},
		{
			ObjectVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			ObjectVal(map[string]Value{
				"othernum": NumberIntVal(1),
			}),
			BoolVal(false),
		},
		{
			ObjectVal(map[string]Value{
				"num":  NumberIntVal(1),
				"flag": BoolVal(true),
			}),
			ObjectVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			BoolVal(false),
		},
		{
			ObjectVal(map[string]Value{
				"num":  NumberIntVal(1),
				"flag": BoolVal(true),
			}),
			ObjectVal(map[string]Value{
				"num":  NumberIntVal(1),
				"flag": BoolVal(false),
			}),
			BoolVal(false),
		},

		// Tuples
		{
			EmptyTupleVal,
			EmptyTupleVal,
			BoolVal(true),
		},
		{
			TupleVal([]Value{NumberIntVal(1)}),
			TupleVal([]Value{NumberIntVal(1)}),
			BoolVal(true),
		},
		{
			TupleVal([]Value{NumberIntVal(1)}),
			TupleVal([]Value{NumberIntVal(2)}),
			BoolVal(false),
		},
		{
			TupleVal([]Value{StringVal("hi")}),
			TupleVal([]Value{NumberIntVal(1)}),
			BoolVal(false),
		},
		{
			TupleVal([]Value{NumberIntVal(1)}),
			TupleVal([]Value{NumberIntVal(1), NumberIntVal(2)}),
			BoolVal(false),
		},
		{
			TupleVal([]Value{NumberIntVal(1), NumberIntVal(2)}),
			TupleVal([]Value{NumberIntVal(1)}),
			BoolVal(false),
		},
		{
			TupleVal([]Value{NumberIntVal(1), NumberIntVal(2)}),
			TupleVal([]Value{NumberIntVal(1), NumberIntVal(2)}),
			BoolVal(true),
		},
		{
			TupleVal([]Value{UnknownVal(Number)}),
			TupleVal([]Value{NumberIntVal(1)}),
			UnknownVal(Bool),
		},
		{
			TupleVal([]Value{UnknownVal(Number)}),
			TupleVal([]Value{UnknownVal(Number)}),
			UnknownVal(Bool),
		},
		{
			TupleVal([]Value{NumberIntVal(1)}),
			TupleVal([]Value{UnknownVal(Number)}),
			UnknownVal(Bool),
		},
		{
			TupleVal([]Value{NumberIntVal(1)}),
			TupleVal([]Value{DynamicVal}),
			UnknownVal(Bool),
		},
		{
			TupleVal([]Value{DynamicVal}),
			TupleVal([]Value{NumberIntVal(1)}),
			UnknownVal(Bool),
		},
		{
			TupleVal([]Value{NumberIntVal(1)}),
			UnknownVal(Tuple([]Type{Number})),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Tuple([]Type{Number})),
			TupleVal([]Value{NumberIntVal(1)}),
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			TupleVal([]Value{NumberIntVal(1)}),
			UnknownVal(Bool),
		},
		{
			TupleVal([]Value{NumberIntVal(1)}),
			DynamicVal,
			UnknownVal(Bool),
		},

		// Lists
		{
			ListValEmpty(Number),
			ListValEmpty(Number),
			BoolVal(true),
		},
		{
			ListValEmpty(Number),
			ListValEmpty(Bool),
			BoolVal(false),
		},
		{
			ListVal([]Value{
				NumberIntVal(1),
			}),
			ListVal([]Value{
				NumberIntVal(1),
			}),
			BoolVal(true),
		},
		{
			ListVal([]Value{
				NumberIntVal(1),
			}),
			ListValEmpty(String),
			BoolVal(false),
		},
		{
			ListVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			ListVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			BoolVal(true),
		},
		{
			ListVal([]Value{
				NumberIntVal(1),
			}),
			ListVal([]Value{
				NumberIntVal(2),
			}),
			BoolVal(false),
		},
		{
			ListVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			ListVal([]Value{
				NumberIntVal(1),
			}),
			BoolVal(false),
		},
		{
			ListVal([]Value{
				NumberIntVal(1),
			}),
			ListVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			BoolVal(false),
		},

		// Maps
		{
			MapValEmpty(Number),
			MapValEmpty(Number),
			BoolVal(true),
		},
		{
			MapValEmpty(Number),
			MapValEmpty(Bool),
			BoolVal(false),
		},
		{
			MapVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			MapVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			BoolVal(true),
		},
		{
			MapVal(map[string]Value{
				"h\u00e9llo": NumberIntVal(1), // precombined é
			}),
			MapVal(map[string]Value{
				"he\u0301llo": NumberIntVal(1), // e with combining acute accent
			}),
			BoolVal(true),
		},
		{
			MapVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			MapValEmpty(String),
			BoolVal(false),
		},
		{
			MapVal(map[string]Value{
				"num1": NumberIntVal(1),
				"num2": NumberIntVal(2),
			}),
			MapVal(map[string]Value{
				"num1": NumberIntVal(1),
				"num2": NumberIntVal(2),
			}),
			BoolVal(true),
		},
		{
			MapVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			MapVal(map[string]Value{
				"num": NumberIntVal(2),
			}),
			BoolVal(false),
		},
		{
			MapVal(map[string]Value{
				"num": NumberIntVal(1),
			}),
			MapVal(map[string]Value{
				"othernum": NumberIntVal(1),
			}),
			BoolVal(false),
		},
		{
			MapVal(map[string]Value{
				"num1": NumberIntVal(1),
				"num2": NumberIntVal(2),
			}),
			MapVal(map[string]Value{
				"num1": NumberIntVal(1),
			}),
			BoolVal(false),
		},
		{
			MapVal(map[string]Value{
				"num1": NumberIntVal(1),
			}),
			MapVal(map[string]Value{
				"num1": NumberIntVal(1),
				"num2": NumberIntVal(2),
			}),
			BoolVal(false),
		},
		{
			MapVal(map[string]Value{
				"num1": NumberIntVal(1),
				"num2": NumberIntVal(2),
			}),
			MapVal(map[string]Value{
				"num1": NumberIntVal(1),
				"num2": NumberIntVal(3),
			}),
			BoolVal(false),
		},

		// Sets
		{
			SetValEmpty(Number),
			SetValEmpty(Number),
			BoolVal(true),
		},
		{
			SetValEmpty(Number),
			SetValEmpty(Bool),
			BoolVal(false),
		},
		{
			SetVal([]Value{
				NumberIntVal(1),
			}),
			SetVal([]Value{
				NumberIntVal(1),
			}),
			BoolVal(true),
		},
		{
			SetVal([]Value{
				NumberIntVal(1),
			}),
			SetValEmpty(String),
			BoolVal(false),
		},
		{
			SetVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			SetVal([]Value{
				NumberIntVal(2),
				NumberIntVal(1),
			}),
			BoolVal(true),
		},
		{
			SetVal([]Value{
				NumberIntVal(1),
			}),
			SetVal([]Value{
				NumberIntVal(2),
			}),
			BoolVal(false),
		},
		{
			SetVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			SetVal([]Value{
				NumberIntVal(1),
			}),
			BoolVal(false),
		},
		{
			SetVal([]Value{
				NumberIntVal(1),
			}),
			SetVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			BoolVal(false),
		},

		// Capsules
		{
			capsuleA,
			capsuleA,
			True,
		},
		{
			capsuleA,
			capsuleB,
			False,
		},
		{
			capsuleA,
			capsuleC,
			False,
		},
		{
			capsuleA,
			UnknownVal(capsuleTestType1), // same type
			UnknownVal(Bool),
		},
		{
			capsuleA,
			UnknownVal(capsuleTestType2), // different type
			False,
		},

		// Unknowns and Dynamics
		{
			NumberIntVal(2),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			BoolVal(true),
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			ListVal([]Value{
				StringVal("hi"),
				DynamicVal,
			}),
			ListVal([]Value{
				StringVal("hi"),
				DynamicVal,
			}),
			UnknownVal(Bool),
		},
		{
			ListVal([]Value{
				StringVal("hi"),
				UnknownVal(String),
			}),
			ListVal([]Value{
				StringVal("hi"),
				UnknownVal(String),
			}),
			UnknownVal(Bool),
		},
		{
			MapVal(map[string]Value{
				"static":  StringVal("hi"),
				"dynamic": DynamicVal,
			}),
			MapVal(map[string]Value{
				"static":  StringVal("hi"),
				"dynamic": DynamicVal,
			}),
			UnknownVal(Bool),
		},
		{
			MapVal(map[string]Value{
				"static":  StringVal("hi"),
				"dynamic": UnknownVal(String),
			}),
			MapVal(map[string]Value{
				"static":  StringVal("hi"),
				"dynamic": UnknownVal(String),
			}),
			UnknownVal(Bool),
		},
		{
			NullVal(String),
			NullVal(DynamicPseudoType),
			True,
		},
		{
			NullVal(String),
			NullVal(String),
			True,
		},
		{
			UnknownVal(String),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			StringVal(""),
			NullVal(DynamicPseudoType),
			False,
		},
		{
			StringVal(""),
			NullVal(String),
			False,
		},
		{
			StringVal(""),
			UnknownVal(String),
			UnknownVal(Bool),
		},
		{
			NullVal(DynamicPseudoType),
			NullVal(DynamicPseudoType),
			True,
		},
		{
			NullVal(String),
			UnknownVal(Number),
			UnknownVal(Bool), // because second operand might eventually be null
		},
		{
			UnknownVal(String),
			NullVal(Number),
			UnknownVal(Bool), // because first operand might eventually be null
		},
		{
			UnknownVal(String),
			UnknownVal(Number),
			UnknownVal(Bool), // because both operands might eventually be null
		},
		{
			StringVal("hello"),
			UnknownVal(Number),
			False, // because no number value -- even null -- can be equal to a non-null string
		},
		{
			UnknownVal(String),
			NumberIntVal(1),
			False, // because no string value -- even null -- can be equal to a non-null number
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Equals(%#v)", test.LHS, test.RHS), func(t *testing.T) {
			got := test.LHS.Equals(test.RHS)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Equals returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueAdd(t *testing.T) {
	tests := []struct {
		LHS      Value
		RHS      Value
		Expected Value
	}{
		{
			NumberIntVal(1),
			NumberIntVal(2),
			NumberIntVal(3),
		},
		{
			NumberIntVal(1),
			NumberIntVal(-2),
			NumberIntVal(-1),
		},
		{
			NumberIntVal(1),
			NumberFloatVal(0.5),
			NumberFloatVal(1.5),
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Number),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Number),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Add(%#v)", test.LHS, test.RHS), func(t *testing.T) {
			got := test.LHS.Add(test.RHS)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Add returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueSubtract(t *testing.T) {
	tests := []struct {
		LHS      Value
		RHS      Value
		Expected Value
	}{
		{
			NumberIntVal(1),
			NumberIntVal(2),
			NumberIntVal(-1),
		},
		{
			NumberIntVal(1),
			NumberIntVal(-2),
			NumberIntVal(3),
		},
		{
			NumberIntVal(1),
			NumberFloatVal(0.5),
			NumberFloatVal(0.5),
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Number),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Number),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Subtract(%#v)", test.LHS, test.RHS), func(t *testing.T) {
			got := test.LHS.Subtract(test.RHS)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Subtract returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueNegate(t *testing.T) {
	tests := []struct {
		Receiver Value
		Expected Value
	}{
		{
			NumberIntVal(1),
			NumberIntVal(-1),
		},
		{
			NumberFloatVal(0.5),
			NumberFloatVal(-0.5),
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			DynamicVal,
			UnknownVal(Number),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Negate()", test.Receiver), func(t *testing.T) {
			got := test.Receiver.Negate()
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Negate returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueMultiply(t *testing.T) {
	tests := []struct {
		LHS      Value
		RHS      Value
		Expected Value
	}{
		{
			NumberIntVal(4),
			NumberIntVal(2),
			NumberIntVal(8),
		},
		{
			NumberIntVal(1),
			NumberIntVal(-2),
			NumberIntVal(-2),
		},
		{
			NumberIntVal(5),
			NumberFloatVal(0.5),
			NumberFloatVal(2.5),
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Number),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Number),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Multiply(%#v)", test.LHS, test.RHS), func(t *testing.T) {
			got := test.LHS.Multiply(test.RHS)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Multiply returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueDivide(t *testing.T) {
	tests := []struct {
		LHS      Value
		RHS      Value
		Expected Value
	}{
		{
			NumberIntVal(10),
			NumberIntVal(2),
			NumberIntVal(5),
		},
		{
			NumberIntVal(1),
			NumberIntVal(-2),
			NumberFloatVal(-0.5),
		},
		{
			NumberIntVal(5),
			NumberFloatVal(0.5),
			NumberIntVal(10),
		},
		{
			NumberIntVal(5),
			NumberIntVal(0),
			PositiveInfinity,
		},
		{
			NumberIntVal(-5),
			NumberIntVal(0),
			NegativeInfinity,
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Number),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Number),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Divide(%#v)", test.LHS, test.RHS), func(t *testing.T) {
			got := test.LHS.Divide(test.RHS)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Divide returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueModulo(t *testing.T) {
	tests := []struct {
		LHS      Value
		RHS      Value
		Expected Value
	}{
		{
			NumberIntVal(10),
			NumberIntVal(2),
			NumberIntVal(0),
		},
		{
			NumberIntVal(-10),
			NumberIntVal(2),
			NumberIntVal(0),
		},
		{
			NumberIntVal(11),
			NumberIntVal(2),
			NumberIntVal(1),
		},
		{
			NumberIntVal(-11),
			NumberIntVal(2),
			NumberIntVal(-1),
		},
		{
			NumberIntVal(1),
			NumberIntVal(-2),
			NumberFloatVal(1),
		},
		{
			NumberIntVal(5),
			NumberFloatVal(0.5),
			NumberIntVal(0),
		},
		{
			NumberIntVal(5),
			NumberFloatVal(1.5),
			NumberFloatVal(0.5),
		},
		{
			NumberIntVal(5),
			NumberIntVal(0),
			NumberIntVal(5),
		},
		{
			NumberIntVal(-5),
			NumberIntVal(0),
			NumberIntVal(-5),
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Number),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Number),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Modulo(%#v)", test.LHS, test.RHS), func(t *testing.T) {
			got := test.LHS.Modulo(test.RHS)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Modulo returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueAbsolute(t *testing.T) {
	tests := []struct {
		Receiver Value
		Expected Value
	}{
		{
			NumberIntVal(1),
			NumberIntVal(1),
		},
		{
			NumberIntVal(-1),
			NumberIntVal(1),
		},
		{
			NumberFloatVal(0.5),
			NumberFloatVal(0.5),
		},
		{
			NumberFloatVal(-0.5),
			NumberFloatVal(0.5),
		},
		{
			PositiveInfinity,
			PositiveInfinity,
		},
		{
			NegativeInfinity,
			PositiveInfinity,
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
		},
		{
			DynamicVal,
			UnknownVal(Number),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Absolute()", test.Receiver), func(t *testing.T) {
			got := test.Receiver.Absolute()
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Absolute returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueGetAttr(t *testing.T) {
	tests := []struct {
		Object   Value
		AttrName string
		Expected Value
	}{
		{
			ObjectVal(map[string]Value{
				"greeting": StringVal("hello"),
			}),
			"greeting",
			StringVal("hello"),
		},
		{
			ObjectVal(map[string]Value{
				"greeting": StringVal("hello"),
			}),
			"greeting",
			StringVal("hello"),
		},
		{
			UnknownVal(Object(map[string]Type{
				"gr\u00e9eting": String, // precombined é
			})),
			"gre\u0301eting", // e with combining acute accent
			UnknownVal(String),
		},
		{
			DynamicVal,
			"hello",
			DynamicVal,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.GetAttr(%q)", test.Object, test.AttrName), func(t *testing.T) {
			got := test.Object.GetAttr(test.AttrName)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("GetAttr returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueIndex(t *testing.T) {
	tests := []struct {
		Collection Value
		Key        Value
		Expected   Value
	}{
		{
			ListVal([]Value{StringVal("hello")}),
			NumberIntVal(0),
			StringVal("hello"),
		},
		{
			ListVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberIntVal(1),
			StringVal("world"),
		},
		{
			ListVal([]Value{StringVal("hello")}),
			UnknownVal(Number),
			UnknownVal(String),
		},
		{
			ListVal([]Value{StringVal("hello")}),
			DynamicVal,
			UnknownVal(String),
		},
		{
			UnknownVal(List(String)),
			NumberIntVal(0),
			UnknownVal(String),
		},
		{
			MapVal(map[string]Value{"greeting": StringVal("hello")}),
			StringVal("greeting"),
			StringVal("hello"),
		},
		{
			MapVal(map[string]Value{"gr\u00e9eting": StringVal("hello")}), // precombined é
			StringVal("gre\u0301eting"),                                   // e with combining acute accent
			StringVal("hello"),
		},
		{
			MapVal(map[string]Value{"greeting": True}),
			UnknownVal(String),
			UnknownVal(Bool),
		},
		{
			MapVal(map[string]Value{"greeting": True}),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			UnknownVal(Map(String)),
			StringVal("greeting"),
			UnknownVal(String),
		},
		{
			DynamicVal,
			StringVal("hello"),
			DynamicVal,
		},
		{
			DynamicVal,
			NumberIntVal(0),
			DynamicVal,
		},
		{
			TupleVal([]Value{StringVal("hello")}),
			NumberIntVal(0),
			StringVal("hello"),
		},
		{
			TupleVal([]Value{StringVal("hello"), NumberIntVal(5)}),
			NumberIntVal(0),
			StringVal("hello"),
		},
		{
			TupleVal([]Value{StringVal("hello"), NumberIntVal(5)}),
			NumberIntVal(1),
			NumberIntVal(5),
		},
		{
			TupleVal([]Value{StringVal("hello"), DynamicVal}),
			NumberIntVal(0),
			StringVal("hello"),
		},
		{
			TupleVal([]Value{StringVal("hello"), DynamicVal}),
			NumberIntVal(1),
			DynamicVal,
		},
		{
			TupleVal([]Value{StringVal("hello"), UnknownVal(Number)}),
			NumberIntVal(0),
			StringVal("hello"),
		},
		{
			TupleVal([]Value{StringVal("hello"), UnknownVal(Number)}),
			NumberIntVal(1),
			UnknownVal(Number),
		},
		{
			TupleVal([]Value{StringVal("hello"), UnknownVal(Number)}),
			UnknownVal(Number),
			DynamicVal,
		},
		{
			UnknownVal(Tuple([]Type{String})),
			NumberIntVal(0),
			UnknownVal(String),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Index(%q)", test.Collection, test.Key), func(t *testing.T) {
			got := test.Collection.Index(test.Key)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Index returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueHasIndex(t *testing.T) {
	tests := []struct {
		Collection Value
		Key        Value
		Expected   Value
	}{
		{
			ListVal([]Value{StringVal("hello")}),
			NumberIntVal(0),
			True,
		},
		{
			ListVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberIntVal(1),
			True,
		},
		{
			ListVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberIntVal(2),
			False,
		},
		{
			ListVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberIntVal(-1),
			False,
		},
		{
			ListVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberFloatVal(0.5),
			False,
		},
		{
			ListVal([]Value{StringVal("hello"), StringVal("world")}),
			StringVal("greeting"),
			False,
		},
		{
			ListVal([]Value{StringVal("hello"), StringVal("world")}),
			True,
			False,
		},
		{
			ListVal([]Value{StringVal("hello")}),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			ListVal([]Value{StringVal("hello")}),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			UnknownVal(List(String)),
			NumberIntVal(0),
			UnknownVal(Bool),
		},
		{
			UnknownVal(List(String)),
			StringVal("hello"),
			False,
		},
		{
			MapVal(map[string]Value{"greeting": StringVal("hello")}),
			StringVal("greeting"),
			True,
		},
		{
			MapVal(map[string]Value{"gre\u0301eting": StringVal("hello")}), // e with combining acute accent
			StringVal("gr\u00e9eting"),                                     // precombined é
			True,
		},
		{
			MapVal(map[string]Value{"greeting": StringVal("hello")}),
			StringVal("grouting"),
			False,
		},
		{
			MapVal(map[string]Value{"greeting": StringVal("hello")}),
			StringVal(""),
			False,
		},
		{
			MapVal(map[string]Value{"greeting": StringVal("hello")}),
			Zero,
			False,
		},
		{
			MapVal(map[string]Value{"greeting": StringVal("hello")}),
			True,
			False,
		},
		{
			MapVal(map[string]Value{"greeting": StringVal("hello")}),
			UnknownVal(String),
			UnknownVal(Bool),
		},
		{
			MapVal(map[string]Value{"greeting": StringVal("hello")}),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			UnknownVal(Map(String)),
			StringVal("hello"),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Map(String)),
			NumberIntVal(0),
			False,
		},
		{
			TupleVal([]Value{StringVal("hello")}),
			NumberIntVal(0),
			True,
		},
		{
			TupleVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberIntVal(1),
			True,
		},
		{
			TupleVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberIntVal(2),
			False,
		},
		{
			TupleVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberIntVal(-1),
			False,
		},
		{
			TupleVal([]Value{StringVal("hello"), StringVal("world")}),
			NumberFloatVal(0.5),
			False,
		},
		{
			TupleVal([]Value{StringVal("hello"), StringVal("world")}),
			StringVal("greeting"),
			False,
		},
		{
			TupleVal([]Value{StringVal("hello"), StringVal("world")}),
			True,
			False,
		},
		{
			TupleVal([]Value{StringVal("hello")}),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Tuple([]Type{String})),
			NumberIntVal(0),
			True,
		},
		{
			TupleVal([]Value{StringVal("hello")}),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			StringVal("hello"),
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			NumberIntVal(0),
			UnknownVal(Bool),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.HasIndex(%q)", test.Collection, test.Key), func(t *testing.T) {
			got := test.Collection.HasIndex(test.Key)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("HasIndex returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueForEachElement(t *testing.T) {
	type call struct {
		Key     Value
		Element Value
	}
	tests := []struct {
		Receiver Value
		Expected []call
		Stopped  bool
	}{
		{
			ListValEmpty(String),
			[]call{},
			false,
		},
		{
			ListVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			[]call{
				{NumberIntVal(0), NumberIntVal(1)},
				{NumberIntVal(1), NumberIntVal(2)},
			},
			false,
		},
		{
			ListVal([]Value{
				StringVal("hey"),
				StringVal("stop"),
				StringVal("hey"),
			}),
			[]call{
				{NumberIntVal(0), StringVal("hey")},
				{NumberIntVal(1), StringVal("stop")},
			},
			true,
		},
		{
			SetValEmpty(String),
			[]call{},
			false,
		},
		{
			SetVal([]Value{
				NumberIntVal(1),
				NumberIntVal(2),
			}),
			[]call{
				// Ordering is arbitrary but consistent, so future changes
				// to the set implementation may reorder these.
				{NumberIntVal(2), NumberIntVal(2)},
				{NumberIntVal(1), NumberIntVal(1)},
			},
			false,
		},
		{
			SetVal([]Value{
				StringVal("hi"),
				StringVal("stop"),
				StringVal("hey"),
			}),
			[]call{
				// Ordering is arbitrary but consistent, so future changes
				// to the set implementation may reorder these.
				{StringVal("hi"), StringVal("hi")},
				{StringVal("stop"), StringVal("stop")},
			},
			true,
		},
		{
			MapVal(map[string]Value{
				"second": NumberIntVal(2),
				"first":  NumberIntVal(1),
			}),
			[]call{
				{StringVal("first"), NumberIntVal(1)},
				{StringVal("second"), NumberIntVal(2)},
			},
			false,
		},
		{
			MapVal(map[string]Value{
				"item2": StringVal("value2"),
				"item1": StringVal("stop"),
				"item0": StringVal("value0"),
			}),
			[]call{
				{StringVal("item0"), StringVal("value0")},
				{StringVal("item1"), StringVal("stop")},
			},
			true,
		},
		{
			EmptyTupleVal,
			[]call{},
			false,
		},
		{
			TupleVal([]Value{
				StringVal("hello"),
				NumberIntVal(2),
			}),
			[]call{
				{NumberIntVal(0), StringVal("hello")},
				{NumberIntVal(1), NumberIntVal(2)},
			},
			false,
		},
		{
			TupleVal([]Value{
				NumberIntVal(5),
				StringVal("stop"),
				True,
			}),
			[]call{
				{NumberIntVal(0), NumberIntVal(5)},
				{NumberIntVal(1), StringVal("stop")},
			},
			true,
		},
		{
			EmptyObjectVal,
			[]call{},
			false,
		},
		{
			ObjectVal(map[string]Value{
				"bool":   True,
				"string": StringVal("hello"),
			}),
			[]call{
				{StringVal("bool"), True},
				{StringVal("string"), StringVal("hello")},
			},
			false,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.ForEachElement()", test.Receiver), func(t *testing.T) {
			calls := make([]call, 0)
			stopped := test.Receiver.ForEachElement(
				func(key Value, elem Value) (stop bool) {
					calls = append(calls, call{
						Key:     key,
						Element: elem,
					})
					if elem.v == "stop" {
						stop = true
					}
					return
				},
			)
			if !reflect.DeepEqual(calls, test.Expected) {
				t.Errorf(
					"wrong calls from ForEachElement\ngot:  %#v\nwant: %#v",
					calls, test.Expected,
				)
			}
			if stopped != test.Stopped {
				t.Errorf(
					"ForEachElement returned %#v; want %#v",
					stopped, test.Stopped,
				)
			}
		})
	}
}

func TestValueNot(t *testing.T) {
	tests := []struct {
		Receiver Value
		Expected Value
	}{
		{
			True,
			False,
		},
		{
			False,
			True,
		},
		{
			UnknownVal(Bool),
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			UnknownVal(Bool),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Not()", test.Receiver), func(t *testing.T) {
			got := test.Receiver.Not()
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Not returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueAnd(t *testing.T) {
	tests := []struct {
		Receiver Value
		Other    Value
		Expected Value
	}{
		{
			False,
			False,
			False,
		},
		{
			False,
			True,
			False,
		},
		{
			True,
			False,
			False,
		},
		{
			True,
			True,
			True,
		},
		{
			UnknownVal(Bool),
			UnknownVal(Bool),
			UnknownVal(Bool),
		},
		{
			True,
			UnknownVal(Bool),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Bool),
			True,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			True,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			True,
			UnknownVal(Bool),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.And(%#v)", test.Receiver, test.Other), func(t *testing.T) {
			got := test.Receiver.And(test.Other)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("And returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestValueOr(t *testing.T) {
	tests := []struct {
		Receiver Value
		Other    Value
		Expected Value
	}{
		{
			False,
			False,
			False,
		},
		{
			False,
			True,
			True,
		},
		{
			True,
			False,
			True,
		},
		{
			True,
			True,
			True,
		},
		{
			UnknownVal(Bool),
			UnknownVal(Bool),
			UnknownVal(Bool),
		},
		{
			True,
			UnknownVal(Bool),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Bool),
			True,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			True,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			True,
			UnknownVal(Bool),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.Or(%#v)", test.Receiver, test.Other), func(t *testing.T) {
			got := test.Receiver.Or(test.Other)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("Or returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestLessThan(t *testing.T) {
	tests := []struct {
		Receiver Value
		Other    Value
		Expected Value
	}{
		{
			NumberIntVal(0),
			NumberIntVal(1),
			True,
		},
		{
			NumberIntVal(1),
			NumberIntVal(0),
			False,
		},
		{
			NumberIntVal(0),
			NumberIntVal(0),
			False,
		},
		{
			NumberFloatVal(0.1),
			NumberFloatVal(0.2),
			True,
		},
		{
			NumberFloatVal(0.2),
			NumberFloatVal(0.1),
			False,
		},
		{
			NumberIntVal(0),
			NumberFloatVal(0.2),
			True,
		},
		{
			NumberFloatVal(0.2),
			NumberIntVal(0),
			False,
		},
		{
			NumberFloatVal(0.2),
			NumberFloatVal(0.2),
			False,
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Number),
			NumberIntVal(1),
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			NumberIntVal(1),
			UnknownVal(Bool),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.LessThan(%#v)", test.Receiver, test.Other), func(t *testing.T) {
			got := test.Receiver.LessThan(test.Other)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("LessThan returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestGreaterThan(t *testing.T) {
	tests := []struct {
		Receiver Value
		Other    Value
		Expected Value
	}{
		{
			NumberIntVal(0),
			NumberIntVal(1),
			False,
		},
		{
			NumberIntVal(1),
			NumberIntVal(0),
			True,
		},
		{
			NumberIntVal(0),
			NumberIntVal(0),
			False,
		},
		{
			NumberFloatVal(0.1),
			NumberFloatVal(0.2),
			False,
		},
		{
			NumberFloatVal(0.2),
			NumberFloatVal(0.1),
			True,
		},
		{
			NumberIntVal(0),
			NumberFloatVal(0.2),
			False,
		},
		{
			NumberFloatVal(0.2),
			NumberIntVal(0),
			True,
		},
		{
			NumberFloatVal(0.2),
			NumberFloatVal(0.2),
			False,
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Number),
			NumberIntVal(1),
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			NumberIntVal(1),
			UnknownVal(Bool),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.GreaterThan(%#v)", test.Receiver, test.Other), func(t *testing.T) {
			got := test.Receiver.GreaterThan(test.Other)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("GreaterThan returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestLessThanOrEqualTo(t *testing.T) {
	tests := []struct {
		Receiver Value
		Other    Value
		Expected Value
	}{
		{
			NumberIntVal(0),
			NumberIntVal(1),
			True,
		},
		{
			NumberIntVal(1),
			NumberIntVal(0),
			False,
		},
		{
			NumberIntVal(0),
			NumberIntVal(0),
			True,
		},
		{
			NumberFloatVal(0.1),
			NumberFloatVal(0.2),
			True,
		},
		{
			NumberFloatVal(0.2),
			NumberFloatVal(0.1),
			False,
		},
		{
			NumberIntVal(0),
			NumberFloatVal(0.2),
			True,
		},
		{
			NumberFloatVal(0.2),
			NumberIntVal(0),
			False,
		},
		{
			NumberFloatVal(0.2),
			NumberFloatVal(0.2),
			True,
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Number),
			NumberIntVal(1),
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			NumberIntVal(1),
			UnknownVal(Bool),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.LessThanOrEqualTo(%#v)", test.Receiver, test.Other), func(t *testing.T) {
			got := test.Receiver.LessThanOrEqualTo(test.Other)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("LessThanOrEqualTo returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}

func TestGreaterThanOrEqualTo(t *testing.T) {
	tests := []struct {
		Receiver Value
		Other    Value
		Expected Value
	}{
		{
			NumberIntVal(0),
			NumberIntVal(1),
			False,
		},
		{
			NumberIntVal(1),
			NumberIntVal(0),
			True,
		},
		{
			NumberIntVal(0),
			NumberIntVal(0),
			True,
		},
		{
			NumberFloatVal(0.1),
			NumberFloatVal(0.2),
			False,
		},
		{
			NumberFloatVal(0.2),
			NumberFloatVal(0.1),
			True,
		},
		{
			NumberIntVal(0),
			NumberFloatVal(0.2),
			False,
		},
		{
			NumberFloatVal(0.2),
			NumberIntVal(0),
			True,
		},
		{
			NumberFloatVal(0.2),
			NumberFloatVal(0.2),
			True,
		},
		{
			UnknownVal(Number),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			UnknownVal(Number),
			UnknownVal(Bool),
		},
		{
			UnknownVal(Number),
			NumberIntVal(1),
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			NumberIntVal(1),
			DynamicVal,
			UnknownVal(Bool),
		},
		{
			DynamicVal,
			NumberIntVal(1),
			UnknownVal(Bool),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v.GreaterThanOrEqualTo(%#v)", test.Receiver, test.Other), func(t *testing.T) {
			got := test.Receiver.GreaterThanOrEqualTo(test.Other)
			if !got.RawEquals(test.Expected) {
				t.Fatalf("GreaterThanOrEqualTo returned %#v; want %#v", got, test.Expected)
			}
		})
	}
}
