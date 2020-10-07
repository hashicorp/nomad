package gohcl

import (
	"fmt"
	"math"
	"math/big"
	"reflect"

	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty/convert"
	"github.com/zclconf/go-cty/cty/gocty"
)

type SelfDecoder interface {
	DecodeHCL(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics
}

type ExprDecoderFunc func(expr hcl.Expression, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics
type BodyDecoderFunc func(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics

type typeSchema struct {
	schema  *hcl.BodySchema
	partial bool
}
type Decoder struct {
	exprConvertors map[reflect.Type]ExprDecoderFunc
	bodyConvertors map[reflect.Type]BodyDecoderFunc
	typeSchemas    map[reflect.Type]*typeSchema
}

var global = &Decoder{}

func (d *Decoder) RegisterSchema(typ reflect.Type, schema *hcl.BodySchema, partial bool) {
	if d.typeSchemas == nil {
		d.typeSchemas = map[reflect.Type]*typeSchema{}
	}

	d.typeSchemas[typ] = &typeSchema{schema, partial}
}

func (d *Decoder) RegisterExprDecoder(typ reflect.Type, fn ExprDecoderFunc) {
	if d.exprConvertors == nil {
		d.exprConvertors = map[reflect.Type]ExprDecoderFunc{}
	}
	d.exprConvertors[typ] = fn
}

func (d *Decoder) RegisterBlockDecoder(typ reflect.Type, fn BodyDecoderFunc) {
	if d.bodyConvertors == nil {
		d.bodyConvertors = map[reflect.Type]BodyDecoderFunc{}
	}
	d.bodyConvertors[typ] = fn
}

// DecodeBody extracts the configuration within the given body into the given
// value. This value must be a non-nil pointer to either a struct or
// a map, where in the former case the configuration will be decoded using
// struct tags and in the latter case only attributes are allowed and their
// values are decoded into the map.
//
// The given EvalContext is used to resolve any variables or functions in
// expressions encountered while decoding. This may be nil to require only
// constant values, for simple applications that do not support variables or
// functions.
//
// The returned diagnostics should be inspected with its HasErrors method to
// determine if the populated value is valid and complete. If error diagnostics
// are returned then the given value may have been partially-populated but
// may still be accessed by a careful caller for static analysis and editor
// integration use-cases.
func DecodeBody(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	return global.DecodeBody(body, ctx, val)
}

func (d *Decoder) DecodeBody(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("target value must be a pointer, not %s", rv.Type().String()))
	}

	return d.decodeBodyToValue(body, ctx, rv.Elem())
}

func (d *Decoder) decodeBodyToValue(body hcl.Body, ctx *hcl.EvalContext, val reflect.Value) hcl.Diagnostics {
	et := val.Type()
	switch et.Kind() {
	case reflect.Struct:
		return d.decodeBodyToStruct(body, ctx, val)
	case reflect.Map:
		return d.decodeBodyToMap(body, ctx, val)
	default:
		panic(fmt.Sprintf("target value must be pointer to struct or map, not %s", et.String()))
	}
}

func (d *Decoder) decodeBodyToStruct(body hcl.Body, ctx *hcl.EvalContext, val reflect.Value) hcl.Diagnostics {
	if decoder, ok := val.Addr().Interface().(SelfDecoder); ok {
		return decoder.DecodeHCL(body, ctx)
	}
	if fn, ok := d.bodyConvertors[val.Type()]; ok {
		return fn(body, ctx, val.Addr().Interface())
	}

	schema, partial := ImpliedBodySchema(val.Interface())

	var content *hcl.BodyContent
	var leftovers hcl.Body
	var diags hcl.Diagnostics
	if partial {
		content, leftovers, diags = body.PartialContent(schema)
	} else {
		content, diags = body.Content(schema)
	}
	if content == nil {
		return diags
	}

	tags := getFieldTags(val.Type())

	if tags.Remain != nil {
		fieldIdx := *tags.Remain
		field := val.Type().Field(fieldIdx)
		fieldV := val.Field(fieldIdx)
		switch {
		case bodyType.AssignableTo(field.Type):
			fieldV.Set(reflect.ValueOf(leftovers))
		case attrsType.AssignableTo(field.Type):
			attrs, attrsDiags := leftovers.JustAttributes()
			if len(attrsDiags) > 0 {
				diags = append(diags, attrsDiags...)
			}
			fieldV.Set(reflect.ValueOf(attrs))
		default:
			diags = append(diags, d.decodeBodyToValue(leftovers, ctx, fieldV)...)
		}
	}

	for name, fieldIdx := range tags.Attributes {
		attr := content.Attributes[name]
		field := val.Type().Field(fieldIdx)
		fieldV := val.Field(fieldIdx)

		if attr == nil {
			if !exprType.AssignableTo(field.Type) {
				continue
			}

			// As a special case, if the target is of type hcl.Expression then
			// we'll assign an actual expression that evalues to a cty null,
			// so the caller can deal with it within the cty realm rather
			// than within the Go realm.
			synthExpr := hcl.StaticExpr(cty.NullVal(cty.DynamicPseudoType), body.MissingItemRange())
			fieldV.Set(reflect.ValueOf(synthExpr))
			continue
		}

		switch {
		case attrType.AssignableTo(field.Type):
			fieldV.Set(reflect.ValueOf(attr))
		case exprType.AssignableTo(field.Type):
			fieldV.Set(reflect.ValueOf(attr.Expr))
		default:
			diags = append(diags, d.DecodeExpression(
				attr.Expr, ctx, fieldV.Addr().Interface(),
			)...)
		}
	}

	blocksByType := content.Blocks.ByType()

	for typeName, fieldIdx := range tags.Blocks {
		blocks := blocksByType[typeName]
		field := val.Type().Field(fieldIdx)

		ty := field.Type
		isSlice := false
		isPtr := false
		isMap := false
		if ty.Kind() == reflect.Slice {
			isSlice = true
			ty = ty.Elem()
		}
		if ty.Kind() == reflect.Ptr {
			isPtr = true
			ty = ty.Elem()
		}
		if ty.Kind() == reflect.Map {
			isMap = true
		}

		if len(blocks) > 1 && !isSlice && !(isMap && len(blocks[0].Labels) == 1) {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Duplicate %s block", typeName),
				Detail: fmt.Sprintf(
					"Only one %s block is allowed. Another was defined at %s.",
					typeName, blocks[0].DefRange.String(),
				),
				Subject: &blocks[1].DefRange,
			})
			continue
		}

		if len(blocks) == 0 {
			if isSlice || isPtr || isMap {
				if val.Field(fieldIdx).IsNil() {
					val.Field(fieldIdx).Set(reflect.Zero(field.Type))
				}
			} else {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("Missing %s block", typeName),
					Detail:   fmt.Sprintf("A %s block is required.", typeName),
					Subject:  body.MissingItemRange().Ptr(),
				})
			}
			continue
		}

		switch {

		case isSlice:
			elemType := ty
			if isPtr {
				elemType = reflect.PtrTo(ty)
			}
			sli := val.Field(fieldIdx)
			if sli.IsNil() {
				sli = reflect.MakeSlice(reflect.SliceOf(elemType), len(blocks), len(blocks))
			}

			for i, block := range blocks {
				if isPtr {
					if i >= sli.Len() {
						sli = reflect.Append(sli, reflect.New(ty))
					}
					v := sli.Index(i)
					if v.IsNil() {
						v = reflect.New(ty)
					}
					diags = append(diags, d.decodeBlockToValue(block, ctx, v.Elem())...)
					sli.Index(i).Set(v)
				} else {
					diags = append(diags, d.decodeBlockToValue(block, ctx, sli.Index(i))...)
				}
			}

			if sli.Len() > len(blocks) {
				sli.SetLen(len(blocks))
			}

			val.Field(fieldIdx).Set(sli)
		case isMap && len(blocks[0].Labels) == 1:
			v := val.Field(fieldIdx)
			if v.IsNil() {
				v.Set(reflect.MakeMap(ty))
			}

			for _, block := range blocks {
				tyv := ty.Elem()
				isPtr := false
				if tyv.Kind() == reflect.Ptr {
					isPtr = true
					tyv = tyv.Elem()
				}
				ev := reflect.New(tyv)
				diags = append(diags, d.decodeBodyToValue(block.Body, ctx, ev.Elem())...)

				blockTags := getFieldTags(tyv)
				lv := block.Labels[0]
				lfieldIdx := blockTags.Labels[0].FieldIndex
				f := ev.Elem().Field(lfieldIdx)
				if f.Kind() == reflect.Ptr {
					f.Set(reflect.ValueOf(&lv))
				} else {
					f.SetString(lv)
				}

				if !isPtr {
					ev = ev.Elem()
				}
				v.SetMapIndex(reflect.ValueOf(lv), ev)
			}

		default:
			block := blocks[0]
			if isPtr {
				v := val.Field(fieldIdx)
				if v.IsNil() {
					v = reflect.New(ty)
				}
				diags = append(diags, d.decodeBlockToValue(block, ctx, v.Elem())...)
				val.Field(fieldIdx).Set(v)
			} else {
				diags = append(diags, d.decodeBlockToValue(block, ctx, val.Field(fieldIdx))...)
			}

		}

	}

	return diags
}

