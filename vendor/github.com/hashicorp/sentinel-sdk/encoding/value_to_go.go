package encoding

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel-sdk/proto/go"
)

var (
	interfaceTyp = reflect.TypeOf((*interface{})(nil)).Elem()
	boolTyp      = reflect.TypeOf(true)
	intTyp       = reflect.TypeOf(int64(0))
	floatTyp     = reflect.TypeOf(float64(0))
	stringTyp    = reflect.TypeOf("")
)

// ValueToGo converts a protobuf Value structure to a native Go value.
func ValueToGo(v *proto.Value, t reflect.Type) (interface{}, error) {
	return valueToGo(v, t)
}

func valueToGo(v *proto.Value, t reflect.Type) (interface{}, error) {
	// t == nil if you call reflect.TypeOf(interface{}{}) or
	// if the user explicitly send in nil which we make to mean
	// the same thing.
	kind := reflect.Interface
	if t != nil {
		kind = t.Kind()
	}
	if kind == reflect.Interface {
		switch v.Type {
		case proto.Value_BOOL:
			kind = reflect.Bool

		case proto.Value_INT:
			kind = reflect.Int64

		case proto.Value_FLOAT:
			kind = reflect.Float64

		case proto.Value_STRING:
			kind = reflect.String

		case proto.Value_MAP:
			kind = reflect.Map

		case proto.Value_LIST:
			kind = reflect.Slice

		case proto.Value_NULL:
			return sdk.Null, nil

		case proto.Value_UNDEFINED:
			return sdk.Undefined, nil

		default:
			return nil, convertErr(v, "interface{}")
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
			t = valueMapType(v)

		case reflect.Slice:
			t = valueSliceType(v)

		default:
			return nil, convertErr(v, "nil type")
		}
	}

	switch kind {
	case reflect.Bool:
		return convertValueBool(v)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := convertValueInt64(v)
		if err != nil {
			return v, err
		}

		// This is pretty expensive but makes the implementation easy.
		// The performance is likely to be overshadowed by the RPC cost
		// and function cost itself.
		return reflect.ValueOf(v).Convert(t).Interface(), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := convertValueUint64(v)
		if err != nil {
			return v, err
		}

		return reflect.ValueOf(v).Convert(t).Interface(), nil

	case reflect.Float32:
		v, err := convertValueFloat(v, 32)
		if err != nil {
			return v, err
		}

		return float32(v.(float64)), nil

	case reflect.Float64:
		return convertValueFloat(v, 64)

	case reflect.String:
		return convertValueString(v)

	case reflect.Slice:
		return convertValueSlice(v, t)

	case reflect.Map:
		return convertValueMap(v, t)

	case reflect.Ptr:
		switch v.Type {
		case proto.Value_NULL:
			return sdk.Null, nil

		case proto.Value_UNDEFINED:
			return sdk.Undefined, nil
		}

		fallthrough

	default:
		return nil, convertErr(v, t.Kind().String())
	}
}

func convertValueBool(raw *proto.Value) (interface{}, error) {
	if raw.Type == proto.Value_BOOL {
		return raw.Value.(*proto.Value_ValueBool).ValueBool, nil
	}

	return nil, convertErr(raw, "bool")
}

func convertValueInt64(raw *proto.Value) (interface{}, error) {
	switch raw.Type {
	case proto.Value_INT:
		return raw.Value.(*proto.Value_ValueInt).ValueInt, nil

	case proto.Value_STRING:
		return strconv.ParseInt(raw.Value.(*proto.Value_ValueString).ValueString, 0, 64)

	default:
		return nil, convertErr(raw, "int")
	}
}

func convertValueUint64(raw *proto.Value) (interface{}, error) {
	switch raw.Type {
	case proto.Value_INT:
		value := raw.Value.(*proto.Value_ValueInt).ValueInt
		if value < 0 {
			return nil, fmt.Errorf(
				"expected unsigned value, got negative integer")
		}

		return uint64(value), nil

	case proto.Value_STRING:
		return strconv.ParseUint(raw.Value.(*proto.Value_ValueString).ValueString, 0, 64)

	default:
		return nil, convertErr(raw, "uint")
	}
}

