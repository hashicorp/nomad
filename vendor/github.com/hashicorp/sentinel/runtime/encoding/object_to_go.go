package encoding

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/hashicorp/sentinel/lang/object"
)

// ObjectToGo converts a Sentinel object to a Go value with the given type.
// If no error is returned, then interface value returned is guaranteed to
// be the requested type.
func ObjectToGo(raw object.Object, t reflect.Type) (interface{}, error) {
	return objectToGo(raw, t)
}

var (
	interfaceTyp = reflect.TypeOf((*interface{})(nil)).Elem()
	boolTyp      = reflect.TypeOf(true)
	intTyp       = reflect.TypeOf(int64(0))
	floatTyp     = reflect.TypeOf(float64(0))
	stringTyp    = reflect.TypeOf("")
)

func objectToGo(raw object.Object, t reflect.Type) (interface{}, error) {
	// t == nil if you call reflect.TypeOf(interface{}{}) or
	// if the user explicitly send in nil which we make to mean
	// the same thing.
	kind := reflect.Interface
	if t != nil {
		kind = t.Kind()
	}
	if kind == reflect.Interface {
		switch raw.Type() {
		case object.BOOL:
			kind = reflect.Bool

		case object.INT:
			kind = reflect.Int64

		case object.FLOAT:
			kind = reflect.Float64

		case object.STRING:
			kind = reflect.String

		case object.LIST:
			kind = reflect.Slice

		case object.MAP:
			kind = reflect.Map

		default:
			return nil, convertErr(raw, "interface{}")
		}
	}

	// If the type is nil, we set a default based on the kind
	if t == nil || t.Kind() == reflect.Interface {
		switch kind {
		case reflect.Bool:
			t = boolTyp

		case reflect.Int64:
			t = intTyp

		case reflect.Float64:
			t = floatTyp

		case reflect.String:
			t = stringTyp

		case reflect.Map:
			t = objectMapType(raw)

		case reflect.Slice:
			t = objectSliceType(raw)

		default:
			return nil, convertErr(raw, "nil type")
		}
	}

	switch kind {
	case reflect.Bool:
		return convertObjectBool(raw)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := convertObjectInt64(raw)
		if err != nil {
			return v, err
		}

		// This is pretty expensive but makes the implementation easy.
		// The performance is likely to be overshadowed by the RPC cost
		// and function cost itself.
		return reflect.ValueOf(v).Convert(t).Interface(), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := convertObjectUint64(raw)
		if err != nil {
			return v, err
		}

		return reflect.ValueOf(v).Convert(t).Interface(), nil

	case reflect.Float32:
		v, err := convertObjectFloat(raw, 32)
		if err != nil {
			return v, err
		}

		return float32(v.(float64)), nil

	case reflect.Float64:
		return convertObjectFloat(raw, 64)

	case reflect.String:
		return convertObjectString(raw)

	case reflect.Slice:
		return convertObjectSlice(raw, t)

	case reflect.Map:
		return convertObjectMap(raw, t)

	default:
		return nil, convertErr(raw, t.Kind().String())
	}
}

func convertObjectBool(raw object.Object) (interface{}, error) {
	switch x := raw.(type) {
	case *object.BoolObj:
		return x.Value, nil

	default:
		return nil, convertErr(raw, "bool")
	}
}

func convertObjectInt64(raw object.Object) (interface{}, error) {
	switch x := raw.(type) {
	case *object.IntObj:
		return x.Value, nil

	case *object.FloatObj:
		return int64(x.Value), nil

	case *object.StringObj:
		return strconv.ParseInt(x.Value, 0, 64)

	default:
		return nil, convertErr(raw, "int")
	}
}

func convertObjectUint64(raw object.Object) (interface{}, error) {
	switch x := raw.(type) {
	case *object.IntObj:
		if x.Value < 0 {
			return nil, fmt.Errorf(
				"expected unsigned value, got negative integer")
		}

		return uint64(x.Value), nil

	case *object.StringObj:
		return strconv.ParseUint(x.Value, 0, 64)

	default:
		return nil, convertErr(raw, "uint")
	}
}

