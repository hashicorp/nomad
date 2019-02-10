package cty

import (
	"fmt"
	"testing"
)

func TestObjectTypeEquals(t *testing.T) {
	tests := []struct {
		LHS      Type // Must be typeObject
		RHS      Type
		Expected bool
	}{
		{
			Object(map[string]Type{}),
			Object(map[string]Type{}),
			true,
		},
		{
			Object(map[string]Type{
				"name": String,
			}),
			Object(map[string]Type{
				"name": String,
			}),
			true,
		},
		{
			// Attribute names should be normalized
			Object(map[string]Type{
				"h\u00e9llo": String, // precombined é
			}),
			Object(map[string]Type{
				"he\u0301llo": String, // e with combining acute accent
			}),
			true,
		},
		{
			Object(map[string]Type{
				"person": Object(map[string]Type{
					"name": String,
				}),
			}),
			Object(map[string]Type{
				"person": Object(map[string]Type{
					"name": String,
				}),
			}),
			true,
		},
		{
			Object(map[string]Type{
				"name": String,
			}),
			Object(map[string]Type{}),
			false,
		},
		{
			Object(map[string]Type{
				"name": String,
			}),
			Object(map[string]Type{
				"name": Number,
			}),
			false,
		},
		{
			Object(map[string]Type{
				"name": String,
			}),
			Object(map[string]Type{
				"nombre": String,
			}),
			false,
		},
		{
			Object(map[string]Type{
				"name": String,
			}),
			Object(map[string]Type{
				"name": String,
				"age":  Number,
			}),
			false,
		},
		{
			Object(map[string]Type{
				"person": Object(map[string]Type{
					"name": String,
				}),
			}),
			Object(map[string]Type{
				"person": Object(map[string]Type{
					"name": String,
					"age":  Number,
				}),
			}),
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
