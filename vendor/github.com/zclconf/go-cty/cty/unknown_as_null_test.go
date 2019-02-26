package cty

import (
	"testing"
)

func TestUnknownAsNull(t *testing.T) {
	tests := []struct {
		Input Value
		Want  Value
	}{
		{
			StringVal("hello"),
			StringVal("hello"),
		},
		{
			NullVal(String),
			NullVal(String),
		},
		{
			UnknownVal(String),
			NullVal(String),
		},

		{
			NullVal(DynamicPseudoType),
			NullVal(DynamicPseudoType),
		},
		{
			NullVal(Object(map[string]Type{"test": String})),
			NullVal(Object(map[string]Type{"test": String})),
		},
		{
			DynamicVal,
			NullVal(DynamicPseudoType),
		},

		{
			ListValEmpty(String),
			ListValEmpty(String),
		},
		{
			ListVal([]Value{
				StringVal("hello"),
			}),
			ListVal([]Value{
				StringVal("hello"),
			}),
		},
		{
			ListVal([]Value{
				NullVal(String),
			}),
			ListVal([]Value{
				NullVal(String),
			}),
		},
		{
			ListVal([]Value{
				UnknownVal(String),
			}),
			ListVal([]Value{
				NullVal(String),
			}),
		},

		{
			SetValEmpty(String),
			SetValEmpty(String),
		},
		{
			SetVal([]Value{
				StringVal("hello"),
			}),
			SetVal([]Value{
				StringVal("hello"),
			}),
		},
		{
			SetVal([]Value{
				NullVal(String),
			}),
			SetVal([]Value{
				NullVal(String),
			}),
		},
		{
			SetVal([]Value{
				UnknownVal(String),
			}),
			SetVal([]Value{
				NullVal(String),
			}),
		},

		{
			EmptyTupleVal,
			EmptyTupleVal,
		},
		{
			TupleVal([]Value{
				StringVal("hello"),
			}),
			TupleVal([]Value{
				StringVal("hello"),
			}),
		},
		{
			TupleVal([]Value{
				NullVal(String),
			}),
			TupleVal([]Value{
				NullVal(String),
			}),
		},
		{
			TupleVal([]Value{
				UnknownVal(String),
			}),
			TupleVal([]Value{
				NullVal(String),
			}),
		},

		{
			MapValEmpty(String),
			MapValEmpty(String),
		},
		{
			MapVal(map[string]Value{
				"greeting": StringVal("hello"),
			}),
			MapVal(map[string]Value{
				"greeting": StringVal("hello"),
			}),
		},
		{
			MapVal(map[string]Value{
				"greeting": NullVal(String),
			}),
			MapVal(map[string]Value{
				"greeting": NullVal(String),
			}),
		},
		{
			MapVal(map[string]Value{
				"greeting": UnknownVal(String),
			}),
			MapVal(map[string]Value{
				"greeting": NullVal(String),
			}),
		},

		{
			EmptyObjectVal,
			EmptyObjectVal,
		},
		{
			ObjectVal(map[string]Value{
				"greeting": StringVal("hello"),
			}),
			ObjectVal(map[string]Value{
				"greeting": StringVal("hello"),
			}),
		},
		{
			ObjectVal(map[string]Value{
				"greeting": NullVal(String),
			}),
			ObjectVal(map[string]Value{
				"greeting": NullVal(String),
			}),
		},
		{
			ObjectVal(map[string]Value{
				"greeting": UnknownVal(String),
			}),
			ObjectVal(map[string]Value{
				"greeting": NullVal(String),
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.Input.GoString(), func(t *testing.T) {
			got := UnknownAsNull(test.Input)
			if !got.RawEquals(test.Want) {
				t.Errorf(
					"wrong result\ninput: %#v\ngot:   %#v\nwant:  %#v",
					test.Input, got, test.Want,
				)
			}
		})
	}
}
