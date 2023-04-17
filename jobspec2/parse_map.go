// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jobspec2

import (
	"fmt"
	"math"
	"math/big"
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/mitchellh/reflectwalk"
	"github.com/zclconf/go-cty/cty"
)

// decodeMapInterfaceType decodes hcl instances of `map[string]interface{}` fields
// of v.
//
// The HCL parser stores the hcl AST as the map values, and decodeMapInterfaceType
// evaluates the AST and converts them to the native golang types.
func decodeMapInterfaceType(v interface{}, ctx *hcl.EvalContext) hcl.Diagnostics {
	w := &walker{ctx: ctx}
	err := reflectwalk.Walk(v, w)
	if err != nil {
		w.diags = append(w.diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "unexpected internal error",
			Detail:   err.Error(),
		})
	}
	return w.diags
}

type walker struct {
	ctx   *hcl.EvalContext
	diags hcl.Diagnostics
}

var mapStringInterfaceType = reflect.TypeOf(map[string]interface{}{})

func (w *walker) Map(m reflect.Value) error {
	if !m.Type().AssignableTo(mapStringInterfaceType) {
		return nil
	}

	// ignore private map fields
	if !m.CanSet() {
		return nil
	}

	for _, k := range m.MapKeys() {
		v := m.MapIndex(k)
		if attr, ok := v.Interface().(*hcl.Attribute); ok {
			c, diags := decodeInterface(attr.Expr, w.ctx)
			w.diags = append(w.diags, diags...)

			m.SetMapIndex(k, reflect.ValueOf(c))
		}
	}
	return nil
}

func (w *walker) MapElem(m, k, v reflect.Value) error {
	return nil
}
func decodeInterface(expr hcl.Expression, ctx *hcl.EvalContext) (interface{}, hcl.Diagnostics) {
	srvVal, diags := expr.Value(ctx)

	dst, err := interfaceFromCtyValue(srvVal)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "unsuitable value type",
			Detail:   fmt.Sprintf("Unsuitable value: %s", err.Error()),
			Subject:  expr.StartRange().Ptr(),
			Context:  expr.Range().Ptr(),
		})
	}

	return dst, diags
}

func interfaceFromCtyValue(val cty.Value) (interface{}, error) {
	t := val.Type()

	if val.IsNull() {
		return nil, nil
	}

	if !val.IsKnown() {
		return nil, fmt.Errorf("value is not known")
	}

	// The caller should've guaranteed that the given val is conformant with
	// the given type t, so we'll proceed under that assumption here.

	switch {
	case t.IsPrimitiveType():
		switch t {
		case cty.String:
			return val.AsString(), nil
		case cty.Number:
			if val.RawEquals(cty.PositiveInfinity) {
				return math.Inf(1), nil
			} else if val.RawEquals(cty.NegativeInfinity) {
				return math.Inf(-1), nil
			} else {
				return smallestNumber(val.AsBigFloat()), nil
			}
		case cty.Bool:
			return val.True(), nil
		default:
			panic("unsupported primitive type")
		}
	case isCollectionOfMaps(t):
		result := []map[string]interface{}{}

		it := val.ElementIterator()
		for it.Next() {
			_, ev := it.Element()
			evi, err := interfaceFromCtyValue(ev)
			if err != nil {
				return nil, err
			}
			result = append(result, evi.(map[string]interface{}))
		}
		return result, nil
	case t.IsListType(), t.IsSetType(), t.IsTupleType():
		result := []interface{}{}

		it := val.ElementIterator()
		for it.Next() {
			_, ev := it.Element()
			evi, err := interfaceFromCtyValue(ev)
			if err != nil {
				return nil, err
			}
			result = append(result, evi)
		}
		return result, nil
	case t.IsMapType():
		result := map[string]interface{}{}
		it := val.ElementIterator()
		for it.Next() {
			ek, ev := it.Element()

			ekv := ek.AsString()
			evv, err := interfaceFromCtyValue(ev)
			if err != nil {
				return nil, err
			}

			result[ekv] = evv
		}
		return result, nil
	case t.IsObjectType():
		result := map[string]interface{}{}

		for k := range t.AttributeTypes() {
			av := val.GetAttr(k)
			avv, err := interfaceFromCtyValue(av)
			if err != nil {
				return nil, err
			}

			result[k] = avv
		}
		return result, nil
	case t.IsCapsuleType():
		rawVal := val.EncapsulatedValue()
		return rawVal, nil
	default:
		// should never happen
		return nil, fmt.Errorf("cannot serialize %s", t.FriendlyName())
	}
}

func isCollectionOfMaps(t cty.Type) bool {
	switch {
	case t.IsCollectionType():
		et := t.ElementType()
		return et.IsMapType() || et.IsObjectType()
	case t.IsTupleType():
		ets := t.TupleElementTypes()
		for _, et := range ets {
			if !et.IsMapType() && !et.IsObjectType() {
				return false
			}
		}

		return len(ets) > 0
	default:
		return false
	}
}

func smallestNumber(b *big.Float) interface{} {
	if v, acc := b.Int64(); acc == big.Exact {
		// check if it fits in int
		if int64(int(v)) == v {
			return int(v)
		}
		return v
	}

	v, _ := b.Float64()
	return v
}