func convertValueFloat(raw *proto.Value, bitSize int) (interface{}, error) {
	switch raw.Type {
	case proto.Value_INT:
		return float64(raw.Value.(*proto.Value_ValueInt).ValueInt), nil

	case proto.Value_FLOAT:
		return raw.Value.(*proto.Value_ValueFloat).ValueFloat, nil

	case proto.Value_STRING:
		return strconv.ParseFloat(raw.Value.(*proto.Value_ValueString).ValueString, bitSize)

	default:
		return nil, convertErr(raw, "float")
	}
}

func convertValueString(raw *proto.Value) (interface{}, error) {
	switch raw.Type {
	case proto.Value_INT:
		return strconv.FormatInt(raw.Value.(*proto.Value_ValueInt).ValueInt, 10), nil

	case proto.Value_STRING:
		return raw.Value.(*proto.Value_ValueString).ValueString, nil

	default:
		return nil, convertErr(raw, "string")
	}
}

func convertValueSlice(raw *proto.Value, t reflect.Type) (interface{}, error) {
	if raw.Type != proto.Value_LIST {
		return nil, convertErr(raw, "list")
	}

	list := raw.Value.(*proto.Value_ValueList).ValueList
	elemTyp := t.Elem()
	sliceVal := reflect.MakeSlice(t, len(list.Elems), len(list.Elems))
	for i, elt := range list.Elems {
		v, err := valueToGo(elt, elemTyp)
		if err != nil {
			return nil, fmt.Errorf("element %d: %s", i, err)
		}

		sliceVal.Index(i).Set(reflect.ValueOf(v))
	}

	return sliceVal.Interface(), nil
}

func convertValueMap(raw *proto.Value, t reflect.Type) (interface{}, error) {
	if raw.Type != proto.Value_MAP {
		return nil, convertErr(raw, "map")
	}

	if t.Kind() != reflect.Map {
		return nil, fmt.Errorf("target type is not map, is: %s", t.Kind())
	}

	m := raw.Value.(*proto.Value_ValueMap).ValueMap
	keyTyp := t.Key()
	elemTyp := t.Elem()
	mapVal := reflect.MakeMap(t)
	for _, elt := range m.Elems {
		// Convert the key
		key, err := valueToGo(elt.Key, keyTyp)
		if err != nil {
			return nil, fmt.Errorf("key %s: %s", elt.Key.String(), err)
		}

		// Convert the value
		elem, err := valueToGo(elt.Value, elemTyp)
		if err != nil {
			return nil, fmt.Errorf("element for key %s: %s", elt.Key.String(), err)
		}

		// Set it
		mapVal.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(elem))
	}

	return mapVal.Interface(), nil
}

// valueMapType creates a map type to match the keys/values in the value.
func valueMapType(raw *proto.Value) reflect.Type {
	m := raw.Value.(*proto.Value_ValueMap).ValueMap
	var keys []*proto.Value
	var values []*proto.Value
	for _, elt := range m.Elems {
		keys = append(keys, elt.Key)
		values = append(values, elt.Value)
	}

	return reflect.MapOf(elemType(keys), elemType(values))
}

// valueSliceType creates a slice type to match the keys/values in the value.
func valueSliceType(raw *proto.Value) reflect.Type {
	list := raw.Value.(*proto.Value_ValueList).ValueList
	return reflect.SliceOf(elemType(list.Elems))
}

// elemTyp determines the least common type for a set of values, defaulting
// to interface{} as the most generic type.
func elemType(vs []*proto.Value) reflect.Type {
	current := proto.Value_INVALID
	for _, v := range vs {
		// If we haven't set a type yet, set it to this one
		if current == proto.Value_INVALID {
			current = v.Type
		}

		// If the types don't match, we have an interface type
		if current != v.Type {
			return interfaceTyp
		}
	}

	// We found a matching type, return the type based on the proto type
	switch current {
	case proto.Value_BOOL:
		return boolTyp

	case proto.Value_INT:
		return intTyp

	case proto.Value_FLOAT:
		return floatTyp

	case proto.Value_STRING:
		return stringTyp

	default:
		return interfaceTyp
	}
}

func convertErr(raw *proto.Value, t string) error {
	return fmt.Errorf("cannot convert to %s: %s", t, raw.Type)
}
