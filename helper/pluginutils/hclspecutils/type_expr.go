// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hclspecutils

import (
	"fmt"
	"reflect"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

var typeType = cty.Capsule("type", reflect.TypeOf(cty.NilType))

var typeEvalCtx = &hcl.EvalContext{
	Variables: map[string]cty.Value{
		"string": wrapTypeType(cty.String),
		"bool":   wrapTypeType(cty.Bool),
		"number": wrapTypeType(cty.Number),
		"any":    wrapTypeType(cty.DynamicPseudoType),
	},
	Functions: map[string]function.Function{
		"list": function.New(&function.Spec{
			Params: []function.Parameter{
				{
					Name: "element_type",
					Type: typeType,
				},
			},
			Type: function.StaticReturnType(typeType),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				ety := unwrapTypeType(args[0])
				ty := cty.List(ety)
				return wrapTypeType(ty), nil
			},
		}),
		"set": function.New(&function.Spec{
			Params: []function.Parameter{
				{
					Name: "element_type",
					Type: typeType,
				},
			},
			Type: function.StaticReturnType(typeType),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				ety := unwrapTypeType(args[0])
				ty := cty.Set(ety)
				return wrapTypeType(ty), nil
			},
		}),
		"map": function.New(&function.Spec{
			Params: []function.Parameter{
				{
					Name: "element_type",
					Type: typeType,
				},
			},
			Type: function.StaticReturnType(typeType),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				ety := unwrapTypeType(args[0])
				ty := cty.Map(ety)
				return wrapTypeType(ty), nil
			},
		}),
		"tuple": function.New(&function.Spec{
			Params: []function.Parameter{
				{
					Name: "element_types",
					Type: cty.List(typeType),
				},
			},
			Type: function.StaticReturnType(typeType),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				etysVal := args[0]
				etys := make([]cty.Type, 0, etysVal.LengthInt())
				for it := etysVal.ElementIterator(); it.Next(); {
					_, wrapEty := it.Element()
					etys = append(etys, unwrapTypeType(wrapEty))
				}
				ty := cty.Tuple(etys)
				return wrapTypeType(ty), nil
			},
		}),
		"object": function.New(&function.Spec{
			Params: []function.Parameter{
				{
					Name: "attribute_types",
					Type: cty.Map(typeType),
				},
			},
			Type: function.StaticReturnType(typeType),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				atysVal := args[0]
				atys := make(map[string]cty.Type)
				for it := atysVal.ElementIterator(); it.Next(); {
					nameVal, wrapAty := it.Element()
					name := nameVal.AsString()
					atys[name] = unwrapTypeType(wrapAty)
				}
				ty := cty.Object(atys)
				return wrapTypeType(ty), nil
			},
		}),
	},
}

func evalTypeExpr(expr hcl.Expression) (cty.Type, hcl.Diagnostics) {
	result, diags := expr.Value(typeEvalCtx)
	if result.IsNull() {
		return cty.DynamicPseudoType, diags
	}
	if !result.Type().Equals(typeType) {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid type expression",
			Detail:   fmt.Sprintf("A type is required, not %s.", result.Type().FriendlyName()),
		})
		return cty.DynamicPseudoType, diags
	}

	return unwrapTypeType(result), diags
}

func wrapTypeType(ty cty.Type) cty.Value {
	return cty.CapsuleVal(typeType, &ty)
}

func unwrapTypeType(val cty.Value) cty.Type {
	return *(val.EncapsulatedValue().(*cty.Type))
}
