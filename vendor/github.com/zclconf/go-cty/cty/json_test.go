package cty

import (
	"encoding/json"
	"testing"
)

func TestTypeJSONable(t *testing.T) {
	tests := []struct {
		Type Type
		Want string
	}{
		{
			String,
			`"string"`,
		},
		{
			Number,
			`"number"`,
		},
		{
			Bool,
			`"bool"`,
		},
		{
			List(Bool),
			`["list","bool"]`,
		},
		{
			Map(Bool),
			`["map","bool"]`,
		},
		{
			Set(Bool),
			`["set","bool"]`,
		},
		{
			List(Map(Bool)),
			`["list",["map","bool"]]`,
		},
		{
			Tuple([]Type{Bool, String}),
			`["tuple",["bool","string"]]`,
		},
		{
			Object(map[string]Type{"bool": Bool, "string": String}),
			`["object",{"bool":"bool","string":"string"}]`,
		},
		{
			DynamicPseudoType,
			`"dynamic"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Type.GoString(), func(t *testing.T) {
			result, err := json.Marshal(test.Type)

			if err != nil {
				t.Fatalf("unexpected error from Marshal: %s", err)
			}

			resultStr := string(result)

			if resultStr != test.Want {
				t.Errorf(
					"wrong result\ntype: %#v\ngot:  %s\nwant: %s",
					test.Type, resultStr, test.Want,
				)
			}

			var ty Type
			err = json.Unmarshal(result, &ty)
			if err != nil {
				t.Fatalf("unexpected error from Unmarshal: %s", err)
			}

			if !ty.Equals(test.Type) {
				t.Errorf(
					"type did not unmarshal correctly\njson: %s\ngot:  %#v\nwant: %#v",
					resultStr, ty, test.Type,
				)
			}
		})
	}
}
