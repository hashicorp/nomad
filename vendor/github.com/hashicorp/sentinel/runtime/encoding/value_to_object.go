package encoding

import (
	"errors"
	"fmt"

	"github.com/hashicorp/sentinel/lang/object"
	"github.com/hashicorp/sentinel/proto/go"
)

// ValueToObject converts a protobuf Value structure to a Sentinel object.
func ValueToObject(v *proto.Value) (object.Object, error) {
	switch v.Type {
	case proto.Value_INVALID:
		return nil, errors.New("invalid value")

	case proto.Value_UNDEFINED:
		panic("TODO")

	case proto.Value_NULL:
		return object.Null, nil

	case proto.Value_BOOL:
		return object.Bool(v.Value.(*proto.Value_ValueBool).ValueBool), nil

	case proto.Value_INT:
		return &object.IntObj{
			Value: v.Value.(*proto.Value_ValueInt).ValueInt,
		}, nil

	case proto.Value_STRING:
		return &object.StringObj{
			Value: v.Value.(*proto.Value_ValueString).ValueString,
		}, nil

	case proto.Value_LIST:
		list := v.Value.(*proto.Value_ValueList).ValueList
		elems := make([]object.Object, len(list.Elems))
		for i, elem := range list.Elems {
			obj, err := ValueToObject(elem)
			if err != nil {
				return nil, err
			}

			elems[i] = obj
		}

		return &object.ListObj{Elts: elems}, nil

	case proto.Value_MAP:
		m := v.Value.(*proto.Value_ValueMap).ValueMap
		elems := make([]object.KeyedObj, len(m.Elems))
		for i, elem := range m.Elems {
			keyObj, err := ValueToObject(elem.Key)
			if err != nil {
				return nil, err
			}

			valueObj, err := ValueToObject(elem.Value)
			if err != nil {
				return nil, err
			}

			elems[i] = object.KeyedObj{
				Key:   keyObj,
				Value: valueObj,
			}
		}

		return &object.MapObj{Elts: elems}, nil

	default:
		return nil, fmt.Errorf("unknown value type: %s", v.Type)
	}
}
