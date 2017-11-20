package encoding

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel-sdk/proto/go"
)

// GoToValue converts the Go value to a protobuf Object.
//
// The Go value must contain only primitives, collections of primitives,
// and structures. It must not contain any other type of value or an error
// will be returned.
//
// The primitive types byte and rune are aliases to integer types (as
// defined by the Go spec) and are treated as integers in conversion.
func GoToValue(raw interface{}) (*proto.Value, error) {
	return toValue_reflect(reflect.ValueOf(raw))
}

func toValue_reflect(v reflect.Value) (*proto.Value, error) {
	// Null pointer
	if !v.IsValid() {
		return &proto.Value{Type: proto.Value_NULL}, nil
	}

	// Decode depending on the type. We need to redo all of the primitives
	// above unfortunately since they may fall to this point if they're
	// wrapped in an interface type.
	switch v.Kind() {
	case reflect.Interface:
		return toValue_reflect(v.Elem())

	case reflect.Ptr:
		switch v.Interface() {
		case sdk.Null:
			return &proto.Value{Type: proto.Value_NULL}, nil

		case sdk.Undefined:
			return &proto.Value{Type: proto.Value_UNDEFINED}, nil
		}

		return toValue_reflect(v.Elem())

	case reflect.Bool:
		return &proto.Value{
			Type:  proto.Value_BOOL,
			Value: &proto.Value_ValueBool{ValueBool: v.Bool()},
		}, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &proto.Value{
			Type:  proto.Value_INT,
			Value: &proto.Value_ValueInt{ValueInt: v.Int()},
		}, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &proto.Value{
			Type:  proto.Value_INT,
			Value: &proto.Value_ValueInt{ValueInt: int64(v.Uint())},
		}, nil

	case reflect.Float32, reflect.Float64:
		return &proto.Value{
			Type:  proto.Value_FLOAT,
			Value: &proto.Value_ValueFloat{ValueFloat: v.Float()},
		}, nil

	case reflect.Complex64, reflect.Complex128:
		return nil, errors.New("cannot convert complex number to Sentinel value")

	case reflect.String:
		return &proto.Value{
			Type:  proto.Value_STRING,
			Value: &proto.Value_ValueString{ValueString: v.String()},
		}, nil

	case reflect.Array, reflect.Slice:
		return toValue_array(v)

	case reflect.Map:
		return toValue_map(v)

	case reflect.Struct:
		return toValue_struct(v)

	case reflect.Chan:
		return nil, errors.New("cannot convert channel to Sentinel value")

	case reflect.Func:
		return nil, errors.New("cannot convert func to Sentinel value")
	}

	return nil, fmt.Errorf("cannot convert type %s to Sentinel value", v.Kind())
}

func toValue_array(v reflect.Value) (*proto.Value, error) {
	vs := make([]*proto.Value, v.Len())
	for i := range vs {
		elem, err := toValue_reflect(v.Index(i))
		if err != nil {
			return nil, err
		}

		vs[i] = elem
	}

	return &proto.Value{
		Type: proto.Value_LIST,
		Value: &proto.Value_ValueList{
			ValueList: &proto.Value_List{
				Elems: vs,
			},
		},
	}, nil
}

func toValue_map(v reflect.Value) (*proto.Value, error) {
	vs := make([]*proto.Value_KV, v.Len())
	for i, keyV := range v.MapKeys() {
		key, err := toValue_reflect(keyV)
		if err != nil {
			return nil, err
		}

		value, err := toValue_reflect(v.MapIndex(keyV))
		if err != nil {
			return nil, err
		}

		vs[i] = &proto.Value_KV{
			Key:   key,
			Value: value,
		}
	}

	return &proto.Value{
		Type: proto.Value_MAP,
		Value: &proto.Value_ValueMap{
			ValueMap: &proto.Value_Map{
				Elems: vs,
			},
		},
	}, nil
}

func toValue_struct(v reflect.Value) (*proto.Value, error) {
	// Get the type since we need this to determine what is exported,
	// field tags, etc.
	t := v.Type()

	vs := make([]*proto.Value_KV, v.NumField())
	for i := range vs {
		field := t.Field(i)

		// If PkgPath is non-empty, this is unexported and can be ignored
		if field.PkgPath != "" {
			continue
		}

		// Determine the map key
		key := field.Name
		if v, ok := field.Tag.Lookup("sentinel"); ok {
			// A blank value means to not export this value
			if v == "" {
				continue
			}

			key = v
		}

		// Convert the value
		value, err := toValue_reflect(v.Field(i))
		if err != nil {
			return nil, err
		}

		vs[i] = &proto.Value_KV{
			Value: value,
			Key: &proto.Value{
				Type:  proto.Value_STRING,
				Value: &proto.Value_ValueString{ValueString: key},
			},
		}
	}

	return &proto.Value{
		Type: proto.Value_MAP,
		Value: &proto.Value_ValueMap{
			ValueMap: &proto.Value_Map{
				Elems: vs,
			},
		},
	}, nil
}
