package cty

import (
	"fmt"
	"strings"
	"testing"
)

func TestTypeTestConformance(t *testing.T) {
	tests := []struct {
		Receiver Type
		Given    Type
		Conforms bool
	}{
		{
			Receiver: Number,
			Given:    Number,
			Conforms: true,
		},
		{
			Receiver: Number,
			Given:    String,
			Conforms: false,
		},
		{
			Receiver: Number,
			Given:    DynamicPseudoType,
			Conforms: true,
		},
		{
			Receiver: DynamicPseudoType,
			Given:    DynamicPseudoType,
			Conforms: true,
		},
		{
			Receiver: DynamicPseudoType,
			Given:    Number,
			Conforms: false,
		},
		{
			Receiver: List(Number),
			Given:    List(Number),
			Conforms: true,
		},
		{
			Receiver: List(Number),
			Given:    Map(Number),
			Conforms: false,
		},
		{
			Receiver: List(Number),
			Given:    List(DynamicPseudoType),
			Conforms: true,
		},
		{
			Receiver: List(Number),
			Given:    List(String),
			Conforms: false,
		},
		{
			Receiver: Map(Number),
			Given:    Map(Number),
			Conforms: true,
		},
		{
			Receiver: Map(Number),
			Given:    Set(Number),
			Conforms: false,
		},
		{
			Receiver: List(Number),
			Given:    Map(DynamicPseudoType),
			Conforms: false,
		},
		{
			Receiver: Map(Number),
			Given:    Map(DynamicPseudoType),
			Conforms: true,
		},
		{
			Receiver: Map(Number),
			Given:    Map(String),
			Conforms: false,
		},
		{
			Receiver: Set(Number),
			Given:    Set(Number),
			Conforms: true,
		},
		{
			Receiver: Set(Number),
			Given:    List(Number),
			Conforms: false,
		},
		{
			Receiver: Set(Number),
			Given:    List(DynamicPseudoType),
			Conforms: false,
		},
		{
			Receiver: Set(Number),
			Given:    Set(DynamicPseudoType),
			Conforms: true,
		},
		{
			Receiver: Set(Number),
			Given:    Set(String),
			Conforms: false,
		},
		{
			Receiver: EmptyObject,
			Given:    EmptyObject,
			Conforms: true,
		},
		{
			Receiver: EmptyObject,
			Given:    Object(map[string]Type{"name": String}),
			Conforms: false,
		},
		{
			Receiver: Object(map[string]Type{"name": String}),
			Given:    EmptyObject,
			Conforms: false,
		},
		{
			Receiver: Object(map[string]Type{"name": String}),
			Given:    Object(map[string]Type{"name": String}),
			Conforms: true,
		},
		{
			Receiver: Object(map[string]Type{"name": String}),
			Given:    Object(map[string]Type{"gnome": String}),
			Conforms: false,
		},
		{
			Receiver: Object(map[string]Type{"name": Number}),
			Given:    Object(map[string]Type{"name": String}),
			Conforms: false,
		},
		{
			Receiver: Object(map[string]Type{"name": Number}),
			Given:    Object(map[string]Type{"name": String, "number": Number}),
			Conforms: false,
		},
		{
			Receiver: EmptyTuple,
			Given:    EmptyTuple,
			Conforms: true,
		},
		{
			Receiver: EmptyTuple,
			Given:    Tuple([]Type{String}),
			Conforms: false,
		},
		{
			Given:    Tuple([]Type{String}),
			Receiver: EmptyTuple,
			Conforms: false,
		},
		{
			Receiver: Tuple([]Type{String}),
			Given:    Tuple([]Type{String}),
			Conforms: true,
		},
		{
			Receiver: Tuple([]Type{String}),
			Given:    Tuple([]Type{Number}),
			Conforms: false,
		},
		{
			Receiver: Tuple([]Type{String, Number}),
			Given:    Tuple([]Type{String, Number}),
			Conforms: true,
		},
		{
			Receiver: Tuple([]Type{String}),
			Given:    Tuple([]Type{String, Number}),
			Conforms: false,
		},
		{
			Receiver: Tuple([]Type{String, Number}),
			Given:    Tuple([]Type{String}),
			Conforms: false,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("(%#v).TestConformance(%#v)", test.Receiver, test.Given), func(t *testing.T) {
			errs := test.Receiver.TestConformance(test.Given)
			if test.Conforms {
				if errs != nil {
					errStrs := make([]string, 0, len(errs))
					for _, err := range errs {
						if pathErr, ok := err.(PathError); ok {
							errStrs = append(errStrs, fmt.Sprintf("at %#v: %s", pathErr.Path, pathErr))
						} else {
							errStrs = append(errStrs, err.Error())
						}
					}
					t.Errorf("(%#v).TestConformance(%#v): unexpected errors\n%s", test.Receiver, test.Given, strings.Join(errStrs, "\n"))
				}
			} else {
				if errs == nil {
					t.Errorf("(%#v).TestConformance(%#v): expected errors, but got none", test.Receiver, test.Given)
				}
			}
		})
	}
}