func (d *Decoder) decodeBodyToMap(body hcl.Body, ctx *hcl.EvalContext, v reflect.Value) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if attrs == nil {
		return diags
	}

	mv := reflect.MakeMap(v.Type())

	for k, attr := range attrs {
		switch {
		case v.Type().Elem().Kind() == reflect.Interface:
			v, vdiags := decodeInterface(attr.Expr, ctx)
			diags = append(diags, vdiags...)
			mv.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
		case attrType.AssignableTo(v.Type().Elem()):
			mv.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(attr))
		case exprType.AssignableTo(v.Type().Elem()):
			mv.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(attr.Expr))
		default:
			ev := reflect.New(v.Type().Elem())
			diags = append(diags, DecodeExpression(attr.Expr, ctx, ev.Interface())...)
			mv.SetMapIndex(reflect.ValueOf(k), ev.Elem())
		}
	}

	v.Set(mv)

	return diags
}

func (d *Decoder) decodeBlockToValue(block *hcl.Block, ctx *hcl.EvalContext, v reflect.Value) hcl.Diagnostics {
	var diags hcl.Diagnostics

	ty := v.Type()

	switch {
	case blockType.AssignableTo(ty):
		v.Elem().Set(reflect.ValueOf(block))
	case bodyType.AssignableTo(ty):
		v.Elem().Set(reflect.ValueOf(block.Body))
	case attrsType.AssignableTo(ty):
		attrs, attrsDiags := block.Body.JustAttributes()
		if len(attrsDiags) > 0 {
			diags = append(diags, attrsDiags...)
		}
		v.Elem().Set(reflect.ValueOf(attrs))
	default:
		diags = append(diags, d.decodeBodyToValue(block.Body, ctx, v)...)

		if len(block.Labels) > 0 {
			blockTags := getFieldTags(ty)
			for li, lv := range block.Labels {
				lfieldIdx := blockTags.Labels[li].FieldIndex
				f := v.Field(lfieldIdx)
				if f.Kind() == reflect.Ptr {
					f.Set(reflect.ValueOf(&lv))
				} else {
					f.SetString(lv)
				}
			}
		}

	}

	return diags
}