func convertObjectFloat(raw object.Object, bitSize int) (interface{}, error) {
	switch x := raw.(type) {
	case *object.IntObj:
		return float64(x.Value), nil

	case *object.FloatObj:
		return x.Value, nil

	case *object.StringObj:
		return strconv.ParseFloat(x.Value, bitSize)

	default:
		return nil, convertErr(raw, "float")
	}
}

func convertObjectString(raw object.Object) (interface{}, error) {
	switch x := raw.(type) {
	case *object.IntObj:
		return strconv.FormatInt(x.Value, 10), nil

	case *object.FloatObj:
		return strconv.FormatInt(int64(x.Value), 10), nil

	case *object.StringObj:
		return x.Value, nil

	default:
		return nil, convertErr(raw, "string")
	}
}

func convertObjectSlice(raw object.Object, t reflect.Type) (interface{}, error) {
	list, ok := raw.(*object.ListObj)
	if !ok {
		return nil, convertErr(raw, "list")
	}

	elemTyp := t.Elem()
	sliceVal := reflect.MakeSlice(t, len(list.Elts), len(list.Elts))
	for i, elt := range list.Elts {
		v, err := objectToGo(elt, elemTyp)
		if err != nil {
			return nil, fmt.Errorf("element %d: %s", i, err)
		}

		sliceVal.Index(i).Set(reflect.ValueOf(v))
	}

	return sliceVal.Interface(), nil
}

func convertObjectMap(raw object.Object, t reflect.Type) (interface{}, error) {
	mapObj, ok := raw.(*object.MapObj)
	if !ok {
		return nil, convertErr(raw, "map")
	}

	keyTyp := t.Key()
	elemTyp := t.Elem()
	mapVal := reflect.MakeMap(t)
	for _, elt := range mapObj.Elts {
		// Convert the key
		key, err := objectToGo(elt.Key, keyTyp)
		if err != nil {
			return nil, fmt.Errorf("key %s: %s", elt.Key.String(), err)
		}

		// Convert the value
		elem, err := objectToGo(elt.Value, elemTyp)
		if err != nil {
			return nil, fmt.Errorf("element for key %s: %s", elt.Key.String(), err)
		}

		// Set it
		mapVal.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(elem))
	}

	return mapVal.Interface(), nil
}

// objectMapType creates a map type to match the keys/values in the value.
func objectMapType(raw object.Object) reflect.Type {
	mapObj, ok := raw.(*object.MapObj)
	if !ok {
		return interfaceTyp
	}

	var keys []object.Object
	var values []object.Object
	for _, elt := range mapObj.Elts {
		keys = append(keys, elt.Key)
		values = append(values, elt.Value)
	}

	return reflect.MapOf(elemType(keys), elemType(values))
}

// objectSliceType creates a slice type to match the keys/values in the value.
func objectSliceType(raw object.Object) reflect.Type {
	list, ok := raw.(*object.ListObj)
	if !ok {
		return interfaceTyp
	}

	return reflect.SliceOf(elemType(list.Elts))
}

// elemTyp determines the least common type for a set of values, defaulting
// to interface{} as the most generic type.
func elemType(vs []object.Object) reflect.Type {
	current := object.ILLEGAL
	for _, v := range vs {
		// If we haven't set a type yet, set it to this one
		if current == object.ILLEGAL {
			current = v.Type()
		}

		// If the types don't match, we have an interface type
		if current != v.Type() {
			return interfaceTyp
		}
	}

	// We found a matching type, return the type based on the proto type
	switch current {
	case object.BOOL:
		return boolTyp

	case object.INT:
		return intTyp

	case object.FLOAT:
		return floatTyp

	case object.STRING:
		return stringTyp

	default:
		return interfaceTyp
	}
}

func convertErr(raw object.Object, t string) error {
	return fmt.Errorf("cannot convert to %s: %s", t, raw)
}