// DecodeExpression extracts the value of the given expression into the given
// value. This value must be something that gocty is able to decode into,
// since the final decoding is delegated to that package.
//
// The given EvalContext is used to resolve any variables or functions in
// expressions encountered while decoding. This may be nil to require only
// constant values, for simple applications that do not support variables or
// functions.
//
// The returned diagnostics should be inspected with its HasErrors method to
// determine if the populated value is valid and complete. If error diagnostics
// are returned then the given value may have been partially-populated but
// may still be accessed by a careful caller for static analysis and editor
// integration use-cases.
func DecodeExpression(expr hcl.Expression, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	return global.DecodeExpression(expr, ctx, val)
}

func (d *Decoder) decodeCustomExpression(expr hcl.Expression, ctx *hcl.EvalContext, val interface{}) (hcl.Diagnostics, bool) {
	ty := reflect.TypeOf(val).Elem()
	fn, ok := d.exprConvertors[ty]
	if !ok {
		return nil, false
	}
	diags := fn(expr, ctx, val)
	return diags, true
}

func (d *Decoder) DecodeExpression(expr hcl.Expression, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	if diags, ok := d.decodeCustomExpression(expr, ctx, val); ok {
		return diags
	}

	srcVal, diags := expr.Value(ctx)

	convTy, err := gocty.ImpliedType(val)
	if err != nil {
		panic(fmt.Sprintf("unsuitable DecodeExpression target: %s", err))
	}

	srcVal, err = convert.Convert(srcVal, convTy)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsuitable value type",
			Detail:   fmt.Sprintf("Unsuitable value: %s", err.Error()),
			Subject:  expr.StartRange().Ptr(),
			Context:  expr.Range().Ptr(),
		})
		return diags
	}

	err = gocty.FromCtyValue(srcVal, val)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsuitable value type",
			Detail:   fmt.Sprintf("Unsuitable value: %s", err.Error()),
			Subject:  expr.StartRange().Ptr(),
			Context:  expr.Range().Ptr(),
		})
	}

	return diags
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
	//if val.IsMarked() {
	//	return fmt.Errorf("value has marks, so it cannot be serialized as JSON")
	//}

	//// If we're going to decode as DynamicPseudoType then we need to save
	//// dynamic type information to recover the real type.
	//if t == cty.DynamicPseudoType && val.Type() != cty.DynamicPseudoType {
	//	return marshalDynamic(val, path, b)
	//}

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
		return []map[string]interface{}{result}, nil
		//	case t.IsTupleType():
		//		b.WriteRune('[')
		//		etys := t.TupleElementTypes()
		//		it := val.ElementIterator()
		//		path := append(path, nil) // local override of 'path' with extra element
		//		i := 0
		//		for it.Next() {
		//			if i > 0 {
		//				b.WriteRune(',')
		//			}
		//			ety := etys[i]
		//			ek, ev := it.Element()
		//			path[len(path)-1] = cty.IndexStep{
		//				Key: ek,
		//			}
		//			err := marshal(ev, ety, path, b)
		//			if err != nil {
		//				return err
		//			}
		//			i++
		//		}
		//		b.WriteRune(']')
		//		return nil
	case t.IsObjectType():
		result := map[string]interface{}{}

		for k, _ := range t.AttributeTypes() {
			av := val.GetAttr(k)
			avv, err := interfaceFromCtyValue(av)
			if err != nil {
				return nil, err
			}

			result[k] = avv
		}
		return []map[string]interface{}{result}, nil
	case t.IsCapsuleType():
		rawVal := val.EncapsulatedValue()
		return rawVal, nil
	default:
		// should never happen
		return nil, fmt.Errorf("cannot serialize %s", t.FriendlyName())
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

	if v, acc := b.Float64(); acc == big.Exact || acc == big.Above {
		return v
	}

	return b
}
